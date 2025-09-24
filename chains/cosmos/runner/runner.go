package runner

import (
	"context"
	"fmt"
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
	"github.com/skip-mev/catalyst/chains/cosmos/client"
	"github.com/skip-mev/catalyst/chains/cosmos/metrics"
	"github.com/skip-mev/catalyst/chains/cosmos/txfactory"
	inttypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	"github.com/skip-mev/catalyst/chains/cosmos/wallet"
	logging "github.com/skip-mev/catalyst/chains/log"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

// MsgGasEstimation stores gas estimation for a specific message type
type MsgGasEstimation struct {
	gasUsed int64
	weight  float64
	numTxs  int
}

// Runner represents a load test runner that executes a single LoadTestSpec
type Runner struct {
	spec               loadtesttypes.LoadTestSpec
	clients            []*client.Chain
	wallets            []*wallet.InteractingWallet
	blockGasLimit      int64
	gasEstimations     map[loadtesttypes.LoadTestMsg]MsgGasEstimation
	totalTxsPerBlock   int
	mu                 sync.Mutex
	numBlocksProcessed int
	collector          metrics.Collector
	logger             *zap.Logger
	sentTxs            []inttypes.SentTx
	sentTxsMu          sync.RWMutex
	txFactory          *txfactory.TxFactory
	accountNumbers     map[string]uint64
	walletNonces       map[string]uint64
	walletNoncesMu     sync.Mutex

	chainCfg inttypes.ChainConfig
}

// NewRunner creates a new load test runner for a given spec
func NewRunner(ctx context.Context, spec loadtesttypes.LoadTestSpec) (*Runner, error) {
	logger, _ := zap.NewDevelopment()
	chainCfg := spec.ChainCfg.(*inttypes.ChainConfig)

	if err := spec.Validate(); err != nil {
		return nil, err
	}

	var clients []*client.Chain
	for _, node := range chainCfg.NodesAddresses {
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

	privKeys := make([]types.PrivKey, 0, spec.NumWallets)
	if spec.NumWallets > 0 {
		for i := range spec.NumWallets {
			derivedPrivKey, err := hd.Secp256k1.Derive()(
				spec.BaseMnemonic,
				strconv.Itoa(i),
				"44'/118'/0'/0/0",
			)
			if err != nil {
				return nil, fmt.Errorf("failed to derive private key from mnemonic: %w", err)
			}
			privKeys = append(privKeys, &secp256k1.PrivKey{Key: derivedPrivKey})
		}
	}

	if len(privKeys) == 0 {
		return nil, fmt.Errorf("no private keys available: either provide base mnemonic or private keys")
	}

	var wallets []*wallet.InteractingWallet
	for i, privKey := range privKeys {
		client := clients[i%len(clients)]
		wallet := wallet.NewInteractingWallet(privKey, chainCfg.Bech32Prefix, client)
		wallets = append(wallets, wallet)
	}

	runner := &Runner{
		spec:           spec,
		clients:        clients,
		wallets:        wallets,
		collector:      metrics.NewCollector(),
		logger:         logging.FromContext(ctx),
		sentTxs:        make([]inttypes.SentTx, 0),
		accountNumbers: make(map[string]uint64),
		walletNonces:   make(map[string]uint64),
		chainCfg:       *chainCfg,
	}

	runner.txFactory = txfactory.NewTxFactory(chainCfg.GasDenom, wallets)

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

	r.gasEstimations = make(map[loadtesttypes.LoadTestMsg]MsgGasEstimation)
	r.totalTxsPerBlock = 0

	gasEstimations, err := r.calculateMsgGasEstimations(ctx, client)
	if err != nil {
		return err
	}

	return r.initNumOfTxsWorkflow(gasEstimations)
}

// calculateMsgGasEstimations calculates gas estimations for all message types
func (r *Runner) calculateMsgGasEstimations(ctx context.Context, client *client.Chain) (map[loadtesttypes.LoadTestMsg]uint64, error) {
	fromWallet := r.wallets[0]
	gasEstimations := make(map[loadtesttypes.LoadTestMsg]uint64)

	for _, msgSpec := range r.spec.Msgs {
		var msgs []sdk.Msg
		var err error

		if msgSpec.Type == inttypes.MsgArr {
			msgs, err = r.txFactory.CreateMsgs(msgSpec, fromWallet)
			if err != nil {
				return nil, fmt.Errorf("failed to create messages for gas estimation: %w", err)
			}

		} else {
			msg, err := r.txFactory.CreateMsg(msgSpec, fromWallet)
			if err != nil {
				return nil, fmt.Errorf("failed to create message for gas estimation: %w", err)
			}
			msgs = []sdk.Msg{msg}
		}

		acc, err := client.GetAccount(ctx, fromWallet.FormattedAddress())
		if err != nil {
			return nil, fmt.Errorf("failed to get account: %w", err)
		}

		memo := RandomString(16)
		tx, err := fromWallet.CreateSignedTx(ctx, client, 0, sdk.Coins{}, acc.GetSequence(), acc.GetAccountNumber(),
			memo, r.chainCfg.UnorderedTxs, r.spec.TxTimeout, msgs...)
		if err != nil {
			return nil, fmt.Errorf("failed to create transaction for simulation: %w", err)
		}

		txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to encode transaction: %w", err)
		}

		gasUsed, err := client.EstimateGasUsed(ctx, txBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to estimate gas: %w", err)
		}

		gasEstimations[msgSpec] = gasUsed
	}

	return gasEstimations, nil
}

// initNumOfTxsWorkflow initializes the gas estimations based on number of transactions
func (r *Runner) initNumOfTxsWorkflow(gasEstimations map[loadtesttypes.LoadTestMsg]uint64) error {
	for _, msgSpec := range r.spec.Msgs {
		gasUsed := gasEstimations[msgSpec]

		numTxs := int(float64(r.spec.NumOfTxs) * msgSpec.Weight)

		r.gasEstimations[msgSpec] = MsgGasEstimation{
			gasUsed: int64(gasUsed), //nolint:gosec // G115: overflow unlikely in practice
			weight:  msgSpec.Weight,
			numTxs:  numTxs,
		}
		r.totalTxsPerBlock += numTxs

		r.logger.Info("transaction allocation based on NumOfTxs workflow",
			zap.String("msgType", msgSpec.Type.String()),
			zap.Int("totalTxs", r.spec.NumOfTxs),
			zap.Float64("weight", msgSpec.Weight),
			zap.Int("numTxs", numTxs))
	}

	if r.totalTxsPerBlock <= 0 {
		return fmt.Errorf("calculated total number of transactions per block is zero or negative: %d", r.totalTxsPerBlock)
	}

	return nil
}

// Run executes the load test
func (r *Runner) Run(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	if err := r.initAccountNumbers(ctx); err != nil {
		return loadtesttypes.LoadTestResult{}, err
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
		return loadtesttypes.LoadTestResult{}, ctx.Err()
	case <-done:
		if err := <-subscriptionErr; err != nil && err != context.Canceled {
			return loadtesttypes.LoadTestResult{}, fmt.Errorf("subscription error: %w", err)
		}

		// Make sure all txs are processed
		time.Sleep(30 * time.Second)

		collectorStartTime := time.Now()
		r.collector.GroupSentTxs(ctx, r.sentTxs, r.clients, startTime)
		collectorResults := r.collector.ProcessResults(r.blockGasLimit, r.spec.NumOfBlocks)
		collectorEndTime := time.Now()
		r.logger.Debug("collector running time",
			zap.Float64("duration_seconds", collectorEndTime.Sub(collectorStartTime).Seconds()))

		return collectorResults, nil
	case err := <-subscriptionErr:
		if err != context.Canceled {
			return loadtesttypes.LoadTestResult{}, fmt.Errorf("failed to subscribe to blocks: %w", err)
		}
		return loadtesttypes.LoadTestResult{}, fmt.Errorf("subscription ended unexpectedly. error: %w", err)
	}
}

// sendBlockTransactions sends transactions for a single block
func (r *Runner) sendBlockTransactions(ctx context.Context) (int, error) { //nolint:unparam // error may be returned in future versions
	txsSent := 0
	var sentTxs []inttypes.SentTx
	var sentTxsMu sync.Mutex

	r.logger.Info("starting to send transactions for block", zap.Int("block_number", r.numBlocksProcessed), zap.Int("expected_txs", r.totalTxsPerBlock))

	getLatestNonce := func(walletAddr string, client *client.Chain) uint64 { //nolint:unparam // client may be used in future versions
		r.walletNoncesMu.Lock()
		defer r.walletNoncesMu.Unlock()
		return r.walletNonces[walletAddr]
	}

	updateNonce := func(walletAddr string) {
		r.walletNoncesMu.Lock()
		defer r.walletNoncesMu.Unlock()
		r.walletNonces[walletAddr]++
	}

	var wg sync.WaitGroup
	var txsSentMu sync.Mutex

	for mspSpec, estimation := range r.gasEstimations {
		for i := 0; i < estimation.numTxs; i++ {
			wg.Add(1)
			go func(msgSpec loadtesttypes.LoadTestMsg, txIndex int) { //nolint:unparam // txIndex may be used in future versions
				defer wg.Done()

				if sentTx, _ := r.processSingleTransaction(ctx, msgSpec, getLatestNonce, updateNonce, &txsSentMu, &txsSent); sentTx != (inttypes.SentTx{}) {
					sentTxsMu.Lock()
					sentTxs = append(sentTxs, sentTx)
					sentTxsMu.Unlock()
				}
			}(mspSpec, i)
		}
	}

	wg.Wait()

	r.sentTxsMu.Lock()
	r.sentTxs = append(r.sentTxs, sentTxs...)
	r.sentTxsMu.Unlock()

	r.logger.Info("completed sending transactions for block",
		zap.Int("block_number", r.numBlocksProcessed), zap.Int("txs_sent", txsSent),
		zap.Int("expected_txs", r.totalTxsPerBlock))

	return txsSent, nil
}

// processSingleTransaction handles creating, signing, and broadcasting a single transaction
func (r *Runner) processSingleTransaction(
	ctx context.Context,
	msgSpec loadtesttypes.LoadTestMsg,
	getLatestNonce func(string, *client.Chain) uint64,
	updateNonce func(string),
	txsSentMu *sync.Mutex,
	txsSent *int,
) (inttypes.SentTx, bool) {
	// Select a random wallet for this transaction
	fromWallet := r.wallets[rand.Intn(len(r.wallets))]
	walletAddress := fromWallet.FormattedAddress()
	client := fromWallet.GetClient()

	msgs, err := r.createMessagesForType(msgSpec, fromWallet)
	if err != nil {
		r.logger.Error("failed to create message",
			zap.Error(err),
			zap.String("node", client.GetNodeAddress().RPC))
		return inttypes.SentTx{}, false
	}

	maxRetries := 3
	var sentTx inttypes.SentTx
	success := false

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		nonce := getLatestNonce(walletAddress, client)

		sentTx, success = r.createAndSendTransaction(
			ctx, msgSpec, fromWallet, client, msgs, nonce,
			updateNonce, txsSentMu, txsSent,
		)

		if success {
			break
		}
	}

	return sentTx, success
}

