package wallet

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/rpc"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// simulatedClientWrapper wraps a simulated.Client to add the Client() method
// We use a named field instead of embedding to avoid naming conflicts
type simulatedClientWrapper struct {
	sc simulated.Client
}

// Client implements the wallet.Client interface
func (w *simulatedClientWrapper) Client() *rpc.Client {
	// The simulated backend doesn't expose the underlying RPC client
	// For testing purposes, we can return nil since batch operations
	// aren't used in these tests
	return nil
}

// Forward the necessary methods from simulated.Client
func (w *simulatedClientWrapper) BlockNumber(ctx context.Context) (uint64, error) {
	return w.sc.BlockNumber(ctx)
}

func (w *simulatedClientWrapper) ChainID(ctx context.Context) (*big.Int, error) {
	return w.sc.ChainID(ctx)
}

func (w *simulatedClientWrapper) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return w.sc.EstimateGas(ctx, msg)
}

func (w *simulatedClientWrapper) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return w.sc.SuggestGasPrice(ctx)
}

func (w *simulatedClientWrapper) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return w.sc.SuggestGasTipCap(ctx)
}

func (w *simulatedClientWrapper) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	return w.sc.BalanceAt(ctx, account, blockNumber)
}

func (w *simulatedClientWrapper) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return w.sc.PendingNonceAt(ctx, account)
}

func (w *simulatedClientWrapper) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return w.sc.SendTransaction(ctx, tx)
}

func (w *simulatedClientWrapper) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	return w.sc.HeaderByNumber(ctx, number)
}

func (w *simulatedClientWrapper) TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	return w.sc.TransactionByHash(ctx, hash)
}

func (w *simulatedClientWrapper) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return w.sc.TransactionReceipt(ctx, txHash)
}

func (w *simulatedClientWrapper) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return w.sc.BlockByHash(ctx, hash)
}

func (w *simulatedClientWrapper) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return w.sc.BlockByNumber(ctx, number)
}

func (w *simulatedClientWrapper) CodeAt(ctx context.Context, account common.Address, blockNumber *big.Int) ([]byte, error) {
	return w.sc.CodeAt(ctx, account, blockNumber)
}

func (w *simulatedClientWrapper) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	return w.sc.CallContract(ctx, msg, blockNumber)
}

func (w *simulatedClientWrapper) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return w.sc.HeaderByHash(ctx, hash)
}

func (w *simulatedClientWrapper) TransactionSender(ctx context.Context, tx *types.Transaction, block common.Hash, index uint) (common.Address, error) {
	// Simulated backend doesn't implement this directly, return empty address
	return common.Address{}, ethereum.NotFound
}

func (w *simulatedClientWrapper) TransactionCount(ctx context.Context, blockHash common.Hash) (uint, error) {
	// Simulated backend doesn't implement this directly
	return 0, ethereum.NotFound
}

func (w *simulatedClientWrapper) TransactionInBlock(ctx context.Context, blockHash common.Hash, index uint) (*types.Transaction, error) {
	// Simulated backend doesn't implement this directly
	return nil, ethereum.NotFound
}

func (w *simulatedClientWrapper) PendingBalanceAt(ctx context.Context, account common.Address) (*big.Int, error) {
	return w.sc.PendingBalanceAt(ctx, account)
}

func (w *simulatedClientWrapper) PendingStorageAt(ctx context.Context, account common.Address, key common.Hash) ([]byte, error) {
	return w.sc.PendingStorageAt(ctx, account, key)
}

func (w *simulatedClientWrapper) PendingCodeAt(ctx context.Context, account common.Address) ([]byte, error) {
	return w.sc.PendingCodeAt(ctx, account)
}

func (w *simulatedClientWrapper) PendingTransactionCount(ctx context.Context) (uint, error) {
	return w.sc.PendingTransactionCount(ctx)
}

func (w *simulatedClientWrapper) PendingCallContract(ctx context.Context, msg ethereum.CallMsg) ([]byte, error) {
	return w.sc.PendingCallContract(ctx, msg)
}

