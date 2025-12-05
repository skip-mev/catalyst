package runner

import (
	"context"
	"fmt"
	"math"
	"sync"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

func (r *Runner) runPersistent(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// TODO Eric -- all runners do this--refactor it out
	// deploy the initial contracts needed by the runner.
	if err := r.deployInitialContracts(ctx); err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}

	// We fund InitialWallets * 2^N wallets every block where N == the number of bootstrap loads sent.
	// We therefore require (log(num_wallets) - log(initial_wallets))/log(2) bootstrap loads to full fund.
	requiredBootstrapLoads := uint64((math.Log10(float64(r.spec.NumWallets))-math.Log10(float64(r.spec.InitialWallets)))/math.Log10(2)) + 1
	var blocksProcessed uint64
	// boostrapBackoff controls how many blocks are between load publication while we're still bootstrapping (funding wallets).
	bootstrapBackoff := uint64(5)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Run one tx to get gas baselines -- this won't work as soon as the feemarket is enabled
	if err := r.txFactory.SetBaselines(ctx, r.spec.Msgs); err != nil {
		return loadtesttypes.LoadTestResult{}, fmt.Errorf("failed to set Baseline txs: %w", err)
	}

	blockCh := make(chan *gethtypes.Header, 1)
	subscription, err := r.wsClients[0].SubscribeNewHead(ctx, blockCh)
	if err != nil {
		return loadtesttypes.LoadTestResult{}, err
	}
	defer subscription.Unsubscribe()

	var maxLoadSize int
	for _, msgSpec := range r.spec.Msgs {
		maxLoadSize += msgSpec.NumMsgs
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				r.logger.Info("ctx cancelled")
				return
			case err := <-subscription.Err():
				if err != nil {
					r.logger.Error("subscription error", zap.Error(err))
				}
				cancel()
				return
			case block, ok := <-blockCh:
				blocksProcessed++
				if !ok {
					r.logger.Error("block header channel closed")
					cancel()
					return
				}
				r.logger.Info(
					"processing block",
					zap.Uint64("height", block.Number.Uint64()),
					zap.Uint64("time", block.Time),
					zap.Uint64("gas_used", block.GasUsed),
					zap.Uint64("gas_limit", block.GasLimit),
				)

				sentBootstrapLoads := blocksProcessed / bootstrapBackoff
				// Only throttle load creation if we're still bootstrapping.
				// In that case we publish load every bootstrapBackoff blocks.
				if (sentBootstrapLoads <= requiredBootstrapLoads) && (blocksProcessed%bootstrapBackoff != 0) {
					continue
				}
				numTxsSubmitted := r.submitLoadPersistent(ctx, maxLoadSize)
				r.logger.Info("submitted transactions", zap.Uint64("height", block.Number.Uint64()), zap.Int("num_submitted", numTxsSubmitted))
			}
		}
	})

	wg.Wait()

	return loadtesttypes.LoadTestResult{}, nil
}

func (r *Runner) submitLoadPersistent(ctx context.Context, maxLoadSize int) int {
	// first we build the tx load. this constructs all the ethereum txs based in the spec.
	r.logger.Info("building loads", zap.Int("num_msg_specs", len(r.spec.Msgs)))
	var txs []*gethtypes.Transaction
	for _, msgSpec := range r.spec.Msgs {
		load := r.buildLoadPersistent(msgSpec, maxLoadSize, false)
		if len(load) == 0 {
			continue
		}
		txs = append(txs, load...)
	}

	// submit each tx in a go routine
	wg := sync.WaitGroup{}
	sentTxs := make([]*inttypes.SentTx, len(txs))
	for i, tx := range txs {
		wg.Go(func() {
			// send the tx from the wallet assigned to this transaction's sender
			fromWallet := r.getWalletForTx(tx)
			err := fromWallet.SendTransaction(ctx, tx)
			if err != nil {
				r.logger.Info("failed to send transaction", zap.String("tx_hash", tx.Hash().String()), zap.Error(err))
			}

			txType := inttypes.ContractCall
			sentTxs[i] = &inttypes.SentTx{
				TxHash:  tx.Hash(),
				MsgType: txType,
				Err:     err,
				Tx:      tx,
			}
		})
	}

	wg.Wait()

	r.sentTxs = append(r.sentTxs, sentTxs...)
	r.txFactory.ResetWalletAllocation()
	return len(sentTxs)
}

func (r *Runner) buildLoadPersistent(msgSpec loadtesttypes.LoadTestMsg, maxLoadSize int, useBaseline bool) []*gethtypes.Transaction {
	r.logger.Info("building load", zap.Int("maxLoadSize", maxLoadSize))
	var txnLoad []*gethtypes.Transaction
	var wg sync.WaitGroup
	txChan := make(chan *gethtypes.Transaction, maxLoadSize)
	for range maxLoadSize {
		wg.Go(func() {
			sender := r.txFactory.GetNextSender()

			if sender == nil {
				return
			}
			nonce, ok := r.nonces.Load(sender.Address())
			if !ok {
				// this really should not happen ever. better safe than sorry.
				r.logger.Error("nonce for wallet not found", zap.String("wallet", sender.Address().String()))
				return
			}
			tx, err := r.txFactory.BuildTxs(msgSpec, sender, nonce.(uint64), useBaseline)
			if err != nil {
				r.logger.Error("failed to build txs", zap.Error(err))
				return
			}
			lastTx := tx[len(tx)-1]
			if lastTx == nil {
				return
			}
			r.nonces.Store(sender.Address(), lastTx.Nonce()+1)
			// Only use single txn builders here
			for _, txn := range tx {
				txChan <- txn
			}
		})
	}
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		doneChan <- struct{}{}
	}()
	for {
		select {
		case txn := <-txChan:
			txnLoad = append(txnLoad, txn)
		case <-doneChan:
			r.logger.Info("Generated load txs", zap.Int("num_txs", len(txnLoad)))
			return txnLoad
		}
	}
}