// createMessagesForType creates the appropriate messages based on message type
func (r *Runner) createMessagesForType(msgSpec loadtesttypes.LoadTestMsg, fromWallet *wallet.InteractingWallet) ([]sdk.Msg, error) {
	var msgs []sdk.Msg
	var err error

	if msgSpec.Type == inttypes.MsgArr {
		if msgSpec.ContainedType == "" {
			return nil, fmt.Errorf("msgSpec.ContainedType must not be empty")
		}

		msgs, err = r.txFactory.CreateMsgs(msgSpec, fromWallet)
	} else {
		msg, err := r.txFactory.CreateMsg(msgSpec, fromWallet)
		if err == nil {
			msgs = []sdk.Msg{msg}
		}
	}

	return msgs, err
}

// createAndSendTransaction creates and sends a transaction, handling the response
func (r *Runner) createAndSendTransaction(
	ctx context.Context,
	mspSpec loadtesttypes.LoadTestMsg,
	fromWallet *wallet.InteractingWallet,
	client *client.Chain,
	msgs []sdk.Msg,
	nonce uint64,
	updateNonce func(string),
	txsSentMu *sync.Mutex,
	txsSent *int,
) (inttypes.SentTx, bool) {
	walletAddress := fromWallet.FormattedAddress()

	gasBufferFactor := 1.1
	estimation := r.gasEstimations[mspSpec]
	gasWithBuffer := int64(float64(estimation.gasUsed) * gasBufferFactor)
	fees := sdk.NewCoins(sdk.NewCoin(r.chainCfg.GasDenom, sdkmath.NewInt(gasWithBuffer)))
	accountNumber := r.accountNumbers[walletAddress]
	memo := RandomString(16) // Avoid ErrTxInMempoolCache

	tx, err := fromWallet.CreateSignedTx(ctx, client, uint64(gasWithBuffer), fees, nonce, accountNumber, //nolint:gosec // G115: overflow unlikely in practice
		memo, r.chainCfg.UnorderedTxs, r.spec.TxTimeout, msgs...)
	if err != nil {
		r.logger.Error("failed to create signed tx",
			zap.Error(err),
			zap.String("node", client.GetNodeAddress().RPC))
		return inttypes.SentTx{}, false
	}

	txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
	if err != nil {
		r.logger.Error("failed to encode tx",
			zap.Error(err),
			zap.String("node", client.GetNodeAddress().RPC))
		return inttypes.SentTx{}, false
	}

	return r.broadcastAndHandleResponse(ctx, client, txBytes, mspSpec.Type, walletAddress, nonce, updateNonce, txsSentMu, txsSent, msgs)
}

