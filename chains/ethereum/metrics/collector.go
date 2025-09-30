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
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

type BlockStat struct {
	Number    int
	TotalTxs  int
	Timestamp time.Time
}

// ProcessResults processes the results of the load test.
func ProcessResults(ctx context.Context, logger *zap.Logger, sentTxs []*types.SentTx, startBlock, endBlock uint64, clients []*ethclient.Client) (*loadtesttypes.LoadTestResult, error) {
	logger.Info("collecting metrics", zap.Uint64("starting_block", startBlock), zap.Uint64("ending_block", endBlock))

	// number of txs per block index of the load test
	blockStats := make([]BlockStat, endBlock-startBlock+1)

	var wg sync.WaitGroup
	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		wg.Add(1)
		go func(blockNum uint64) {
			defer wg.Done()

			client := clients[rand.Intn(len(clients))]

			logger.Info("fetching block", zap.Uint64("block_num", blockNum))
			block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNum))) //nolint:gosec // G115: overflow unlikely in practice
			if err != nil {
				logger.Error("error fetching block by number", zap.Uint64("block_num", blockNum), zap.Error(err))
				return
			}

			numTxs := len(block.Transactions())
			logger.Info("fetched block", zap.Uint64("block_num", blockNum), zap.Int("num_txs", numTxs))

			blockStats[blockNum-startBlock] = BlockStat{Number: int(block.Number().Int64()), TotalTxs: int(numTxs), Timestamp: time.Unix(int64(block.Time()), 0)}

		}(blockNum)
	}
	wg.Wait()

	// remove any 0 tx blocks from the beginning and ends of block stats.
	// this can happen if we started processing before txs landed on chain.
	blockStats, err := trimBlocks(blockStats)
	if err != nil {
		return nil, fmt.Errorf("failed to trim blocks: %w", err)
	}

	logger.Info("analyzing blocks...", zap.Int("num_blocks", len(blockStats)))

	// timings / tps.
	var totalIncluded int
	for _, blockStat := range blockStats {
		totalIncluded += blockStat.TotalTxs
	}

	startBlockStat := blockStats[0]
	endBlockStat := blockStats[len(blockStats)-1]
	startTime := startBlockStat.Timestamp
	endTime := endBlockStat.Timestamp
	runtime := endTime.Sub(startTime)
	tps := float64(totalIncluded) / runtime.Seconds()

	logger.Info(
		"load test transaction info",
		zap.Float64("tps", tps),
		zap.Int("total_included", totalIncluded),
		zap.Int("total_attempted", len(sentTxs)),
	)

	logger.Info(
		"load test block info",
		zap.Int("start_block", startBlockStat.Number),
		zap.String("start_time", startBlockStat.Timestamp.Format(time.RFC3339)),
		zap.Int("end_block", endBlockStat.Number),
		zap.String("end_time", endBlockStat.Timestamp.Format(time.RFC3339)),
		zap.Int("total_blocks", endBlockStat.Number-startBlockStat.Number),
		zap.Duration("total_time", runtime),
	)

	// final results.
	result := &loadtesttypes.LoadTestResult{}

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

func trimBlocks(blocks []BlockStat) ([]BlockStat, error) {
	endTxIndex := -1
	for i := len(blocks) - 1; i >= 0; i-- {
		if blocks[i].TotalTxs == 0 {
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
		if blocks[i].TotalTxs == 0 {
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
