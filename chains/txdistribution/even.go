package txdistribution

import (
	"sync"
)

// TxDistributionEven is a generic wallet distribution strategy that evenly splits
// wallets into sender and receiver pools with role rotation between loads.
// W is the wallet type (e.g. *cosmosWallet.InteractingWallet or *ethWallet.InteractingWallet).
type TxDistributionEven[W any] struct {
	mu      sync.Mutex
	wallets []W
	// Wallet allocation tracking for minimizing reuse with role rotation
	SenderIndex    int
	ReceiverIndex  int
	LoadGeneration int // tracks which load we're on for role rotation
	NumWallets     int // cached number of wallets for pool calculations
}

// NewEven creates a new even distribution.
func NewEven[W any](wallets []W) *TxDistributionEven[W] {
	return &TxDistributionEven[W]{
		wallets:    wallets,
		NumWallets: len(wallets),
	}
}

// GetNextSender returns the next sender wallet using round-robin within the current load.
func (d *TxDistributionEven[W]) GetNextSender() W {
	d.mu.Lock()
	defer d.mu.Unlock()

	senderPool := d.GetCurrentSenderPool()
	sender := senderPool[d.SenderIndex]
	d.SenderIndex = (d.SenderIndex + 1) % len(senderPool)

	return sender
}

// GetCurrentSenderPool returns the current sender pool with role rotation.
func (d *TxDistributionEven[W]) GetCurrentSenderPool() []W {
	if d.NumWallets < 2 {
		return d.wallets
	}

	// Rotate roles: wallets that were receivers in previous loads become senders
	// This ensures balance distribution over time
	offset := (d.LoadGeneration * (d.NumWallets / 2)) % d.NumWallets
	midpoint := d.NumWallets / 2

	senderPool := make([]W, 0, midpoint)
	for i := 0; i < midpoint; i++ {
		idx := (offset + i) % d.NumWallets
		senderPool = append(senderPool, d.wallets[idx])
	}

	return senderPool
}

// GetCurrentReceiverPool returns the current receiver pool with role rotation.
func (d *TxDistributionEven[W]) GetCurrentReceiverPool() []W {
	if d.NumWallets < 2 {
		return d.wallets
	}

	// Receivers are the complement of senders in this load
	offset := (d.LoadGeneration * (d.NumWallets / 2)) % d.NumWallets
	midpoint := d.NumWallets / 2
	receiverOffset := (offset + midpoint) % d.NumWallets

	receiverPool := make([]W, 0, d.NumWallets-midpoint)
	for i := 0; i < d.NumWallets-midpoint; i++ {
		idx := (receiverOffset + i) % d.NumWallets
		receiverPool = append(receiverPool, d.wallets[idx])
	}

	return receiverPool
}

// GetNextReceiver returns the next receiver wallet using round-robin within the current load.
func (d *TxDistributionEven[W]) GetNextReceiver() W {
	d.mu.Lock()
	defer d.mu.Unlock()

	receiverPool := d.GetCurrentReceiverPool()
	receiver := receiverPool[d.ReceiverIndex]
	d.ReceiverIndex = (d.ReceiverIndex + 1) % len(receiverPool)

	return receiver
}

// ResetWalletAllocation resets wallet allocation for a new load and rotates roles.
// Returns (0, 0) as TxDistributionEven does not track funded wallet counts.
func (d *TxDistributionEven[W]) ResetWalletAllocation() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.SenderIndex = 0
	d.ReceiverIndex = 0
	d.LoadGeneration++
	return 0, 0
}

// GetWallet returns the wallet at the given index.
func (d *TxDistributionEven[W]) GetWallet(index int) W {
	return d.wallets[index]
}
