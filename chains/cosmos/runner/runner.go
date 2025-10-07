package runner

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
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

// Runner represents a load test runner that executes a single LoadTestSpec
type Runner struct {
	spec           loadtesttypes.LoadTestSpec
	clients        []*client.Chain
	wallets        []*wallet.InteractingWallet
	gasEstimations map[loadtesttypes.LoadTestMsg]uint64
	collector      metrics.Collector
	logger         *zap.Logger
	sentTxs        []inttypes.SentTx
	txFactory      *txfactory.TxFactory
	accountNumbers map[string]uint64
	walletNonces   map[string]uint64
	walletNoncesMu sync.Mutex

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

// initGasEstimation performs initial gas estimation
func (r *Runner) initGasEstimation(ctx context.Context) error {
	client := r.clients[0]

	gasEstimations, err := r.calculateMsgGasEstimations(ctx, client)
	if err != nil {
		return err
	}
	r.gasEstimations = gasEstimations

	return nil
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

// buildLoad creates transactions for a single message specification
func (r *Runner) buildLoad(ctx context.Context, msgSpec loadtesttypes.LoadTestMsg) ([][]byte, error) {
	fromWallet := r.wallets[rand.Intn(len(r.wallets))]
	client := fromWallet.GetClient()
	walletAddress := fromWallet.FormattedAddress()

	msgs, err := r.createMessagesForType(msgSpec, fromWallet)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	gasBufferFactor := 1.5
	gasUsed := r.gasEstimations[msgSpec]
	gasWithBuffer := int64(float64(gasUsed) * gasBufferFactor)
	fees := sdk.NewCoins(sdk.NewCoin(r.chainCfg.GasDenom, sdkmath.NewInt(gasWithBuffer)))
	accountNumber := r.accountNumbers[walletAddress]

	r.walletNoncesMu.Lock()
	nonce := r.walletNonces[walletAddress]
	r.walletNonces[walletAddress]++
	r.walletNoncesMu.Unlock()

	memo := RandomString(16) // Avoid ErrTxInMempoolCache

	tx, err := fromWallet.CreateSignedTx(ctx, client, uint64(gasWithBuffer), fees, nonce, accountNumber, //nolint:gosec // G115: overflow unlikely in practice
		memo, r.chainCfg.UnorderedTxs, r.spec.TxTimeout, msgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to create signed tx: %w", err)
	}

	txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to encode tx: %w", err)
	}

	return [][]byte{txBytes}, nil
}

// buildFullLoad builds the full transaction load for interval-based sending
func (r *Runner) buildFullLoad(ctx context.Context) ([][][]byte, error) {
	r.logger.Info("Building load...", zap.Int("num_batches", r.spec.NumBatches))
	batchLoads := make([][][]byte, 0, r.spec.NumBatches)
	total := 0

	for i := range r.spec.NumBatches {
		batch := make([][]byte, 0)
		for _, msgSpec := range r.spec.Msgs {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("ctx cancelled during load building: %w", ctx.Err())
			default:
			}
			for range msgSpec.NumTxs {
				txs, err := r.buildLoad(ctx, msgSpec)
				if err != nil {
					return nil, fmt.Errorf("failed to build load for %s: %w", msgSpec.Type, err)
				}
				batch = append(batch, txs...)
				total += len(txs)
			}
		}
		r.logger.Info(fmt.Sprintf("built batch %d/%d", i+1, r.spec.NumBatches))
		batchLoads = append(batchLoads, batch)
	}
	r.logger.Info("Load built, starting loadtest", zap.Int("total_txs", total))
	return batchLoads, nil
}

