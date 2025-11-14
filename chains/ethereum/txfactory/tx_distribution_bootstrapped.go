package txfactory

import (
	"context"
	"go.uber.org/zap"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
)

var _ TxDistribution = &TxDistributionBootstrapped{}

type TxDistributionBootstrapped struct {
	mu      sync.Mutex
	logger  *zap.Logger
	wallets []*ethwallet.InteractingWallet

	// Wallet allocation tracking for minimizing reuse with role rotation
	fundedWallets int // the number of wallets which have funds--wallets[0:fundedWallets]
	senderIndex   int // Next wallet to use as a sender
	receiverIndex int // Next wallet to use as a receiver
	numWallets    int // the length of wallets
	// Map of address to balance (common.Address => *big.Int)
	balanceCache *sync.Map
}

func NewTxDistributionBootstrapped(logger *zap.Logger, wallets []*ethwallet.InteractingWallet,
	fundedWallets int) *TxDistributionBootstrapped {
	numWallets := len(wallets)
	receiverIndex := fundedWallets
	if fundedWallets == numWallets {
		receiverIndex = numWallets / 2
	}
	balanceCache := &sync.Map{}
	for i, w := range wallets {
		if i%10000 == 0 {
			logger.Info("initializing balances", zap.Int("progress", i))
		}
		bal, err := w.GetBalance(context.Background())
		if err != nil {
			logger.Info("Failed to bootstrap balance", zap.String("account", w.Address().String()), zap.Error(err))
			// Break out early if we're getting 0 balances for non-funded wallets--assume the rest are 0
			if i > fundedWallets {
				break
			}
			continue
		}
		balanceCache.Store(w.Address(), bal)
	}
	return &TxDistributionBootstrapped{
		logger:        logger,
		wallets:       wallets,
		fundedWallets: fundedWallets,
		senderIndex:   0,
		receiverIndex: receiverIndex,
		numWallets:    numWallets,
		balanceCache:  balanceCache,
	}
}

func (d *TxDistributionBootstrapped) GetAccountBalance(addr common.Address) *big.Int {
	bal, ok := d.balanceCache.Load(addr)
	if !ok {
		return nil
	}
	return bal.(*big.Int)
}

func (d *TxDistributionBootstrapped) SetAccountBalance(addr common.Address, bal *big.Int) {
	d.balanceCache.Store(addr, bal)
}

// GetNextSender returns the next sender wallet
// Returns nil if no further senders can be used during this load generation.
func (d *TxDistributionBootstrapped) GetNextSender() *ethwallet.InteractingWallet {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.senderIndex >= d.fundedWallets {
		return nil
	}
	nextSender := d.wallets[d.senderIndex]
	d.senderIndex = (d.senderIndex + 1) % d.numWallets
	return nextSender
}

// getNextReceiver returns the next receiver wallet using round-robin within the current load
func (d *TxDistributionBootstrapped) GetNextReceiver() common.Address {
	d.mu.Lock()
	defer d.mu.Unlock()

	receiverAddress := d.wallets[d.receiverIndex].Address()
	d.receiverIndex = (d.receiverIndex + 1) % d.numWallets
	return receiverAddress
}

// ResetWalletAllocation resets wallet allocation for a new load and rotates roles
func (d *TxDistributionBootstrapped) ResetWalletAllocation() {
	d.mu.Lock()
	defer d.mu.Unlock()

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
}

func (d *TxDistributionBootstrapped) GetBaselineWallet() *ethwallet.InteractingWallet {
	return d.wallets[0]
}
