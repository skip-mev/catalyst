package runner

import (
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
)

func TestTryIFTBaselineUsesNextWalletAfterFailure(t *testing.T) {
	wallets := []*wallet.InteractingWallet{
		new(wallet.InteractingWallet),
		new(wallet.InteractingWallet),
		new(wallet.InteractingWallet),
	}
	attempts := 0

	err := tryIFTBaseline(wallets, func(candidate *wallet.InteractingWallet) error {
		attempts++
		if candidate != wallets[2] {
			return errors.New("cannot estimate")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 3, attempts)
}

func TestTryIFTBaselineReturnsErrorWhenAllWalletsFail(t *testing.T) {
	wallets := []*wallet.InteractingWallet{
		new(wallet.InteractingWallet),
		new(wallet.InteractingWallet),
	}

	err := tryIFTBaseline(wallets, func(*wallet.InteractingWallet) error {
		return errors.New("cannot estimate")
	})

	require.Error(t, err)
}

func TestSentOnlyResultSeparatesBroadcastFailures(t *testing.T) {
	sent := []*inttypes.SentTx{
		{MsgType: inttypes.ContractCall},
		{MsgType: inttypes.ContractCall, SendTransactionErr: errors.New("rejected")},
		{MsgType: inttypes.MsgIFTTransfer, RelayErr: errors.New("relay failed")},
	}

	result := sentOnlyResult(sent)

	require.True(t, result.ReceiptCollectionSkipped)
	require.Equal(t, 2, result.Overall.TotalTransactions)
	require.Equal(t, 1, result.Overall.BroadcastFailures)
	require.Equal(t, 1, result.Overall.RelayFailures)
	require.Equal(t, 1, result.ByMessage[inttypes.ContractCall].Transactions.TotalSent)
	require.Equal(t, 1, result.ByMessage[inttypes.ContractCall].Transactions.BroadcastFailures)
	require.Equal(t, 1, result.ByMessage[inttypes.MsgIFTTransfer].Transactions.TotalSent)
	require.Equal(t, 1, result.ByMessage[inttypes.MsgIFTTransfer].Transactions.RelayFailures)
}

func TestTxCaching(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tx_cache")
	assert.NoError(t, err)
	defer f.Close()

	numBatches := 200
	perBatch := 20
	originalBatches := make([][]*types.Transaction, numBatches)
	for i := range numBatches {
		for range perBatch {
			originalBatches[i] = append(originalBatches[i], newTx())
		}
	}

	assert.NoError(t, WriteTxnsToCache(f.Name(), originalBatches))

	cachedBatches, err := ReadTxnsFromCache(f.Name(), numBatches)
	assert.NoError(t, err)

	assert.Len(t, cachedBatches, numBatches)
	for _, batch := range cachedBatches {
		assert.Len(t, batch, 20)
	}

	for i, batch := range cachedBatches {
		for j, tx := range batch {
			assert.Equal(
				t,
				originalBatches[i][j].Hash(),
				tx.Hash(),
				fmt.Sprintf("mismatch between tx in batch %d index %d", i, j),
			)
		}
	}
}

func TestTxCachingMismatchNumBatches(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tx_cache")
	assert.NoError(t, err)
	defer f.Close()

	numBatches := 3
	perBatch := 3
	originalBatches := make([][]*types.Transaction, numBatches)
	for i := range numBatches {
		for range perBatch {
			originalBatches[i] = append(originalBatches[i], newTx())
		}
	}

	assert.NoError(t, WriteTxnsToCache(f.Name(), originalBatches))

	cachedBatches, err := ReadTxnsFromCache(f.Name(), 2)
	assert.NoError(t, err)
	assert.Len(t, cachedBatches, 2)
}

func newTx() *types.Transaction {
	chainID := big.NewInt(1337) // Example Chain ID for a local network.
	nonce := big.NewInt(1)
	gasLimit := uint64(21000)

	// Random values for fee caps and value.
	maxPriorityFeePerGas := big.NewInt(500)
	maxFeePerGas := big.NewInt(500)
	value := big.NewInt(500)

	// Generate random addresses for the sender and recipient.
	toAddress, err := generateRandomAddress()
	if err != nil {
		log.Fatalf("Failed to generate random 'to' address: %v", err)
	}
	to := common.HexToAddress(toAddress)

	// Generate random data for the transaction payload.
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		log.Fatalf("Failed to generate random data: %v", err)
	}

	// 3. Create the EIP-1559 transaction.
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce.Uint64(),
		To:        &to,
		Value:     value,
		Gas:       gasLimit,
		GasTipCap: maxPriorityFeePerGas,
		GasFeeCap: maxFeePerGas,
		Data:      data,
	})
}

func generateRandomAddress() (string, error) {
	privateKey, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return address.Hex(), nil
}
