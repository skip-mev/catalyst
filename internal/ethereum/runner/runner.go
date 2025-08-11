package runner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/skip-mev/catalyst/internal/ethereum/txfactory"
	"github.com/skip-mev/catalyst/internal/ethereum/types"
	"github.com/skip-mev/catalyst/internal/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/internal/types"
	"go.uber.org/zap"
)

type Runner struct {
	logger *zap.Logger

	clients []*ethclient.Client

	spec          types.LoadTestSpec
	wallets       []*wallet.InteractingWallet
	blockGasLimit int64

	mu              sync.Mutex
	txFactory       *txfactory.TxFactory
	sentTxs         []*gethtypes.Transaction
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
		client := clients[i%len(clients)]
		wallet := wallet.NewInteractingWallet(&privKey, &spec.ChainID, client)
		wallets = append(wallets, wallet)
	}

	txf := txfactory.NewTxFactory(wallets)

	r := &Runner{
		logger:          logger,
		clients:         clients,
		spec:            spec,
		wallets:         wallets,
		mu:              sync.Mutex{},
		txFactory:       txf,
		sentTxs:         make([]*gethtypes.Transaction, 0, 100),
		blocksProcessed: new(atomic.Int64),
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
				// TODO: send txs for block

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
	txsSent := 0
}
