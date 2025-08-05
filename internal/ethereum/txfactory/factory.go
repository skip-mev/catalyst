package txfactory

import ethwallet "github.com/skip-mev/catalyst/internal/ethereum/wallet"

type TxFactory struct {
	wallets []*ethwallet.InteractingWallet
}

// Contract Creation
// Simple Eth Transfers
// Contract interaction with large payload
// Contract interaction that calls into other contracts.
// Contract interaction with heavy state reads
// Contract interaction with heavy state writes

func NewTxFactory(wallets []*ethwallet.InteractingWallet) *TxFactory {
	return &TxFactory{wallets: wallets}
}
