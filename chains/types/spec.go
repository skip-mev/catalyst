package types

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type LoadTestSpec struct {
	Name         string        `yaml:"name" json:"name"`
	Description  string        `yaml:"description" json:"description"`
	Kind         string        `yaml:"kind" json:"kind"` // "cosmos" | "evm" (discriminator)
	ChainID      string        `yaml:"chain_id" json:"chain_id"`
	NumOfTxs     int           `yaml:"num_of_txs,omitempty" json:"num_of_txs,omitempty"`
	NumOfBlocks  int           `yaml:"num_of_blocks" json:"num_of_blocks"`
	SendInterval time.Duration `yaml:"send_interval" json:"send_interval"`
	NumBatches   int           `yaml:"num_batches" json:"num_batches"`
	BaseMnemonic string        `yaml:"base_mnemonic" json:"base_mnemonic"`
	NumWallets   int           `yaml:"num_wallets" json:"num_wallets"`
	Msgs         []LoadTestMsg `yaml:"msgs" json:"msgs"`
	TxTimeout    time.Duration `yaml:"tx_timeout,omitempty" json:"tx_timeout,omitempty"`
	ChainCfg     ChainConfig   `yaml:"-" json:"-"` // decoded via custom UnmarshalYAML
	Cache        CacheConfig   `yaml:"cache_config" json:"cache_config"`
}

type CacheConfig struct {
	WalletsFile        string `yaml:"wallets_file" json:"wallets_file"`
	TxsFile            string `yaml:"txs_file" json:"txs_file"`
	ShouldCacheTxs     bool   `yaml:"should_cache_txs" json:"should_cache_txs"`
	ShouldCacheWallets bool   `yaml:"should_cache_wallets" json:"should_cache_wallets"`
}

func (cc CacheConfig) Validate() error {
	if cc.ShouldCacheWallets && cc.WalletsFile == "" {
		return fmt.Errorf("wallets_file must be specified if should_cache_wallets is true")
	}
	if cc.ShouldCacheTxs && cc.TxsFile == "" {
		return fmt.Errorf("txs_file must be specified if should_cache_txs is true")
	}
	return nil
}

type loadTestSpecAlias LoadTestSpec

func (s *LoadTestSpec) UnmarshalYAML(n *yaml.Node) error {
	var raw struct {
		loadTestSpecAlias `yaml:",inline"`
		SpecNode          yaml.Node `yaml:"chain_config"`
	}
	if err := n.Decode(&raw); err != nil {
		return err
	}
	*s = LoadTestSpec(raw.loadTestSpecAlias)

	cfg, err := NewForKind(s.Kind)
	if err != nil {
		return err
	}
	if err := raw.SpecNode.Decode(cfg); err != nil {
		return fmt.Errorf("decode chain_config (%s): %w", s.Kind, err)
	}
	s.ChainCfg = cfg
	return nil
}

func (s LoadTestSpec) MarshalYAML() (any, error) {
	type Alias LoadTestSpec
	out := struct {
		Alias       `yaml:",inline"`
		ChainConfig any `yaml:"chain_config,omitempty" json:"chain_config"`
	}{
		Alias:       Alias(s),
		ChainConfig: s.ChainCfg, // concrete value behind the interface
	}
	return out, nil
}

// Validate validates the LoadTestSpec and returns an error if it's invalid
func (s *LoadTestSpec) Validate() error {
	if s.ChainID == "" {
		return fmt.Errorf("chain ID must be specified")
	}

	if len(s.Msgs) == 0 {
		return fmt.Errorf("no messages specified for load testing")
	}

	if len(s.BaseMnemonic) == 0 {
		return fmt.Errorf("BaseMnemonic must be provided")
	}

	if s.NumWallets <= 0 {
		return fmt.Errorf("NumWallets must be greater than zero")
	}

	if err := s.ChainCfg.Validate(*s); err != nil {
		return fmt.Errorf("validating chain config: %w", err)
	}

	if err := s.Cache.Validate(); err != nil {
		return fmt.Errorf("validating cache config: %w", err)
	}

	return nil
}
