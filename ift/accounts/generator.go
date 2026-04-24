package accounts

import (
	"fmt"

	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

type Generator interface {
	GenerateRecipients(count, offset int) ([]string, error)
}

func NewGenerator(spec loadtesttypes.LoadTestSpec) (Generator, error) {
	if spec.IFT == nil {
		return nil, fmt.Errorf("ift config is required")
	}

	switch spec.IFT.Destination.Kind {
	case "evm":
		return newEVMGenerator(spec.BaseMnemonic), nil
	case "cosmos":
		return newCosmosGenerator(spec.BaseMnemonic, spec.IFT.Destination.Cosmos.Bech32Prefix), nil
	default:
		return nil, fmt.Errorf("unsupported destination kind %q", spec.IFT.Destination.Kind)
	}
}

func GenerateRecipients(spec loadtesttypes.LoadTestSpec) ([]string, error) {
	if spec.IFT == nil {
		return nil, nil
	}

	count := spec.IFT.RecipientCount(spec.NumWallets)
	if count <= 0 {
		return nil, fmt.Errorf("ift recipient count must be greater than zero")
	}

	offset := spec.IFT.RecipientOffset(spec.NumWallets)

	generator, err := NewGenerator(spec)
	if err != nil {
		return nil, err
	}

	return generator.GenerateRecipients(count, offset)
}
