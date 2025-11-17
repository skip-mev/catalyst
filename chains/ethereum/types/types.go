package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	types2 "github.com/skip-mev/catalyst/types"
)

// Types to delineate txs/receipts.
const (
	ContractCreate types2.MsgType = "contract_create"
	ContractCall   types2.MsgType = "contract_call"
)

const (
	// MsgCreateContract deploys a contract
	MsgCreateContract types2.MsgType = "MsgCreateContract"
	// MsgWriteTo makes the contract iterate a number of times and write to a mapping each time.
	MsgWriteTo types2.MsgType = "MsgWriteTo"
	// MsgCrossContractCall calls a contract method that calls another contract.
	MsgCrossContractCall types2.MsgType = "MsgCrossContractCall"
	// MsgCallDataBlast sends a bunch of calldata to the contract
	MsgCallDataBlast types2.MsgType = "MsgCallDataBlast"

	// MsgDeployERC20 deploys a weth ERC20 contract. The contract is modified to never fail.
	// That is, you do not need to modify or initiate balances. Every call always passes.
	MsgDeployERC20 types2.MsgType = "MsgDeployERC20"
	// MsgTransferERC0 transfers a random number of tokens to a random address.
	// Transfers always succeed, no matter the balance.
	MsgTransferERC0 types2.MsgType = "MsgTransferERC0"

	// MsgNativeTransferERC20 calls the cosmos/evm native ERC20 precompile contract.
	MsgNativeTransferERC20 types2.MsgType = "MsgNativeTransferERC20"

	MsgNativeGasTransfer types2.MsgType = "MsgNativeGasTransfer"
)

var (
	ValidMessages = []types2.MsgType{MsgCreateContract, MsgWriteTo, MsgCrossContractCall, MsgCallDataBlast, MsgDeployERC20, MsgTransferERC0}

	// LoaderDependencies are the msg types that require the presence of the Loader contract.
	LoaderDependencies = []types2.MsgType{MsgWriteTo, MsgCrossContractCall, MsgCallDataBlast}
	// ERC20Dependencies are the msg types that require the presence of the WETH contract.
	ERC20Dependencies = []types2.MsgType{MsgTransferERC0}
)

type SentTx struct {
	TxHash      common.Hash
	NodeAddress string
	MsgType     types2.MsgType
	Err         error
	Tx          *gethtypes.Transaction
	Receipt     *gethtypes.Receipt
}

type NodeAddress struct {
	RPC       string `yaml:"rpc"`
	Websocket string `yaml:"websocket"`
}

type TxOpts struct {
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

func (s ChainConfig) Validate(_ types2.LoadTestSpec) error {
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
	types2.Register("eth", func() types2.ChainConfig { return &ChainConfig{} })
}
