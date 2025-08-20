package txfactory

import (
	"context"
	rand2 "crypto/rand"
	"fmt"
	"math/big"
	"math/rand"
	"sync"

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
	mu                sync.Mutex
	txOpts            ethtypes.TxOpts

	// baseLines are baseline transactions for a given message type. This is useful for upfront load building.
	// Instead of using the client to make gas estimations over and over again for the same tx, we can just re-use these initial values.
	// Be sure to call txFactory.SetBaseLines before using this.
	baseLines map[loadtesttypes.MsgType][]*types.Transaction
}

func NewTxFactory(logger *zap.Logger, wallets []*ethwallet.InteractingWallet, txOpts ethtypes.TxOpts) *TxFactory {
	return &TxFactory{logger: logger.With(zap.String("module", "tx_factory")), wallets: wallets, mu: sync.Mutex{}, txOpts: txOpts, baseLines: map[loadtesttypes.MsgType][]*types.Transaction{}}
}

// SetBaselines sets the baseline transaction for each message type.
func (f *TxFactory) SetBaselines(ctx context.Context) error {
	f.logger.Info("Setting baselines for transactions...")
	for _, msg := range ethtypes.ValidMessages {
		wallet := f.wallets[rand.Intn(len(f.wallets))]
		nonce, err := wallet.GetNonce(ctx)
		if err != nil {
			return fmt.Errorf("failed to get nonce of %s: %w", wallet.FormattedAddress(), err)
		}
		spec := loadtesttypes.LoadTestMsg{
			Type:    msg,
			NumMsgs: 1,
		}
		txs, err := f.BuildTxs(spec, wallet, nonce, false)
		if err != nil {
			return fmt.Errorf("failed to build txs: %w", err)
		}
		f.baseLines[msg] = txs
	}
	return nil
}

// applyBaselinesToTxOpts applies baseline transaction values to transact options, while respecting
// the static gas values set by the user in the spec..
func applyBaselinesToTxOpts(baselineTx *types.Transaction, txOpts *bind.TransactOpts) {
	if txOpts.GasPrice != nil {
		txOpts.GasPrice = baselineTx.GasPrice()
	}
	if txOpts.GasTipCap != nil {
		txOpts.GasTipCap = baselineTx.GasTipCap()
	}
	if txOpts.GasFeeCap != nil {
		txOpts.GasFeeCap = baselineTx.GasFeeCap()
	}
	txOpts.GasLimit = baselineTx.Gas()
}

// BuildTxs builds the transactions for the message.
// NOTE: This function does NOT return NumMsgs amount of transactions. This logic is delegated one level above, in runner,
// to allow for easier nonce/wallet management.
// UseBaseline can be specified if you want the tx to use the baseline gas values instead of estimating via network client.
// This is useful for the runOnInterval loadtest, which builds all of its load up front. For heavy loads,
// building with baselines speeds up load building by an extremely large order of magnitude.
func (f *TxFactory) BuildTxs(msgSpec loadtesttypes.LoadTestMsg, fromWallet *ethwallet.InteractingWallet, nonce uint64, useBaseline bool) ([]*types.Transaction, error) {
	ctx := context.Background()
	switch msgSpec.Type {
	case ethtypes.MsgCreateContract:
		return f.createMsgCreateContract(ctx, fromWallet, nil, nonce, useBaseline)
	case ethtypes.MsgWriteTo:
		tx, err := f.createMsgWriteTo(ctx, fromWallet, msgSpec.NumOfIterations, nonce, useBaseline)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	case ethtypes.MsgCallDataBlast:
		tx, err := f.createMsgCallDataBlast(ctx, fromWallet, msgSpec.CalldataSize, nonce, useBaseline)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	case ethtypes.MsgCrossContractCall:
		tx, err := f.createMsgCrossContractCall(ctx, fromWallet, msgSpec.NumOfIterations, nonce, useBaseline)
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

func (f *TxFactory) createMsgCreateContract(ctx context.Context, fromWallet *ethwallet.InteractingWallet, targets *int, nonce uint64, useBaseline bool) ([]*types.Transaction, error) {
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
	txOpts := &bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		// CreateContract creates n>2 contracts. The first n contracts are embedded, the last is the loader.
		baseLineTx := f.baseLines[ethtypes.MsgCreateContract][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}

	for range numTargets {
		txOpts.Nonce = big.NewInt(int64(nonce)) //nolint:gosec // G115: overflow unlikely in practice
		addr, tx, _, err := target.DeployTarget(txOpts, fromWallet.GetClient())
		if err != nil {
			return nil, fmt.Errorf("failed to create target contract transaction: %w", err)
		}
		targetContractAddrs = append(targetContractAddrs, addr)
		targetDeployTxs = append(targetDeployTxs, tx)
		nonce++
	}

	if useBaseline {
		txs := f.baseLines[ethtypes.MsgCreateContract]
		baselineTx := f.baseLines[ethtypes.MsgCreateContract][len(txs)-1]
		applyBaselinesToTxOpts(baselineTx, txOpts)
	}
	txOpts.Nonce = big.NewInt(int64(nonce)) //nolint:gosec // G115: overflow unlikely in practice
	_, loaderDeployTx, _, err := loader.DeployLoader(txOpts, fromWallet.GetClient(), targetContractAddrs)
	if err != nil {
		return nil, fmt.Errorf("failed to create loader contract transaction: %w", err)
	}

	return append(targetDeployTxs, loaderDeployTx), nil
}

func (f *TxFactory) createMsgWriteTo(ctx context.Context, fromWallet *ethwallet.InteractingWallet, iterations int, nonce uint64, useBaseline bool) (*types.Transaction, error) {
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
	txOpts := &bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgWriteTo][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := loaderInstance.TestStorageWrites(txOpts, big.NewInt(int64(iterations)))
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for writeTo function at %s: %w", contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgCallDataBlast(ctx context.Context, fromWallet *ethwallet.InteractingWallet, dataSize int, nonce uint64, useBaseline bool) (*types.Transaction, error) {
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
	txOpts := &bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgCallDataBlast][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := loaderInstance.TestLargeCalldata(txOpts, randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for testLargeCallData function at %s: %w", contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgCrossContractCall(ctx context.Context, fromWallet *ethwallet.InteractingWallet, iterations int, nonce uint64, useBaseline bool) (*types.Transaction, error) {
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
	txOpts := &bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		GasPrice:  f.txOpts.GasPrice,
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgCrossContractCall][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := loaderInstance.TestCrossContractCalls(txOpts, big.NewInt(int64(iterations)))
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for writeTo function at %s: %w", contractAddr.String(), err)
	}
	return tx, nil
}