func (w *simulatedClientWrapper) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	return w.sc.FilterLogs(ctx, q)
}

func (w *simulatedClientWrapper) SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error) {
	return w.sc.SubscribeFilterLogs(ctx, q, ch)
}

func (w *simulatedClientWrapper) FeeHistory(ctx context.Context, blockCount uint64, lastBlock *big.Int, rewardPercentiles []float64) (*ethereum.FeeHistory, error) {
	return w.sc.FeeHistory(ctx, blockCount, lastBlock, rewardPercentiles)
}

func (w *simulatedClientWrapper) StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error) {
	return w.sc.StorageAt(ctx, account, key, blockNumber)
}

func (w *simulatedClientWrapper) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	return w.sc.NonceAt(ctx, account, blockNumber)
}

func (w *simulatedClientWrapper) SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error) {
	return w.sc.SubscribeNewHead(ctx, ch)
}

func setupSimulatedBackend(alloc types.GenesisAlloc) *simulated.Backend {
	backend := simulated.NewBackend(alloc)
	return backend
}

func TestBuildWallets(t *testing.T) {
	baseMnemonic := "copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

	expectedAddrs := []string{
		"0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101",
		"0x9A271A6A9C60936f2E6E32e460Fda5d2C92e4368",
		"0xBf2B5547C06662DdEB4b1D43542fb65BFd4e6330",
		"0x6dE3b4C24cef32bEA6D0dCa96727A5BDAEa56D22",
	}
	spec := loadtesttypes.LoadTestSpec{
		BaseMnemonic: baseMnemonic,
		NumWallets:   4,
		ChainID:      "262144",
	}

	client := &ethclient.Client{}

	logger := zaptest.NewLogger(t)
	wallets, err := NewWalletsFromSpec(logger, spec, []*ethclient.Client{client})
	require.NoError(t, err)

	require.Len(t, wallets, spec.NumWallets)
	for i, wallet := range wallets {
		require.Equal(t, expectedAddrs[i], wallet.Address().String())
	}
}

func TestTransaction(t *testing.T) {
	genesisBalance := big.NewInt(12000000000000000)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	alloc := types.GenesisAlloc{
		addr: {Balance: genesisBalance},
	}
	sim := setupSimulatedBackend(alloc)

	ctx := context.Background()
	client := &simulatedClientWrapper{sc: sim.Client()}
	id, err := client.ChainID(ctx)
	require.NoError(t, err)

	wallet := NewInteractingWallet(key, id, client)

	addr2 := getRandomAddr(t)
	nonce := uint64(0)
	tx, err := wallet.CreateSignedTransaction(ctx, &addr2, nil, 21000, nil, nil, &nonce)
	require.NoError(t, err)
	err = wallet.SendTransaction(ctx, tx)
	require.NoError(t, err)

	sim.Commit()
	block, err := sim.Client().BlockByNumber(ctx, big.NewInt(1))
	require.NoError(t, err)

	txInBlockHash := block.Transactions()[0].Hash()
	sentTxHash := tx.Hash()

	require.Zero(t, sentTxHash.Cmp(txInBlockHash))

	receipt, err := GetTxReceipt(ctx, wallet.GetClient(), tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)

	gotTx, isPending, err := GetTxByHash(ctx, client, receipt.TxHash)
	require.NoError(t, err)
	require.False(t, isPending)
	require.Equal(t, gotTx.Hash(), tx.Hash())
}

