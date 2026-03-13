package txfactory

import (
	"github.com/skip-mev/catalyst/chains/cosmos/wallet"
)

// TxDistribution controls sender/receiver selection for load testing.
type TxDistribution interface {
	GetNextSender() *wallet.InteractingWallet
	GetNextReceiver() *wallet.InteractingWallet
	ResetWalletAllocation()
}
