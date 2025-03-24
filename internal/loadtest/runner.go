package loadtest

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
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
	blockGasLimit      int64
	gasEstimations     map[inttypes.MsgType]MsgGasEstimation
	totalTxsPerBlock   int
	mu                 sync.Mutex
	numBlocksProcessed int
	collector          metrics.MetricsCollector
	logger             *zap.Logger
	sentTxs            []inttypes.SentTx
	sentTxsMu          sync.RWMutex
	txFactory          *txfactory.TxFactory
	accountNumbers     map[string]uint64
	walletNonces       map[string]uint64
}

// NewRunner creates a new load test runner for a given spec
func NewRunner(ctx context.Context, spec inttypes.LoadTestSpec) (*Runner, error) {
	logger, _ := zap.NewDevelopment()
	var clients []*client.Chain
	for _, node := range spec.NodesAddresses {
		client, err := client.NewClient(ctx, node.RPC, node.GRPC, spec.ChainID)
		if err != nil {
			logger.Warn("failed to create client for node", zap.String("rpc", node.RPC), zap.Error(err))
			continue
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

	runner := &Runner{
		spec:           spec,
		clients:        clients,
		wallets:        wallets,
		collector:      metrics.NewMetricsCollector(),
		logger:         logging.FromContext(ctx),
		sentTxs:        make([]inttypes.SentTx, 0),
		accountNumbers: make(map[string]uint64),
		walletNonces:   make(map[string]uint64),
	}

	runner.txFactory = txfactory.NewTxFactory(spec.GasDenom, wallets)

	if err := runner.initGasEstimation(ctx); err != nil {
		return nil, err
	}

	return runner, nil
}

// initGasEstimation performs initial gas estimation to determine how many transactions
// to send to chain
func (r *Runner) initGasEstimation(ctx context.Context) error {
	client := r.clients[0]

	blockGasLimit, err := client.GetGasLimit(ctx)
	if err != nil {
		return fmt.Errorf("failed to get block gas limit: %w", err)
	}
	r.blockGasLimit = blockGasLimit

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
		numTxs := int(math.Ceil(targetGasLimit/(float64(gasUsed))) * 1.4)

		r.gasEstimations[msgSpec.Type] = MsgGasEstimation{
			gasUsed: int64(gasUsed),
			weight:  msgSpec.Weight,
			numTxs:  numTxs,
		}
		r.totalTxsPerBlock += numTxs

		r.logger.Info("gas estimation results",
			zap.String("msgType", msgSpec.Type.String()),
			zap.Int64("blockGasLimit", blockGasLimit),
			zap.Uint64("txGasEstimation", gasUsed),
			zap.Float64("targetGasLimit", targetGasLimit),
			zap.Int("numTxs", numTxs))
	}

	if r.totalTxsPerBlock <= 0 {
		return fmt.Errorf("calculated total number of transactions per block is zero or negative: %d", r.totalTxsPerBlock)
	}

	return nil
}

// Run executes the load test
func (r *Runner) Run(ctx context.Context) (inttypes.LoadTestResult, error) {
	if err := r.initAccountNumbers(ctx); err != nil {
		return inttypes.LoadTestResult{}, err
	}

	startTime := time.Now()
	done := make(chan struct{})

	subCtx, cancelSub := context.WithCancel(ctx)
	defer cancelSub()

	subscriptionErr := make(chan error, 1)
	blockCh := make(chan inttypes.Block, 1)

	go func() {
		err := r.clients[0].SubscribeToBlocks(subCtx, r.blockGasLimit, func(block inttypes.Block) {
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

				r.logger.Info("processed block", zap.Int64("height", block.Height))

				if r.numBlocksProcessed >= r.spec.NumOfBlocks {
					r.logger.Info("load test completed- number of blocks desired reached",
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
		r.logger.Info("load test interrupted")
		return inttypes.LoadTestResult{}, ctx.Err()
	case <-done:
		if err := <-subscriptionErr; err != nil && err != context.Canceled {
			return inttypes.LoadTestResult{}, fmt.Errorf("subscription error: %w", err)
		}

		// Make sure all txs are processed
		time.Sleep(10 * time.Second)

		collectorStartTime := time.Now()
		collectorClient := r.clients[0]

		r.collector.GroupSentTxs(ctx, r.sentTxs, collectorClient, startTime)
		collectorResults := r.collector.ProcessResults(r.blockGasLimit, r.spec.NumOfBlocks)

		collectorEndTime := time.Now()
		r.logger.Debug("collector running time",
			zap.Float64("duration_seconds", collectorEndTime.Sub(collectorStartTime).Seconds()))

		return collectorResults, nil
	case err := <-subscriptionErr:
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

	r.logger.Info("starting to send transactions for block",
		zap.Int("block_number", r.numBlocksProcessed))

	var latestNoncesMu sync.Mutex

	getLatestNonce := func(walletAddr string, client *client.Chain) uint64 {
		latestNoncesMu.Lock()
		defer latestNoncesMu.Unlock()

		return r.walletNonces[walletAddr]
	}

	updateNonce := func(walletAddr string) {
		latestNoncesMu.Lock()
		defer latestNoncesMu.Unlock()

		r.walletNonces[walletAddr]++
	}

	var wg sync.WaitGroup
	var txsSentMu sync.Mutex

	for msgType, estimation := range r.gasEstimations {
		for i := 0; i < estimation.numTxs; i++ {
			wg.Add(1)

			go func(msgType inttypes.MsgType, txIndex int) {
				defer wg.Done()

				fromWallet := r.wallets[rand.Intn(len(r.wallets))]
				walletAddress := fromWallet.FormattedAddress()
				client := fromWallet.GetClient()

				msg, err := r.txFactory.CreateMsg(msgType, fromWallet)
				if err != nil {
					r.logger.Error("failed to create message",
						zap.Error(err),
						zap.String("node", client.GetNodeAddress().RPC))
					return
				}

				maxRetries := 3
				var sentTx inttypes.SentTx
				success := false

				for attempt := 0; attempt < maxRetries && !success; attempt++ {
					if attempt > 0 {
						r.logger.Debug("retrying transaction due to nonce mismatch",
							zap.Int("attempt", attempt+1),
							zap.String("wallet", walletAddress))
						// Add a small delay before retrying
						time.Sleep(100 * time.Millisecond)
					}

					nonce := getLatestNonce(walletAddress, client)
					if err != nil {
						r.logger.Error("failed to get latest nonce",
							zap.Error(err),
							zap.String("node", client.GetNodeAddress().RPC))
						continue
					}

					r.logger.Debug("using nonce for transaction",
						zap.Uint64("nonce", nonce),
						zap.String("wallet", walletAddress),
						zap.String("msgType", msgType.String()),
						zap.Int("attempt", attempt+1))

					gasWithBuffer := int64(float64(estimation.gasUsed) * 2.5)
					fees := sdk.NewCoins(sdk.NewCoin(r.spec.GasDenom, sdkmath.NewInt(gasWithBuffer)))

					accountNumber := r.accountNumbers[walletAddress]

					// memo added to avoid ErrTxInMempoolCache https://github.com/cosmos/cosmos-sdk/blob/main/types/errors/errors.go#L67
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
						if res != nil && res.Code == 32 && strings.Contains(res.RawLog, "account sequence mismatch") {
							r.logger.Debug("nonce mismatch detected, will retry",
								zap.String("wallet", walletAddress),
								zap.Uint64("used_nonce", nonce),
								zap.String("raw_log", res.RawLog))

							expectedNonceStr := regexp.MustCompile(`expected (\d+)`).FindStringSubmatch(res.RawLog)
							if len(expectedNonceStr) > 1 {
								if expectedNonce, err := strconv.ParseUint(expectedNonceStr[1], 10, 64); err == nil {
									latestNoncesMu.Lock()
									r.walletNonces[walletAddress] = expectedNonce
									latestNoncesMu.Unlock()

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

						updateNonce(walletAddress)

						success = true

						txsSentMu.Lock()
						txsSent++
						txsSentMu.Unlock()

						r.logger.Info("transaction sent successfully",
							zap.String("txHash", res.TxHash),
							zap.String("wallet", walletAddress),
							zap.Uint64("nonce", nonce))
					}
				}

				if sentTx != (inttypes.SentTx{}) {
					sentTxsMu.Lock()
					sentTxs = append(sentTxs, sentTx)
					sentTxsMu.Unlock()
				}
			}(msgType, i)
		}
	}

	wg.Wait()

	r.sentTxsMu.Lock()
	r.sentTxs = append(r.sentTxs, sentTxs...)
	r.sentTxsMu.Unlock()

	r.logger.Info("completed sending transactions for block",
		zap.Int("block_number", r.numBlocksProcessed),
		zap.Int("txs_sent", txsSent),
		zap.Int("expected_txs", r.totalTxsPerBlock))

	return txsSent, nil
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

func (r *Runner) initAccountNumbers(ctx context.Context) error {
	r.logger.Info("Initializing account numbers and nonces for all wallets")
	for _, wallet := range r.wallets {
		walletAddress := wallet.FormattedAddress()
		client := wallet.GetClient()

		acc, err := client.GetAccount(ctx, walletAddress)
		if err != nil {
			return fmt.Errorf("failed to initialize account for wallet %s: %w", walletAddress, err)
		}

		r.accountNumbers[walletAddress] = acc.GetAccountNumber()
		r.walletNonces[walletAddress] = acc.GetSequence()

		r.logger.Debug("Initialized account data",
			zap.String("wallet", walletAddress),
			zap.Uint64("accountNumber", acc.GetAccountNumber()),
			zap.Uint64("nonce", acc.GetSequence()))
	}
	r.logger.Info("Account numbers and nonces initialized successfully for all wallets")
	return nil
}
