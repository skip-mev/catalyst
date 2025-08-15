package txfactory

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	loader "github.com/skip-mev/catalyst/chains/ethereum/contracts/load"
	"github.com/skip-mev/catalyst/chains/ethereum/contracts/load/target"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCreateContract_SuccessfulTxs(t *testing.T) {
	// since the createContract involves some randomness, we do this test a few times.
	logger := zaptest.NewLogger(t)
	for range 10 {
		sim, wallet := setupTest(t)
		ctx := context.Background()
		f := NewTxFactory(logger, []*ethwallet.InteractingWallet{wallet}, 0)
		nonce, err := wallet.GetNonce(ctx)
		require.NoError(t, err)
		txs, err := f.createMsgCreateContract(ctx, wallet, nil, nonce)
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
	f := NewTxFactory(logger, []*ethwallet.InteractingWallet{wallet}, 0)
	deployContract(t, sim, f)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgWriteTo(ctx, wallet, 100, nonce)
	require.NoError(t, err)
	err = wallet.SendTransaction(ctx, tx)
	require.NoError(t, err)

	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)

	loader, err := loader.NewLoader(f.contractAddresses[0], wallet.GetClient())
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
	f := NewTxFactory(logger, []*ethwallet.InteractingWallet{wallet}, 0)
	deployContract(t, sim, f)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgCallDataBlast(ctx, wallet, 1024, nonce)
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
	f := NewTxFactory(logger, []*ethwallet.InteractingWallet{wallet}, 0)
	deployContract(t, sim, f)

	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	tx, err := f.createMsgCrossContractCall(ctx, wallet, 15, nonce)
	require.NoError(t, err)
	err = wallet.SendTransaction(ctx, tx)
	require.NoError(t, err)
	sim.Commit()
	receipt, err := sim.Client().TransactionReceipt(ctx, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)

	loader, err := loader.NewLoader(f.contractAddresses[0], wallet.GetClient())
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

func deployContract(t *testing.T, sim *simulated.Backend, f *TxFactory) {
	ctx := context.Background()
	numContracts := 1
	wallet := f.wallets[0]
	nonce, err := wallet.GetNonce(ctx)
	require.NoError(t, err)
	txs, err := f.createMsgCreateContract(ctx, wallet, &numContracts, nonce)
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
			f.SetContractAddrs(receipt.ContractAddress)
		}
	}
}

func setupTest(t *testing.T) (*simulated.Backend, *ethwallet.InteractingWallet) {
	genesisBalance := big.NewInt(12000000000000000)
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
