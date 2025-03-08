package loadtest

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	"github.com/skip-mev/catalyst/internal/cosmos/client"
	"github.com/skip-mev/catalyst/internal/cosmos/txfactory"
	"github.com/skip-mev/catalyst/internal/cosmos/wallet"
	"github.com/skip-mev/catalyst/internal/metrics"
	logging "github.com/skip-mev/catalyst/internal/shared"
	inttypes "github.com/skip-mev/catalyst/internal/types"
)

// MsgGasEstimation stores gas estimation for a specific message type
type MsgGasEstimation struct {
	gasUsed int64
	weight  float64
	numTxs  int
}

// Runner represents a load test runner that executes a single LoadTestSpec
type Runner struct {
	spec               inttypes.LoadTestSpec
	clients            []*client.Chain
	wallets            []*wallet.InteractingWallet
	gasEstimations     map[inttypes.MsgType]MsgGasEstimation
	gasLimit           int
	totalTxsPerBlock   int
	mu                 sync.Mutex
	numBlocksProcessed int
	collector          metrics.MetricsCollector
	logger             *zap.Logger
	sentTxs            []inttypes.SentTx
	sentTxsMu          sync.RWMutex
	txFactory          *txfactory.TxFactory
	walletNonces       map[string]uint64
	walletNoncesMu     sync.RWMutex
	accountNumbers     map[string]uint64
	accountNumbersMu   sync.RWMutex
}

// NewRunner creates a new load test runner for a given spec
func NewRunner(ctx context.Context, spec inttypes.LoadTestSpec) (*Runner, error) {
	var clients []*client.Chain
	for _, node := range spec.NodesAddresses {
		client, err := client.NewClient(ctx, node.RPC, node.GRPC, spec.ChainID)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for node %s: %v", node.RPC, err)
		}
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no valid clients created")
	}

	if len(spec.Mnemonics) > 0 {
		var privKeys []types.PrivKey
		for _, mnemonic := range spec.Mnemonics {
			derivedPrivKey, err := hd.Secp256k1.Derive()(
				mnemonic,
				"",
				"44'/118'/0'/0/0",
			)
			if err != nil {
				return nil, fmt.Errorf("failed to derive private key from mnemonic: %w", err)
			}
			privKeys = append(privKeys, &secp256k1.PrivKey{Key: derivedPrivKey})
		}
		spec.PrivateKeys = privKeys
	}

	if len(spec.PrivateKeys) == 0 {
		return nil, fmt.Errorf("no private keys available: either provide mnemonics or private keys")
	}

	var wallets []*wallet.InteractingWallet
	for i, privKey := range spec.PrivateKeys {
		client := clients[i%len(clients)]
		wallet := wallet.NewInteractingWallet(privKey, spec.Bech32Prefix, client)
		wallets = append(wallets, wallet)
	}

	logger, _ := logging.DefaultLogger()
	runner := &Runner{
		spec:           spec,
		clients:        clients,
		wallets:        wallets,
		collector:      metrics.NewMetricsCollector(),
		logger:         logger,
		sentTxs:        make([]inttypes.SentTx, 0),
		txFactory:      txfactory.NewTxFactory(spec.GasDenom, wallets),
		walletNonces:   make(map[string]uint64),
		accountNumbers: make(map[string]uint64),
	}

	if err := runner.initGasEstimation(ctx); err != nil {
		return nil, err
	}

	if err := runner.initWalletNonces(ctx); err != nil {
		return nil, err
	}

	return runner, nil
}

