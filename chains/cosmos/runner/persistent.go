package runner

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	inttypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	"github.com/skip-mev/catalyst/chains/cosmos/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

const (
	PersistentBlockTimeout = time.Minute
)

type persistentTx struct {
	txBytes       []byte
	walletAddress string
	msgType       loadtesttypes.MsgType
}

func (r *Runner) runPersistent(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// Only init account numbers for funded wallets initially.
	initialWallets := r.spec.InitialWallets
	if initialWallets <= 0 {
		initialWallets = len(r.wallets)
	}
	if err := r.initAccountNumbersForWallets(ctx, r.wallets[:initialWallets]); err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}

	// We fund InitialWallets * 2^N wallets every block where N == the number of bootstrap loads sent.
	// We therefore require (log(num_wallets) - log(initial_wallets))/log(2) bootstrap loads to fully fund.
	requiredBootstrapLoads := uint64(
		(math.Log10(float64(r.spec.NumWallets))-math.Log10(float64(initialWallets)))/math.Log10(2),
	) + 1
	var blocksProcessed uint64
	bootstrapBackoff := uint64(5)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	blockCh := make(chan inttypes.Block, 1)
	subscriptionErr := make(chan error, 1)

	go func() {
		err := r.clients[0].SubscribeToBlocks(ctx, r.blockGasLimit, func(block inttypes.Block) {
			select {
			case blockCh <- block:
			case <-ctx.Done():
				return
			}
		})
		subscriptionErr <- err
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.NewTicker(PersistentBlockTimeout)
		defer timeout.Stop()
		for {
			select {
			case <-ctx.Done():
				r.logger.Info("ctx cancelled")
				return
			case err := <-subscriptionErr:
				if err != nil {
					r.logger.Error("subscription error", zap.Error(err))
				}
				cancel()
				return
			case block, ok := <-blockCh:
				timeout.Reset(PersistentBlockTimeout)
				blocksProcessed++
				if !ok {
					r.logger.Error("block channel closed")
					cancel()
					return
				}
				r.logger.Info(
					"processing block",
					zap.Int64("height", block.Height),
					zap.Time("timestamp", block.Timestamp),
					zap.Int64("gas_limit", block.GasLimit),
				)

				sentBootstrapLoads := blocksProcessed / bootstrapBackoff
				// Only throttle load creation if we're still bootstrapping.
				if (sentBootstrapLoads <= requiredBootstrapLoads) && (blocksProcessed%bootstrapBackoff != 0) {
					continue
				}
				numTxsSubmitted := r.submitLoadPersistent(ctx)
				r.logger.Info(
					"submitted transactions",
					zap.Int64("height", block.Height),
					zap.Int("num_submitted", numTxsSubmitted),
				)
			case <-timeout.C:
				r.logger.Error("timed out waiting for a new block to be processed")
				cancel()
				return
			}
		}
	}()

	wg.Wait()

	return loadtesttypes.LoadTestResult{}, nil
}

func (r *Runner) submitLoadPersistent(ctx context.Context) int {
	r.logger.Info("building loads", zap.Int("num_msg_specs", len(r.spec.Msgs)))

	var allTxs []persistentTx

	for _, msgSpec := range r.spec.Msgs {
		numTxs := int(float64(r.spec.NumOfTxs) * msgSpec.Weight)
		txCh := make(chan persistentTx, numTxs)

		var wg sync.WaitGroup
		for range numTxs {
			wg.Add(1)
			go func() {
				defer wg.Done()

				sender := r.txFactory.GetNextSender()
				if sender == nil {
					return
				}
				walletAddress := sender.FormattedAddress()
				client := sender.GetClient()

				// Get nonce optimistically
				r.walletNoncesMu.Lock()
				nonce := r.walletNonces[walletAddress]
				r.walletNonces[walletAddress] = nonce + 1
				r.walletNoncesMu.Unlock()

				msgs, err := r.createMessagesForType(msgSpec, sender)
				if err != nil {
					r.logger.Error("failed to create message", zap.Error(err))
					return
				}

				gasBufferFactor := 2.0
				estimation := r.gasEstimations[msgSpec]
				gasWithBuffer := int64(float64(estimation.gasUsed) * gasBufferFactor)
				fees := sdk.NewCoins(sdk.NewCoin(r.chainCfg.GasDenom, sdkmath.NewInt(gasWithBuffer)))
				accountNumber := r.accountNumbers[walletAddress]
				memo := RandomString(16)

				tx, err := sender.CreateSignedTx(
					ctx,
					client,
					uint64(gasWithBuffer), //nolint:gosec // G115: overflow unlikely in practice
					fees,
					nonce,
					accountNumber,
					memo,
					r.chainCfg.UnorderedTxs,
					r.spec.TxTimeout,
					msgs...)
				if err != nil {
					r.logger.Error("failed to create signed tx", zap.Error(err))
					return
				}

				txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
				if err != nil {
					r.logger.Error("failed to encode tx", zap.Error(err))
					return
				}

				txCh <- persistentTx{
					txBytes:       txBytes,
					walletAddress: walletAddress,
					msgType:       msgSpec.Type,
				}
			}()
		}

		go func() {
			wg.Wait()
			close(txCh)
		}()

		for tx := range txCh {
			allTxs = append(allTxs, tx)
		}
	}

	if r.spec.MetricsEnabled {
		r.sendAndRecordPersistent(ctx, allTxs)
	} else {
		r.sendAsyncPersistent(ctx, allTxs)
	}

	r.txFactory.ResetWalletAllocation()

	// Init account numbers for any newly funded wallets after reset
	r.initNewlyFundedWallets(ctx)

	return len(allTxs)
}

