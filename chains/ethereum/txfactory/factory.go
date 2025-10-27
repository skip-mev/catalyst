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
	"github.com/skip-mev/catalyst/chains/ethereum/contracts/load/weth"
	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

type TxFactory struct {
	logger          *zap.Logger
	wallets         []*ethwallet.InteractingWallet
	loaderAddresses []common.Address
	wethAddresses   []common.Address
	mu              sync.Mutex
	txOpts          ethtypes.TxOpts

	// nativeERC20PrecompileAddress is the native precompile address of the EVMD chain.
	// see: EVMD genesis.
	nativeERC20PrecompileAddress common.Address

	// baseLines are baseline transactions for a given message type. This is useful for upfront load building.
	// Instead of using the client to make gas estimations over and over again for the same tx, we can just re-use these initial values.
	// Be sure to call txFactory.SetBaseLines before using this.
	baseLines map[loadtesttypes.MsgType][]*types.Transaction

	// Wallet allocation tracking for minimizing reuse with role rotation
	senderIndex    int
	receiverIndex  int
	loadGeneration int // tracks which load we're on for role rotation
	numWallets     int // cached number of wallets for pool calculations
}

func NewTxFactory(logger *zap.Logger, wallets []*ethwallet.InteractingWallet, txOpts ethtypes.TxOpts) *TxFactory {
	return &TxFactory{
		logger:                       logger.With(zap.String("module", "tx_factory")),
		wallets:                      wallets,
		mu:                           sync.Mutex{},
		txOpts:                       txOpts,
		baseLines:                    map[loadtesttypes.MsgType][]*types.Transaction{},
		nativeERC20PrecompileAddress: common.HexToAddress("0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"), // see: evmd
		loaderAddresses:              []common.Address{},
		wethAddresses:                []common.Address{},
		senderIndex:                  0,
		receiverIndex:                0,
		loadGeneration:               0,
		numWallets:                   len(wallets),
	}
}

// SetBaselines sets the baseline transaction for each message type.
// This is useful for transactions that do not want to use the client to get gas values.
func (f *TxFactory) SetBaselines(ctx context.Context, msgs []loadtesttypes.LoadTestMsg) error {
	f.logger.Info("Setting baselines for transactions...")
	for _, msg := range msgs {
		wallet := f.wallets[rand.Intn(len(f.wallets))]
		nonce, err := wallet.GetNonce(ctx)
		if err != nil {
			return fmt.Errorf("failed to get nonce of %s: %w", wallet.FormattedAddress(), err)
		}
		spec := loadtesttypes.LoadTestMsg{
			Type:    msg.Type,
			NumMsgs: 1,
		}
		txs, err := f.BuildTxs(spec, wallet, nonce, false)
		if err != nil {
			return fmt.Errorf("failed to build txs: %w", err)
		}
		f.baseLines[msg.Type] = txs
	}
	return nil
}

// applyBaselinesToTxOpts applies baseline transaction values to transact options, while respecting
// the static gas values set by the user in the spec.
func applyBaselinesToTxOpts(baselineTx *types.Transaction, txOpts *bind.TransactOpts) {
	if txOpts.GasTipCap == nil {
		txOpts.GasTipCap = baselineTx.GasTipCap()
	}
	if txOpts.GasFeeCap == nil {
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
	case ethtypes.MsgDeployERC20:
		tx, err := f.createMsgDeployERC20(ctx, fromWallet, nonce, useBaseline)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	case ethtypes.MsgTransferERC0:
		tx, err := f.createMsgTransferERC20(ctx, fromWallet, nonce, useBaseline)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	case ethtypes.MsgNativeTransferERC20:
		tx, err := f.createMsgNativeTransferERC20(ctx, fromWallet, nonce, useBaseline)
		if err != nil {
			return nil, err
		}
		return []*types.Transaction{tx}, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %q", msgSpec.Type)
	}
}

func (f *TxFactory) SetLoaderAddresses(addrs ...common.Address) {
	f.loaderAddresses = append(f.loaderAddresses, addrs...)
}

func (f *TxFactory) SetWETHAddresses(addrs ...common.Address) {
	f.wethAddresses = append(f.wethAddresses, addrs...)
}

