package runner

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/skip-mev/catalyst/chains/ethereum/metrics"
	"github.com/skip-mev/catalyst/chains/ethereum/txfactory"
	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

// TODO: we likely need to be more sophisticated here for problems that may arise when txs fail.
// i.e. nonces could be out of whack if one batch fails but we still want to continue.

type Runner struct {
	logger *zap.Logger

	clients   []*ethclient.Client
	wsClients []*ethclient.Client

	spec        loadtesttypes.LoadTestSpec
	chainConfig inttypes.ChainConfig
	nonces      *sync.Map
	wallets     []*wallet.InteractingWallet

	txFactory *txfactory.TxFactory

	sentTxs         []*inttypes.SentTx
	blocksProcessed uint64
}

func NewRunner(ctx context.Context, logger *zap.Logger, spec loadtesttypes.LoadTestSpec) (*Runner, error) {
	chainCfg := spec.ChainCfg.(*inttypes.ChainConfig)
	clients := make([]*ethclient.Client, 0, len(chainCfg.NodesAddresses))
	wsClients := make([]*ethclient.Client, 0, len(chainCfg.NodesAddresses))
	for _, nodeAddress := range chainCfg.NodesAddresses {
		tr := &http.Transport{
			MaxConnsPerHost:     256,  // cap concurrency per host
			MaxIdleConns:        2048, // large idle pool
			MaxIdleConnsPerHost: 1024,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
				// DO NOT set LocalAddr unless you really need to.
			}).DialContext,
			// Keep-alives are on by default; don't disable them.
		}
		hc := &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second, // per-request ceiling
		}
		rpcClient, err := rpc.DialOptions(ctx, nodeAddress.RPC, rpc.WithHTTPClient(hc))
		if err != nil {
			return nil, fmt.Errorf("failed construct RPC client for %s: %w", nodeAddress.RPC, err)
		}
		client := ethclient.NewClient(rpcClient)
		clients = append(clients, client)

		wsClient, err := ethclient.DialContext(ctx, nodeAddress.Websocket)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to ws node %s: %w", nodeAddress, err)
		}
		wsClients = append(wsClients, wsClient)
	}

	wallets, err := wallet.NewWalletsFromSpec(spec, wsClients)
	if err != nil {
		return nil, err
	}

	txf := txfactory.NewTxFactory(logger, wallets, chainCfg.TxOpts)
	nonces := sync.Map{}
	for _, wallet := range wallets {
		nonce, err := wallet.GetNonce(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce of %s: %w", wallet.Address(), err)
		}
		nonces.Store(wallet.Address(), nonce)
	}

	r := &Runner{
		logger:          logger,
		clients:         clients,
		wsClients:       wsClients,
		spec:            spec,
		chainConfig:     *chainCfg,
		wallets:         wallets,
		txFactory:       txf,
		sentTxs:         make([]*inttypes.SentTx, 0, 100),
		blocksProcessed: 0,
		nonces:          &nonces,
	}

	return r, nil
}

func (r *Runner) PrintResults(result loadtesttypes.LoadTestResult) {
	metrics.PrintResults(result)
}

// ContractDeployer encapsulates the contract deployment configuration
type ContractDeployer struct {
	msgType        loadtesttypes.MsgType
	setAddressFunc func(...common.Address)
}

// deployContracts is a generic function that handles the common deployment logic
func (r *Runner) deployContracts(ctx context.Context, deployer ContractDeployer) error {
	numInitialDeploy := r.chainConfig.NumInitialContracts
	if numInitialDeploy == 0 {
		numInitialDeploy = 5
	}

	contractDeploy := loadtesttypes.LoadTestMsg{Type: deployer.msgType}
	deployedTxs := make([]*gethtypes.Transaction, 0)

	// Deploy contracts
	for range numInitialDeploy {
		txs, err := r.buildLoad(contractDeploy, false)
		if err != nil {
			return fmt.Errorf("failed to deploy contracts in PreRun: %w", err)
		}

		for _, tx := range txs {
			if err := r.wallets[0].SendTransaction(ctx, tx); err != nil {
				return fmt.Errorf("failed to send transaction in PreRun: %w", err)
			}
		}

		// Get the main contract transaction (always the last one)
		mainContractTx := txs[len(txs)-1]
		deployedTxs = append(deployedTxs, mainContractTx)
	}

	// Wait for receipts and collect addresses
	addresses := make([]common.Address, len(deployedTxs))
	wg := sync.WaitGroup{}

	for i, tx := range deployedTxs {
		wg.Add(1)
		go func(index int, transaction *gethtypes.Transaction) {
			defer wg.Done()
			rec, err := r.wallets[rand.Intn(len(r.wallets))].WaitForTxReceipt(ctx, transaction.Hash(), 5*time.Second)
			if err == nil {
				addresses[index] = rec.ContractAddress
			} else {
				r.logger.Error("failed to find receipt", zap.String("msg_type", deployer.msgType.String()), zap.Error(err))
			}
		}(i, tx)
	}
	wg.Wait()

	// Set addresses in the factory
	for _, addr := range addresses {
		if addr.Cmp(common.Address{}) != 0 {
			deployer.setAddressFunc(addr)
		}
	}

	return nil
}