// broadcastAndHandleResponse broadcasts a transaction and handles the response
func (r *Runner) broadcastAndHandleResponse(
	ctx context.Context,
	client *client.Chain,
	txBytes []byte,
	msgType loadtesttypes.MsgType,
	walletAddress string,
	nonce uint64,
	updateNonce func(string),
	txsSentMu *sync.Mutex,
	txsSent *int,
	_ []sdk.Msg,
) (inttypes.SentTx, bool) {
	res, err := client.BroadcastTx(ctx, txBytes)
	if err != nil {
		if res != nil && res.Code == 32 && strings.Contains(res.RawLog, "account sequence mismatch") {
			r.handleNonceMismatch(walletAddress, nonce, res.RawLog)
			return inttypes.SentTx{}, false
		}

		sentTx := inttypes.SentTx{
			Err:         err,
			NodeAddress: client.GetNodeAddress().RPC,
			MsgType:     msgType,
		}
		if res != nil {
			sentTx.TxHash = res.TxHash
		}
		r.logger.Error("failed to broadcast tx",
			zap.Error(err),
			zap.Any("tx", sentTx))

		return sentTx, false
	}

	sentTx := inttypes.SentTx{
		TxHash:      res.TxHash,
		NodeAddress: client.GetNodeAddress().RPC,
		MsgType:     msgType,
		Err:         nil,
	}

	updateNonce(walletAddress)

	txsSentMu.Lock()
	*txsSent++
	txsSentMu.Unlock()

	return sentTx, true
}

