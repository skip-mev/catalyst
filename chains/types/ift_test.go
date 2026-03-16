package types_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	cosmostypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

func TestIFTConfigValidate_CosmosToEVM(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind:         "cosmos",
		ChainID:      "chain-a",
		BaseMnemonic: "test test test test test test test test test test test junk",
		NumWallets:   1,
		Msgs: []loadtesttypes.LoadTestMsg{
			{Type: cosmostypes.MsgIFTTransfer, NumMsgs: 1},
		},
		IFT: &loadtesttypes.IFTConfig{
			ClientID: "client-0",
			Amount:   "1",
			Timeout:  time.Second,
			Cosmos: &loadtesttypes.IFTCosmosConfig{
				Denom:      "stake",
				MsgTypeURL: "/skip.ift.MsgIFTTransfer",
			},
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: "evm",
				EVM:  &loadtesttypes.IFTDestinationEVMConfig{},
			},
			Relayer: loadtesttypes.IFTRelayerConfig{
				URL: "127.0.0.1:8080",
			},
		},
	}

	require.NoError(t, spec.IFT.Validate(spec))
}

func TestIFTConfigValidate_EthToEVMRejected(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind: "eth",
		IFT: &loadtesttypes.IFTConfig{
			ClientID: "client-0",
			Amount:   "1",
			Timeout:  time.Second,
			EVM: &loadtesttypes.IFTEVMConfig{
				ContractAddress: "0x1234",
			},
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: "evm",
				EVM:  &loadtesttypes.IFTDestinationEVMConfig{},
			},
			Relayer: loadtesttypes.IFTRelayerConfig{
				URL: "127.0.0.1:8080",
			},
		},
	}

	require.NoError(t, spec.IFT.Validate(spec))
}

func TestIFTConfigValidate_EthToCosmos(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind: "eth",
		IFT: &loadtesttypes.IFTConfig{
			ClientID: "client-0",
			Amount:   "1",
			Timeout:  time.Second,
			EVM: &loadtesttypes.IFTEVMConfig{
				ContractAddress: "0x1234",
			},
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: "cosmos",
				Cosmos: &loadtesttypes.IFTDestinationCosmosConfig{
					Bech32Prefix: "cosmos",
				},
			},
			Relayer: loadtesttypes.IFTRelayerConfig{
				URL: "127.0.0.1:8080",
			},
		},
	}

	require.NoError(t, spec.IFT.Validate(spec))
}

func TestIFTConfigValidate_EthRequiresEVMConfig(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind: "eth",
		IFT: &loadtesttypes.IFTConfig{
			ClientID: "client-0",
			Amount:   "1",
			Timeout:  time.Second,
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: "cosmos",
				Cosmos: &loadtesttypes.IFTDestinationCosmosConfig{
					Bech32Prefix: "cosmos",
				},
			},
			Relayer: loadtesttypes.IFTRelayerConfig{
				URL: "127.0.0.1:8080",
			},
		},
	}

	err := spec.IFT.Validate(spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ift.evm must be specified")
}

func TestIFTConfigValidate_RejectsNonIFTMessagesInIFTMode(t *testing.T) {
	spec := loadtesttypes.LoadTestSpec{
		Kind:         "cosmos",
		ChainID:      "chain-a",
		BaseMnemonic: "test test test test test test test test test test test junk",
		NumWallets:   1,
		Msgs: []loadtesttypes.LoadTestMsg{
			{Type: cosmostypes.MsgSend, NumMsgs: 1},
		},
		IFT: &loadtesttypes.IFTConfig{
			ClientID: "client-0",
			Amount:   "1",
			Timeout:  time.Second,
			Cosmos: &loadtesttypes.IFTCosmosConfig{
				Denom:      "stake",
				MsgTypeURL: "/skip.ift.MsgIFTTransfer",
			},
			Destination: loadtesttypes.IFTDestinationConfig{
				Kind: "evm",
				EVM:  &loadtesttypes.IFTDestinationEVMConfig{},
			},
			Relayer: loadtesttypes.IFTRelayerConfig{
				URL: "127.0.0.1:8080",
			},
		},
	}

	err := spec.IFT.Validate(spec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ift mode only supports MsgIFTTransfer messages")
}
