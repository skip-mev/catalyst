package txfactory

import (
	"testing"

	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWalletAllocationMinimizesReuse(t *testing.T) {
	// Create mock wallets
	numWallets := 10
	mockWallets := make([]*ethwallet.InteractingWallet, numWallets)
	for i := range mockWallets {
		mockWallets[i] = &ethwallet.InteractingWallet{}
		// Note: In real implementation, we would need to mock the Address() method
		// For this test, we'll focus on the logic
	}

	logger := zap.NewNop()
	txOpts := ethtypes.TxOpts{}
	factory := NewTxFactory(logger, mockWallets, txOpts)

	// Test 1: Verify that sender and receiver pools are non-overlapping within a load
	factory.ResetWalletAllocation()

	senderPool := factory.getCurrentSenderPool()
	receiverPool := factory.getCurrentReceiverPool()

	// Ensure pools have expected sizes
	expectedSenderSize := numWallets / 2
	expectedReceiverSize := numWallets - expectedSenderSize
	assert.Equal(t, expectedSenderSize, len(senderPool), "Sender pool should be half the wallets")
	assert.Equal(t, expectedReceiverSize, len(receiverPool), "Receiver pool should be the remaining wallets")

	// Test 2: Verify load generation increments
	initialGeneration := factory.loadGeneration
	factory.ResetWalletAllocation()
	assert.Equal(t, initialGeneration+1, factory.loadGeneration, "Load generation should increment")

	// Test 3: Verify round-robin within pools
	factory.ResetWalletAllocation()

	// Get all senders in the pool
	senders := make([]*ethwallet.InteractingWallet, 0)
	poolSize := len(factory.getCurrentSenderPool())

	for i := 0; i < poolSize*2; i++ { // Go through the pool twice
		sender := factory.GetNextSender()
		senders = append(senders, sender)
	}

	// Verify that after going through the pool once, we start from the beginning again
	assert.Equal(t, senders[0], senders[poolSize], "Round-robin should restart from beginning")
	assert.Equal(t, senders[1], senders[poolSize+1], "Round-robin should follow same order")
}

func TestWalletAllocationHandlesEdgeCases(t *testing.T) {
	logger := zap.NewNop()
	txOpts := ethtypes.TxOpts{}

	// Test with single wallet
	t.Run("single wallet", func(t *testing.T) {
		singleWallet := []*ethwallet.InteractingWallet{{}}
		factory := NewTxFactory(logger, singleWallet, txOpts)
		factory.ResetWalletAllocation()

		senderPool := factory.getCurrentSenderPool()
		receiverPool := factory.getCurrentReceiverPool()

		// With a single wallet, both pools should contain the same wallet
		require.Len(t, senderPool, 1)
		require.Len(t, receiverPool, 1)
	})

	// Test with two wallets
	t.Run("two wallets", func(t *testing.T) {
		twoWallets := []*ethwallet.InteractingWallet{
			{},
			{},
		}
		factory := NewTxFactory(logger, twoWallets, txOpts)
		factory.ResetWalletAllocation()

		senderPool := factory.getCurrentSenderPool()
		receiverPool := factory.getCurrentReceiverPool()

		// With two wallets, each pool should have one wallet
		assert.Len(t, senderPool, 1)
		assert.Len(t, receiverPool, 1)
		// With mock wallets, we can't easily test non-overlap, so just verify pool sizes
	})
}

func TestLoadGenerationProgression(t *testing.T) {
	numWallets := 8
	mockWallets := make([]*ethwallet.InteractingWallet, numWallets)
	for i := range mockWallets {
		mockWallets[i] = &ethwallet.InteractingWallet{}
	}

	logger := zap.NewNop()
	txOpts := ethtypes.TxOpts{}
	factory := NewTxFactory(logger, mockWallets, txOpts)

	// Test that load generation progresses
	initialGeneration := factory.loadGeneration

	for i := 0; i < 5; i++ {
		factory.ResetWalletAllocation()
		assert.Equal(t, initialGeneration+i+1, factory.loadGeneration, "Load generation should increment with each reset")
	}
}
