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
	Name           string        `yaml:"name" json:"Name"`
	Description    string        `yaml:"description" json:"Description"`
	IsEvmChain     bool          `yaml:"is_evm_chain" json:"IsEvmChain"`
	ChainID        string        `yaml:"chain_id" json:"ChainID"`
	NumOfTxs       int           `yaml:"num_of_txs,omitempty" json:"NumOfTxs,omitempty"`
	NumOfBlocks    int           `yaml:"num_of_blocks" json:"NumOfBlocks"`
	NodesAddresses []NodeAddress `yaml:"nodes_addresses" json:"NodesAddresses"`
	Mnemonics      []string      `yaml:"mnemonics" json:"Mnemonics"`
	PrivateKeys    []types.PrivKey
	GasDenom       string        `yaml:"gas_denom" json:"GasDenom"`
	Bech32Prefix   string        `yaml:"bech32_prefix" json:"Bech32Prefix"`
	Msgs           []LoadTestMsg `yaml:"msgs" json:"Msgs"`
	UnorderedTxs   bool          `yaml:"unordered_txs,omitempty" json:"UnorderedTxs,omitempty"`
	TxTimeout      time.Duration `yaml:"tx_timeout,omitempty" json:"TxTimeout,omitempty"`
}

func (s *LoadTestSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type LoadTestSpecAux struct {
		Name           string        `yaml:"name" json:"Name"`
		Description    string        `yaml:"description" json:"Description"`
		ChainID        string        `yaml:"chain_id" json:"ChainID"`
		NumOfTxs       int           `yaml:"num_of_txs,omitempty" json:"NumOfTxs,omitempty"`
		NumOfBlocks    int           `yaml:"num_of_blocks" json:"NumOfBlocks"`
		NodesAddresses []NodeAddress `yaml:"nodes_addresses" json:"NodesAddresses"`
		Mnemonics      []string      `yaml:"mnemonics" json:"Mnemonics"`
		GasDenom       string        `yaml:"gas_denom" json:"GasDenom"`
		Bech32Prefix   string        `yaml:"bech32_prefix" json:"Bech32Prefix"`
		Msgs           []LoadTestMsg `yaml:"msgs" json:"Msgs"`
		UnorderedTxs   bool          `yaml:"unordered_txs,omitempty" json:"UnorderedTxs,omitempty"`
		TxTimeout      time.Duration `yaml:"tx_timeout,omitempty" json:"TxTimeout,omitempty"`
	}

	var aux LoadTestSpecAux
	if err := unmarshal(&aux); err != nil {
		return err
	}

	*s = LoadTestSpec{
		Name:           aux.Name,
		Description:    aux.Description,
		ChainID:        aux.ChainID,
		NumOfTxs:       aux.NumOfTxs,
		NumOfBlocks:    aux.NumOfBlocks,
		NodesAddresses: aux.NodesAddresses,
		Mnemonics:      aux.Mnemonics,
		GasDenom:       aux.GasDenom,
		Bech32Prefix:   aux.Bech32Prefix,
		Msgs:           aux.Msgs,
		UnorderedTxs:   aux.UnorderedTxs,
		TxTimeout:      aux.TxTimeout,
	}

	return nil
}

// Validate validates the LoadTestSpec and returns an error if it's invalid
func (s *LoadTestSpec) Validate() error {
	if len(s.NodesAddresses) == 0 {
		return fmt.Errorf("no node addresses provided")
	}

	if s.UnorderedTxs == true && s.TxTimeout == 0 {
		return fmt.Errorf("tx_timeout must be set if unordered txs is set to true")
	}

	if s.ChainID == "" {
		return fmt.Errorf("chain ID must be specified")
	}

	if s.NumOfTxs <= 0 {
		return fmt.Errorf("num_of_txs must be greater than 0")
	}

	if s.NumOfBlocks <= 0 {
		return fmt.Errorf("num_of_blocks must be greater than 0")
	}

	if len(s.Msgs) == 0 {
		return fmt.Errorf("no messages specified for load testing")
	}

	seenMsgTypes := make(map[MsgType]bool)
	seenMsgArrTypes := make(map[MsgType]bool)

	var totalWeight float64
	for _, msg := range s.Msgs {
		totalWeight += msg.Weight

		if msg.Type == MsgArr {
			if msg.ContainedType == "" {
				return fmt.Errorf("contained_type must be specified for MsgArr")
			}

			if msg.NumMsgs <= 0 {
				return fmt.Errorf("num_msgs must be greater than 0 for MsgArr")
			}

			if seenMsgArrTypes[msg.ContainedType] {
				return fmt.Errorf("duplicate MsgArr with contained_type %s", msg.ContainedType)
			}
			seenMsgArrTypes[msg.ContainedType] = true
		} else if msg.Type == MsgMultiSend {
			if msg.NumOfRecipients > len(s.Mnemonics) {
				return fmt.Errorf("number of recipients must be less than or equal to number of mneomnics available")
			}
		} else {
			if seenMsgTypes[msg.Type] {
				return fmt.Errorf("duplicate message type: %s", msg.Type)
			}
			seenMsgTypes[msg.Type] = true
		}
	}

	if totalWeight != 1 {
		return fmt.Errorf("total message weights must add up to 1.0, got %f", totalWeight)
	}

	if len(s.Mnemonics) == 0 && len(s.PrivateKeys) == 0 {
		return fmt.Errorf("either mnemonics or private keys must be provided")
	}

	if s.GasDenom == "" {
		return fmt.Errorf("gas denomination must be specified")
	}

	if s.Bech32Prefix == "" {
		return fmt.Errorf("bech32 prefix must be specified")
	}

	return nil
}

type MsgType string

const (
	MsgSend      MsgType = "MsgSend"
	MsgMultiSend MsgType = "MsgMultiSend"
	MsgArr       MsgType = "MsgArr"
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
	case "MsgArr":
		*m = MsgArr
	default:
		return fmt.Errorf("unknown MsgType: %s", s)
	}

	return nil
}

type LoadTestMsg struct {
	Weight          float64 `yaml:"weight"`
	Type            MsgType `yaml:"type"`
	NumMsgs         int     `yaml:"num_msgs,omitempty" json:"NumMsgs,omitempty"`                  // Number of messages to include in MsgArr
	ContainedType   MsgType `yaml:"contained_type,omitempty" json:"ContainedType,omitempty"`      // Type of messages to include in MsgArr
	NumOfRecipients int     `yaml:"num_of_recipients,omitempty" json:"NumOfRecipients,omitempty"` // Number of recipients to include for MsgMultiSend
}
