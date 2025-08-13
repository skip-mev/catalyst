package metrics

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

// MetricsCollector collects and processes metrics for load tests
type MetricsCollector struct {
	startTime         time.Time
	endTime           time.Time
	blocksProcessed   int
	txsByBlock        map[int64][]types.SentTx
	txsByNode         map[string][]types.SentTx
	txsByMsgType      map[loadtesttypes.MsgType][]types.SentTx
	gasUsageByMsgType map[loadtesttypes.MsgType][]int64
	txNotFoundCount   int
	logger            *zap.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *zap.Logger) MetricsCollector {
	return MetricsCollector{
		txsByBlock:        make(map[int64][]types.SentTx),
		txsByNode:         make(map[string][]types.SentTx),
		txsByMsgType:      make(map[loadtesttypes.MsgType][]types.SentTx),
		gasUsageByMsgType: make(map[loadtesttypes.MsgType][]int64),
		logger:            logger.With(zap.String("module", "eth_metrics_collector")),
	}
}

// GroupSentTxs groups sent txs by block, node, and message type
func (m *MetricsCollector) GroupSentTxs(ctx context.Context, sentTxs []types.SentTx, clients []wallet.Client, startTime time.Time) {
	m.startTime = startTime
	m.endTime = time.Now()

	maxWorkers := runtime.NumCPU() * 2

	type workItem struct {
		index int
		tx    *types.SentTx
	}

	workChan := make(chan workItem, len(sentTxs))
	var wg sync.WaitGroup

	var mu sync.Mutex
	var txNotFoundCount int

	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for work := range workChan {
				tx := work.tx
				if tx.TxHash.Cmp(common.Hash{}) == 0 {
					m.logger.Info("found empty string tx hash", zap.Any("tx", tx))
					continue
				}

				if tx.Err == nil {
					randomClient := clients[rand.Intn(len(clients))]
					txReceipt, err := wallet.GetTxReceipt(ctx, randomClient, tx.TxHash)
					if err != nil {
						m.logger.Error("tx not found", zap.Error(err), zap.String("tx_hash", tx.TxHash.String()))
						tx.Err = err
						mu.Lock()
						txNotFoundCount++
						mu.Unlock()
						continue
					}

					tx.Receipt = txReceipt

					mu.Lock()
					m.txsByBlock[tx.Receipt.BlockNumber.Int64()] = append(m.txsByBlock[tx.Receipt.BlockNumber.Int64()], *tx)

					if tx.Receipt.GasUsed > 0 {
						m.gasUsageByMsgType[tx.MsgType] = append(m.gasUsageByMsgType[tx.MsgType], int64(tx.Receipt.GasUsed))
					}
					sentTxs[work.index] = *tx
					mu.Unlock()
				}
			}
		}()
	}

	for i := range sentTxs {
		tx := &sentTxs[i]

		if tx.Err == nil {
			workChan <- workItem{index: i, tx: tx}
		}
	}

	close(workChan)
	wg.Wait()

	m.txNotFoundCount = txNotFoundCount
	m.logger.Info("Completed processing transactions", zap.Int("tx_not_found_count", txNotFoundCount))

	for i := range sentTxs {
		tx := &sentTxs[i]
		m.txsByNode[tx.NodeAddress] = append(m.txsByNode[tx.NodeAddress], *tx)
		m.txsByMsgType[tx.MsgType] = append(m.txsByMsgType[tx.MsgType], *tx)
	}

	m.blocksProcessed = len(m.txsByBlock)
}

// calculateGasStats calculates gas statistics for a slice of gas values
func (m *MetricsCollector) calculateGasStats(gasUsage []int64) loadtesttypes.GasStats {
	if len(gasUsage) == 0 {
		return loadtesttypes.GasStats{}
	}

	var total int64
	min := gasUsage[0]
	max := gasUsage[0]

	for _, gas := range gasUsage {
		total += gas
		if gas < min {
			min = gas
		}
		if gas > max {
			max = gas
		}
	}

	return loadtesttypes.GasStats{
		Average: total / int64(len(gasUsage)),
		Min:     min,
		Max:     max,
		Total:   total,
	}
}

