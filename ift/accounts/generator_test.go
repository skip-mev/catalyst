package accounts

import (
	"testing"

	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

func TestGenerateRecipientsForEVM(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		BaseMnemonic: "test test test test test test test test test test test junk",
		NumWallets:   2,
		IFT: &loadtesttypes.IFTConfig{
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: "evm",
				EVM:  &loadtesttypes.IFTDestinationEVMConfig{},
			},
		},
	}

	recipients, err := GenerateRecipients(spec)
	if err != nil {
		t.Fatalf("GenerateRecipients returned error: %v", err)
	}

	if got, want := len(recipients), 2; got != want {
		t.Fatalf("len(recipients) = %d, want %d", got, want)
	}
}

func TestGenerateRecipientsForCosmos(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		BaseMnemonic: "test test test test test test test test test test test junk",
		NumWallets:   2,
		IFT: &loadtesttypes.IFTConfig{
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: "cosmos",
				Cosmos: &loadtesttypes.IFTDestinationCosmosConfig{
					Bech32Prefix: "cosmos",
				},
			},
		},
	}

	recipients, err := GenerateRecipients(spec)
	if err != nil {
		t.Fatalf("GenerateRecipients returned error: %v", err)
	}

	if got, want := len(recipients), 2; got != want {
		t.Fatalf("len(recipients) = %d, want %d", got, want)
	}
}
