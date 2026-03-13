package runner

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	"github.com/skip-mev/catalyst/chains/cosmos/client"
	inttypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	"github.com/skip-mev/catalyst/chains/cosmos/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

const (
	persistentBlockTimeout = time.Minute
)

type persistentTx struct {
	txBytes       []byte
	walletAddress string
	msgType       loadtesttypes.MsgType
	client        *client.Chain
}

func (r *Runner) runPersistent(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// Only init account numbers for funded wallets initially.
	initialWallets := r.spec.InitialWallets
	if initialWallets <= 0 {
		initialWallets = len(r.wallets)
	}
	if err := r.initWallets(ctx, r.wallets[:initialWallets]); err != nil {
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
		timeout := time.NewTicker(persistentBlockTimeout)
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
				timeout.Reset(persistentBlockTimeout)
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

	buildStart := time.Now()
	var allTxs []persistentTx

	for _, msgSpec := range r.spec.Msgs {
		numTxs := int(float64(r.spec.NumOfTxs) * msgSpec.Weight)
		r.logger.Info("building load for msg spec",
			zap.String("msg_type", msgSpec.Type.String()),
			zap.Float64("weight", msgSpec.Weight),
			zap.Int("num_txs", numTxs),
		)
		txCh := make(chan persistentTx, numTxs)

		var (
			skippedNoSender  atomic.Int64
			skippedNoAccount atomic.Int64
			wg               sync.WaitGroup
		)
		for range numTxs {
			wg.Add(1)
			go func() {
				defer wg.Done()

				sender := r.txFactory.GetNextSender()
				if sender == nil {
					skippedNoSender.Add(1)
					return
				}
				walletAddress := sender.FormattedAddress()
				client := sender.GetClient()

				// Skip senders that haven't been initialized yet (no cached account number)
				if _, ok := r.accountNumbers[walletAddress]; !ok {
					skippedNoAccount.Add(1)
					return
				}

				var msgs []sdk.Msg
				var err error
				if msgSpec.Type == inttypes.MsgSend {
					// Query balance and send half so receivers can cover fees as senders
					balance, balErr := client.GetBalance(ctx, walletAddress, r.chainCfg.GasDenom)
					if balErr != nil {
						r.logger.Error("failed to query balance", zap.Error(balErr))
						return
					}
					sendAmount := balance.Quo(sdkmath.NewInt(2))
					if !sendAmount.IsPositive() {
						return
					}
					msg, msgErr := r.txFactory.CreateMsgSendWithAmount(sender, sendAmount)
					if msgErr != nil {
						r.logger.Error("failed to create message", zap.Error(msgErr))
						return
					}
					msgs = []sdk.Msg{msg}
				} else {
					msgs, err = r.createMessagesForType(msgSpec, sender)
					if err != nil {
						r.logger.Error("failed to create message", zap.Error(err))
						return
					}
				}

				gasBufferFactor := 1.3
				gasWithBuffer := int64(float64(r.gasEstimations[msgSpec].gasUsed) * gasBufferFactor)

				// Get nonce optimistically just before signing — after all fallible
				// message-creation steps so a failure there does not burn a nonce.
				r.walletNoncesMu.Lock()
				nonce := r.walletNonces[walletAddress]
				r.walletNonces[walletAddress] = nonce + 1
				r.walletNoncesMu.Unlock()

				tx, err := sender.CreateSignedTx(
					ctx,
					client,
					uint64(gasWithBuffer), //nolint:gosec // G115: overflow unlikely in practice
					r.computeFees(gasWithBuffer),
					nonce,
					r.accountNumbers[walletAddress],
					RandomString(16), // Avoid ErrTxInMempoolCache
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
					client:        client,
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

		if s := skippedNoSender.Load(); s > 0 {
			r.logger.Info("skipped txs: no sender available", zap.Int64("count", s))
		}
		if s := skippedNoAccount.Load(); s > 0 {
			r.logger.Info("skipped txs: sender not initialized", zap.Int64("count", s))
		}
	}
	buildDuration := time.Since(buildStart)
	r.logger.Info("built load", zap.Int("num_txs", len(allTxs)), zap.Duration("duration", buildDuration))

	sendStart := time.Now()
	failures := r.sendTxs(ctx, allTxs)
	sendDuration := time.Since(sendStart)

	totalFailures := 0
	for _, count := range failures {
		totalFailures += count
	}
	r.logger.Info("sent load",
		zap.Int("num_txs", len(allTxs)),
		zap.Int("succeeded", len(allTxs)-totalFailures),
		zap.Int("failed", totalFailures),
		zap.Duration("duration", sendDuration),
	)
	for code, count := range failures {
		r.logger.Warn("broadcast failures", zap.Uint32("code", code), zap.Int("count", count))
	}

	oldFunded, newFunded := r.txFactory.ResetWalletAllocation()

	if newFunded > oldFunded {
		// Init account numbers for newly funded wallets.
		// Wallets whose funding tx failed won't exist on chain yet — skip them silently.
		// They'll be funded in a future round once nonces correct.
		_ = r.initWallets(ctx, r.wallets[oldFunded:newFunded])
	}

	return len(allTxs)
}

func (r *Runner) sendTxs(ctx context.Context, txs []persistentTx) map[uint32]int {
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		failures = make(map[uint32]int)
	)
	for _, tx := range txs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := tx.client.BroadcastTx(ctx, tx.txBytes)
			if err != nil {
				if res != nil {
					mu.Lock()
					failures[res.Code]++
					mu.Unlock()
					if res.Code == 32 && strings.Contains(res.RawLog, "account sequence mismatch") {
						r.handleNonceMismatch(tx.walletAddress, 0, res.RawLog)
					}
				} else {
					mu.Lock()
					failures[0]++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	return failures
}

type accountInfo struct {
	address       string
	accountNumber uint64
	sequence      uint64
}

type accountFetchResult struct {
	info accountInfo
	err  error
}

// fetchAccountInfo queries account info for all wallets and returns one result
// per wallet. Each result contains either valid account info or an error.
func fetchAccountInfo(ctx context.Context, wallets []*wallet.InteractingWallet) []accountFetchResult {
	results := make([]accountFetchResult, len(wallets))
	var wg sync.WaitGroup
	for i, w := range wallets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			walletAddress := w.FormattedAddress()
			acc, err := w.GetClient().GetAccount(ctx, walletAddress)
			if err != nil {
				results[i] = accountFetchResult{
					info: accountInfo{address: walletAddress},
					err:  fmt.Errorf("failed to initialize account for wallet %s: %w", walletAddress, err),
				}
				return
			}
			results[i] = accountFetchResult{
				info: accountInfo{
					address:       walletAddress,
					accountNumber: acc.GetAccountNumber(),
					sequence:      acc.GetSequence(),
				},
			}
		}()
	}
	wg.Wait()
	return results
}

// initNewlyFundedWallets queries account info for the given wallets concurrently.
// Wallets whose funding tx hasn't landed yet are silently skipped.
func (r *Runner) initWallets(ctx context.Context, walletsToInit []*wallet.InteractingWallet) error {
	start := time.Now()

	results := fetchAccountInfo(ctx, walletsToInit)

	var errs []error
	initialized := 0
	for _, res := range results {
		if res.err != nil {
			errs = append(errs, res.err)
			continue
		}
		r.accountNumbers[res.info.address] = res.info.accountNumber
		r.walletNoncesMu.Lock()
		r.walletNonces[res.info.address] = res.info.sequence
		r.walletNoncesMu.Unlock()
		initialized++
	}

	r.logger.Info("initialized wallets",
		zap.Int("initialized", initialized),
		zap.Int("checked", len(walletsToInit)),
		zap.Duration("duration", time.Since(start)),
	)
	return errors.Join(errs...)
}