// Run executes the load test
func (r *Runner) Run(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	if err := r.initAccountNumbers(ctx); err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}

	r.logger.Info("Running loadtest on interval", zap.Duration("interval", r.spec.SendInterval), zap.Int("num_batches", r.spec.NumBatches))

	// build the full load upfront
	batchLoads, err := r.buildFullLoad(ctx)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}
	amountPerBatch := len(batchLoads[0])
	total := len(batchLoads) * amountPerBatch

	crank := time.NewTicker(r.spec.SendInterval)
	defer crank.Stop()

	// sleeping once before we start
	time.Sleep(2 * time.Second)
	startTime := time.Now()

	// load index is the index into the batchLoads slice
	loadIndex := 0

	// go routines will send transactions and then push results to collectionChannel
	mu := &sync.Mutex{}
	sentTxs := make([]inttypes.SentTx, 0, total)
	collectionChannel := make(chan inttypes.SentTx, amountPerBatch)
	collectionDone := make(chan struct{})

	go func() {
		defer close(collectionDone)
		for {
			select {
			case <-ctx.Done():
				return
			case tx, ok := <-collectionChannel:
				if !ok { // channel closed
					return
				}
				mu.Lock()
				sentTxs = append(sentTxs, tx)
				mu.Unlock()
			}
		}
	}()

	// waitgroup for every tx
	wg := sync.WaitGroup{}

loop:
	for {
		select {
		case <-crank.C:
			// get the load, initialize result slice
			load := batchLoads[loadIndex]
			r.logger.Info("Sending txs", zap.Int("num_txs", len(load)))

			// send each tx in a go routine
			for i, txBytes := range load {
				wg.Add(1)
				go func(txBytes []byte, index int) {
					defer wg.Done()
					sentTx := inttypes.SentTx{MsgType: "unknown"} // TODO: track message types properly
					// select random client for sending
					client := r.clients[rand.Intn(len(r.clients))]
					res, err := client.BroadcastTx(ctx, txBytes)
					if err != nil {
						r.logger.Error("failed to send tx", zap.Error(err), zap.Int("index", index), zap.Int("load_index", loadIndex))
						sentTx.Err = err
						sentTx.NodeAddress = client.GetNodeAddress().RPC
					} else {
						sentTx.TxHash = res.TxHash
						sentTx.NodeAddress = client.GetNodeAddress().RPC
					}
					collectionChannel <- sentTx
				}(txBytes, i)
			}

			loadIndex++
			if loadIndex >= len(batchLoads) {
				// exit the loadtest loop. we have finished.
				break loop
			}
		case <-ctx.Done(): // A channel to signal stopping the ticker
			return loadtesttypes.LoadTestResult{}, fmt.Errorf("ctx cancelled during load firing: %w", ctx.Err())
		}
	}

	r.logger.Info("All transactions sent. Waiting for go routines to finish")
	wg.Wait()
	close(collectionChannel)
	<-collectionDone // wait for collection to finish

	r.logger.Info("go routines have completed", zap.Int("total_txs", len(sentTxs)))
	r.sentTxs = sentTxs

	r.logger.Info("Loadtest complete. Waiting for final txs to complete")
	time.Sleep(10 * time.Second)

	r.logger.Info("Collecting metrics", zap.Int("num_txs", len(r.sentTxs)))
	collectorStartTime := time.Now()
	r.collector.GroupSentTxs(ctx, r.sentTxs, r.clients, startTime)

	gasLimit, err := r.clients[0].GetGasLimit(ctx)
	if err != nil {
		r.logger.Warn("failed to get gas limit from chain, using 0", zap.Error(err))
		gasLimit = 0
	}

	collectorResults := r.collector.ProcessResults(gasLimit)
	collectorEndTime := time.Now()
	r.logger.Debug("collector running time",
		zap.Float64("duration_seconds", collectorEndTime.Sub(collectorStartTime).Seconds()))

	return collectorResults, nil
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

func (r *Runner) GetCollector() *metrics.Collector {
	return &r.collector
}

func (r *Runner) PrintResults(result loadtesttypes.LoadTestResult) {
	r.collector.PrintResults(result)
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

func RandomString(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
