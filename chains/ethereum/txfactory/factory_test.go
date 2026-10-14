package txfactory

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	loader "github.com/skip-mev/catalyst/chains/ethereum/contracts/load"
	iftbindings "github.com/skip-mev/catalyst/chains/ethereum/contracts/load/ift"
	"github.com/skip-mev/catalyst/chains/ethereum/contracts/load/target"
	ethift "github.com/skip-mev/catalyst/chains/ethereum/ift"
	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
	"github.com/skip-mev/catalyst/chains/txdistribution"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

const testIFTReceiver = "cosmos1receiver"

func TestApplyBaselinesToTxOpts(t *testing.T) {
	makeDynamicBaseline := func() *types.Transaction {
		to := common.Address{}
		return types.NewTx(&types.DynamicFeeTx{
			ChainID:   big.NewInt(1),
			Nonce:     0,
			GasTipCap: big.NewInt(2_000_000_000),  // 2 gwei
			GasFeeCap: big.NewInt(30_000_000_000), // 30 gwei
			Gas:       21000,
			To:        &to,
			Value:     big.NewInt(0),
		})
	}
	makeLegacyBaseline := func() *types.Transaction {
		to := common.Address{}
		return types.NewTx(&types.LegacyTx{
			Nonce:    0,
			GasPrice: big.NewInt(10_000_000_000), // 10 gwei
			Gas:      21000,
			To:       &to,
			Value:    big.NewInt(0),
		})
	}

	t.Run("fills all nil from dynamic baseline", func(t *testing.T) {
		baseline := makeDynamicBaseline()
		opts := &bind.TransactOpts{} // all nil/zero

		applyBaselinesToTxOpts(baseline, opts)

		require.Equal(t, opts.GasPrice, opts.GasPrice) // should be unchanged.
		require.Equal(t, baseline.GasTipCap(), opts.GasTipCap)
		require.Equal(t, baseline.GasFeeCap(), opts.GasFeeCap)
		require.Equal(t, baseline.Gas(), opts.GasLimit)
	})

	t.Run("preserves preset values and fills only missing", func(t *testing.T) {
		baseline := makeDynamicBaseline()
		presetGasPrice := big.NewInt(99)
		presetTipCap := big.NewInt(88)

		opts := &bind.TransactOpts{
			GasPrice:  new(big.Int).Set(presetGasPrice),
			GasTipCap: new(big.Int).Set(presetTipCap),
			// GasFeeCap nil -> should copy from baseline
			// GasLimit 0 -> should copy from baseline
		}

		applyBaselinesToTxOpts(baseline, opts)

		// preserved
		require.Equal(t, presetGasPrice, opts.GasPrice)
		require.Equal(t, presetTipCap, opts.GasTipCap)

		// filled from baseline
		require.Equal(t, baseline.GasFeeCap(), opts.GasFeeCap)
		require.Equal(t, baseline.Gas(), opts.GasLimit)
	})

	t.Run("legacy baseline mirrors legacy fields and leaves 1559 caps as in baseline (nil)", func(t *testing.T) {
		baseline := makeLegacyBaseline()
		opts := &bind.TransactOpts{} // all nil/zero

		applyBaselinesToTxOpts(baseline, opts)

		require.Equal(t, opts.GasPrice, opts.GasPrice)
		require.Equal(t, baseline.GasTipCap(), opts.GasTipCap)
		require.Equal(t, baseline.GasFeeCap(), opts.GasFeeCap)
		require.Equal(t, baseline.Gas(), opts.GasLimit)
	})

	t.Run("does not overwrite user-provided fee caps with legacy baseline", func(t *testing.T) {
		baseline := makeLegacyBaseline()
		userTip := big.NewInt(123)
		userCap := big.NewInt(456)

		opts := &bind.TransactOpts{
			GasTipCap: new(big.Int).Set(userTip),
			GasFeeCap: new(big.Int).Set(userCap),
		}

		applyBaselinesToTxOpts(baseline, opts)

		// user-provided values are preserved
		require.Equal(t, userTip, opts.GasTipCap)
		require.Equal(t, userCap, opts.GasFeeCap)
		// gas price gets filled from legacy baseline if nil
		require.Equal(t, opts.GasPrice, opts.GasPrice)
		require.Equal(t, baseline.Gas(), opts.GasLimit)
	})
}

func TestEstimateIFTGas(t *testing.T) {
	require.Equal(t, uint64(120), estimateIFTGas(100))
	require.Equal(t, uint64(6), estimateIFTGas(5))
	require.Equal(t, uint64(0), estimateIFTGas(0))
}