// processMessageTypeStats processes statistics for each message type and returns overall totals
func (m *MetricsCollector) processMessageTypeStats(result *loadtesttypes.LoadTestResult) (int, int, int, int64) {
	var totalTxs, successfulTxs, failedTxs int
	var totalGasUsed int64

	result.ByMessage = make(map[loadtesttypes.MsgType]loadtesttypes.MessageStats, len(m.txsByMsgType))

	for msgType, txs := range m.txsByMsgType {
		successful := 0
		failed := 0
		errorCounts := make(map[string]int)
		broadcastErrors := make([]loadtesttypes.BroadcastError, 0)

		for _, tx := range txs {
			if tx.Err != nil {
				failed++
				errMsg := tx.Err.Error()
				errorCounts[errMsg]++
				broadcastError := loadtesttypes.BroadcastError{
					TxHash:      tx.TxHash.String(),
					Error:       errMsg,
					MsgType:     msgType,
					NodeAddress: tx.NodeAddress,
				}
				if tx.Receipt != nil {
					broadcastError.BlockHeight = tx.Receipt.BlockNumber.Int64()
				}
				broadcastErrors = append(broadcastErrors, broadcastError)
			} else {
				successful++
			}
		}

		stats := loadtesttypes.MessageStats{
			Transactions: loadtesttypes.TransactionStats{
				Total:      len(txs),
				Successful: successful,
				Failed:     failed,
			},
			Gas: m.calculateGasStats(m.gasUsageByMsgType[msgType]),
		}

		result.ByMessage[msgType] = stats
		totalTxs += stats.Transactions.Total
		successfulTxs += stats.Transactions.Successful
		failedTxs += stats.Transactions.Failed
		totalGasUsed += stats.Gas.Total
	}

	return totalTxs, successfulTxs, failedTxs, totalGasUsed
}

// processNodeStats processes statistics for each node
func (m *MetricsCollector) processNodeStats(result *loadtesttypes.LoadTestResult) {
	result.ByNode = make(map[string]loadtesttypes.NodeStats, len(m.txsByNode))

	for nodeAddr, txs := range m.txsByNode {
		msgCounts := make(map[loadtesttypes.MsgType]int)
		gasUsage := make([]int64, 0, len(txs))

		stats := loadtesttypes.NodeStats{
			Address: nodeAddr,
			TransactionStats: loadtesttypes.TransactionStats{
				Total: len(txs),
			},
			MessageCounts: msgCounts,
		}

		successful := 0
		failed := 0

		for _, tx := range txs {
			msgCounts[tx.MsgType]++

			if tx.Err != nil {
				failed++
			} else {
				successful++
			}

			if tx.Receipt != nil && tx.Receipt.GasUsed > 0 {
				gasUsage = append(gasUsage, int64(tx.Receipt.GasUsed))
			}
		}

		stats.TransactionStats.Successful = successful
		stats.TransactionStats.Failed = failed
		stats.GasStats = m.calculateGasStats(gasUsage)
		result.ByNode[nodeAddr] = stats
	}
}

// processBlockStats processes statistics for each block
func (m *MetricsCollector) processBlockStats(result *loadtesttypes.LoadTestResult, gasLimit int64,
	numberOfBlocksRequested int) {
	var blockHeights []int64
	for height := range m.txsByBlock {
		blockHeights = append(blockHeights, height)
	}
	sort.Slice(blockHeights, func(i, j int) bool {
		return blockHeights[i] < blockHeights[j]
	})
	// ignore any extra blocks where txs landed in block
	if len(blockHeights) > numberOfBlocksRequested {
		m.logger.Info("found extra blocks, excluding from gas utilization stats",
			zap.Int("number_of_blocks_requested", numberOfBlocksRequested),
			zap.Int("number_of_blocks_found", len(blockHeights)),
			zap.Int("number_of_blocks_excluded", len(blockHeights)-numberOfBlocksRequested))
		blockHeights = blockHeights[:numberOfBlocksRequested]
	}

	result.ByBlock = make([]loadtesttypes.BlockStat, 0, len(blockHeights))
	var totalGasUtilization float64
	for _, height := range blockHeights {
		txs := m.txsByBlock[height]
		msgStats := make(map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats)
		var blockGasUsed int64

		for _, tx := range txs {
			stats := msgStats[tx.MsgType]
			stats.TransactionsSent++

			if tx.Err != nil {
				stats.FailedTxs++
				if tx.Receipt != nil && tx.Receipt.GasUsed > 0 {
					stats.GasUsed += int64(tx.Receipt.GasUsed)
					blockGasUsed += int64(tx.Receipt.GasUsed)
				}
			} else if tx.Receipt != nil {
				stats.SuccessfulTxs++
				stats.GasUsed += int64(tx.Receipt.GasUsed)
				blockGasUsed += int64(tx.Receipt.GasUsed)
			}

			msgStats[tx.MsgType] = stats
		}

		gasUtilization := float64(blockGasUsed) / float64(gasLimit)

		blockStats := loadtesttypes.BlockStat{
			BlockHeight:    height,
			MessageStats:   msgStats,
			TotalGasUsed:   blockGasUsed,
			GasLimit:       gasLimit,
			GasUtilization: gasUtilization,
		}

		// Get block timestamp from any transaction in the block
		// TODO: do we need this? idk. receipt doesnt include time. we would need to query the block.
		//if len(txs) > 0 {
		//	timestamp, err := time.Parse(time.RFC3339, txs[0].Receipt)
		//	if err != nil {
		//		m.logger.Error("failed to parse tx timestamp", zap.String("tx_hash", txs[0].TxHash),
		//			zap.String("timestamp", txs[0].TxResponse.Timestamp), zap.Error(err))
		//	}
		//	blockStats.Timestamp = timestamp
		//}

		result.ByBlock = append(result.ByBlock, blockStats)
		totalGasUtilization += gasUtilization
	}

	if len(result.ByBlock) > 0 {
		result.Overall.AvgBlockGasUtilization = totalGasUtilization / float64(len(result.ByBlock))
	}
}

