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

type ChainConfig struct {
	GasDenom       string        `yaml:"gas_denom" json:"GasDenom"`
	Bech32Prefix   string        `yaml:"bech32_prefix" json:"Bech32Prefix"`
	UnorderedTxs   bool          `yaml:"unordered_txs,omitempty" json:"UnorderedTxs,omitempty"`
	NodesAddresses []NodeAddress `yaml:"nodes_addresses" json:"NodesAddresses"`
}

func (s ChainConfig) Validate(mainCfg loadtesttypes.LoadTestSpec) error {
	if len(s.NodesAddresses) == 0 {
		return fmt.Errorf("no node addresses provided")
	}
	if s.UnorderedTxs && mainCfg.TxTimeout == 0 {
		return fmt.Errorf("tx_timeout must be set if unordered txs is set to true")
	}
	seenMsgTypes := make(map[loadtesttypes.MsgType]bool)
	seenMsgArrTypes := make(map[loadtesttypes.MsgType]bool)

	for _, msg := range mainCfg.Msgs {
		if err := validateMsgType(msg); err != nil {
			return err
		}

		switch msg.Type {
		case MsgArr:
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
		case MsgMultiSend:
			if msg.NumOfRecipients > mainCfg.NumWallets {
				return fmt.Errorf("number of recipients must be less than or equal to number of wallets available")
			}
		default:
			if seenMsgTypes[msg.Type] {
				return fmt.Errorf("duplicate message type: %s", msg.Type)
			}
			seenMsgTypes[msg.Type] = true
		}
	}

	if s.GasDenom == "" {
		return fmt.Errorf("gas denomination must be specified")
	}

	if s.Bech32Prefix == "" {
		return fmt.Errorf("bech32 prefix must be specified")
	}
	return nil
}

func (ChainConfig) IsChainConfig() {}

func init() {
	Register()
}

func Register() {
	loadtesttypes.Register("cosmos", func() loadtesttypes.ChainConfig { return &ChainConfig{} })
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