func TestCreateContract_SuccessfulTxs(t *testing.T) {
	// since the createContract involves some randomness, we do this test a few times.
	logger := zaptest.NewLogger(t)
	for range 10 {
		sim, wallet := setupTest(t)
		ctx := context.Background()
		distr := txdistribution.NewEven([]*ethwallet.InteractingWallet{wallet})
		f := NewTxFactory(logger, ethtypes.TxOpts{}, distr)
		nonce, err := wallet.GetNonce(ctx)
		require.NoError(t, err)
		txs, err := f.createMsgCreateContract(ctx, wallet, nil, nonce, false)
		require.NoError(t, err)

		for _, tx := range txs {
			err = wallet.SendTransaction(ctx, tx)
			require.NoError(t, err)
		}

		sim.Commit()

		for _, tx := range txs {
			receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
			require.NoError(t, err)
			require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)
		}
	}
}

func TestCreateMsgWriteTo(t *testing.T) {
	logger := zaptest.NewLogger(t)

	sim, wallet := setupTest(t)
	ctx := context.Background()
	distr := txdistribution.NewEven([]*ethwallet.InteractingWallet{wallet})
	f := NewTxFactory(logger, ethtypes.TxOpts{}, distr)
	deployContract(t, sim, f, distr)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgWriteTo(ctx, wallet, 100, nonce, false)
	require.NoError(t, err)
	err = wallet.SendTransaction(ctx, tx)
	require.NoError(t, err)

	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)

	loader, err := loader.NewLoader(f.loaderAddresses[0], wallet.GetClient())
	require.NoError(t, err)
	slot5, err := loader.Storage1(&bind.CallOpts{}, big.NewInt(5))
	require.NoError(t, err)
	// the storage just stores i * 2.
	require.Equal(t, slot5.Int64(), int64(10))
}

func TestCallDataBlast(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sim, wallet := setupTest(t)
	ctx := context.Background()
	distr := txdistribution.NewEven([]*ethwallet.InteractingWallet{wallet})
	f := NewTxFactory(logger, ethtypes.TxOpts{}, distr)
	deployContract(t, sim, f, distr)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgCallDataBlast(ctx, wallet, 1024, nonce, false)
	require.NoError(t, err)
	err = wallet.SendTransaction(ctx, tx)
	require.NoError(t, err)
	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)
}

func TestCrossContractCall(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sim, wallet := setupTest(t)
	ctx := context.Background()
	distr := txdistribution.NewEven([]*ethwallet.InteractingWallet{wallet})
	f := NewTxFactory(logger, ethtypes.TxOpts{}, distr)
	deployContract(t, sim, f, distr)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgCrossContractCall(ctx, wallet, 15, nonce, false)
	require.NoError(t, err)
	err = wallet.SendTransaction(ctx, tx)
	require.NoError(t, err)
	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)

	loader, err := loader.NewLoader(f.loaderAddresses[0], wallet.GetClient())
	require.NoError(t, err)
	addr, err := loader.Targets(&bind.CallOpts{}, big.NewInt(0))
	require.NoError(t, err)

	targ, err := target.NewTarget(addr, wallet.GetClient())
	require.NoError(t, err)
	value, err := targ.Data(&bind.CallOpts{}, big.NewInt(1))
	require.NoError(t, err)
	// target stores values of loop_index * 2.
	require.Equal(t, value.Int64(), int64(2))
}

func TestCreateMsgIFTTransfer_Gas(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sim, wallet := setupTest(t)
	ctx := context.Background()

	auth := &bind.TransactOpts{
		From:    wallet.Address(),
		Signer:  wallet.SignerFnLegacy(),
		Context: ctx,
	}
	addr, deployTx, _, err := iftbindings.DeployIft(auth, sim.Client())
	require.NoError(t, err)
	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, deployTx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)

	contract, err := ethift.NewTransferContract(addr.Hex())
	require.NoError(t, err)

	distr := txdistribution.NewEven([]*ethwallet.InteractingWallet{wallet})
	f := NewTxFactory(logger, ethtypes.TxOpts{}, distr)
	f.SetIFTConfig(contract, []string{testIFTReceiver}, "client-0", big.NewInt(1), time.Hour)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgIFTTransfer(ctx, wallet, nonce, false)
	require.NoError(t, err)
	require.Positive(t, tx.Gas())
	price := tx.GasPrice()
	if feeCap := tx.GasFeeCap(); feeCap != nil && feeCap.Sign() > 0 {
		price = feeCap
	}
	require.Positive(t, price.Sign())

	require.NoError(t, wallet.SendTransaction(ctx, tx))
	sim.Commit()
	receipt, err = sim.Client().TransactionReceipt(ctx, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
}