// initGasEstimation performs initial gas estimation for transactions
func (r *Runner) initGasEstimation(ctx context.Context) error {
	client := r.clients[0]

	blockGasLimit, err := client.GetGasLimit(ctx)
	if err != nil {
		return fmt.Errorf("failed to get block gas limit: %w", err)
	}

	r.gasLimit = blockGasLimit
	if r.spec.BlockGasLimitTarget <= 0 || r.spec.BlockGasLimitTarget > 1 {
		return fmt.Errorf("block gas limit target must be between 0 and 1, got %f", r.spec.BlockGasLimitTarget)
	}

	var totalWeight float64
	for _, msg := range r.spec.Msgs {
		totalWeight += msg.Weight
	}
	if totalWeight != 1.0 {
		return fmt.Errorf("total message weights must add up to 1.0, got %f", totalWeight)
	}

	fromWallet := r.wallets[0]
	r.gasEstimations = make(map[inttypes.MsgType]MsgGasEstimation)
	r.totalTxsPerBlock = 0

	for _, msgSpec := range r.spec.Msgs {
		msg, err := r.txFactory.CreateMsg(msgSpec.Type, fromWallet)
		if err != nil {
			return fmt.Errorf("failed to create message for gas estimation: %w", err)
		}

		acc, err := client.GetAccount(ctx, fromWallet.FormattedAddress())
		if err != nil {
			return fmt.Errorf("failed to get account: %w", err)
		}

		memo := RandomString(16)
		tx, err := fromWallet.CreateSignedTx(ctx, client, 0, sdk.Coins{}, acc.GetSequence(), acc.GetAccountNumber(), memo, msg)
		if err != nil {
			return fmt.Errorf("failed to create transaction for simulation: %w", err)
		}

		txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
		if err != nil {
			return fmt.Errorf("failed to encode transaction: %w", err)
		}

		gasUsed, err := client.EstimateGasUsed(ctx, txBytes)
		if err != nil {
			return fmt.Errorf("failed to estimate gas: %w", err)
		}

		targetGasLimit := float64(blockGasLimit) * r.spec.BlockGasLimitTarget * msgSpec.Weight
		numTxs := int(math.Ceil(targetGasLimit / float64(gasUsed)))

		r.gasEstimations[msgSpec.Type] = MsgGasEstimation{
			gasUsed: int64(gasUsed),
			weight:  msgSpec.Weight,
			numTxs:  numTxs,
		}
		r.totalTxsPerBlock += numTxs

		r.logger.Info("Gas estimation results",
			zap.String("msgType", msgSpec.Type.String()),
			zap.Int("blockGasLimit", blockGasLimit),
			zap.Uint64("txGasEstimation", gasUsed),
			zap.Float64("targetGasLimit", targetGasLimit),
			zap.Int("numTxs", numTxs))
	}

	if r.totalTxsPerBlock <= 0 {
		return fmt.Errorf("calculated total number of transactions per block is zero or negative: %d", r.totalTxsPerBlock)
	}

	return nil
}

func (r *Runner) initWalletNonces(ctx context.Context) error {
	r.logger.Info("Initializing wallet nonces and account numbers")

	var wg sync.WaitGroup
	var mu sync.Mutex
	var initErr error

	for i := range r.wallets {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			w := r.wallets[index]
			address := w.FormattedAddress()
			client := w.GetClient()

			acc, err := client.GetAccount(ctx, address)
			if err != nil {
				mu.Lock()
				initErr = fmt.Errorf("failed to get account for %s: %w", address, err)
				mu.Unlock()
				return
			}

			r.walletNoncesMu.Lock()
			r.walletNonces[address] = acc.GetSequence()
			r.walletNoncesMu.Unlock()

			r.accountNumbersMu.Lock()
			r.accountNumbers[address] = acc.GetAccountNumber()
			r.accountNumbersMu.Unlock()

			r.logger.Debug("Initialized wallet",
				zap.String("address", address),
				zap.Uint64("nonce", acc.GetSequence()),
				zap.Uint64("account_number", acc.GetAccountNumber()))
		}(i)
	}

	wg.Wait()

	if initErr != nil {
		return initErr
	}

	r.logger.Info("Successfully initialized wallet nonces and account numbers",
		zap.Int("wallets", len(r.wallets)))
	return nil
}