// handleNonceMismatch extracts the expected nonce from the error message and updates the wallet nonce
func (r *Runner) handleNonceMismatch(walletAddress string, nonce uint64, rawLog string) { //nolint:unparam // nonce may be used in future versions
	expectedNonceStr := regexp.MustCompile(`expected (\d+)`).FindStringSubmatch(rawLog)
	if len(expectedNonceStr) > 1 {
		if expectedNonce, err := strconv.ParseUint(expectedNonceStr[1], 10, 64); err == nil {
			r.walletNoncesMu.Lock()
			r.walletNonces[walletAddress] = expectedNonce
			r.walletNoncesMu.Unlock()
		}
	}
}

func (r *Runner) GetCollector() *metrics.Collector {
	return &r.collector
}

func (r *Runner) PrintResults(result loadtesttypes.LoadTestResult) {
	r.collector.PrintResults(result)
}

func RandomString(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func (r *Runner) initAccountNumbers(ctx context.Context) error {
	for _, wallet := range r.wallets {
		walletAddress := wallet.FormattedAddress()
		client := wallet.GetClient()

		acc, err := client.GetAccount(ctx, walletAddress)
		if err != nil {
			return fmt.Errorf("failed to initialize account for wallet %s: %w", walletAddress, err)
		}

		r.accountNumbers[walletAddress] = acc.GetAccountNumber()

		r.walletNoncesMu.Lock()
		r.walletNonces[walletAddress] = acc.GetSequence()
		r.walletNoncesMu.Unlock()
	}
	r.logger.Info("Account numbers and nonces initialized successfully for all wallets")
	return nil
}
