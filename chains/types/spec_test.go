package types_test

import (
	"testing"
	"time"

	cosmostypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"gopkg.in/yaml.v3"
)

// This test assumes loadtesttypes.LoadTestSpec has a custom UnmarshalYAML
// that uses loadtesttypes.NewForKind(spec.Kind) to obtain a concrete ChainConfig,
// and then decodes "chain_config" into it.
//
// Cosmos package defines:
//   type ChainConfig struct { ... }
//   func (ChainConfig) IsChainConfig() {}
//   func Register() { loadtesttypes.Register("cosmos", func() loadtesttypes.ChainConfig { return &ChainConfig{} }) }

func TestLoadTestSpec_Unmarshal_Cosmos(t *testing.T) {
	// Arrange: register cosmos chain config factory
	cosmostypes.Register()

	// Sample YAML that targets the cosmos implementation.
	// Keep nodes_addresses empty to avoid depending on its exact fields here.
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

	var spec loadtesttypes.LoadTestSpec

	// Act: unmarshal into the shared spec
	if err := yaml.Unmarshal(yml, &spec); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	// Assert: top-level fields decoded
	if spec.Name != "worker" {
		t.Errorf("Name = %q, want %q", spec.Name, "worker")
	}
	if spec.Kind != "cosmos" {
		t.Errorf("Kind = %q, want %q", spec.Kind, "cosmos")
	}
	if spec.ChainID != "cosmoshub-4" {
		t.Errorf("ChainID = %q, want %q", spec.ChainID, "cosmoshub-4")
	}
	if spec.NumOfBlocks != 200 {
		t.Errorf("NumOfBlocks = %d, want %d", spec.NumOfBlocks, 200)
	}
	if spec.TxTimeout != 30*time.Second {
		t.Errorf("TxTimeout = %v, want %v", spec.TxTimeout, 30*time.Second)
	}

	// Assert: chain_config is the concrete cosmos type
	cfg, ok := spec.ChainCfg.(*cosmostypes.ChainConfig)
	if !ok {
		t.Fatalf("ChainCfg type = %T, want *cosmos.ChainConfig", spec.ChainCfg)
	}

	// Assert: cosmos fields decoded correctly
	if cfg.GasDenom != "uatom" {
		t.Errorf("GasDenom = %q, want %q", cfg.GasDenom, "uatom")
	}
	if cfg.Bech32Prefix != "cosmos" {
		t.Errorf("Bech32Prefix = %q, want %q", cfg.Bech32Prefix, "cosmos")
	}
	if !cfg.UnorderedTxs {
		t.Errorf("UnorderedTxs = false, want true")
	}
	if cfg.NodesAddresses == nil {
		t.Errorf("NodesAddresses = nil, want [] (possibly empty)")
	}
}

func TestLoadTestSpec_Unmarshal_UnknownKind(t *testing.T) {
	// Arrange: do NOT register anything for "unknown"
	yml := []byte(`
name: test
kind: not-a-real-kind
chain_id: whatever
num_of_blocks: 1
chain_config: {}
`)

	var spec loadtesttypes.LoadTestSpec

	// Act: should fail because NewForKind("not-a-real-kind") errors
	if err := yaml.Unmarshal(yml, &spec); err == nil {
		t.Fatalf("expected error for unknown kind, got nil")
	}
}