func TestIFTGasOncePerBlock(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sim, wallet := setupTest(t)
	ctx := context.Background()

	auth := &bind.TransactOpts{
		From:    wallet.Address(),
		Signer:  wallet.SignerFnLegacy(),
		Context: ctx,
	}
	addr, deployTx, _, err := iftbindings.DeployIft(auth, sim.Client())
	require.NoError(t, err)
	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, deployTx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)

	contract, err := ethift.NewTransferContract(addr.Hex())
	require.NoError(t, err)

	distr := txdistribution.NewEven([]*ethwallet.InteractingWallet{wallet})
	f := NewTxFactory(logger, ethtypes.TxOpts{}, distr)
	f.SetIFTConfig(contract, []string{testIFTReceiver}, "client-0", big.NewInt(1), time.Hour)

	err = f.SetBaselines(ctx, []loadtesttypes.LoadTestMsg{{Type: ethtypes.MsgIFTTransfer, NumMsgs: 1}})
	require.NoError(t, err)
	require.Positive(t, f.iftGasLimit)

	sample := f.baseLines[ethtypes.MsgIFTTransfer][0].Gas()
	require.Equal(t, estimateIFTGas(sample), f.iftGasLimit)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx1, err := f.createMsgIFTTransfer(ctx, wallet, nonce, true)
	require.NoError(t, err)
	tx2, err := f.createMsgIFTTransfer(ctx, wallet, nonce+1, true)
	require.NoError(t, err)

	require.Equal(t, f.iftGasLimit, tx1.Gas())
	require.Equal(t, f.iftGasLimit, tx2.Gas())
	require.Equal(t, tx1.Gas(), tx2.Gas())

	// Prove later builds read the stored block constant, not a fresh estimate*1.20.
	f.iftGasLimit = 99_999
	tx3, err := f.createMsgIFTTransfer(ctx, wallet, nonce+2, true)
	require.NoError(t, err)
	require.Equal(t, uint64(99_999), tx3.Gas())
}

func TestIFTGaslessBaseline(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sim, wallet := setupTest(t)
	ctx := context.Background()

	auth := &bind.TransactOpts{
		From:    wallet.Address(),
		Signer:  wallet.SignerFnLegacy(),
		Context: ctx,
	}
	addr, deployTx, _, err := iftbindings.DeployIft(auth, sim.Client())
	require.NoError(t, err)
	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, deployTx.Hash())
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)

	contract, err := ethift.NewTransferContract(addr.Hex())
	require.NoError(t, err)

	distr := txdistribution.NewEven([]*ethwallet.InteractingWallet{wallet})
	f := NewTxFactory(logger, ethtypes.TxOpts{
		GasTipCap: big.NewInt(1_000_000_000),
		GasFeeCap: big.NewInt(30_000_000_000),
	}, distr)
	f.SetIFTConfig(contract, []string{testIFTReceiver}, "client-0", big.NewInt(1), time.Hour)

	// A 0-gas sample (gasless chain) becomes iftGasLimit 0 via estimateIFTGas.
	to := contract.Address()
	zeroSample := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		GasTipCap: big.NewInt(1_000_000_000),
		GasFeeCap: big.NewInt(30_000_000_000),
		Gas:       0,
		To:        &to,
		Value:     big.NewInt(0),
	})
	f.baseLines[ethtypes.MsgIFTTransfer] = []*types.Transaction{zeroSample}
	f.iftGasLimit = estimateIFTGas(zeroSample.Gas())
	require.Equal(t, uint64(0), f.iftGasLimit)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgIFTTransfer(ctx, wallet, nonce, true)
	require.NoError(t, err)
	require.Equal(t, uint64(0), tx.Gas())
}

func TestApplyBaselinesToTxOpts_NoIFTMargin(t *testing.T) {
	to := common.Address{}
	baseline := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		GasTipCap: big.NewInt(2_000_000_000),
		GasFeeCap: big.NewInt(30_000_000_000),
		Gas:       100_000,
		To:        &to,
		Value:     big.NewInt(0),
	})
	opts := &bind.TransactOpts{}
	applyBaselinesToTxOpts(baseline, opts)
	require.Equal(t, uint64(100_000), opts.GasLimit)
	require.NotEqual(t, estimateIFTGas(100_000), opts.GasLimit)
}

func deployContract(t *testing.T, sim *simulated.Backend, f *TxFactory, distr TxDistribution) {
	t.Helper()
	ctx := context.Background()
	numContracts := 1
	wallet := distr.GetWallet(0)
	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	txs, err := f.createMsgCreateContract(ctx, wallet, &numContracts, nonce, false)
	require.NoError(t, err)
	for _, tx := range txs {
		err = wallet.SendTransaction(ctx, tx)
		require.NoError(t, err)
	}
	sim.Commit()
	for i, tx := range txs {
		receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
		require.NoError(t, err)
		require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)
		if i == len(txs)-1 {
			f.SetLoaderAddresses(receipt.ContractAddress)
		}
	}
}

func setupTest(t *testing.T) (*simulated.Backend, *ethwallet.InteractingWallet) {
	t.Helper()
	genesisBalance := big.NewInt(1_000_000_000_000_000_000)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	alloc := types.GenesisAlloc{
		addr: {Balance: genesisBalance},
	}
	sim := setupSimulatedBackend(alloc)

	ctx := context.Background()
	id, err := sim.Client().ChainID(ctx)
	require.NoError(t, err)

	wallet := ethwallet.NewInteractingWallet(key, id, sim.Client())
	return sim, wallet
}

func setupSimulatedBackend(alloc types.GenesisAlloc) *simulated.Backend {
	backend := simulated.NewBackend(alloc)
	return backend
}
