package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/skip-mev/catalyst/chains/ethereum/metrics"
	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

// runOnInterval starts the runner configured for interval load sending.
func (r *Runner) runOnInterval(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// deploy the initial contracts needed by the runner.
	if err := r.deployInitialContracts(ctx); err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}

	var batchLoads [][]*gethtypes.Transaction
	if r.spec.Cache.ReadTxsFrom != "" {
		txs, err := ReadTxnsFromCache(r.spec.Cache.ReadTxsFrom, r.spec.NumBatches)
		if err != nil {
			return loadtesttypes.LoadTestResult{}, fmt.Errorf("reading txs from cache at %s with batch size %d: %w", r.spec.Cache.ReadTxsFrom, r.spec.NumBatches, err)
		}
		if len(txs) == 0 {
			return loadtesttypes.LoadTestResult{}, fmt.Errorf("no txs in cache at %s with batch size %d", r.spec.Cache.ReadTxsFrom, r.spec.NumBatches)
		}
		batchLoads = txs
		r.logger.Info("loaded txs from cache", zap.Int("num_batches", len(batchLoads)), zap.String("file", r.spec.Cache.ReadTxsFrom))
	}

	if len(batchLoads) == 0 {
		// we build the full load upfront. that is, num_batches * [msg * msg spec amount].
		txs, err := r.buildFullLoad(ctx)
		if err != nil {
			return loadtesttypes.LoadTestResult{}, err
		}
		batchLoads = txs
	}

	if len(batchLoads) > 0 && r.spec.Cache.WriteTxsTo != "" {
		if err := WriteTxnsToCache(r.spec.Cache.WriteTxsTo, batchLoads); err != nil {
			r.logger.Error("caching txs", zap.Error(err), zap.Int("num_batches", len(batchLoads)), zap.String("file", r.spec.Cache.WriteTxsTo))
		} else {
			r.logger.Info("successfully cached txs", zap.Int("num_batches", len(batchLoads)), zap.String("file", r.spec.Cache.WriteTxsTo))
		}
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
					// send the tx from the wallet assigned to this transaction's sender
					wallet := r.getWalletForTx(tx)
					err = wallet.SendTransaction(ctx, tx)
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

	waitForEmptyMempool(ctx, r.clients, r.logger, 1*time.Minute)
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

func (r *Runner) buildFullLoad(ctx context.Context) ([][]*gethtypes.Transaction, error) {
	if err := r.txFactory.SetBaselines(ctx, r.spec.Msgs); err != nil {
		return nil, fmt.Errorf("failed to set Baseline txs: %w", err)
	}

	r.logger.Info("Building load...", zap.Int("num_batches", r.spec.NumBatches))
	batchLoads := make([][]*gethtypes.Transaction, 0, 100)
	total := 0
	for i := range r.spec.NumBatches {
		// Reset wallet allocation for each batch to enable role rotation
		r.txFactory.ResetWalletAllocation()

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
