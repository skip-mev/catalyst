package types

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

type LoadTestSpec struct {
	Name        string        `yaml:"name" json:"name"`
	Description string        `yaml:"description" json:"description"`
	Kind        string        `yaml:"kind" json:"kind"` // "cosmos" | "evm" (discriminator)
	ChainID     string        `yaml:"chain_id" json:"chain_id"`
	NumOfTxs    int           `yaml:"num_of_txs,omitempty" json:"num_of_txs,omitempty"`
	NumOfBlocks int           `yaml:"num_of_blocks" json:"num_of_blocks"`
	Mnemonics   []string      `yaml:"mnemonics" json:"mnemonics"`
	Msgs        []LoadTestMsg `yaml:"msgs" json:"msgs"`
	TxTimeout   time.Duration `yaml:"tx_timeout,omitempty" json:"tx_timeout,omitempty"`
	ChainCfg    ChainConfig   `yaml:"-" json:"-"` // decoded via custom UnmarshalYAML
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
		ChainConfig any `yaml:"chain_config,omitempty"`
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

	if s.NumOfTxs <= 0 {
		return fmt.Errorf("num_of_txs must be greater than 0")
	}

	if s.NumOfBlocks <= 0 {
		return fmt.Errorf("num_of_blocks must be greater than 0")
	}

	if len(s.Msgs) == 0 {
		return fmt.Errorf("no messages specified for load testing")
	}

	if len(s.Mnemonics) == 0 {
		return fmt.Errorf("mnemonics must be provided")
	}

	return s.ChainCfg.Validate(*s)
}
