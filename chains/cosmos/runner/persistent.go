package runner

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	"github.com/skip-mev/catalyst/chains/cosmos/client"
	"github.com/skip-mev/catalyst/chains/cosmos/metrics"
	inttypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	"github.com/skip-mev/catalyst/chains/cosmos/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

const (
	persistentBlockTimeout = time.Minute
)

// errSkipped is a sentinel returned by buildPersistentTx for expected skip conditions.
var errSkipped = fmt.Errorf("skipped")

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
		return loadtesttypes.LoadTestResult{}, err[0]
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

	var tracker *metrics.TPSTracker
	if r.spec.MetricsEnabled {
		tracker = metrics.NewTPSTracker(r.logger)
		defer tracker.Close()
	}

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
				numTxsSubmitted, numSucceeded := r.submitLoadPersistent(ctx)
				if tracker != nil {
					tracker.RecordSend(numSucceeded, numTxsSubmitted)
				}
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

func (r *Runner) submitLoadPersistent(ctx context.Context) (int, int) {
	r.logger.Info("building loads", zap.Int("num_msg_specs", len(r.spec.Msgs)))

	buildStart := time.Now()
	var allTxs []persistentTx
	for _, msgSpec := range r.spec.Msgs {
		allTxs = append(allTxs, r.buildTxsForMsgSpec(ctx, msgSpec)...)
	}
	r.logger.Info("built load", zap.Int("num_txs", len(allTxs)), zap.Duration("duration", time.Since(buildStart)))

	sendStart := time.Now()
	failures := r.sendTxs(ctx, allTxs)
	r.logSendResults(allTxs, failures, time.Since(sendStart))

	r.txFactory.ResetWalletAllocation()

	totalFailures := 0
	for _, count := range failures {
		totalFailures += count
	}

	return len(allTxs), len(allTxs) - totalFailures
}

// buildTxsForMsgSpec collects senders, initializes any uninitialized ones, then
// concurrently builds transactions for a single message spec.
func (r *Runner) buildTxsForMsgSpec(ctx context.Context, msgSpec loadtesttypes.LoadTestMsg) []persistentTx {
	numTxs := int(float64(r.spec.NumOfTxs) * msgSpec.Weight)
	r.logger.Info("building load for msg spec",
		zap.String("msg_type", msgSpec.Type.String()),
		zap.Float64("weight", msgSpec.Weight),
		zap.Int("num_txs", numTxs),
	)

	// Collect senders for this round.
	senders := make([]*wallet.InteractingWallet, 0, numTxs)
	for range numTxs {
		sender := r.txFactory.GetNextSender()
		if sender == nil {
			break
		}
		senders = append(senders, sender)
	}
	if skipped := numTxs - len(senders); skipped > 0 {
		r.logger.Warn("skipped txs: no sender available", zap.Int("count", skipped))
	}

	// Initialize any senders missing from accountNumbers.
	r.initUninitializedSenders(ctx, senders)

	// Build txs concurrently.
	txCh := make(chan persistentTx, len(senders))
	var wg sync.WaitGroup
	for _, sender := range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := r.buildPersistentTx(ctx, msgSpec, sender)
			if err != nil {
				return
			}
			txCh <- *tx
		}()
	}

	go func() {
		wg.Wait()
		close(txCh)
	}()

	var txs []persistentTx
	for tx := range txCh {
		txs = append(txs, tx)
	}
	return txs
}

// initUninitializedSenders filters to wallets not yet in accountNumbers and
// initializes them, silently skipping failures.
func (r *Runner) initUninitializedSenders(ctx context.Context, senders []*wallet.InteractingWallet) {
	var toInit []*wallet.InteractingWallet
	for _, s := range senders {
		if _, ok := r.accountNumbers[s.FormattedAddress()]; !ok {
			toInit = append(toInit, s)
		}
	}
	if len(toInit) == 0 {
		return
	}
	if err := r.initWallets(ctx, toInit); err != nil {
		r.logger.Warn("some senders not yet on chain", zap.Int("count", len(err)))
	}
}