// ProcessResults returns the final load test results
func (m *MetricsCollector) ProcessResults(gasLimit int64, numOfBlocksRequested int) loadtesttypes.LoadTestResult {
	result := loadtesttypes.LoadTestResult{
		Overall: loadtesttypes.OverallStats{
			StartTime:       m.startTime,
			EndTime:         m.endTime,
			Runtime:         m.endTime.Sub(m.startTime),
			BlocksProcessed: m.blocksProcessed,
		},
		ByMessage: make(map[loadtesttypes.MsgType]loadtesttypes.MessageStats, len(m.txsByMsgType)),
		ByNode:    make(map[string]loadtesttypes.NodeStats, len(m.txsByNode)),
		ByBlock:   make([]loadtesttypes.BlockStat, 0, len(m.txsByBlock)),
	}

	totalTxs, successfulTxs, failedTxs, totalGasUsed := m.processMessageTypeStats(&result)

	// Update overall stats
	result.Overall.TotalTransactions = totalTxs
	result.Overall.SuccessfulTransactions = successfulTxs
	result.Overall.FailedTransactions = failedTxs
	totalTxsWithGasData := 0
	for _, gasUsage := range m.gasUsageByMsgType {
		totalTxsWithGasData += len(gasUsage)
	}
	if totalTxsWithGasData > 0 {
		result.Overall.AvgGasPerTransaction = totalGasUsed / int64(totalTxsWithGasData)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		m.processNodeStats(&result)
	}()

	go func() {
		defer wg.Done()
		m.processBlockStats(&result, gasLimit, numOfBlocksRequested)
	}()

	wg.Wait()

	tps, _, _, _ := m.calculateTPS(result.ByBlock, result.Overall.SuccessfulTransactions)
	if tps > 0 {
		result.Overall.TPS = tps
	}

	return result
}

// calculateTPS calculates transactions per second based on block timestamps
func (m *MetricsCollector) calculateTPS(blocks []loadtesttypes.BlockStat, successfulTxs int) (float64, float64, int64, int64) {
	if len(blocks) == 0 {
		return 0, 0, 0, 0
	}
	firstBlock := blocks[0]
	lastBlock := blocks[len(blocks)-1]

	if firstBlock.Timestamp.IsZero() || lastBlock.Timestamp.IsZero() {
		return 0, 0, 0, 0
	}

	blockTimespan := lastBlock.Timestamp.Sub(firstBlock.Timestamp).Seconds()

	if blockTimespan <= 0 {
		return 0, 0, firstBlock.BlockHeight, lastBlock.BlockHeight
	}

	tps := float64(successfulTxs) / blockTimespan
	return tps, blockTimespan, firstBlock.BlockHeight, lastBlock.BlockHeight
}

