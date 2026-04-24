package ift

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"

	iftbindings "github.com/skip-mev/catalyst/chains/ethereum/contracts/load/ift"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
)

type TransferContract struct {
	address common.Address
	abi     abi.ABI
}

func NewTransferContract(address string) (*TransferContract, error) {
	if !common.IsHexAddress(address) {
		return nil, fmt.Errorf("invalid IFT contract address %q", address)
	}

	parsedABI, err := iftbindings.IftMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("parse ift transfer abi: %w", err)
	}

	return &TransferContract{
		address: common.HexToAddress(address),
		abi:     *parsedABI,
	}, nil
}

func (c *TransferContract) BuildTransferTx(
	ctx context.Context,
	fromWallet *ethwallet.InteractingWallet,
	clientID string,
	receiver string,
	amount *big.Int,
	timeoutTimestamp uint64,
	nonce uint64,
	gasFeeCap *big.Int,
	gasTipCap *big.Int,
	gasLimit uint64,
) (*gethtypes.Transaction, error) {
	calldata, err := c.abi.Pack("iftTransfer", clientID, receiver, amount, timeoutTimestamp)
	if err != nil {
		return nil, fmt.Errorf("pack iftTransfer calldata: %w", err)
	}

	return fromWallet.CreateSignedDynamicFeeTx(
		ctx,
		&c.address,
		big.NewInt(0),
		gasLimit,
		gasFeeCap,
		gasTipCap,
		calldata,
		&nonce,
	)
}