// buildPersistentTx creates a single signed transaction for persistent load.
func (r *Runner) buildPersistentTx(
	ctx context.Context,
	msgSpec loadtesttypes.LoadTestMsg,
	sender *wallet.InteractingWallet,
) (*persistentTx, error) {
	walletAddress := sender.FormattedAddress()
	client := sender.GetClient()

	r.accountNumbersMu.Lock()
	_, initialized := r.accountNumbers[walletAddress]
	r.accountNumbersMu.Unlock()
	if !initialized {
		return nil, errSkipped
	}

	msgs, err := r.createMsgsForPersistent(ctx, msgSpec, sender, client, walletAddress)
	if err != nil {
		return nil, err
	}
	if msgs == nil {
		return nil, errSkipped // zero balance
	}

	gasWithBuffer := int64(float64(r.gasEstimations[msgSpec].gasUsed) * 2)

	// Read account number and nonce under their respective locks, just before signing.
	r.accountNumbersMu.Lock()
	accountNumber := r.accountNumbers[walletAddress]
	r.accountNumbersMu.Unlock()

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
		accountNumber,
		RandomString(16), // Avoid ErrTxInMempoolCache
		r.chainCfg.UnorderedTxs,
		r.spec.TxTimeout,
		msgs...)
	if err != nil {
		r.logger.Error("failed to create signed tx", zap.Error(err))
		return nil, err
	}

	txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
	if err != nil {
		r.logger.Error("failed to encode tx", zap.Error(err))
		return nil, err
	}

	return &persistentTx{
		txBytes:       txBytes,
		walletAddress: walletAddress,
		msgType:       msgSpec.Type,
		client:        client,
	}, nil
}

// createMsgsForPersistent builds the SDK messages for a single persistent tx.
// Returns nil msgs (no error) when the sender has zero balance for MsgSend.
func (r *Runner) createMsgsForPersistent(
	ctx context.Context,
	msgSpec loadtesttypes.LoadTestMsg,
	sender *wallet.InteractingWallet,
	client *client.Chain,
	walletAddress string,
) ([]sdk.Msg, error) {
	if msgSpec.Type != inttypes.MsgSend {
		return r.createMessagesForType(msgSpec, sender)
	}
	// Query balance and send half so receivers can cover fees as senders
	balance, err := client.GetBalance(ctx, walletAddress, r.chainCfg.GasDenom)
	if err != nil {
		r.logger.Error("failed to query balance", zap.Error(err))
		return nil, err
	}
	sendAmount := balance.Quo(sdkmath.NewInt(2))
	if !sendAmount.IsPositive() {
		return nil, nil
	}
	msg, err := r.txFactory.CreateMsgSendWithAmount(sender, sendAmount)
	if err != nil {
		r.logger.Error("failed to create message", zap.Error(err))
		return nil, err
	}
	return []sdk.Msg{msg}, nil
}

func (r *Runner) logSendResults(txs []persistentTx, failures map[uint32]int, duration time.Duration) {
	totalFailures := 0
	for _, count := range failures {
		totalFailures += count
	}
	r.logger.Info("sent load",
		zap.Int("num_txs", len(txs)),
		zap.Int("succeeded", len(txs)-totalFailures),
		zap.Int("failed", totalFailures),
		zap.Duration("duration", duration),
	)
	for code, count := range failures {
		desc := errorsmod.ABCIError("sdk", code, "").Error()
		r.logger.Warn(
			"broadcast failures",
			zap.Uint32("code", code),
			zap.String("description", desc),
			zap.Int("count", count),
		)
	}
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
					if r.promMetrics != nil {
						r.promMetrics.RecordBroadcastFailure(res.Code)
					}
					mu.Lock()
					failures[res.Code]++
					mu.Unlock()
					if res.Code == 32 && strings.Contains(res.RawLog, "account sequence mismatch") {
						r.handleNonceMismatch(tx.walletAddress, 0, res.RawLog)
					}
				} else {
					if r.promMetrics != nil {
						r.promMetrics.RecordBroadcastFailure(0)
					}
					mu.Lock()
					failures[0]++
					mu.Unlock()
				}
				return
			}
			if r.promMetrics != nil {
				r.promMetrics.BroadcastSuccess.Add(1)
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

// initWallets queries account info for the given wallets concurrently.
// Wallets whose funding tx hasn't landed yet are silently skipped.
func (r *Runner) initWallets(ctx context.Context, walletsToInit []*wallet.InteractingWallet) []error {
	start := time.Now()

	results := fetchAccountInfo(ctx, walletsToInit)

	var errs []error
	initialized := 0
	for _, res := range results {
		if res.err != nil {
			errs = append(errs, res.err)
			continue
		}
		r.accountNumbersMu.Lock()
		r.accountNumbers[res.info.address] = res.info.accountNumber
		r.accountNumbersMu.Unlock()
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
	return errs
}
