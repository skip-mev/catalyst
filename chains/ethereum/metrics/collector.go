package metrics

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

// ProcessResults processes the results of the load test.
func ProcessResults(ctx context.Context, logger *zap.Logger, sentTxs []*types.SentTx, startBlock, endBlock uint64, clients []wallet.Client) (*loadtesttypes.LoadTestResult, error) {
	wg := sync.WaitGroup{}
	blockStats := make([]loadtesttypes.BlockStat, endBlock-startBlock+1)
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
			receipts, err := getReceiptsForBlockTxs(ctx, block, client)
			if err != nil {
				logger.Error("Error getting receipts for block", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}
			stats := buildBlockStats(block, receipts)
			blockStats[blockNum-startBlock] = stats
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
		msgStats[msgType] = stat
	}

	// here we are using transactions from the blocks to update each msg type's statistics.
	avgGasUtilization := 0.0
	for i, blockStat := range blockStats {
		// rolling average of gas utilization.
		avgGasUtilization += (blockStat.GasUtilization - avgGasUtilization) / float64(i+1)
		// iterate over block transactions and extract gas/tx stats.
		for msgType, stat := range blockStat.MessageStats {
			msgStat := msgStats[msgType]
			// update transaction status counts
			msgStat.Transactions.TotalIncluded += stat.SuccessfulTxs + stat.FailedTxs
			msgStat.Transactions.Successful += stat.SuccessfulTxs
			msgStat.Transactions.Failed += stat.FailedTxs

			// update transaction gas counts
			if msgStat.Gas.Min == 0 || stat.GasUsed < msgStat.Gas.Min {
				msgStat.Gas.Min = stat.GasUsed
			}
			if msgStat.Gas.Max == 0 || stat.GasUsed > msgStat.Gas.Max {
				msgStat.Gas.Max = stat.GasUsed
			}
			msgStat.Gas.Total += stat.GasUsed

			// save
			msgStats[msgType] = msgStat
		}
	}

	// calculate global totals
	totalIncluded, totalSuccess, totalFailed := 0, 0, 0
	totalSent := len(sentTxs)
	for msgType, msgStat := range msgStats {
		// update totals
		totalIncluded += msgStat.Transactions.TotalIncluded
		totalSuccess += msgStat.Transactions.Successful
		totalFailed += msgStat.Transactions.Failed

		// hijacking this loop to update gas averages for the msg stats.
		if msgStat.Transactions.TotalIncluded > 0 {
			msgStat.Gas.Average = msgStat.Gas.Total / int64(msgStat.Transactions.TotalIncluded)
		}
		msgStats[msgType] = msgStat
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
