package collector

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

func calculateTotalSentByType(sentTxs []*types.SentTx) map[loadtesttypes.MsgType]uint64 {
	totalSentByType := make(map[loadtesttypes.MsgType]uint64)
	for _, tx := range sentTxs {
		totalSentByType[tx.MsgType] += 1
	}
	return totalSentByType
}

func ProcessResults(ctx context.Context, logger *zap.Logger, sentTxs []*types.SentTx, startBlock, endBlock uint64, clients []wallet.Client) (*loadtesttypes.LoadTestResult, error) {
	wg := sync.WaitGroup{}
	blockStats := make([]loadtesttypes.BlockStat, endBlock-startBlock+1)
	logger.Info("collecting metrics", zap.Uint64("starting_block", startBlock), zap.Uint64("ending_block", endBlock))

	// block stats
	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := clients[rand.Intn(len(clients))]
			block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNum)))
			if err != nil {
				logger.Error("Error getting block by number", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}
			receipts, err := getReceiptsForBlockTxs(ctx, block, client)
			if err != nil {
				logger.Error("Error getting receipts for block", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}
			logger.Info("constructed block data", zap.Uint64("block_num", blockNum), zap.Int("receipts", len(receipts)))
			stats := buildBlockStats(block, receipts)
			blockStats[blockNum-startBlock] = stats
		}()
	}
	wg.Wait()

	logger.Info("block stats before", zap.Int("length", len(blockStats)))
	blockStats = trimBlocks(blockStats)
	logger.Info("block stats after", zap.Int("length", len(blockStats)))

	// message stats
	msgStats := make(map[loadtesttypes.MsgType]loadtesttypes.MessageStats)
	totalSentByType := calculateTotalSentByType(sentTxs)
	for msgType, totalSent := range totalSentByType {
		stat := msgStats[msgType]
		stat.Transactions.TotalSent = int(totalSent)
		msgStats[msgType] = stat
	}

	avgGasUtilization := 0.0
	for i, blockStat := range blockStats {
		avgGasUtilization += (blockStat.GasUtilization - avgGasUtilization) / float64(i+1)
		for msgType, stat := range blockStat.MessageStats {
			msgStat := msgStats[msgType]
			msgStat.Transactions.TotalIncluded += stat.SuccessfulTxs + stat.FailedTxs
			msgStat.Transactions.Successful += stat.SuccessfulTxs
			msgStat.Transactions.Failed += stat.FailedTxs
			if msgStat.Gas.Min == 0 || stat.GasUsed < msgStat.Gas.Min {
				msgStat.Gas.Min = stat.GasUsed
			}
			if msgStat.Gas.Max == 0 || stat.GasUsed > msgStat.Gas.Max {
				msgStat.Gas.Max = stat.GasUsed
			}
			msgStat.Gas.Total += stat.GasUsed
			msgStats[msgType] = msgStat
		}
	}
	logger.Info("gas block utilization value", zap.Float64("gas_utilization_value", avgGasUtilization))

	totalIncluded, totalSuccess, totalFailed := 0, 0, 0
	for msgType, msgStat := range msgStats {
		// get totals.
		totalIncluded += msgStat.Transactions.TotalIncluded
		totalSuccess += msgStat.Transactions.Successful
		totalFailed += msgStat.Transactions.Failed

		// update the gas average.
		msgStat.Gas.Average = msgStat.Gas.Total / int64(msgStat.Transactions.TotalIncluded)
		msgStats[msgType] = msgStat
	}

	startTime := blockStats[0].Timestamp
	endTime := blockStats[len(blockStats)-1].Timestamp
	runtime := endTime.Sub(startTime)
	tps := float64(totalIncluded) / runtime.Seconds()

	result := &loadtesttypes.LoadTestResult{
		Overall: loadtesttypes.OverallStats{
			TotalTransactions:         len(sentTxs),
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
		Error:     "",
	}

	return result, nil
}

var (
	ContractCreate loadtesttypes.MsgType = "contract_create"
	ContractCall   loadtesttypes.MsgType = "contract_call"
)

func buildBlockStats(block *gethtypes.Block, receipts gethtypes.Receipts) loadtesttypes.BlockStat {
	msgStats := make(map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats)
	for _, r := range receipts {
		// if the receipt didnt have a created contract address, its a contract call receipt.
		var txType loadtesttypes.MsgType
		if r.ContractAddress.Cmp(common.Address{}) == 0 {
			txType = ContractCall
		} else {
			txType = ContractCreate
		}
		stat := msgStats[txType]
		if r.Status == gethtypes.ReceiptStatusSuccessful {
			stat.SuccessfulTxs++
		} else {
			stat.FailedTxs++
		}
		stat.GasUsed += int64(r.GasUsed)
		msgStats[txType] = stat
	}
	stats := loadtesttypes.BlockStat{
		BlockHeight:    block.Number().Int64(),
		Timestamp:      time.Unix(int64(block.Time()), 0),
		GasLimit:       int64(block.GasLimit()),
		TotalGasUsed:   int64(block.GasUsed()),
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