// deployWETH deploys WETH contracts using the generic deployment function
func (r *Runner) deployWETH(ctx context.Context) error {
	deployer := ContractDeployer{
		msgType:        inttypes.MsgDeployERC20,
		setAddressFunc: r.txFactory.SetWETHAddresses,
	}
	return r.deployContracts(ctx, deployer)
}

// deployLoader deploys loader contracts using the generic deployment function
func (r *Runner) deployLoader(ctx context.Context) error {
	deployer := ContractDeployer{
		msgType:        inttypes.MsgCreateContract,
		setAddressFunc: r.txFactory.SetLoaderAddresses,
	}
	return r.deployContracts(ctx, deployer)
}

// deployInitialContracts deploys the contracts needed for the messages in the spec.
func (r *Runner) deployInitialContracts(ctx context.Context) error {
	hasLoaderDependencies := slices.ContainsFunc(r.spec.Msgs, func(msg loadtesttypes.LoadTestMsg) bool {
		return slices.Contains(inttypes.LoaderDependencies, msg.Type)
	})
	hasERC20Dependencies := slices.ContainsFunc(r.spec.Msgs, func(msg loadtesttypes.LoadTestMsg) bool {
		return slices.Contains(inttypes.ERC20Dependencies, msg.Type)
	})
	r.logger.Info("deploy loader?", zap.Bool("hasLoaderDependencies", hasLoaderDependencies))
	r.logger.Info("deploy erc20?", zap.Bool("hasERC20Deps", hasERC20Dependencies))
	if hasLoaderDependencies {
		if err := r.deployLoader(ctx); err != nil {
			return err
		}
	}
	if hasERC20Dependencies {
		if err := r.deployWETH(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) buildFullLoad(ctx context.Context) ([][]*gethtypes.Transaction, error) {
	if err := r.txFactory.SetBaselines(ctx, r.spec.Msgs); err != nil {
		return nil, fmt.Errorf("failed to set Baseline txs: %w", err)
	}

	r.logger.Info("Building load...", zap.Int("num_batches", r.spec.NumBatches))
	batchLoads := make([][]*gethtypes.Transaction, 0, 100)
	total := 0
	for i := range r.spec.NumBatches {
		batch := make([]*gethtypes.Transaction, 0)
		for _, msgSpec := range r.spec.Msgs {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("ctx cancelled during load building: %w", ctx.Err())
			default:
			}
			for range msgSpec.NumMsgs {
				txs, err := r.buildLoad(msgSpec, true)
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

func (r *Runner) Run(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// when batches and interval are specified, user wants to run on a timed interval
	if r.spec.NumBatches > 0 && r.spec.SendInterval > 0 {
		r.logger.Info("Running loadtest on interval", zap.Duration("interval", r.spec.SendInterval), zap.Int("num_batches", r.spec.NumBatches))
		return r.runOnInterval(ctx)
	}
	// otherwise we run on blocks
	return r.runOnBlocks(ctx)
}

// runOnInterval starts the runner configured for interval load sending.
func (r *Runner) runOnInterval(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// deploy the initial contracts needed by the runner.
	if err := r.deployInitialContracts(ctx); err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}
	// we build the full load upfront. that is, num_batches * [msg * msg spec amount].
	batchLoads, err := r.buildFullLoad(ctx)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}
	amountPerBatch := len(batchLoads[0])
	total := len(batchLoads) * amountPerBatch

	crank := time.NewTicker(r.spec.SendInterval)
	defer crank.Stop()

	// sleeping once before we start, as the initial contracts were showing up in results.
	time.Sleep(2 * time.Second)
	blockNum, err := r.wallets[0].GetClient().BlockNumber(ctx)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, fmt.Errorf("failed to get block number: %w", err)
	}
	startingBlock := blockNum

	// load index is the index into the batchLoads slice.
	loadIndex := 0

	// go routines will send transactions and then push results to collectionChannel.
	mu := &sync.Mutex{}
	sentTxs := make([]*inttypes.SentTx, 0, total)
	collectionChannel := make(chan *inttypes.SentTx, amountPerBatch)
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
			// get the load, initialize result slice.
			load := batchLoads[loadIndex]
			r.logger.Info("Sending txs", zap.Int("num_txs", len(load)))

			// send each tx in a go routine.
			for i, tx := range load {
				wg.Add(1)
				go func() {
					defer wg.Done()
					sentTx := inttypes.SentTx{Tx: tx, TxHash: tx.Hash(), MsgType: getTxType(tx)}
					// send the tx from a random wallet.
					wallet := r.wallets[rand.Intn(len(r.wallets))]
					err := wallet.SendTransaction(ctx, tx)
					if err != nil {
						r.logger.Error("failed to send tx", zap.Error(err), zap.Int("index", i), zap.Int("load_index", loadIndex))
						sentTx.Err = err
					}
					collectionChannel <- &sentTx
				}()
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

	r.logger.Info("Loadtest complete. Waiting for mempool to clear")

	r.waitForEmptyMempool(ctx, 1*time.Minute)
	// sleep here for a sec because, even though the mempool may be empty, we could still be in process of executing those txs.
	time.Sleep(5 * time.Second)
	blockNum, err = r.wallets[0].GetClient().BlockNumber(ctx)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, fmt.Errorf("failed to get ending block number: %w", err)
	}
	endingBlock := blockNum

	// collect metrics.
	r.logger.Info("Collecting metrics", zap.Int("num_txs", len(r.sentTxs)))
	// we pass in 0 for the numOfBlockRequested, because we are not running a block based loadtest.
	// The collector understands that 0 means we are on a time interval loadtest.
	collectorStartTime := time.Now()
	collectorResults, err := metrics.ProcessResults(ctx, r.logger, r.sentTxs, startingBlock, endingBlock, r.clients)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, fmt.Errorf("failed to collect metrics: %w", err)
	}
	r.logger.Debug("collector running time",
		zap.Float64("duration_seconds", time.Since(collectorStartTime).Seconds()))
	return *collectorResults, nil
}

func getTxType(tx *gethtypes.Transaction) loadtesttypes.MsgType {
	if tx.To() == nil {
		return inttypes.ContractCreate
	}
	return inttypes.ContractCall
}

// runOnBlocks runs the loadtest via block signal.
// It sets up a subscription to block headers, then builds and deploys the load when it receives a header.
func (r *Runner) runOnBlocks(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	if err := r.deployInitialContracts(ctx); err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // Ensure cancel is always called

	blockCh := make(chan *gethtypes.Header, 1)
	subscription, err := r.wsClients[0].SubscribeNewHead(ctx, blockCh)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}

	defer subscription.Unsubscribe()
	done := make(chan struct{}, 1)
	defer close(done)
	gotStartingBlock := false
	startingBlock := uint64(0)
	endingBlock := uint64(0)
	go func() {
		for {
			select {
			case <-ctx.Done():
				r.logger.Debug("ctx cancelled")
				return
			case err := <-subscription.Err():
				if err != nil {
					r.logger.Error("subscription error", zap.Error(err))
				}
				cancel()
				return
			case block, ok := <-blockCh:
				if !ok {
					r.logger.Error("block header channel closed")
					cancel()
					return
				}
				if !gotStartingBlock {
					startingBlock = block.Number.Uint64()
					gotStartingBlock = true
				}
				r.blocksProcessed++
				r.logger.Debug(
					"processing block",
					zap.Uint64("height", block.Number.Uint64()),
					zap.Uint64("time", block.Time),
					zap.Uint64("gas_used", block.GasUsed),
					zap.Uint64("gas_limit", block.GasLimit),
				)
				numTxsSubmitted, err := r.submitLoad(ctx)
				if err != nil {
					r.logger.Error("error during tx submission", zap.Error(err), zap.Uint64("height", block.Number.Uint64()))
				}

				r.logger.Debug("submitted transactions", zap.Uint64("height", block.Number.Uint64()), zap.Int("num_submitted", numTxsSubmitted))

				r.logger.Info("processed block", zap.Uint64("height", block.Number.Uint64()), zap.Uint64("num_blocks_processed", r.blocksProcessed))
				if r.blocksProcessed >= uint64(r.spec.NumOfBlocks) { //nolint:gosec // G115: overflow unlikely in practice
					endingBlock = block.Number.Uint64()
					r.logger.Info("load test completed - number of blocks desired reached",
						zap.Uint64("blocks", r.blocksProcessed))
					done <- struct{}{}
					return
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		r.logger.Info("ctx cancelled")
		return loadtesttypes.LoadTestResult{}, ctx.Err()
	case <-done:
		r.logger.Info("load test completed. sleeping 30s for final txs to complete")

		r.waitForEmptyMempool(ctx, 1*time.Minute)

		collectorStartTime := time.Now()
		collectorResults, err := metrics.ProcessResults(ctx, r.logger, r.sentTxs, startingBlock, endingBlock, r.clients)
		if err != nil {
			return loadtesttypes.LoadTestResult{Error: err.Error()}, fmt.Errorf("failed to collect metrics: %w", err)
		}
		collectorEndTime := time.Now()
		r.logger.Debug("collector running time",
			zap.Float64("duration_seconds", collectorEndTime.Sub(collectorStartTime).Seconds()))

		return *collectorResults, nil
	}
}

func (r *Runner) submitLoad(ctx context.Context) (int, error) {
	// first we build the tx load. this constructs all the ethereum txs based in the spec.
	r.logger.Debug("building loads", zap.Int("num_msg_specs", len(r.spec.Msgs)))
	txs := make([]*gethtypes.Transaction, 0, len(r.spec.Msgs))
	for _, msgSpec := range r.spec.Msgs {
		for i := 0; i < msgSpec.NumMsgs; i++ {
			load, err := r.buildLoad(msgSpec, false)
			if err != nil {
				return 0, fmt.Errorf("failed to build load: %w", err)
			}
			if len(load) == 0 {
				continue
			}
			txs = append(txs, load...)
		}
	}

	// submit each tx in a go routine
	wg := sync.WaitGroup{}
	sentTxs := make([]*inttypes.SentTx, len(txs))
	for i, tx := range txs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fromWallet := r.wallets[rand.Intn(len(r.wallets))]
			err := fromWallet.SendTransaction(ctx, tx)
			if err != nil {
				r.logger.Debug("failed to send transaction", zap.String("tx_hash", tx.Hash().String()), zap.Error(err))
			}

			// TODO: for now its just easier to differ based on contract creation. ethereum txs dont really have
			// obvious "msgtypes" inside the tx object itself. we would have to map txhash to the spec that built the tx to get anything more specific.
			txType := inttypes.ContractCall
			if tx.To() == nil {
				txType = inttypes.ContractCreate
			}
			sentTxs[i] = &inttypes.SentTx{
				TxHash:      tx.Hash(),
				NodeAddress: "", // TODO: figure out what to do here.
				MsgType:     txType,
				Err:         err,
				Tx:          tx,
			}
		}()
	}

	wg.Wait()

	r.sentTxs = append(r.sentTxs, sentTxs...)
	return len(sentTxs), nil
}

func (r *Runner) buildLoad(msgSpec loadtesttypes.LoadTestMsg, useBaseline bool) ([]*gethtypes.Transaction, error) {
	fromWallet := r.wallets[rand.Intn(len(r.wallets))]

	nonce, ok := r.nonces.Load(fromWallet.Address())
	if !ok {
		// this really should not happen ever. better safe than sorry.
		return nil, fmt.Errorf("nonce for wallet %s not found", fromWallet.Address())
	}
	txs, err := r.txFactory.BuildTxs(msgSpec, fromWallet, nonce.(uint64), useBaseline)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %q: %w", msgSpec.Type, err)
	}
	if len(txs) == 0 {
		return nil, nil
	}

	// some cases, like contract creation, will give us more than one tx to send.
	// the tx factory will correctly handle setting the correct nonces for these txs.
	// naturally, the final tx will have the latest nonce that should be set for the account.
	lastTx := txs[len(txs)-1]
	if lastTx == nil {
		return nil, nil
	}
	r.nonces.Store(fromWallet.Address(), lastTx.Nonce()+1)
	return txs, nil
}

