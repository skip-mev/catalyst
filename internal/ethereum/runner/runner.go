package runner

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	inttypes "github.com/skip-mev/catalyst/internal/cosmos/types"
	"github.com/skip-mev/catalyst/internal/ethereum/txfactory"
	"github.com/skip-mev/catalyst/internal/ethereum/types"
	"github.com/skip-mev/catalyst/internal/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/internal/types"
	"go.uber.org/zap"
)

// TODO: we likely need to be more sophisticated here for problems that may arise when txs fail.
// i.e. nonces could be out of whack if one batch fails but we still want to continue.

type Runner struct {
	logger *zap.Logger

	clients []*ethclient.Client

	spec          types.LoadTestSpec
	nonces        sync.Map
	wallets       []*wallet.InteractingWallet
	blockGasLimit int64

	mu              sync.Mutex
	txFactory       *txfactory.TxFactory
	sentTxs         []inttypes.SentTx
	blocksProcessed *atomic.Int64
}

func NewRunner(ctx context.Context, logger *zap.Logger, spec types.LoadTestSpec) (*Runner, error) {
	clients := make([]*ethclient.Client, 0, len(spec.NodesAddresses))
	for _, nodeAddress := range spec.NodesAddresses {
		client, err := ethclient.DialContext(ctx, nodeAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to node %s: %w", nodeAddress, err)
		}
		clients = append(clients, client)
	}
	wallets := make([]*wallet.InteractingWallet, 0, len(spec.PrivateKeys))
	for i, privKey := range spec.PrivateKeys {
		pk, err := crypto.HexToECDSA(privKey)
		if err != nil {
			return nil, fmt.Errorf("failed to convert hex private key: %w", err)
		}
		client := clients[i%len(clients)]
		wallet := wallet.NewInteractingWallet(pk, &spec.ChainID, client)
		wallets = append(wallets, wallet)
	}

	txf := txfactory.NewTxFactory(wallets)
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
		spec:            spec,
		wallets:         wallets,
		mu:              sync.Mutex{},
		txFactory:       txf,
		sentTxs:         make([]inttypes.SentTx, 0, 100),
		blocksProcessed: new(atomic.Int64),
		nonces:          nonces,
	}

	return r, nil
}

func (r *Runner) Run(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// TODO: the coordination of contexts and done signals is not correct here. a more straightforward method is needed.
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	blockCh := make(chan *gethtypes.Header, 1)
	subscription, err := r.clients[0].SubscribeNewHead(ctx, blockCh)
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
				if r.blocksProcessed.Load() >= r.spec.NumOfBlocks {
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
		// TODO: collector stuff
	}

	return loadtesttypes.LoadTestResult{}, nil
}

func (r *Runner) submitLoad(ctx context.Context) (int, error) {
	// first we build the tx load. this constructs all the ethereum txs based in the spec.
	txs := make([]*gethtypes.Transaction, 0, len(r.spec.Msgs))
	for _, msgSpec := range r.spec.Msgs {
		for i := 0; i < msgSpec.NumMsgs; i++ {
			load, err := r.buildLoad(msgSpec)
			if err != nil {
				return 0, fmt.Errorf("failed to build load: %w", err)
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
				TxHash:            tx.Hash().String(),
				NodeAddress:       "", // TODO: figure out what to do here.
				MsgType:           "",
				Err:               err,
				TxResponse:        nil,
				InitialTxResponse: nil,
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

	// some cases, like contract creation, will give us more than one tx to send.
	// the tx factory will correctly handle setting the correct nonces for these txs.
	// naturally, the final tx will have the latest nonce that should be set for the account.
	lastTx := txs[len(txs)-1]
	r.nonces.Store(fromWallet.Address(), lastTx.Nonce()+1)
	return txs, nil
}
