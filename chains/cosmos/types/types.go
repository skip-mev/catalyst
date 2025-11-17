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
	types2 "github.com/skip-mev/catalyst/types"
)

const (
	MsgSend      types2.MsgType = "MsgSend"
	MsgMultiSend types2.MsgType = "MsgMultiSend"
	MsgArr       types2.MsgType = "MsgArr"
)

var (
	validMsgTypes       = []types2.MsgType{MsgSend, MsgMultiSend, MsgArr}
	validContainedTypes = []types2.MsgType{MsgSend, MsgMultiSend}
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
	BlockHeight int64          // Block height where the error occurred (0 indicates tx did not make it to a block)
	TxHash      string         // Hash of the transaction that failed
	Error       string         // Error message
	MsgType     types2.MsgType // Type of message that failed
	NodeAddress string         // Address of the node that returned the error
}

type SentTx struct {
	TxHash            string
	NodeAddress       string
	MsgType           types2.MsgType
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

func (s ChainConfig) Validate(mainCfg types2.LoadTestSpec) error {
	if len(s.NodesAddresses) == 0 {
		return fmt.Errorf("no node addresses provided")
	}
	if s.UnorderedTxs && mainCfg.TxTimeout == 0 {
		return fmt.Errorf("tx_timeout must be set if unordered txs is set to true")
	}
	seenMsgTypes := make(map[types2.MsgType]bool)
	seenMsgArrTypes := make(map[types2.MsgType]bool)

	var totalWeight float64
	for _, msg := range mainCfg.Msgs {
		if err := validateMsgType(msg); err != nil {
			return err
		}
		totalWeight += msg.Weight

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

	if totalWeight != 1 {
		return fmt.Errorf("total message weights must add up to 1.0, got %f", totalWeight)
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
	types2.Register("cosmos", func() types2.ChainConfig { return &ChainConfig{} })
}

func validateMsgType(msg types2.LoadTestMsg) error {
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