func (r *Runner) Run(ctx context.Context) (inttypes.LoadTestResult, error) {
	startTime := time.Now()
	done := make(chan struct{})

	subCtx, cancelSub := context.WithCancel(ctx)
	defer cancelSub()

	subscriptionErr := make(chan error, 1)
	blockCh := make(chan inttypes.Block, 1)

	go func() {
		err := r.clients[0].SubscribeToBlocks(subCtx, func(block inttypes.Block) {
			select {
			case blockCh <- block:
			case <-subCtx.Done():
				return
			}
		})
		subscriptionErr <- err
	}()

	go func() {
		for {
			select {
			case <-subCtx.Done():
				return
			case block := <-blockCh:
				r.mu.Lock()

				r.numBlocksProcessed++
				r.logger.Debug("processing block", zap.Int64("height", block.Height),
					zap.Time("timestamp", block.Timestamp), zap.Int64("gas_limit", block.GasLimit))

				_, err := r.sendBlockTransactions(ctx)
				if err != nil {
					r.logger.Error("error sending block transactions", zap.Error(err))
				}

				r.logger.Info("processed block", zap.Int64("height", block.Height),
					zap.Int("block_number", r.numBlocksProcessed),
					zap.Int("total_blocks", r.spec.NumOfBlocks))

				if r.numBlocksProcessed >= r.spec.NumOfBlocks {
					r.logger.Info("Load test completed- number of blocks desired reached",
						zap.Int("blocks", r.numBlocksProcessed))
					r.mu.Unlock()
					cancelSub()
					done <- struct{}{}
					return
				}

				r.mu.Unlock()
			}
		}
	}()

	select {
	case <-ctx.Done():
		r.logger.Info("Load test interrupted")
		return inttypes.LoadTestResult{}, ctx.Err()
	case <-done:
		if err := <-subscriptionErr; err != nil && err != context.Canceled {
			return inttypes.LoadTestResult{}, fmt.Errorf("subscription error: %w", err)
		}
		client := r.clients[0]

		// Make sure all txs are processed
		time.Sleep(30 * time.Second)

		r.logger.Info("beginning metric collection process")
		r.collector.GroupSentTxs(ctx, r.sentTxs, client, startTime)
		return r.collector.ProcessResults(r.gasLimit), nil
	case err := <-subscriptionErr:
		// Subscription ended with error before completion
		if err != context.Canceled {
			return inttypes.LoadTestResult{}, fmt.Errorf("failed to subscribe to blocks: %w", err)
		}
		return inttypes.LoadTestResult{}, fmt.Errorf("subscription ended unexpectedly. error: %w", err)
	}
}

// sendBlockTransactions sends transactions for a single block
func (r *Runner) sendBlockTransactions(ctx context.Context) (int, error) {
	txsSent := 0
	var sentTxs []inttypes.SentTx
	var sentTxsMu sync.Mutex

	r.logger.Info("Starting to send transactions for block",
		zap.Int("block_number", r.numBlocksProcessed),
		zap.Int("total_blocks", r.spec.NumOfBlocks),
		zap.Int("expected_txs_per_block", r.totalTxsPerBlock))

	totalTxs := 0
	for _, estimation := range r.gasEstimations {
		totalTxs += estimation.numTxs
	}

	maxWorkers := runtime.NumCPU() * 2
	if totalTxs < maxWorkers {
		maxWorkers = totalTxs
	}

	jobs := make(chan struct {
		msgType inttypes.MsgType
		txIndex int
	}, totalTxs)
	results := make(chan int, totalTxs)

	var wg sync.WaitGroup
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				success := r.processTx(ctx, job.msgType, job.txIndex, &sentTxs, &sentTxsMu)
				if success {
					results <- 1
				} else {
					results <- 0
				}
			}
		}()
	}

	for msgType, estimation := range r.gasEstimations {
		for i := 0; i < estimation.numTxs; i++ {
			jobs <- struct {
				msgType inttypes.MsgType
				txIndex int
			}{msgType, i}
		}
	}
	close(jobs)

	go func() {
		for i := 0; i < totalTxs; i++ {
			txsSent += <-results
		}
		close(results)
	}()

	wg.Wait()

	r.sentTxsMu.Lock()
	r.sentTxs = append(r.sentTxs, sentTxs...)
	r.sentTxsMu.Unlock()

	r.logger.Info("Completed sending transactions for block",
		zap.Int("block_number", r.numBlocksProcessed),
		zap.Int("txs_sent", txsSent),
		zap.Int("expected_txs", r.totalTxsPerBlock))

	return txsSent, nil
}

