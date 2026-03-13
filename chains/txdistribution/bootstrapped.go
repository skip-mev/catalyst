package txdistribution

import (
	"sync"

	"go.uber.org/zap"
)

// TxDistributionBootstrapped is a generic wallet distribution strategy that
// supports bootstrapping (gradually funding wallets) and role rotation between
// senders and receivers. W is the wallet type (e.g.
// *cosmosWallet.InteractingWallet or *ethWallet.InteractingWallet).
type TxDistributionBootstrapped[W any] struct {
	mu      sync.Mutex
	logger  *zap.Logger
	wallets []W

	// Wallet allocation tracking for minimizing reuse with role rotation
	FundedWallets int // the number of wallets which have funds--wallets[0:fundedWallets]
	SenderIndex   int // Next wallet to use as a sender
	ReceiverIndex int // Next wallet to use as a receiver
	NumWallets    int // the length of wallets
}

// NewBootstrapped creates a new bootstrapped distribution.
func NewBootstrapped[W any](logger *zap.Logger, wallets []W, fundedWallets int) *TxDistributionBootstrapped[W] {
	numWallets := len(wallets)
	receiverIndex := fundedWallets
	if fundedWallets == numWallets {
		receiverIndex = numWallets / 2
	}
	return &TxDistributionBootstrapped[W]{
		logger:        logger,
		wallets:       wallets,
		FundedWallets: fundedWallets,
		SenderIndex:   0,
		ReceiverIndex: receiverIndex,
		NumWallets:    numWallets,
	}
}

// GetNextSender returns the next sender wallet.
// Returns the zero value of W (nil for pointer types) if no further senders can be used
// during this load generation.
func (d *TxDistributionBootstrapped[W]) GetNextSender() W {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.SenderIndex >= d.FundedWallets {
		var zero W
		return zero
	}
	nextSender := d.wallets[d.SenderIndex]
	d.SenderIndex = (d.SenderIndex + 1) % d.NumWallets
	return nextSender
}

// GetNextReceiver returns the next receiver wallet using round-robin within the current load.
func (d *TxDistributionBootstrapped[W]) GetNextReceiver() W {
	d.mu.Lock()
	defer d.mu.Unlock()

	receiver := d.wallets[d.ReceiverIndex]
	d.ReceiverIndex = (d.ReceiverIndex + 1) % d.NumWallets
	return receiver
}

// GetWallet returns the wallet at the given index.
func (d *TxDistributionBootstrapped[W]) GetWallet(index int) W {
	return d.wallets[index]
}

// ResetWalletAllocation resets wallet allocation for a new load and rotates roles.
// Returns (oldFundedWallets, newFundedWallets).
func (d *TxDistributionBootstrapped[W]) ResetWalletAllocation() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	oldFunded := d.FundedWallets
	newFunded := d.FundedWallets + d.SenderIndex
	newSender := 0
	newReceiver := newFunded
	// If we have funded all the wallets, just increment w/ modulos
	if newFunded >= d.NumWallets {
		newFunded = d.NumWallets
		newSender = d.SenderIndex
		newReceiver = d.ReceiverIndex
	}
	// Make sure we reset the receiver to max distance from sender when we first finish funding
	if d.FundedWallets < d.NumWallets && newFunded == d.NumWallets {
		newSender = 0
		newReceiver = d.NumWallets / 2
	}
	d.FundedWallets = newFunded
	d.SenderIndex = newSender
	d.ReceiverIndex = newReceiver
	return oldFunded, newFunded
}