func TestCreateSignedTransaction(t *testing.T) {
	genesisBalance := big.NewInt(12000000000000000)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	alloc := types.GenesisAlloc{
		addr: {Balance: genesisBalance},
	}
	sim := setupSimulatedBackend(alloc)
	defer sim.Close()

	ctx := context.Background()
	client := &simulatedClientWrapper{sc: sim.Client()}
	id, err := client.ChainID(ctx)
	require.NoError(t, err)

	wallet := NewInteractingWallet(key, id, client)
	toAddr := getRandomAddr(t)
	value := big.NewInt(1000)
	data := []byte("test data")
	providedNonce := uint64(0)
	providedGasPrice := big.NewInt(20000000000) // 20 gwei
	providedGasLimit := uint64(21000)

	tests := []struct {
		name        string
		to          *common.Address
		value       *big.Int
		gasLimit    uint64
		gasPrice    *big.Int
		data        []byte
		nonce       *uint64
		description string
	}{
		{
			name:        "all_provided",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasPrice:    providedGasPrice,
			data:        data,
			nonce:       &providedNonce,
			description: "All parameters provided",
		},
		{
			name:        "auto_nonce",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasPrice:    providedGasPrice,
			data:        data,
			nonce:       nil,
			description: "Nonce auto-retrieved",
		},
		{
			name:        "auto_gas_price",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasPrice:    nil,
			data:        data,
			nonce:       &providedNonce,
			description: "Gas price auto-suggested",
		},
		{
			name:        "auto_gas_limit",
			to:          &toAddr,
			value:       value,
			gasLimit:    0,
			gasPrice:    providedGasPrice,
			data:        data,
			nonce:       &providedNonce,
			description: "Gas limit auto-estimated",
		},
		{
			name:        "contract_creation",
			to:          nil,
			value:       big.NewInt(0),
			gasLimit:    0,
			gasPrice:    nil,
			data:        data,
			nonce:       nil,
			description: "Contract creation with auto-estimation",
		},
		{
			name:        "simple_transfer_no_data",
			to:          &toAddr,
			value:       value,
			gasLimit:    0,
			gasPrice:    nil,
			data:        nil,
			nonce:       nil,
			description: "Simple transfer with all auto-estimation",
		},
		{
			name:        "zero_value_with_data",
			to:          &toAddr,
			value:       big.NewInt(0),
			gasLimit:    0,
			gasPrice:    nil,
			data:        data,
			nonce:       nil,
			description: "Zero value transaction with data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := wallet.CreateSignedTransaction(ctx, tt.to, tt.value, tt.gasLimit, tt.gasPrice, tt.data, tt.nonce)
			require.NoError(t, err, "Failed for case: %s", tt.description)
			require.NotNil(t, tx, "Transaction should not be nil")

			require.NotEqual(t, common.Hash{}, tx.Hash(), "Transaction hash should not be empty")
			require.GreaterOrEqual(t, tx.Nonce(), uint64(0), "Nonce should be set")
			require.Greater(t, tx.GasPrice().Uint64(), uint64(0), "Gas price should be greater than 0")
			require.Greater(t, tx.Gas(), uint64(0), "Gas limit should be greater than 0")

			// transaction type based on 'to' field
			if tt.to == nil {
				require.Nil(t, tx.To(), "Contract creation should have nil 'to' field")
			} else {
				require.Equal(t, *tt.to, *tx.To(), "Transaction 'to' field should match")
			}

			require.Equal(t, tt.value, tx.Value(), "Transaction value should match")
			require.Equal(t, tt.data, tx.Data(), "Transaction data should match")
		})
	}
}

