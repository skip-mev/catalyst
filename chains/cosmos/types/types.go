package types

import (
	"context"
	"fmt"
	"slices"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

const (
	MsgSend      loadtesttypes.MsgType = "MsgSend"
	MsgMultiSend loadtesttypes.MsgType = "MsgMultiSend"
	MsgArr       loadtesttypes.MsgType = "MsgArr"
)

var (
	validMsgTypes       = []loadtesttypes.MsgType{MsgSend, MsgMultiSend, MsgArr}
	validContainedTypes = []loadtesttypes.MsgType{MsgSend, MsgMultiSend}
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

// BroadcastError represents errors during broadcasting transactions
type BroadcastError struct {
	BlockHeight int64                 // Block height where the error occurred (0 indicates tx did not make it to a block)
	TxHash      string                // Hash of the transaction that failed
	Error       string                // Error message
	MsgType     loadtesttypes.MsgType // Type of message that failed
	NodeAddress string                // Address of the node that returned the error
}

type SentTx struct {
	TxHash            string
	NodeAddress       string
	MsgType           loadtesttypes.MsgType
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
	GasDenom       string                      `yaml:"gas_denom" json:"GasDenom"`
	Bech32Prefix   string                      `yaml:"bech32_prefix" json:"Bech32Prefix"`
	Msgs           []loadtesttypes.LoadTestMsg `yaml:"msgs" json:"Msgs"`
	UnorderedTxs   bool                        `yaml:"unordered_txs,omitempty" json:"UnorderedTxs,omitempty"`
	TxTimeout      time.Duration               `yaml:"tx_timeout,omitempty" json:"TxTimeout,omitempty"`
}

func (s *LoadTestSpec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type LoadTestSpecAux struct {
		Name           string                      `yaml:"name" json:"Name"`
		Description    string                      `yaml:"description" json:"Description"`
		ChainID        string                      `yaml:"chain_id" json:"ChainID"`
		NumOfTxs       int                         `yaml:"num_of_txs,omitempty" json:"NumOfTxs,omitempty"`
		NumOfBlocks    int                         `yaml:"num_of_blocks" json:"NumOfBlocks"`
		NodesAddresses []NodeAddress               `yaml:"nodes_addresses" json:"NodesAddresses"`
		Mnemonics      []string                    `yaml:"mnemonics" json:"Mnemonics"`
		GasDenom       string                      `yaml:"gas_denom" json:"GasDenom"`
		Bech32Prefix   string                      `yaml:"bech32_prefix" json:"Bech32Prefix"`
		Msgs           []loadtesttypes.LoadTestMsg `yaml:"msgs" json:"Msgs"`
		UnorderedTxs   bool                        `yaml:"unordered_txs,omitempty" json:"UnorderedTxs,omitempty"`
		TxTimeout      time.Duration               `yaml:"tx_timeout,omitempty" json:"TxTimeout,omitempty"`
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

	seenMsgTypes := make(map[loadtesttypes.MsgType]bool)
	seenMsgArrTypes := make(map[loadtesttypes.MsgType]bool)

	var totalWeight float64
	for _, msg := range s.Msgs {
		if err := validateMsgType(msg); err != nil {
			return err
		}
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

func validateMsgType(msg loadtesttypes.LoadTestMsg) error {
	if !slices.Contains(validMsgTypes, msg.Type) {
		return fmt.Errorf("invalid msg type %s", msg.Type)
	}
	if msg.Type == MsgArr {
		if !slices.Contains(validContainedTypes, msg.ContainedType) {
			return fmt.Errorf("invalid contained type %s", msg.ContainedType)
		}
	}
	return nil
}