// processTx processes a single transaction
func (r *Runner) processTx(ctx context.Context, msgType inttypes.MsgType, txIndex int, sentTxs *[]inttypes.SentTx, sentTxsMu *sync.Mutex) bool {
	fromWallet := r.wallets[rand.Intn(len(r.wallets))]
	walletAddress := fromWallet.FormattedAddress()
	client := fromWallet.GetClient()

	msg, err := r.txFactory.CreateMsg(msgType, fromWallet)
	if err != nil {
		r.logger.Error("failed to create message",
			zap.Error(err),
			zap.String("node", client.GetNodeAddress().RPC))
		return false
	}

	maxRetries := 3
	var sentTx inttypes.SentTx
	success := false

	r.accountNumbersMu.RLock()
	accountNumber, hasAccountNumber := r.accountNumbers[walletAddress]
	r.accountNumbersMu.RUnlock()

	if !hasAccountNumber {
		acc, err := client.GetAccount(ctx, walletAddress)
		if err != nil {
			r.logger.Error("failed to get account",
				zap.Error(err),
				zap.String("node", client.GetNodeAddress().RPC))

			return false
		}

		accountNumber = acc.GetAccountNumber()

		r.accountNumbersMu.Lock()
		r.accountNumbers[walletAddress] = accountNumber
		r.accountNumbersMu.Unlock()
	}

	backoff := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries && !success; attempt++ {
		if attempt > 0 {
			r.logger.Debug("retrying transaction due to nonce mismatch",
				zap.Int("attempt", attempt+1),
				zap.String("wallet", walletAddress))
			time.Sleep(backoff)
			backoff *= 2
		}

		r.walletNoncesMu.RLock()
		nonce := r.walletNonces[walletAddress]
		r.walletNoncesMu.RUnlock()

		r.logger.Debug("using nonce for transaction",
			zap.Uint64("nonce", nonce),
			zap.String("wallet", walletAddress),
			zap.String("msgType", msgType.String()),
			zap.Int("attempt", attempt+1))

		estimation := r.gasEstimations[msgType]
		gasWithBuffer := int64(float64(estimation.gasUsed) * 1.4)

		fees := sdk.NewCoins(sdk.NewCoin(r.spec.GasDenom, sdkmath.NewInt(gasWithBuffer)))

		// memo added to avoid ErrTxInMempoolCache
		memo := RandomString(16)
		tx, err := fromWallet.CreateSignedTx(ctx, client, uint64(gasWithBuffer), fees, nonce, accountNumber, memo, msg)
		if err != nil {
			r.logger.Error("failed to create signed tx",
				zap.Error(err),
				zap.String("node", client.GetNodeAddress().RPC))
			continue
		}

		txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
		if err != nil {
			r.logger.Error("failed to encode tx",
				zap.Error(err),
				zap.String("node", client.GetNodeAddress().RPC))
			continue
		}

		res, err := client.BroadcastTx(ctx, txBytes)
		if err != nil {
			if res != nil && res.Code == 32 {
				r.logger.Debug("nonce mismatch detected, will retry",
					zap.String("wallet", walletAddress),
					zap.Uint64("used_nonce", nonce),
					zap.String("raw_log", res.RawLog))

				expectedNonceStr := regexp.MustCompile(`expected (\d+)`).FindStringSubmatch(res.RawLog)
				if len(expectedNonceStr) > 1 {
					if expectedNonce, err := strconv.ParseUint(expectedNonceStr[1], 10, 64); err == nil {
						r.walletNoncesMu.Lock()
						r.walletNonces[walletAddress] = expectedNonce
						r.walletNoncesMu.Unlock()

						r.logger.Debug("updated nonce based on error message",
							zap.String("wallet", walletAddress),
							zap.Uint64("new_nonce", expectedNonce))
					}
				}
			} else {
				sentTx = inttypes.SentTx{
					TxHash:      res.TxHash,
					Err:         err,
					NodeAddress: client.GetNodeAddress().RPC,
					MsgType:     msgType,
				}
				r.logger.Error("failed to broadcast tx",
					zap.Error(err),
					zap.String("node", client.GetNodeAddress().RPC))

				break
			}
		} else {
			sentTx = inttypes.SentTx{
				TxHash:      res.TxHash,
				NodeAddress: client.GetNodeAddress().RPC,
				MsgType:     msgType,
				Err:         nil,
			}

			r.walletNoncesMu.Lock()
			r.walletNonces[walletAddress]++
			r.walletNoncesMu.Unlock()

			success = true

			r.logger.Info("transaction sent successfully",
				zap.String("txHash", res.TxHash),
				zap.String("wallet", walletAddress),
				zap.Uint64("nonce", nonce))
		}
	}

	sentTxsMu.Lock()
	*sentTxs = append(*sentTxs, sentTx)
	sentTxsMu.Unlock()

	return success
}

func (r *Runner) GetCollector() *metrics.MetricsCollector {
	return &r.collector
}

func RandomString(n int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
