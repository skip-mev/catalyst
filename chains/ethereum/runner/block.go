package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"go.uber.org/zap"

	"github.com/skip-mev/catalyst/chains/ethereum/metrics"
	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

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
					r.logger.Error(
						"error during tx submission",
						zap.Error(err),
						zap.Uint64("height", block.Number.Uint64()),
					)
				}

				r.logger.Debug(
					"submitted transactions",
					zap.Uint64("height", block.Number.Uint64()),
					zap.Int("num_submitted", numTxsSubmitted),
				)

				r.logger.Info(
					"processed block",
					zap.Uint64("height", block.Number.Uint64()),
					zap.Uint64("num_blocks_processed", r.blocksProcessed),
				)

				//nolint:gosec // G115: overflow unlikely in practice
				if r.blocksProcessed >= uint64(r.spec.NumOfBlocks) {
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

		waitForEmptyMempool(ctx, r.clients, r.logger, 1*time.Minute)

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
	// Reset wallet allocation for each block/load to enable role rotation
	r.txFactory.ResetWalletAllocation()

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
			// send the tx from the wallet assigned to this transaction's sender
			fromWallet := r.getWalletForTx(tx)
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
