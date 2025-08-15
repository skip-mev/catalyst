package types

import (
	"time"
)

// LoadTestResult represents the results of a load test
type LoadTestResult struct {
	Overall   OverallStats
	ByMessage map[MsgType]MessageStats
	ByNode    map[string]NodeStats
	ByBlock   []BlockStat
	Error     string `json:"error,omitempty"`
}

// OverallStats represents the overall statistics of the load test
type OverallStats struct {
	TotalTransactions      int
	SuccessfulTransactions int
	FailedTransactions     int
	AvgGasPerTransaction   int64
	AvgBlockGasUtilization float64
	Runtime                time.Duration
	StartTime              time.Time
	EndTime                time.Time
	BlocksProcessed        int
	TPS                    float64 `json:"TPS,omitempty"`
}

// MessageStats represents statistics for a specific message type
type MessageStats struct {
	Transactions TransactionStats
	Gas          GasStats
	// Errors       ErrorStats
}

// TransactionStats represents transaction-related statistics
type TransactionStats struct {
	Total      int
	Successful int
	Failed     int
}

// GasStats represents gas-related statistics
type GasStats struct {
	Average int64
	Min     int64
	Max     int64
	Total   int64
}

// NodeStats represents statistics for a specific node
type NodeStats struct {
	Address          string
	TransactionStats TransactionStats
	MessageCounts    map[MsgType]int
	GasStats         GasStats
}

// BlockStat represents statistics for a specific block
type BlockStat struct {
	BlockHeight    int64
	Timestamp      time.Time
	GasLimit       int64
	TotalGasUsed   int64
	MessageStats   map[MsgType]MessageBlockStats
	GasUtilization float64
}

// MessageBlockStats represents message-specific statistics within a block
type MessageBlockStats struct {
	TransactionsSent int
	SuccessfulTxs    int
	FailedTxs        int
	GasUsed          int64
}

// BroadcastError represents errors during broadcasting transactions
type BroadcastError struct {
	BlockHeight int64   // Block height where the error occurred (0 indicates tx did not make it to a block)
	TxHash      string  // Hash of the transaction that failed
	Error       string  // Error message
	MsgType     MsgType // Type of message that failed
	NodeAddress string  // Address of the node that returned the error
}

type LoadTestMsg struct {
	Weight          float64 `yaml:"weight"`
	Type            MsgType `yaml:"type"`
	NumMsgs         int     `yaml:"num_msgs,omitempty" json:"NumMsgs,omitempty"`                  // Number of messages to include in MsgArr
	ContainedType   MsgType `yaml:"contained_type,omitempty" json:"ContainedType,omitempty"`      // Type of messages to include in MsgArr
	NumOfRecipients int     `yaml:"num_of_recipients,omitempty" json:"NumOfRecipients,omitempty"` // Number of recipients to include for MsgMultiSend
	NumOfIterations int     `yaml:"num_of_iterations,omitempty" json:"NumOfIterations,omitempty"` // Number of iterations for Ethereum contract operations
	CalldataSize    int     `yaml:"calldata_size,omitempty" json:"CalldataSize,omitempty"`        // Size of calldata for Ethereum large payload tests
}

type MsgType string

func (m MsgType) String() string {
	return string(m)
}
