package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

// Types to delineate txs/receipts.
const (
	ContractCreate loadtesttypes.MsgType = "contract_create"
	ContractCall   loadtesttypes.MsgType = "contract_call"
)

const (
	// MsgCreateContract deploys a contract
	MsgCreateContract loadtesttypes.MsgType = "MsgCreateContract"
	// MsgWriteTo makes the contract iterate a number of times and write to a mapping each time.
	MsgWriteTo loadtesttypes.MsgType = "MsgWriteTo"
	// MsgCrossContractCall calls a contract method that calls another contract.
	MsgCrossContractCall loadtesttypes.MsgType = "MsgCrossContractCall"
	// MsgCallDataBlast sends a bunch of calldata to the contract
	MsgCallDataBlast loadtesttypes.MsgType = "MsgCallDataBlast"
)

var ValidMessages = []loadtesttypes.MsgType{MsgCreateContract, MsgWriteTo, MsgCrossContractCall, MsgCallDataBlast}

type SentTx struct {
	TxHash      common.Hash
	NodeAddress string
	MsgType     loadtesttypes.MsgType
	Err         error
	Tx          *gethtypes.Transaction
	Receipt     *gethtypes.Receipt
}

type NodeAddress struct {
	RPC       string `yaml:"rpc"`
	Websocket string `yaml:"websocket"`
}

type TxOpts struct {
	GasPrice  *big.Int `yaml:"gas_price" json:"gas_price"`
	GasFeeCap *big.Int `yaml:"gas_fee_cap" json:"gas_fee_cap"`
	GasTipCap *big.Int `yaml:"gas_tip_cap" json:"gas_tip_cap"`
}

type ChainConfig struct {
	NodesAddresses []NodeAddress `yaml:"nodes_addresses" json:"NodesAddresses"`
	// MaxContracts is the maximum number of contracts that the loadtest runner will hold in memory.
	// The contracts in memory are used for the other load test message types to interact with.
	NumInitialContracts uint64 `yaml:"num_initial_contracts" json:"NumInitialContracts"`
	// Static gas options for transactions.
	TxOpts TxOpts `yaml:"tx_opts" json:"TxOpts"`
}

func init() {
	Register()
}

func (s ChainConfig) Validate(_ loadtesttypes.LoadTestSpec) error {
	if len(s.NodesAddresses) == 0 {
		return fmt.Errorf("no node addresses provided")
	}
	for i, nodeAddress := range s.NodesAddresses {
		if nodeAddress.RPC == "" || nodeAddress.Websocket == "" {
			return fmt.Errorf("invalid node address at index %d", i+1)
		}
	}
	return nil
}
func (ChainConfig) IsChainConfig() {}

func Register() {
	loadtesttypes.Register("eth", func() loadtesttypes.ChainConfig { return &ChainConfig{} })
}