// ResetWalletAllocation resets wallet allocation for a new load and rotates roles
func (f *TxFactory) ResetWalletAllocation() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.senderIndex = 0
	f.receiverIndex = 0
	f.loadGeneration++
}

// getCurrentSenderPool returns the current sender pool with role rotation
func (f *TxFactory) getCurrentSenderPool() []*ethwallet.InteractingWallet {
	if f.numWallets < 2 {
		return f.wallets
	}

	// Rotate roles: wallets that were receivers in previous loads become senders
	// This ensures balance distribution over time
	offset := (f.loadGeneration * (f.numWallets / 2)) % f.numWallets
	midpoint := f.numWallets / 2

	senderPool := make([]*ethwallet.InteractingWallet, 0, midpoint)
	for i := 0; i < midpoint; i++ {
		idx := (offset + i) % f.numWallets
		senderPool = append(senderPool, f.wallets[idx])
	}

	return senderPool
}

// getCurrentReceiverPool returns the current receiver pool with role rotation
func (f *TxFactory) getCurrentReceiverPool() []*ethwallet.InteractingWallet {
	if f.numWallets < 2 {
		return f.wallets
	}

	// Receivers are the complement of senders in this load
	offset := (f.loadGeneration * (f.numWallets / 2)) % f.numWallets
	midpoint := f.numWallets / 2
	receiverOffset := (offset + midpoint) % f.numWallets

	receiverPool := make([]*ethwallet.InteractingWallet, 0, f.numWallets-midpoint)
	for i := 0; i < f.numWallets-midpoint; i++ {
		idx := (receiverOffset + i) % f.numWallets
		receiverPool = append(receiverPool, f.wallets[idx])
	}

	return receiverPool
}

// GetNextSender returns the next sender wallet using round-robin within the current load
func (f *TxFactory) GetNextSender() *ethwallet.InteractingWallet {
	f.mu.Lock()
	defer f.mu.Unlock()

	senderPool := f.getCurrentSenderPool()
	sender := senderPool[f.senderIndex]
	f.senderIndex = (f.senderIndex + 1) % len(senderPool)

	return sender
}

// getNextReceiver returns the next receiver wallet using round-robin within the current load
func (f *TxFactory) getNextReceiver() common.Address {
	f.mu.Lock()
	defer f.mu.Unlock()

	receiverPool := f.getCurrentReceiverPool()
	receiver := receiverPool[f.receiverIndex]
	f.receiverIndex = (f.receiverIndex + 1) % len(receiverPool)

	return receiver.Address()
}

func (f *TxFactory) createMsgCreateContract(ctx context.Context, fromWallet *ethwallet.InteractingWallet, targets *int, nonce uint64, useBaseline bool) ([]*types.Transaction, error) {
	var numTargets int
	if targets != nil {
		numTargets = *targets
	} else {
		// default. simple
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
		iterations = 1
	}
	if len(f.loaderAddresses) == 0 {
		f.logger.Debug("no contract addresses for tx")
		return nil, nil
	}

	// Pick a random contract
	contractAddr := f.loaderAddresses[rand.Intn(len(f.loaderAddresses))]

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
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgWriteTo][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := loaderInstance.TestStorageWrites(txOpts, big.NewInt(int64(iterations)))
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %s at %s: %w", ethtypes.MsgWriteTo.String(), contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgCallDataBlast(ctx context.Context, fromWallet *ethwallet.InteractingWallet, dataSize int, nonce uint64, useBaseline bool) (*types.Transaction, error) {
	if len(f.loaderAddresses) == 0 {
		return nil, nil
	}
	// Pick a random contract
	contractAddr := f.loaderAddresses[rand.Intn(len(f.loaderAddresses))]

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
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgCallDataBlast][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := loaderInstance.TestLargeCalldata(txOpts, randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %s at %s: %w", ethtypes.MsgCallDataBlast.String(), contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgCrossContractCall(ctx context.Context, fromWallet *ethwallet.InteractingWallet, iterations int, nonce uint64, useBaseline bool) (*types.Transaction, error) {
	// Default to 10 iterations if not specified
	if iterations <= 0 {
		iterations = 10
	}
	if len(f.loaderAddresses) == 0 {
		return nil, nil
	}

	// Pick a random contract (this should be a Loader contract)
	contractAddr := f.loaderAddresses[rand.Intn(len(f.loaderAddresses))]

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
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgCrossContractCall][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := loaderInstance.TestCrossContractCalls(txOpts, big.NewInt(int64(iterations)))
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %s at %s: %w", ethtypes.MsgCrossContractCall.String(), contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgDeployERC20(ctx context.Context, fromWallet *ethwallet.InteractingWallet, nonce uint64, useBaseline bool) (*types.Transaction, error) {
	txOpts := &bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baselineTx := f.baseLines[ethtypes.MsgDeployERC20][0]
		applyBaselinesToTxOpts(baselineTx, txOpts)
	}
	txOpts.Nonce = big.NewInt(int64(nonce)) //nolint:gosec // G115: overflow unlikely in practice
	_, loaderDeployTx, _, err := weth.DeployWeth(txOpts, fromWallet.GetClient())
	if err != nil {
		return nil, fmt.Errorf("failed to create loader contract transaction: %w", err)
	}
	return loaderDeployTx, nil
}

