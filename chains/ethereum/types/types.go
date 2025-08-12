package types

import (
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
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

type SentTx struct {
	TxHash      common.Hash
	NodeAddress string
	MsgType     loadtesttypes.MsgType
	Err         error
	Tx          *gethtypes.Transaction
	Receipt     *gethtypes.Receipt
}

type LoadTestSpec struct {
	Name           string                      `yaml:"name" json:"Name"`
	Description    string                      `yaml:"description" json:"Description"`
	ChainID        big.Int                     `yaml:"chain_id" json:"ChainID"`
	NumOfTxs       int                         `yaml:"num_of_txs,omitempty" json:"NumOfTxs,omitempty"`
	NumOfBlocks    int64                       `yaml:"num_of_blocks" json:"NumOfBlocks"`
	NodesAddresses []string                    `yaml:"nodes_addresses" json:"NodesAddresses"`
	Msgs           []loadtesttypes.LoadTestMsg `yaml:"msgs" json:"Msgs"`
	TxTimeout      time.Duration               `yaml:"tx_timeout,omitempty" json:"TxTimeout,omitempty"`
	PrivateKeys    []string                    `yaml:"private_keys" json:"PrivateKeys"`
}

func (spec LoadTestSpec) Validate() error {
	if spec.ChainID.Int64() <= 0 {
		return errors.New("ChainID must be positive")
	}
	if len(spec.NodesAddresses) <= 0 {
		return errors.New("must have NodeAddresses")
	}
	if len(spec.Msgs) <= 0 {
		return errors.New("must have Msgs")
	}
	if len(spec.PrivateKeys) <= 0 {
		return errors.New("must have PrivateKeys")
	}
	return nil
}
