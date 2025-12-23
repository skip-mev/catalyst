package txfactory

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"

	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
)

var _ TxDistribution = &TxDistributionEven{}

type TxDistributionEven struct {
	mu      sync.Mutex
	wallets []*ethwallet.InteractingWallet
	// Wallet allocation tracking for minimizing reuse with role rotation
	senderIndex    int
	receiverIndex  int
	loadGeneration int // tracks which load we're on for role rotation
	numWallets     int // cached number of wallets for pool calculations
}

func NewTxDistributionEven(wallets []*ethwallet.InteractingWallet) *TxDistributionEven {
	return &TxDistributionEven{
		wallets:        wallets,
		senderIndex:    0,
		receiverIndex:  0,
		loadGeneration: 0,
		numWallets:     len(wallets),
	}
}

// GetNextSender returns the next sender wallet using round-robin within the current load
func (d *TxDistributionEven) GetNextSender() *ethwallet.InteractingWallet {
	d.mu.Lock()
	defer d.mu.Unlock()

	senderPool := d.getCurrentSenderPool()
	sender := senderPool[d.senderIndex]
	d.senderIndex = (d.senderIndex + 1) % len(senderPool)

	return sender
}

// getCurrentSenderPool returns the current sender pool with role rotation
func (d *TxDistributionEven) getCurrentSenderPool() []*ethwallet.InteractingWallet {
	if d.numWallets < 2 {
		return d.wallets
	}

	// Rotate roles: wallets that were receivers in previous loads become senders
	// This ensures balance distribution over time
	offset := (d.loadGeneration * (d.numWallets / 2)) % d.numWallets
	midpoint := d.numWallets / 2

	senderPool := make([]*ethwallet.InteractingWallet, 0, midpoint)
	for i := 0; i < midpoint; i++ {
		idx := (offset + i) % d.numWallets
		senderPool = append(senderPool, d.wallets[idx])
	}

	return senderPool
}

// getCurrentReceiverPool returns the current receiver pool with role rotation
func (d *TxDistributionEven) getCurrentReceiverPool() []*ethwallet.InteractingWallet {
	if d.numWallets < 2 {
		return d.wallets
	}

	// Receivers are the complement of senders in this load
	offset := (d.loadGeneration * (d.numWallets / 2)) % d.numWallets
	midpoint := d.numWallets / 2
	receiverOffset := (offset + midpoint) % d.numWallets

	receiverPool := make([]*ethwallet.InteractingWallet, 0, d.numWallets-midpoint)
	for i := 0; i < d.numWallets-midpoint; i++ {
		idx := (receiverOffset + i) % d.numWallets
		receiverPool = append(receiverPool, d.wallets[idx])
	}

	return receiverPool
}

// getNextReceiver returns the next receiver wallet using round-robin within the current load
func (d *TxDistributionEven) GetNextReceiver() common.Address {
	d.mu.Lock()
	defer d.mu.Unlock()

	receiverPool := d.getCurrentReceiverPool()
	receiver := receiverPool[d.receiverIndex]
	d.receiverIndex = (d.receiverIndex + 1) % len(receiverPool)

	return receiver.Address()
}

// ResetWalletAllocation resets wallet allocation for a new load and rotates roles
func (d *TxDistributionEven) ResetWalletAllocation() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.senderIndex = 0
	d.receiverIndex = 0
	d.loadGeneration++
}

func (d *TxDistributionEven) GetBaselineWallet() *ethwallet.InteractingWallet {
	return d.wallets[0]
}
