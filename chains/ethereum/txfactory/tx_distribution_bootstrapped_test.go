package txfactory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
)

func TestBootstrapping(t *testing.T) {
	// Create mock wallets
	numWallets := 1000
	initialWallets := 1
	numMsgs := 100
	numBlocks := 35
	mockWallets := make([]*ethwallet.InteractingWallet, numWallets)
	for i := range mockWallets {
		mockWallets[i] = &ethwallet.InteractingWallet{}
		// Note: In real implementation, we would need to mock the Address() method
		// For this test, we'll focus on the logic
	}
	distr := NewTxDistributionBootstrapped(zap.NewNop(), mockWallets, initialWallets)
	for range numBlocks {
		for range numMsgs {
			if sender := distr.GetNextSender(); sender == nil {
				continue
			}
		}
		distr.ResetWalletAllocation()
	}
	assert.Equal(t, numWallets, distr.fundedWallets)
}

func TestSenderWalletAllocationBootstrapped(t *testing.T) {
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
	distr := NewTxDistributionBootstrapped(zap.NewNop(), mockWallets, 1)
	factory := NewTxFactory(logger, txOpts, distr)

	// 1 starting wallet and 10 total wallets should take 3 full loads and 1 partial
	fundingTxs := 1
	for range 3 {
		for range fundingTxs {
			assert.NotNil(t, factory.GetNextSender())
		}
		fundingTxs *= 2
		assert.Nil(t, factory.GetNextSender())
		factory.ResetWalletAllocation()
	}

	// Final round should only take 2 txns
	for range 2 {
		assert.NotNil(t, factory.GetNextSender())
	}

	factory.ResetWalletAllocation()
	// Verify we start at 0 and 5 for sender/receiver
	assert.Equal(t, 0, distr.senderIndex)
	assert.Equal(t, 5, distr.receiverIndex)
}
