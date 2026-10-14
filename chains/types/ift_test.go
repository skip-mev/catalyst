package types_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cosmostypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

const (
	testClientID     = "client-0"
	testBech32Prefix = "cosmos"
)

func TestIFTConfigValidate_CosmosToEVM(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind:         loadtesttypes.KindCosmos,
		ChainID:      "chain-a",
		BaseMnemonic: "test test test test test test test test test test test junk",
		NumWallets:   1,
		Msgs: []loadtesttypes.LoadTestMsg{
			{Type: cosmostypes.MsgIFTTransfer, NumMsgs: 1},
		},
		IFT: &loadtesttypes.IFTConfig{
			ClientID: testClientID,
			Amount:   "1",
			Timeout:  time.Second,
			Cosmos: &loadtesttypes.IFTCosmosConfig{
				Denom:      "stake",
				MsgTypeURL: "/skip.ift.MsgIFTTransfer",
			},
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: loadtesttypes.KindEVM,
				EVM:  &loadtesttypes.IFTDestinationEVMConfig{},
			},
		},
	}

	require.NoError(t, spec.IFT.Validate(spec))
}

func TestIFTConfigValidate_EthToEVMRejected(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind: loadtesttypes.KindEVM,
		IFT: &loadtesttypes.IFTConfig{
			ClientID: testClientID,
			Amount:   "1",
			Timeout:  time.Second,
			EVM: &loadtesttypes.IFTEVMConfig{
				ContractAddress: "0x1234",
			},
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: loadtesttypes.KindEVM,
				EVM:  &loadtesttypes.IFTDestinationEVMConfig{},
			},
		},
	}

	require.NoError(t, spec.IFT.Validate(spec))
}

func TestIFTConfigValidate_EthToCosmos(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind: loadtesttypes.KindEVM,
		IFT: &loadtesttypes.IFTConfig{
			ClientID: testClientID,
			Amount:   "1",
			Timeout:  time.Second,
			EVM: &loadtesttypes.IFTEVMConfig{
				ContractAddress: "0x1234",
			},
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: loadtesttypes.KindCosmos,
				Cosmos: &loadtesttypes.IFTDestinationCosmosConfig{
					Bech32Prefix: testBech32Prefix,
				},
			},
		},
	}

	require.NoError(t, spec.IFT.Validate(spec))
}

func TestIFTConfigValidate_EthRequiresEVMConfig(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind: loadtesttypes.KindEVM,
		IFT: &loadtesttypes.IFTConfig{
			ClientID: testClientID,
			Amount:   "1",
			Timeout:  time.Second,
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: loadtesttypes.KindCosmos,
				Cosmos: &loadtesttypes.IFTDestinationCosmosConfig{
					Bech32Prefix: testBech32Prefix,
				},
			},
		},
	}

	err := spec.IFT.Validate(spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ift.evm must be specified")
}
