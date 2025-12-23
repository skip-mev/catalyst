package metrics

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"github.com/skip-mev/catalyst/config"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// ProcessResults processes the results of the load test.
func ProcessResults(
	ctx context.Context,
	logger *zap.Logger,
	sentTxs []*types.SentTx,
	startBlock, endBlock uint64,
	clients []*ethclient.Client,
) (*loadtesttypes.LoadTestResult, error) {
	wg := sync.WaitGroup{}
	blockStats := make([]loadtesttypes.BlockStat, endBlock-startBlock+1)
	receipts := make(map[uint64]gethtypes.Receipts)

	fetchReceiptsConcurrently := config.EnvFromContext(ctx).ConcurrentReceipts

	logger.Info(
		"Collecting metrics",
		zap.Uint64("starting_block", startBlock),
		zap.Uint64("ending_block", endBlock),
		zap.Bool("concurrent_receipts", fetchReceiptsConcurrently),
	)

	// block stats. each go routine will query a block, get all receipts, and construct the block stats.
	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		wg.Add(1)

		//nolint:gosec // G115: overflow unlikely in practice
		blockNumBig := big.NewInt(int64(blockNum))

		go func() {
			defer wg.Done()

			start := time.Now()

			client := clients[rand.Intn(len(clients))]
			block, err := client.BlockByNumber(ctx, blockNumBig)
			if err != nil {
				logger.Error("Error getting block by number", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}

			blockReceipts, err := getReceiptsForBlockTxs(ctx, block, client, fetchReceiptsConcurrently)
			if err != nil {
				logger.Error("Error getting receipts for block", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}

			if len(blockReceipts) > 0 {
				receipts[blockReceipts[0].BlockNumber.Uint64()] = blockReceipts
			}
			blockStats[blockNum-startBlock] = buildBlockStats(block, blockReceipts)

			logger.Info(
				"Block collected",
				zap.Uint64("block_num", blockNum),
				zap.Int("receipts", len(blockReceipts)),
				zap.String("duration", time.Since(start).String()),
			)
		}()
	}

	wg.Wait()

	// remove any 0 tx blocks from the beginning and ends of block stats.
	// this can happen if we started processing before txs landed on chain.
	blockStats, err := trimBlocks(blockStats)
	if err != nil {
		return nil, fmt.Errorf("failed to trim blocks: %w", err)
	}

	logger.Info("Analyzing blocks...", zap.Int("num_blocks", len(blockStats)))
	msgStats := make(map[loadtesttypes.MsgType]loadtesttypes.MessageStats)
	totalSentByType := calculateTotalSentByType(sentTxs)
	// update each msgType's total sent transactions
	for msgType, totalSent := range totalSentByType {
		stat := msgStats[msgType]
		stat.Transactions.TotalSent = int(totalSent) //nolint:gosec // G115: overflow unlikely in practice
		stat.Gas.Min = math.MaxInt64                 // sentinel values for next step
		msgStats[msgType] = stat
	}

	// update msg stats and get global tally
	totalIncluded, totalSuccess, totalFailed := 0, 0, 0
	totalSent := len(sentTxs)
	avgGasPerTx := 0.0
	for _, blockReceipts := range receipts {
		for _, receipt := range blockReceipts {
			var msgType loadtesttypes.MsgType
			if receipt.ContractAddress.Cmp(common.Address{}) == 0 {
				msgType = types.ContractCall
			} else {
				msgType = types.ContractCreate
			}
			stat := msgStats[msgType]

			// update gas values
			stat.Gas.Max = max(stat.Gas.Max, int64(receipt.GasUsed)) //nolint:gosec // G115 likely not to happen
			stat.Gas.Min = min(stat.Gas.Min, int64(receipt.GasUsed)) //nolint:gosec // G115 likely not to happen
			stat.Gas.Total += int64(receipt.GasUsed)                 //nolint:gosec // G115 likely not to happen

			// inclusion and statuses.
			stat.Transactions.TotalIncluded++
			totalIncluded++
			if receipt.Status == gethtypes.ReceiptStatusSuccessful {
				totalSuccess++
				stat.Transactions.Successful++
			} else {
				totalFailed++
				stat.Transactions.Failed++
			}

			// gas average
			stat.Gas.Average = stat.Gas.Total / int64(stat.Transactions.TotalIncluded)
			avgGasPerTx += (float64(receipt.GasUsed) - avgGasPerTx) / float64(totalIncluded)
			msgStats[msgType] = stat
		}
	}

	// calculate statistics for ALL txs by type. (totals)
	// here we are using transactions from the blocks to update each msg type's statistics.
	avgGasUtilization := 0.0
	for i, blockStat := range blockStats {
		// rolling average of gas utilization.
		avgGasUtilization += (blockStat.GasUtilization - avgGasUtilization) / float64(i+1)
	}

	// timings / tps.
	startTime := blockStats[0].Timestamp
	endTime := blockStats[len(blockStats)-1].Timestamp
	runtime := endTime.Sub(startTime)
	tps := float64(totalIncluded) / runtime.Seconds()

	// final results.
	result := &loadtesttypes.LoadTestResult{
		Overall: loadtesttypes.OverallStats{
			TotalTransactions:         totalSent,
			TotalIncludedTransactions: totalIncluded,
			SuccessfulTransactions:    totalSuccess,
			FailedTransactions:        totalFailed,
			AvgBlockGasUtilization:    avgGasUtilization,
			AvgGasPerTransaction:      int64(avgGasPerTx),
			Runtime:                   runtime,
			StartTime:                 startTime,
			EndTime:                   endTime,
			BlocksProcessed:           len(blockStats),
			TPS:                       tps,
		},
		ByMessage: msgStats,
		ByNode:    nil, // TODO: we aren't differentiating on node at the moment. not supported.
		ByBlock:   blockStats,
	}

	return result, nil
}

func buildBlockStats(block *gethtypes.Block, receipts gethtypes.Receipts) loadtesttypes.BlockStat {
	msgStats := make(map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats)
	for _, r := range receipts {
		// if the receipt didnt have a created contract address, its a contract call receipt.
		var txType loadtesttypes.MsgType
		if r.ContractAddress.Cmp(common.Address{}) == 0 {
			txType = types.ContractCall
		} else {
			txType = types.ContractCreate
		}
		stat := msgStats[txType]
		if r.Status == gethtypes.ReceiptStatusSuccessful {
			stat.SuccessfulTxs++
		} else {
			stat.FailedTxs++
		}
		stat.GasUsed += int64(r.GasUsed) //nolint:gosec // G115: overflow unlikely in practice
		msgStats[txType] = stat
	}
	stats := loadtesttypes.BlockStat{
		BlockHeight:    block.Number().Int64(),
		Timestamp:      time.Unix(int64(block.Time()), 0), //nolint:gosec // G115: overflow unlikely in practice
		GasLimit:       int64(block.GasLimit()),           //nolint:gosec // G115: overflow unlikely in practice
		TotalGasUsed:   int64(block.GasUsed()),            //nolint:gosec // G115: overflow unlikely in practice
		MessageStats:   msgStats,
		GasUtilization: float64(block.GasUsed()) / float64(block.GasLimit()),
	}
	return stats
}

func getReceiptsForBlockTxs(
	ctx context.Context,
	block *gethtypes.Block,
	client wallet.Client,
	useConcurrency bool,
) ([]*gethtypes.Receipt, error) {
	var (
		txs         = block.Transactions()
		receipts    = make([]*gethtypes.Receipt, len(txs))
		blockNumber = block.Number().Uint64()
	)

	if !useConcurrency {
		for i, tx := range txs {
			receipt, err := client.TransactionReceipt(ctx, tx.Hash())
			if err != nil {
				return nil, fmt.Errorf("unable to get receipt for block %d, tx %d: %w", blockNumber, i, err)
			}

			receipts[i] = receipt
		}

		return receipts, nil
	}

	// fetch receipts concurrently to speed up the process.
	// this can be removed once eth_getBlockReceipts is supported by the rpc.
	const concurrency = 16

	eg, ctx := errgroup.WithContext(ctx)
	mu := sync.Mutex{}

	eg.SetLimit(concurrency)

	for i := 0; i < len(txs); i++ {
		eg.Go(func() error {
			hash := txs[i].Hash()
			receipt, err := client.TransactionReceipt(ctx, hash)
			if err != nil {
				return fmt.Errorf("unable to get receipt for block %d, tx %d: %w", blockNumber, i, err)
			}

			mu.Lock()
			receipts[i] = receipt
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return receipts, nil
}

func trimBlocks(blocks []loadtesttypes.BlockStat) ([]loadtesttypes.BlockStat, error) {
	endTxIndex := -1
	for i := len(blocks) - 1; i >= 0; i-- {
		if len(blocks[i].MessageStats) == 0 {
			continue
		}
		endTxIndex = i
		break
	}

	if endTxIndex == -1 {
		return nil, fmt.Errorf("no blocks with transactions")
	}

	startTxIndex := 0
	for i := range blocks {
		if len(blocks[i].MessageStats) == 0 {
			continue
		}
		startTxIndex = i
		break
	}

	// Include one block before the first transaction block for TPS calculation
	// This ensures we have a proper time span when all transactions are in one block
	if startTxIndex > 0 {
		startTxIndex--
	}

	return blocks[startTxIndex : endTxIndex+1], nil
}

// returns the total amount of transactions sent for each type.
func calculateTotalSentByType(sentTxs []*types.SentTx) map[loadtesttypes.MsgType]uint64 {
	totalSentByType := make(map[loadtesttypes.MsgType]uint64)
	for _, tx := range sentTxs {
		if tx.Tx.To() == nil { // no To == contract creation
			totalSentByType[types.ContractCreate]++
		} else { // has a To = calling that contract
			totalSentByType[types.ContractCall]++
		}
	}
	return totalSentByType
}