func TestCreateSignedDynamicFeeTx(t *testing.T) {
	genesisBalance := big.NewInt(12000000000000000)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	alloc := types.GenesisAlloc{
		addr: {Balance: genesisBalance},
	}
	sim := setupSimulatedBackend(alloc)
	defer sim.Close()

	ctx := context.Background()
	client := &simulatedClientWrapper{sc: sim.Client()}
	id, err := client.ChainID(ctx)
	require.NoError(t, err)

	wallet := NewInteractingWallet(key, id, client)
	toAddr := getRandomAddr(t)
	value := big.NewInt(1000)
	data := []byte("test data")
	providedNonce := uint64(0)
	providedGasFeeCap := big.NewInt(30000000000) // 30 gwei
	providedGasTipCap := big.NewInt(2000000000)  // 2 gwei
	providedGasLimit := uint64(21000)

	tests := []struct {
		name        string
		to          *common.Address
		value       *big.Int
		gasLimit    uint64
		gasFeeCap   *big.Int
		gasTipCap   *big.Int
		data        []byte
		nonce       *uint64
		description string
	}{
		{
			name:        "all_provided",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasFeeCap:   providedGasFeeCap,
			gasTipCap:   providedGasTipCap,
			data:        data,
			nonce:       &providedNonce,
			description: "All parameters provided",
		},
		{
			name:        "auto_nonce",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasFeeCap:   providedGasFeeCap,
			gasTipCap:   providedGasTipCap,
			data:        data,
			nonce:       nil,
			description: "Nonce auto-retrieved",
		},
		{
			name:        "auto_gas_fees",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasFeeCap:   nil,
			gasTipCap:   nil,
			data:        data,
			nonce:       &providedNonce,
			description: "Gas fees auto-suggested",
		},
		{
			name:        "auto_gas_limit",
			to:          &toAddr,
			value:       value,
			gasLimit:    0,
			gasFeeCap:   providedGasFeeCap,
			gasTipCap:   providedGasTipCap,
			data:        data,
			nonce:       &providedNonce,
			description: "Gas limit auto-estimated",
		},
		{
			name:        "auto_fee_cap_only",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasFeeCap:   nil,
			gasTipCap:   providedGasTipCap,
			data:        data,
			nonce:       &providedNonce,
			description: "Only gas fee cap auto-calculated",
		},
		{
			name:        "auto_tip_cap_only",
			to:          &toAddr,
			value:       value,
			gasLimit:    providedGasLimit,
			gasFeeCap:   providedGasFeeCap,
			gasTipCap:   nil,
			data:        data,
			nonce:       &providedNonce,
			description: "Only gas tip cap auto-calculated",
		},
		{
			name:        "contract_creation",
			to:          nil,
			value:       big.NewInt(0),
			gasLimit:    0,
			gasFeeCap:   nil,
			gasTipCap:   nil,
			data:        data,
			nonce:       nil,
			description: "Contract creation with auto-estimation",
		},
		{
			name:        "simple_transfer_all_auto",
			to:          &toAddr,
			value:       value,
			gasLimit:    0,
			gasFeeCap:   nil,
			gasTipCap:   nil,
			data:        nil,
			nonce:       nil,
			description: "Simple transfer with all auto-estimation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := wallet.CreateSignedDynamicFeeTx(ctx, tt.to, tt.value, tt.gasLimit, tt.gasFeeCap, tt.gasTipCap, tt.data, tt.nonce)
			require.NoError(t, err, "Failed for case: %s", tt.description)
			require.NotNil(t, tx, "Transaction should not be nil")

			require.NotEqual(t, common.Hash{}, tx.Hash(), "Transaction hash should not be empty")

			require.GreaterOrEqual(t, tx.Nonce(), uint64(0), "Nonce should be set")

			// gas fees are set for EIP-1559
			require.Greater(t, tx.GasFeeCap().Uint64(), uint64(0), "Gas fee cap should be greater than 0")
			require.Greater(t, tx.GasTipCap().Uint64(), uint64(0), "Gas tip cap should be greater than 0")

			require.Greater(t, tx.Gas(), uint64(0), "Gas limit should be greater than 0")

			if tt.to == nil {
				require.Nil(t, tx.To(), "Contract creation should have nil 'to' field")
			} else {
				require.Equal(t, *tt.to, *tx.To(), "Transaction 'to' field should match")
			}

			require.Equal(t, tt.value, tx.Value(), "Transaction value should match")
			require.Equal(t, tt.data, tx.Data(), "Transaction data should match")
			require.Equal(t, uint8(2), tx.Type(), "Should be EIP-1559 dynamic fee transaction")
		})
	}
}

func getRandomAddr(t *testing.T) common.Address {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return addr
}
