package txfactory

import (
	"sync"

	"go.uber.org/zap"

	"github.com/skip-mev/catalyst/chains/cosmos/wallet"
)

var _ TxDistribution = &TxDistributionBootstrapped{}

type TxDistributionBootstrapped struct {
	mu      sync.Mutex
	logger  *zap.Logger
	wallets []*wallet.InteractingWallet

	// Wallet allocation tracking for minimizing reuse with role rotation
	fundedWallets int // the number of wallets which have funds--wallets[0:fundedWallets]
	senderIndex   int // Next wallet to use as a sender
	receiverIndex int // Next wallet to use as a receiver
	numWallets    int // the length of wallets
}

func NewTxDistributionBootstrapped(logger *zap.Logger, wallets []*wallet.InteractingWallet,
	fundedWallets int,
) *TxDistributionBootstrapped {
	numWallets := len(wallets)
	receiverIndex := fundedWallets
	if fundedWallets == numWallets {
		receiverIndex = numWallets / 2
	}
	return &TxDistributionBootstrapped{
		logger:        logger,
		wallets:       wallets,
		fundedWallets: fundedWallets,
		senderIndex:   0,
		receiverIndex: receiverIndex,
		numWallets:    numWallets,
	}
}

// GetNextSender returns the next sender wallet.
// Returns nil if no further senders can be used during this load generation.
func (d *TxDistributionBootstrapped) GetNextSender() *wallet.InteractingWallet {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.senderIndex >= d.fundedWallets {
		return nil
	}
	nextSender := d.wallets[d.senderIndex]
	d.senderIndex = (d.senderIndex + 1) % d.numWallets
	return nextSender
}

// GetNextReceiver returns the next receiver wallet using round-robin within the current load.
func (d *TxDistributionBootstrapped) GetNextReceiver() *wallet.InteractingWallet {
	d.mu.Lock()
	defer d.mu.Unlock()

	receiver := d.wallets[d.receiverIndex]
	d.receiverIndex = (d.receiverIndex + 1) % d.numWallets
	return receiver
}

// ResetWalletAllocation resets wallet allocation for a new load and rotates roles.
// Returns (oldFundedWallets, newFundedWallets).
func (d *TxDistributionBootstrapped) ResetWalletAllocation() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	oldFunded := d.fundedWallets
	newFunded := d.fundedWallets + d.senderIndex
	newSender := 0
	newReceiver := newFunded
	// If we have funded all the wallets, just increment w/ modulos
	if newFunded >= d.numWallets {
		newFunded = d.numWallets
		newSender = d.senderIndex
		newReceiver = d.receiverIndex
	}
	// Make sure we reset the receiver to max distance from sender when we first finish funding
	if d.fundedWallets < d.numWallets && newFunded == d.numWallets {
		newSender = 0
		newReceiver = d.numWallets / 2
	}
	d.fundedWallets = newFunded
	d.senderIndex = newSender
	d.receiverIndex = newReceiver
	return oldFunded, newFunded
}

