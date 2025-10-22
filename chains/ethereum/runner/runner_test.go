package runner

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestTxCaching(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "tx_cache")
	assert.NoError(t, err)

	numBatches := 200
	perBatch := 20
	originalBatches := make([][]*types.Transaction, numBatches)
	for i := range numBatches {
		for range perBatch {
			originalBatches[i] = append(originalBatches[i], newTx())
		}
	}

	assert.NoError(t, CacheTxs(f.Name(), originalBatches))

	cachedBatches, err := CachedTxs(f.Name(), numBatches)
	assert.NoError(t, err)

	assert.Len(t, cachedBatches, numBatches)
	for _, batch := range cachedBatches {
		assert.Len(t, batch, 20)
	}

	for i, batch := range cachedBatches {
		for j, tx := range batch {
			assert.Equal(t, originalBatches[i][j].Hash(), tx.Hash(), fmt.Sprintf("mismatch between tx in batch %d index %d", i, j))
		}
	}
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
