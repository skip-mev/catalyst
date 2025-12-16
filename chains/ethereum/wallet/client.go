package wallet

import (
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/rpc"
)

// Client is the interface for a go-ethereum client.
type Client interface {
	ethereum.BlockNumberReader
	ethereum.ChainReader
	ethereum.ChainStateReader
	ethereum.ContractCaller
	ethereum.GasEstimator
	ethereum.GasPricer
	ethereum.GasPricer1559
	ethereum.FeeHistoryReader
	ethereum.LogFilterer
	ethereum.PendingStateReader
	ethereum.PendingContractCaller
	ethereum.TransactionReader
	ethereum.TransactionSender
	ethereum.ChainIDReader
	Client() *rpc.Client
}