// PrintResults prints the load test results in a clean, formatted way
func (m *MetricsCollector) PrintResults(result loadtesttypes.LoadTestResult) {
	fmt.Println("\n=== Load Test Results ===")

	fmt.Println("\n🎯 Overall Statistics:")
	fmt.Printf("Total Transactions: %d\n", result.Overall.TotalTransactions)
	fmt.Printf("Successful Transactions: %d\n", result.Overall.SuccessfulTransactions)
	fmt.Printf("Failed Transactions: %d\n", result.Overall.FailedTransactions)
	fmt.Printf("Transactions Not Found: %d\n", m.txNotFoundCount)
	fmt.Printf("Average Gas Per Transaction: %d\n", result.Overall.AvgGasPerTransaction)
	fmt.Printf("Average Block Gas Utilization: %.2f%%\n", result.Overall.AvgBlockGasUtilization*100)
	fmt.Printf("Runtime: %s\n", result.Overall.Runtime)
	fmt.Printf("Blocks Processed: %d\n", result.Overall.BlocksProcessed)

	tps, blockTimespan, firstBlockHeight, lastBlockHeight := m.calculateTPS(result.ByBlock, result.Overall.SuccessfulTransactions)
	if tps > 0 {
		fmt.Printf("Transactions Per Second (TPS): %.2f\n", tps)
		fmt.Printf("Block Timespan: %.2f seconds (from block %d to %d)\n",
			blockTimespan, firstBlockHeight, lastBlockHeight)
	}

	fmt.Println("\n📊 Message Type Statistics:")
	for msgType, stats := range result.ByMessage {
		fmt.Printf("\n%s:\n", msgType)
		fmt.Printf("  Transactions:\n")
		fmt.Printf("    Total: %d\n", stats.Transactions.Total)
		fmt.Printf("    Successful: %d\n", stats.Transactions.Successful)
		fmt.Printf("    Failed: %d\n", stats.Transactions.Failed)
		fmt.Printf("  Gas Usage:\n")
		fmt.Printf("    Average: %d\n", stats.Gas.Average)
		fmt.Printf("    Min: %d\n", stats.Gas.Min)
		fmt.Printf("    Max: %d\n", stats.Gas.Max)
		fmt.Printf("    Total: %d\n", stats.Gas.Total)
		//if len(stats.Errors.BroadcastErrors) > 0 {
		//	fmt.Printf("  Errors:\n")
		//	for errType, count := range stats.Errors.ErrorCounts {
		//		fmt.Printf("    %s: %d occurrences\n", errType, count)
		//	}
		//}
	}

	fmt.Println("\n🖥️  Node Statistics:")
	for nodeAddr, stats := range result.ByNode {
		fmt.Printf("\n%s:\n", nodeAddr)
		fmt.Printf("  Transactions:\n")
		fmt.Printf("    Total: %d\n", stats.TransactionStats.Total)
		fmt.Printf("    Successful: %d\n", stats.TransactionStats.Successful)
		fmt.Printf("    Failed: %d\n", stats.TransactionStats.Failed)
		fmt.Printf("  Message Distribution:\n")
		for msgType, count := range stats.MessageCounts {
			fmt.Printf("    %s: %d\n", msgType, count)
		}
		fmt.Printf("  Gas Usage:\n")
		fmt.Printf("    Average: %d\n", stats.GasStats.Average)
		fmt.Printf("    Min: %d\n", stats.GasStats.Min)
		fmt.Printf("    Max: %d\n", stats.GasStats.Max)
	}

	fmt.Println("\n📦 Block Statistics Summary:")
	fmt.Printf("Total Blocks: %d\n", len(result.ByBlock))
	var totalGasUtilization float64
	var maxGasUtilization float64
	minGasUtilization := result.ByBlock[0].GasUtilization // set first block as min initially
	maxGasBlock := result.ByBlock[0].BlockHeight
	minGasBlock := result.ByBlock[0].BlockHeight
	for _, block := range result.ByBlock {
		totalGasUtilization += block.GasUtilization
		if block.GasUtilization > maxGasUtilization {
			maxGasUtilization = block.GasUtilization
			maxGasBlock = block.BlockHeight
		}
		if block.GasUtilization < minGasUtilization {
			minGasUtilization = block.GasUtilization
			minGasBlock = block.BlockHeight
		}
	}
	avgGasUtilization := totalGasUtilization / float64(len(result.ByBlock))
	fmt.Printf("Average Gas Utilization: %.2f%%\n", avgGasUtilization*100)
	fmt.Printf("Min Gas Utilization: %.2f%% (Block %d)\n", minGasUtilization*100, minGasBlock)
	fmt.Printf("Max Gas Utilization: %.2f%% (Block %d)\n", maxGasUtilization*100, maxGasBlock)
}
