package types

import (
	"context"
	"fmt"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type EncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
}

// BlockHandler is a callback function for new blocks
type BlockHandler func(block Block)

type ChainI interface {
	BroadcastTx(ctx context.Context, txBytes []byte) (*sdk.TxResponse, error)
	EstimateGasUsed(ctx context.Context, txBytes []byte) (uint64, error)
	GetAccount(ctx context.Context, address string) (sdk.AccountI, error)
	GetEncodingConfig() EncodingConfig
	GetChainID() string
	GetNodeAddress() NodeAddress
	SubscribeToBlocks(ctx context.Context, gasLimit int64, handler BlockHandler) error
	GetCometClient() *rpchttp.HTTP
}

type Block struct {
	Height    int64
	GasLimit  int64
	Timestamp time.Time
}

type NodeAddress struct {
	GRPC string `json:"grpc"`
	RPC  string `json:"rpc"`
}

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
	//Errors       ErrorStats
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

// ErrorStats represents error-related statistics
type ErrorStats struct {
	BroadcastErrors []BroadcastError
	ErrorCounts     map[string]int // Error type to count
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

type SentTx struct {
	TxHash            string
	NodeAddress       string
	MsgType           MsgType
	Err               error
	TxResponse        *sdk.TxResponse
	InitialTxResponse *sdk.TxResponse
}

type LoadTestSpec struct {
	ChainID             string        `yaml:"chain_id"`
	BlockGasLimitTarget float64       `yaml:"block_gas_limit_target"` // Target percentage of block gas limit to use (0.0-1.0)
	NumOfBlocks         int           `yaml:"num_of_blocks"`
	NodesAddresses      []NodeAddress `yaml:"nodes_addresses"`
	Mnemonics           []string      `yaml:"mnemonics"`
	PrivateKeys         []types.PrivKey
	GasDenom            string        `yaml:"gas_denom"`
	Bech32Prefix        string        `yaml:"bech32_prefix"`
	Msgs                []LoadTestMsg `yaml:"msgs"`
}

func (s *LoadTestSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type LoadTestSpecAux struct {
		ChainID             string        `yaml:"chain_id"`
		BlockGasLimitTarget float64       `yaml:"block_gas_limit_target"`
		NumOfBlocks         int           `yaml:"num_of_blocks"`
		NodesAddresses      []NodeAddress `yaml:"nodes_addresses"`
		Mnemonics           []string      `yaml:"mnemonics"`
		GasDenom            string        `yaml:"gas_denom"`
		Bech32Prefix        string        `yaml:"bech32_prefix"`
		Msgs                []LoadTestMsg `yaml:"msgs"`
	}

	var aux LoadTestSpecAux
	if err := unmarshal(&aux); err != nil {
		return err
	}

	*s = LoadTestSpec{
		ChainID:             aux.ChainID,
		BlockGasLimitTarget: aux.BlockGasLimitTarget,
		NumOfBlocks:         aux.NumOfBlocks,
		NodesAddresses:      aux.NodesAddresses,
		Mnemonics:           aux.Mnemonics,
		GasDenom:            aux.GasDenom,
		Bech32Prefix:        aux.Bech32Prefix,
		Msgs:                aux.Msgs,
	}

	return nil
}

type LoadTestMsg struct {
	Weight float64 `yaml:"weight"`
	Type   MsgType `yaml:"type"`
}

type MsgType string

const (
	MsgSend      MsgType = "MsgSend"
	MsgMultiSend MsgType = "MsgMultiSend"
)

func (m MsgType) String() string {
	return string(m)
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (m *MsgType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	switch s {
	case "MsgSend":
		*m = MsgSend
	case "MsgMultiSend":
		*m = MsgMultiSend
	default:
		return fmt.Errorf("unknown MsgType: %s", s)
	}

	return nil
}