func (r *Runner) sendAndRecordPersistent(ctx context.Context, txs []persistentTx) {
	var wg sync.WaitGroup
	for _, tx := range txs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := r.clients[0]
			res, err := client.BroadcastTx(ctx, tx.txBytes)
			if err != nil {
				r.logger.Debug("failed to broadcast tx", zap.Error(err))
				if res != nil && res.Code == 32 && strings.Contains(res.RawLog, "account sequence mismatch") {
					r.handleNonceMismatch(tx.walletAddress, 0, res.RawLog)
				}
				return
			}
		}()
	}
	wg.Wait()
}

func (r *Runner) sendAsyncPersistent(ctx context.Context, txs []persistentTx) {
	for _, tx := range txs {
		go func() {
			client := r.clients[0]
			res, err := client.BroadcastTx(ctx, tx.txBytes)
			if err != nil && res != nil && res.Code == 32 && strings.Contains(res.RawLog, "account sequence mismatch") {
				r.handleNonceMismatch(tx.walletAddress, 0, res.RawLog)
			}
		}()
	}
}

// initAccountNumbersForWallets initializes account numbers and nonces for a subset of wallets.
func (r *Runner) initAccountNumbersForWallets(ctx context.Context, wallets []*wallet.InteractingWallet) error {
	for _, w := range wallets {
		walletAddress := w.FormattedAddress()
		client := w.GetClient()

		acc, err := client.GetAccount(ctx, walletAddress)
		if err != nil {
			return fmt.Errorf("failed to initialize account for wallet %s: %w", walletAddress, err)
		}

		r.accountNumbers[walletAddress] = acc.GetAccountNumber()

		r.walletNoncesMu.Lock()
		r.walletNonces[walletAddress] = acc.GetSequence()
		r.walletNoncesMu.Unlock()
	}
	r.logger.Info("Account numbers and nonces initialized", zap.Int("num_wallets", len(wallets)))
	return nil
}

// initNewlyFundedWallets queries account info for wallets that were just funded by bootstrapping.
func (r *Runner) initNewlyFundedWallets(ctx context.Context) {
	if r.txFactory == nil {
		return
	}
	// Check how many wallets are now funded vs how many we have account info for
	numWithAccountInfo := len(r.accountNumbers)
	totalWallets := len(r.wallets)
	if numWithAccountInfo >= totalWallets {
		return
	}

	// Try to init account numbers for wallets we don't have yet
	newWallets := r.wallets[numWithAccountInfo:]
	for _, w := range newWallets {
		walletAddress := w.FormattedAddress()
		if _, exists := r.accountNumbers[walletAddress]; exists {
			continue
		}
		client := w.GetClient()
		acc, err := client.GetAccount(ctx, walletAddress)
		if err != nil {
			// Wallet may not be funded yet — skip silently
			continue
		}
		r.accountNumbers[walletAddress] = acc.GetAccountNumber()
		r.walletNoncesMu.Lock()
		r.walletNonces[walletAddress] = acc.GetSequence()
		r.walletNoncesMu.Unlock()
	}
}
