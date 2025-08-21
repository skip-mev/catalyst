package types_test

import (
	"math/big"
	"testing"
	"time"

	cosmostypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoadTestSpec_Marshal_Unmarshal_Eth(t *testing.T) {
	var spec loadtesttypes.LoadTestSpec
	spec.Name = "worker"
	spec.Description = "eth load test"
	spec.Kind = "eth"
	spec.ChainID = "262144"
	spec.NumOfBlocks = 200
	spec.Mnemonics = []string{"seed phrase goes here"}
	spec.ChainCfg = &ethtypes.ChainConfig{NodesAddresses: []ethtypes.NodeAddress{
		{RPC: "https://foobar:8545", Websocket: "ws://foobar:8546"},
	}}
	spec.Msgs = []loadtesttypes.LoadTestMsg{
		{Weight: 0, NumMsgs: 20, Type: ethtypes.MsgCreateContract},
		{Weight: 0, NumMsgs: 20, Type: ethtypes.MsgWriteTo},
		{Weight: 0, NumMsgs: 20, Type: ethtypes.MsgCrossContractCall},
		{Weight: 0, NumMsgs: 20, Type: ethtypes.MsgCallDataBlast},
	}

	msgBytes, err := yaml.Marshal(&spec)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	var otherLoadtestSpec loadtesttypes.LoadTestSpec
	err = yaml.Unmarshal(msgBytes, &otherLoadtestSpec)
	require.NoError(t, err)

	require.Equal(t, spec, otherLoadtestSpec)
}

func TestEthereum(t *testing.T) {
	yml := []byte(`
name: worker
description: eth load test
kind: eth
chain_id: 2341
num_of_blocks: 200
mnemonics: ["seed phrase goes here"]
tx_timeout: "30s"
chain_config:
  tx_opts:
    gas_fee_cap: 1000000000000
    gas_tip_cap: 1000000000000
`)

	var spec loadtesttypes.LoadTestSpec

	if err := yaml.Unmarshal(yml, &spec); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	cfg, ok := spec.ChainCfg.(*ethtypes.ChainConfig)
	require.True(t, ok)
	expectedTxOpts := ethtypes.TxOpts{
		GasFeeCap: big.NewInt(1000000000000),
		GasTipCap: big.NewInt(1000000000000),
	}
	require.Equal(t, expectedTxOpts, cfg.TxOpts)
}

func TestLoadTestSpec_Unmarshal_Cosmos(t *testing.T) {
	yml := []byte(`
name: worker
description: cosmos load test
kind: cosmos
chain_id: cosmoshub-4
num_of_blocks: 200
mnemonics: ["seed phrase goes here"]
tx_timeout: "30s"
chain_config:
  gas_denom: "uatom"
  bech32_prefix: "cosmos"
  unordered_txs: true
  nodes_addresses: []
`)

	expectedSpec := loadtesttypes.LoadTestSpec{
		Name:        "worker",
		Description: "cosmos load test",
		Kind:        "cosmos",
		ChainID:     "cosmoshub-4",
		NumOfBlocks: 200,
		Mnemonics:   []string{"seed phrase goes here"},
		TxTimeout:   30 * time.Second,
		ChainCfg: &cosmostypes.ChainConfig{
			GasDenom:       "uatom",
			Bech32Prefix:   "cosmos",
			UnorderedTxs:   true,
			NodesAddresses: []cosmostypes.NodeAddress{},
		},
	}
	var spec loadtesttypes.LoadTestSpec

	if err := yaml.Unmarshal(yml, &spec); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	require.Equal(t, expectedSpec, spec)
}

func TestLoadTestSpec_Unmarshal_UnknownKind(t *testing.T) {
	yml := []byte(`
name: test
kind: not-a-real-kind
chain_id: whatever
num_of_blocks: 1
chain_config: {}
`)

	var spec loadtesttypes.LoadTestSpec

	if err := yaml.Unmarshal(yml, &spec); err == nil {
		t.Fatalf("expected error for unknown kind, got nil")
	}
}
