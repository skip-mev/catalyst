package txfactory

import (
	"context"
	rand2 "crypto/rand"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	loader "github.com/skip-mev/catalyst/chains/ethereum/contracts/load"
	"github.com/skip-mev/catalyst/chains/ethereum/contracts/load/target"
	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

type TxFactory struct {
	logger            *zap.Logger
	wallets           []*ethwallet.InteractingWallet
	contractAddresses []common.Address
	maxContracts      uint64
	mu                sync.Mutex
	txOpts            ethtypes.TxOpts
}

var defaultMaxContracts = uint64(10)

func NewTxFactory(logger *zap.Logger, wallets []*ethwallet.InteractingWallet, maxContracts uint64, txOpts ethtypes.TxOpts) *TxFactory {
	if maxContracts <= 0 {
		maxContracts = defaultMaxContracts
	}
	return &TxFactory{logger: logger.With(zap.String("module", "tx_factory")), wallets: wallets, mu: sync.Mutex{}, maxContracts: maxContracts, txOpts: txOpts}
}

func (f *TxFactory) BuildTxs(msgSpec loadtesttypes.LoadTestMsg, fromWallet *ethwallet.InteractingWallet, nonce uint64) ([]*types.Transaction, error) {
	ctx := context.Background()
	switch msgSpec.Type {
	case ethtypes.MsgCreateContract:
		return f.createMsgCreateContract(ctx, fromWallet, nil, nonce)
	case ethtypes.MsgWriteTo:
		tx, err := f.createMsgWriteTo(ctx, fromWallet, msgSpec.NumOfIterations, nonce)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	case ethtypes.MsgCallDataBlast:
		tx, err := f.createMsgCallDataBlast(ctx, fromWallet, msgSpec.CalldataSize, nonce)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	case ethtypes.MsgCrossContractCall:
		tx, err := f.createMsgCrossContractCall(ctx, fromWallet, msgSpec.NumOfIterations, nonce)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %q", msgSpec.Type)
	}
}

func (f *TxFactory) SetContractAddrs(addrs ...common.Address) {
	f.contractAddresses = append(f.contractAddresses, addrs...)
}

func (f *TxFactory) createMsgCreateContract(ctx context.Context, fromWallet *ethwallet.InteractingWallet, targets *int, nonce uint64) ([]*types.Transaction, error) {
	var numTargets int
	if targets != nil {
		numTargets = *targets
	} else {
		// add 1 so we never get 0. can be 1-3.
		numTargets = 2
	}

	// Deploy target contracts first
	targetDeployTxs := make([]*types.Transaction, 0, numTargets)
	targetContractAddrs := make([]common.Address, 0, numTargets)
	for i := 0; i < numTargets; i++ {
		addr, tx, _, err := target.DeployTarget(&bind.TransactOpts{
			From:      fromWallet.Address(),
			Signer:    fromWallet.SignerFnLegacy(),
			Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
			GasTipCap: f.txOpts.GasTipCap,
			GasFeeCap: f.txOpts.GasFeeCap,
			GasPrice:  f.txOpts.GasPrice,
			Context:   ctx,
			NoSend:    true,
		}, fromWallet.GetClient())
		if err != nil {
			return nil, fmt.Errorf("failed to create target contract transaction: %w", err)
		}
		targetContractAddrs = append(targetContractAddrs, addr)
		targetDeployTxs = append(targetDeployTxs, tx)
		nonce++
	}
	_, loaderDeployTx, _, err := loader.DeployLoader(&bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}, fromWallet.GetClient(), targetContractAddrs)
	if err != nil {
		return nil, fmt.Errorf("failed to create loader contract transaction: %w", err)
	}

	// we dont need that many contracts in memory.
	if uint64(len(f.contractAddresses)) < f.maxContracts {
		f.updateContractAddressesAsync(ctx, loaderDeployTx.Hash())
	}
	return append(targetDeployTxs, loaderDeployTx), nil
}

func (f *TxFactory) updateContractAddressesAsync(ctx context.Context, txHash common.Hash) {
	go func() {
		retires := 30
		delay := 500 * time.Millisecond
		client := f.wallets[0].GetClient()
		for i := 0; i < retires; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}
			receipt, err := ethwallet.GetTxReceipt(ctx, client, txHash)
			if err == nil {
				if receipt.Status != types.ReceiptStatusSuccessful {
					f.logger.Debug("unable to update contract address: tx failed", zap.String("tx_hash", txHash.String()))
				} else {
					f.logger.Debug("updating contract address for tx", zap.String("tx_hash", txHash.String()))
					f.mu.Lock()
					f.contractAddresses = append(f.contractAddresses, receipt.ContractAddress)
					f.mu.Unlock()
				}
				return
			}
			time.Sleep(delay)
		}
		f.logger.Debug("unable to update contract addresses for tx", zap.String("tx_hash", txHash.String()))
	}()
}

func (f *TxFactory) createMsgWriteTo(ctx context.Context, fromWallet *ethwallet.InteractingWallet, iterations int, nonce uint64) (*types.Transaction, error) {
	if iterations <= 0 {
		iterations = 3
	}
	if len(f.contractAddresses) == 0 {
		f.logger.Debug("no contract addresses for tx")
		return nil, nil
	}

	// Pick a random contract
	contractAddr := f.contractAddresses[rand.Intn(len(f.contractAddresses))]

	loaderInstance, err := loader.NewLoader(contractAddr, fromWallet.GetClient())
	if err != nil {
		return nil, fmt.Errorf("failed to get loader contract instance at %s: %w", contractAddr.String(), err)
	}
	tx, err := loaderInstance.TestStorageWrites(&bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}, big.NewInt(int64(iterations)))
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for writeTo function at %s: %w", contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgCallDataBlast(ctx context.Context, fromWallet *ethwallet.InteractingWallet, dataSize int, nonce uint64) (*types.Transaction, error) {
	if len(f.contractAddresses) == 0 {
		return nil, nil
	}
	// Pick a random contract
	contractAddr := f.contractAddresses[rand.Intn(len(f.contractAddresses))]

	if dataSize <= 0 {
		dataSize = 1024
	}
	randomBytes := make([]byte, dataSize)
	_, err := rand2.Read(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	loaderInstance, err := loader.NewLoader(contractAddr, fromWallet.GetClient())
	if err != nil {
		return nil, fmt.Errorf("failed to get loader contract instance at %s: %w", contractAddr.String(), err)
	}
	tx, err := loaderInstance.TestLargeCalldata(&bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}, randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for testLargeCallData function at %s: %w", contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgCrossContractCall(ctx context.Context, fromWallet *ethwallet.InteractingWallet, iterations int, nonce uint64) (*types.Transaction, error) {
	// Default to 10 iterations if not specified
	if iterations <= 0 {
		iterations = 10
	}
	if len(f.contractAddresses) == 0 {
		return nil, nil
	}

	// Pick a random contract (this should be a Loader contract)
	contractAddr := f.contractAddresses[rand.Intn(len(f.contractAddresses))]

	loaderInstance, err := loader.NewLoader(contractAddr, fromWallet.GetClient())
	if err != nil {
		return nil, fmt.Errorf("failed to get loader contract instance at %s: %w", contractAddr.String(), err)
	}
	tx, err := loaderInstance.TestCrossContractCalls(&bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}, big.NewInt(int64(iterations)))
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for writeTo function at %s: %w", contractAddr.String(), err)
	}
	return tx, nil
}
