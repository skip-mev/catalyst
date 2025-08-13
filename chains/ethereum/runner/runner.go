package runner

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	ethhd "github.com/cosmos/evm/crypto/hd"
	"github.com/ethereum/go-ethereum/accounts"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
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

	spec    loadtesttypes.LoadTestSpec
	nonces  sync.Map
	wallets []*wallet.InteractingWallet
	// TODO: this is not updated at all.
	blockGasLimit int64
	collector     metrics.MetricsCollector

	txFactory *txfactory.TxFactory
	sentTxs   []inttypes.SentTx
	// TODO: this might not need to be atomic.
	blocksProcessed *atomic.Int64
}

func NewRunner(ctx context.Context, logger *zap.Logger, spec loadtesttypes.LoadTestSpec) (*Runner, error) {
	chainCfg := spec.ChainCfg.(*inttypes.ChainConfig)
	clients := make([]*ethclient.Client, 0, len(chainCfg.NodesAddresses))
	wsClients := make([]*ethclient.Client, 0, len(chainCfg.NodesAddresses))
	for _, nodeAddress := range chainCfg.NodesAddresses {
		client, err := ethclient.DialContext(ctx, nodeAddress.RPC)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to node %s: %w", nodeAddress, err)
		}
		clients = append(clients, client)

		wsClient, err := ethclient.DialContext(ctx, nodeAddress.Websocket)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to ws node %s: %w", nodeAddress, err)
		}
		wsClients = append(wsClients, wsClient)
	}

	wallets := make([]*wallet.InteractingWallet, 0, len(spec.Mnemonics))
	for i, mnemonic := range spec.Mnemonics {
		derivedPrivKey, err := ethhd.EthSecp256k1.Derive()(
			mnemonic,
			"",
			accounts.DefaultRootDerivationPath.String(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to derive private key from mnemonic: %w", err)
		}
		pk, err := crypto.ToECDSA(derivedPrivKey)
		if err != nil {
			return nil, fmt.Errorf("failed to convert private key to ECDSA: %w", err)
		}
		client := clients[i%len(clients)]
		chainID, ok := new(big.Int).SetString(spec.ChainID, 10)
		if !ok {
			return nil, fmt.Errorf("failed to parse chain id from spec.ChainID: %s", spec.ChainID)
		}
		wallet := wallet.NewInteractingWallet(pk, chainID, client)
		wallets = append(wallets, wallet)
	}

	txf := txfactory.NewTxFactory(logger, wallets)
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
		wallets:         wallets,
		txFactory:       txf,
		sentTxs:         make([]inttypes.SentTx, 0, 100),
		blocksProcessed: new(atomic.Int64),
		nonces:          nonces,
		collector:       metrics.NewMetricsCollector(logger),
	}

	return r, nil
}

func (r *Runner) GetCollector() *metrics.MetricsCollector {
	return &r.collector
}

func (r *Runner) Run(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	startTime := time.Now()

	// TODO: the coordination of contexts and done signals is not correct here. a more straightforward method is needed.
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	blockCh := make(chan *gethtypes.Header, 1)
	subscription, err := r.wsClients[0].SubscribeNewHead(ctx, blockCh)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}

	defer subscription.Unsubscribe()
	defer close(blockCh)

	go func() {
		for {
			select {
			case <-subCtx.Done():
				r.logger.Debug("sub cancelled")
				return
			case <-ctx.Done():
				r.logger.Debug("ctx cancelled")
				return
			case block := <-blockCh:
				r.blocksProcessed.Add(1)
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

				r.logger.Info("processed block", zap.Uint64("height", block.Number.Uint64()))
				if r.blocksProcessed.Load() >= int64(r.spec.NumOfBlocks) {
					r.logger.Info("load test completed - number of blocks desired reached",
						zap.Int64("blocks", r.blocksProcessed.Load()))
					subscription.Unsubscribe()
					cancel()
				}
			}
		}
	}()

	select {
	case <-subCtx.Done():
		r.logger.Info("load test interrupted")
		return loadtesttypes.LoadTestResult{}, subCtx.Err()
	case <-ctx.Done():
		r.logger.Info("ctx done")
		time.Sleep(30 * time.Second) // allow txs to finish

		collectorStartTime := time.Now()
		clients := make([]wallet.Client, 0, len(r.wallets))
		for _, wallet := range r.wallets {
			clients = append(clients, wallet.GetClient())
		}
		r.collector.GroupSentTxs(ctx, r.sentTxs, clients, startTime)
		collectorResults := r.collector.ProcessResults(r.blockGasLimit, int(r.spec.NumOfBlocks))
		collectorEndTime := time.Now()
		r.logger.Debug("collector running time",
			zap.Float64("duration_seconds", collectorEndTime.Sub(collectorStartTime).Seconds()))

		return collectorResults, nil
	}
}

func (r *Runner) submitLoad(ctx context.Context) (int, error) {
	// first we build the tx load. this constructs all the ethereum txs based in the spec.
	r.logger.Debug("building loads", zap.Int("num_msg_specs", len(r.spec.Msgs)))
	txs := make([]*gethtypes.Transaction, 0, len(r.spec.Msgs))
	for _, msgSpec := range r.spec.Msgs {
		r.logger.Debug("building load", zap.String("type", msgSpec.Type.String()), zap.Int("num_msgs", msgSpec.NumMsgs))
		for i := 0; i < msgSpec.NumMsgs; i++ {
			load, err := r.buildLoad(msgSpec)
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
	sentTxs := make([]inttypes.SentTx, len(txs))
	for i, tx := range txs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fromWallet := r.wallets[rand.Intn(len(r.wallets))]
			r.logger.Debug("sending transaction", zap.String("tx_hash", tx.Hash().String()))
			err := fromWallet.SendTransaction(ctx, tx)
			if err != nil {
				r.logger.Debug("failed to send transaction", zap.String("tx_hash", tx.Hash().String()), zap.Error(err))
			}
			sentTxs[i] = inttypes.SentTx{
				TxHash:      tx.Hash(),
				NodeAddress: "", // TODO: figure out what to do here.
				MsgType:     "",
				Err:         err,
				Tx:          tx,
			}
		}()
	}

	wg.Wait()

	r.sentTxs = append(r.sentTxs, sentTxs...)
	return len(sentTxs), nil
}

func (r *Runner) buildLoad(msgSpec loadtesttypes.LoadTestMsg) ([]*gethtypes.Transaction, error) {
	fromWallet := r.wallets[rand.Intn(len(r.wallets))]

	nonce, ok := r.nonces.Load(fromWallet.Address())
	if !ok {
		// this really should not happen ever. better safe than sorry.
		return nil, fmt.Errorf("nonce for wallet %s not found", fromWallet.Address())
	}
	txs, err := r.txFactory.BuildTxs(msgSpec, fromWallet, nonce.(uint64))
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
