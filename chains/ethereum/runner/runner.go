package runner

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"sync"
	"time"

	ethhd "github.com/cosmos/evm/crypto/hd"
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

	spec          loadtesttypes.LoadTestSpec
	nonces        *sync.Map
	wallets       []*wallet.InteractingWallet
	blockGasLimit int64
	collector     metrics.Collector

	txFactory       *txfactory.TxFactory
	sentTxs         []inttypes.SentTx
	blocksProcessed uint64
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

	wallets, err := buildWallets(spec, clients)
	if err != nil {
		return nil, err
	}

	txf := txfactory.NewTxFactory(logger, wallets, chainCfg.MaxContracts)
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
		blocksProcessed: 0,
		nonces:          &nonces,
		collector:       metrics.NewCollector(logger, clients),
		blockGasLimit:   30_000_000, // TODO: this is just the max of ethereum. the target is 15m. max is 30m.
	}

	return r, nil
}

func buildWallets(spec loadtesttypes.LoadTestSpec, clients []*ethclient.Client) ([]*wallet.InteractingWallet, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("no clients provided")
	}

	chainIDStr := strings.TrimSpace(spec.ChainID)
	chainID, ok := new(big.Int).SetString(chainIDStr, 0) // allow "9001" or "0x2329"
	if !ok {
		return nil, fmt.Errorf("failed to parse chain id: %q", spec.ChainID)
	}

	// EXACT path used by 'eth_secp256k1' default account in Ethermint-based chains.
	const evmDerivationPath = "m/44'/60'/0'/0/0"

	ws := make([]*wallet.InteractingWallet, len(spec.Mnemonics))
	for i, m := range spec.Mnemonics {
		m = strings.TrimSpace(m)
		if m == "" {
			return nil, fmt.Errorf("mnemonic at index %d is empty", i)
		}

		// derive raw 32-byte private key from mnemonic at ETH path .../0
		derivedPrivKey, err := ethhd.EthSecp256k1.Derive()(m, "", evmDerivationPath)
		if err != nil {
			return nil, fmt.Errorf("mnemonic[%d]: derive failed: %w", i, err)
		}

		pk, err := crypto.ToECDSA(derivedPrivKey)
		if err != nil {
			return nil, fmt.Errorf("mnemonic[%d]: invalid ECDSA key: %w", i, err)
		}

		c := clients[i%len(clients)]
		w := wallet.NewInteractingWallet(pk, chainID, c)
		ws[i] = w
	}
	return ws, nil
}

func (r *Runner) PrintResults(result loadtesttypes.LoadTestResult) {
	r.collector.PrintResults(result)
}

func (r *Runner) Run(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	startTime := time.Now()

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

		// wait for in-flight txs but still respect ctx completion
		timer := time.NewTimer(30 * time.Second)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			r.logger.Info("ctx cancelled during post-completion sleep")
			return loadtesttypes.LoadTestResult{}, ctx.Err()
		}

		collectorStartTime := time.Now()
		clients := make([]wallet.Client, 0, len(r.wallets))
		for _, wallet := range r.wallets {
			clients = append(clients, wallet.GetClient())
		}
		r.collector.GroupSentTxs(ctx, r.sentTxs, clients, startTime)
		collectorResults := r.collector.ProcessResults(r.blockGasLimit, r.spec.NumOfBlocks)
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
			err := fromWallet.SendTransaction(ctx, tx)
			if err != nil {
				r.logger.Debug("failed to send transaction", zap.String("tx_hash", tx.Hash().String()), zap.Error(err))
			}

			// TODO: for now its just easier to differ based on contract creation. ethereum txs dont really have
			// obvious "msgtypes" inside the tx object itself. we would have to map txhash to the spec that built the tx.
			txType := "contract_call"
			if tx.To() == nil {
				txType = "contract_creation"
			}
			sentTxs[i] = inttypes.SentTx{
				TxHash:      tx.Hash(),
				NodeAddress: "", // TODO: figure out what to do here.
				MsgType:     loadtesttypes.MsgType(txType),
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