func (r *Runner) waitForEmptyMempool(ctx context.Context, timeout time.Duration) {
	wg := sync.WaitGroup{}
	for _, c := range r.clients {
		wg.Add(1)
		go func(ethClient *ethclient.Client) {
			client := ethClient.Client()
			defer wg.Done()
			type TxPoolStatus struct {
				Pending hexutil.Uint64 `json:"pending"`
				Queued  hexutil.Uint64 `json:"queued"`
			}
			type TxPoolStatusResponse struct {
				JSONRPC string       `json:"jsonrpc"`
				ID      int          `json:"id"`
				Result  TxPoolStatus `json:"result"`
			}

			started := time.Now()
			timer := time.NewTicker(500 * time.Millisecond)
			timout := time.NewTimer(timeout)
			defer timer.Stop()
			defer timout.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					var res TxPoolStatusResponse
					err := client.CallContext(ctx, &res, "txpool_status")
					if err == nil {
						if res.Result.Pending == 0 {
							r.logger.Debug("mempool clear. done waiting for mempool", zap.Duration("waited", time.Since(started)))
							return
						}
					} else {
						r.logger.Debug("error calling txpool status", zap.Error(err))
					}
				case <-timout.C:
					r.logger.Debug("timed out waiting for mempool to clear", zap.Duration("waited", timeout))
					return
				}
			}
		}(c)
	}
	wg.Wait()
}
