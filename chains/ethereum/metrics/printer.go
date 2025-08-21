package metrics

import (
	"fmt"

	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

func PrintResults(result loadtesttypes.LoadTestResult) {
	fmt.Println("\n=== Load Test Results ===")

	fmt.Println("\n🎯 Overall Statistics:")
	fmt.Printf("Total Transactions: %d\n", result.Overall.TotalTransactions)
	fmt.Printf("Total Included Txs: %d\n", result.Overall.TotalIncludedTransactions)
	fmt.Printf("Successful Transactions: %d\n", result.Overall.SuccessfulTransactions)
	fmt.Printf("Failed Transactions: %d\n", result.Overall.FailedTransactions)
	fmt.Printf("Transactions Not Found: %d\n", result.Overall.TotalTransactions-result.Overall.TotalIncludedTransactions)
	fmt.Printf("Average Gas Per Transaction: %d\n", result.Overall.AvgGasPerTransaction)
	fmt.Printf("Average Block Gas Utilization: %.2f%%\n", result.Overall.AvgBlockGasUtilization*100)
	fmt.Printf("Runtime: %s\n", result.Overall.Runtime)
	fmt.Printf("Blocks Processed: %d\n", result.Overall.BlocksProcessed)
	fmt.Printf("Transactions Per Second (TPS): %.2f\n", result.Overall.TPS)

	fmt.Println("\n📊 Message Type Statistics:")
	for msgType, stats := range result.ByMessage {
		fmt.Printf("\n%s:\n", msgType)
		fmt.Printf("  Transactions:\n")
		fmt.Printf("    Total Sent: %d\n", stats.Transactions.TotalSent)
		fmt.Printf("    Total Included: %d\n", stats.Transactions.TotalIncluded)
		fmt.Printf("    Execution Successful: %d\n", stats.Transactions.Successful)
		fmt.Printf("    Execution Failed: %d\n", stats.Transactions.Failed)
		fmt.Printf("  Gas Usage:\n")
		fmt.Printf("    Average: %d\n", stats.Gas.Average)
		fmt.Printf("    Min: %d\n", stats.Gas.Min)
		fmt.Printf("    Max: %d\n", stats.Gas.Max)
		fmt.Printf("    Total: %d\n", stats.Gas.Total)
	}

	fmt.Println("\n📦 Block Statistics Summary:")
	fmt.Printf("Total Blocks: %d\n", len(result.ByBlock))
	var maxGasUtilization float64
	minGasUtilization := result.ByBlock[0].GasUtilization // set first block as min initially
	maxGasBlock := result.ByBlock[0].BlockHeight
	minGasBlock := result.ByBlock[0].BlockHeight
	for _, block := range result.ByBlock {
		if block.GasUtilization > maxGasUtilization {
			maxGasUtilization = block.GasUtilization
			maxGasBlock = block.BlockHeight
		}
		if block.GasUtilization < minGasUtilization {
			minGasUtilization = block.GasUtilization
			minGasBlock = block.BlockHeight
		}
	}
	fmt.Printf("Average Gas Utilization: %.2f%%\n", result.Overall.AvgBlockGasUtilization*100)
	fmt.Printf("Min Gas Utilization: %.2f%% (Block %d)\n", minGasUtilization*100, minGasBlock)
	fmt.Printf("Max Gas Utilization: %.2f%% (Block %d)\n", maxGasUtilization*100, maxGasBlock)
}
