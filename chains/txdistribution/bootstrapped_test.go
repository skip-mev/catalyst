package txdistribution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func makeWallets(n int) []*int {
	wallets := make([]*int, n)
	for i := range wallets {
		v := i
		wallets[i] = &v
	}
	return wallets
}

func TestBootstrapping(t *testing.T) {
	numWallets := 1000
	initialWallets := 1
	numMsgs := 100
	numBlocks := 35
	wallets := makeWallets(numWallets)
	distr := NewBootstrapped(zap.NewNop(), wallets, initialWallets)
	for range numBlocks {
		for range numMsgs {
			if sender := distr.GetNextSender(); sender == nil {
				continue
			}
		}
		distr.ResetWalletAllocation()
	}
	assert.Equal(t, numWallets, distr.FundedWallets)
}

func TestSenderWalletAllocationBootstrapped(t *testing.T) {
	numWallets := 10
	wallets := makeWallets(numWallets)
	distr := NewBootstrapped(zap.NewNop(), wallets, 1)

	// 1 starting wallet and 10 total wallets should take 3 full loads and 1 partial
	fundingTxs := 1
	for range 3 {
		for range fundingTxs {
			assert.NotNil(t, distr.GetNextSender())
		}
		fundingTxs *= 2
		assert.Nil(t, distr.GetNextSender())
		distr.ResetWalletAllocation()
	}

	// Final round should only take 2 txns
	for range 2 {
		assert.NotNil(t, distr.GetNextSender())
	}

	distr.ResetWalletAllocation()
	// Verify we start at 0 and 5 for sender/receiver
	assert.Equal(t, 0, distr.SenderIndex)
	assert.Equal(t, 5, distr.ReceiverIndex)
}