func (f *TxFactory) createMsgTransferERC20(ctx context.Context, fromWallet *ethwallet.InteractingWallet, nonce uint64, useBaseline bool) (*types.Transaction, error) {
	// Pick a random contract
	contractAddr := f.wethAddresses[rand.Intn(len(f.wethAddresses))]

	// Use optimal recipient selection to minimize reuse and prevent self-transfers
	recipient := f.getNextReceiver()
	// Ensure sender != receiver if we have more than one wallet
	for recipient == fromWallet.Address() && f.numWallets > 1 {
		recipient = f.getNextReceiver()
	}

	// random amount. weth calls amounts wad for some reason.
	wad := big.NewInt(int64(rand.Intn(10_000)))

	wethInstance, err := weth.NewWethTransactor(contractAddr, fromWallet.GetClient())
	if err != nil {
		return nil, fmt.Errorf("failed to get loader contract instance at %s: %w", contractAddr.String(), err)
	}
	txOpts := &bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgTransferERC0][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := wethInstance.Transfer(txOpts, recipient, wad)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %s at %s: %w", ethtypes.MsgTransferERC0.String(), contractAddr.String(), err)
	}
	return tx, nil
}

func (f *TxFactory) createMsgNativeTransferERC20(ctx context.Context, fromWallet *ethwallet.InteractingWallet, nonce uint64, useBaseline bool) (*types.Transaction, error) {
	// Use optimal recipient selection to minimize reuse and prevent self-transfers
	recipient := f.getNextReceiver()
	// Ensure sender != receiver if we have more than one wallet
	for recipient == fromWallet.Address() && f.numWallets > 1 {
		recipient = f.getNextReceiver()
	}

	// random amount. weth calls amounts wad for some reason. we continue that trend here.
	wad := big.NewInt(int64(rand.Intn(10_000)))

	// we use the weth transactor even though were interacting with the native precompile since they share the same interface,
	// and the call data constructed here will be the same.
	wethInstance, err := weth.NewWethTransactor(f.nativeERC20PrecompileAddress, fromWallet.GetClient())
	if err != nil {
		return nil, fmt.Errorf("failed to get erc20 contract instance at %s: %w", f.nativeERC20PrecompileAddress.String(), err)
	}
	txOpts := &bind.TransactOpts{
		From:      fromWallet.Address(),
		Signer:    fromWallet.SignerFnLegacy(),
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: f.txOpts.GasTipCap,
		GasFeeCap: f.txOpts.GasFeeCap,
		Context:   ctx,
		NoSend:    true,
	}
	if useBaseline {
		baseLineTx := f.baseLines[ethtypes.MsgNativeTransferERC20][0]
		applyBaselinesToTxOpts(baseLineTx, txOpts)
	}
	tx, err := wethInstance.Transfer(txOpts, recipient, wad)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %s at %s: %w", ethtypes.MsgNativeTransferERC20.String(), f.nativeERC20PrecompileAddress.String(), err)
	}
	return tx, nil
}
