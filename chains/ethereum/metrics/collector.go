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
	"go.uber.org/zap"
)

// ProcessResults processes the results of the load test.
func ProcessResults(ctx context.Context, logger *zap.Logger, sentTxs []*types.SentTx, startBlock, endBlock uint64, clients []*ethclient.Client) (*loadtesttypes.LoadTestResult, error) {
	wg := sync.WaitGroup{}
	blockStats := make([]loadtesttypes.BlockStat, endBlock-startBlock+1)
	receipts := make(map[uint64]gethtypes.Receipts)
	logger.Info("collecting metrics", zap.Uint64("starting_block", startBlock), zap.Uint64("ending_block", endBlock))
	// block stats. each go routine will query a block, get all receipts, and construct the block stats.
	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := clients[rand.Intn(len(clients))]
			block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNum))) //nolint:gosec // G115: overflow unlikely in practice
			if err != nil {
				logger.Error("Error getting block by number", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}
			blockReceipts, err := getReceiptsForBlockTxs(ctx, block, client)
			if err != nil {
				logger.Error("Error getting receipts for block", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}
			if len(blockReceipts) > 0 {
				receipts[blockReceipts[0].BlockNumber.Uint64()] = blockReceipts
			}
			stats := buildBlockStats(block, blockReceipts)
			blockStats[blockNum-startBlock] = stats
			logger.Info(
				"block stats",
				zap.Int64("block_num", stats.BlockHeight),
				zap.Int64("gas_limit", stats.GasLimit),
				zap.Int64("gas_used", stats.TotalGasUsed),
				zap.Int("num_txs", stats.NumTxs),
			)
		}()
	}
	wg.Wait()

	// remove any 0 tx blocks from the beginning and ends of block stats.
	// this can happen if we started processing before txs landed on chain.
	blockStats = trimBlocks(blockStats)
	logger.Info("analyzing blocks...", zap.Int("num_blocks", len(blockStats)))

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

	tps5s, _, _ := findMaxTPS(blockStats, 5*time.Second)
	tps10s, _, _ := findMaxTPS(blockStats, 10*time.Second)
	tps30s, _, _ := findMaxTPS(blockStats, 30*time.Second)
	tps60s, _, _ := findMaxTPS(blockStats, 60*time.Second)
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
			TPS5SecondWindow:          tps5s,
			TPS10SecondWindow:         tps10s,
			TPS30SecondWindow:         tps30s,
			TPS60SecondWindow:         tps60s,
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
		NumTxs:         len(receipts),
	}
	return stats
}

func getReceiptsForBlockTxs(ctx context.Context, block *gethtypes.Block, client wallet.Client) ([]*gethtypes.Receipt, error) {
	txs := block.Transactions()
	receipts := make([]*gethtypes.Receipt, 0, len(txs))
	for _, tx := range txs {
		receipt, err := client.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			return nil, fmt.Errorf("error getting receipt for block %d: %w", block.Number().Uint64(), err)
		}
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}

func trimBlocks(blocks []loadtesttypes.BlockStat) []loadtesttypes.BlockStat {
	endTxIndex := len(blocks) - 1
	for i := len(blocks) - 1; i >= 0; i-- {
		if len(blocks[i].MessageStats) == 0 {
			continue
		}
		endTxIndex = i
		break
	}

	startTxIndex := 0
	for i := range blocks {
		if len(blocks[i].MessageStats) == 0 {
			continue
		}
		startTxIndex = i
		break
	}

	return blocks[startTxIndex : endTxIndex+1]
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

//nolint:unparam // its fine
func findMaxTPS(stats []loadtesttypes.BlockStat, window time.Duration) (maxTPS float64, windowStart time.Time, windowEnd time.Time) {
	if len(stats) == 0 {
		return 0, time.Time{}, time.Time{}
	}

	maxTPS = 0

	for i := 0; i < len(stats); i++ {
		windowStartTime := stats[i].Timestamp
		windowEndTime := windowStartTime.Add(window)

		totalTxs := 0

		// count all transactions within this 10-second window
		for j := i; j < len(stats) && stats[j].Timestamp.Before(windowEndTime); j++ {
			totalTxs += stats[j].NumTxs
		}

		// Calculate TPS for this window
		tps := float64(totalTxs) / window.Seconds()

		if tps > maxTPS {
			maxTPS = tps
			windowStart = windowStartTime
			windowEnd = windowEndTime
		}
	}

	return maxTPS, windowStart, windowEnd
}
