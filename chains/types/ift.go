package types

import (
	"fmt"
	"time"
)

const (
	ChainTypeCosmos = "cosmos"
	ChainTypeEVM    = "evm"
	ChainTypeETH    = "eth"
)

type IFTConfig struct {
	ClientID    string               `yaml:"client_id"            json:"client_id"`
	Amount      string               `yaml:"amount"               json:"amount"`
	Timeout     time.Duration        `yaml:"timeout"              json:"timeout"`
	Recipients  IFTRecipientsConfig  `yaml:"recipients,omitempty" json:"recipients,omitempty"`
	Destination IFTDestinationConfig `yaml:"destination"          json:"destination"`
	Cosmos      *IFTCosmosConfig     `yaml:"cosmos,omitempty"     json:"cosmos,omitempty"`
	EVM         *IFTEVMConfig        `yaml:"evm,omitempty"        json:"evm,omitempty"`
}

type IFTRecipientsConfig struct {
	Count  int `yaml:"count,omitempty"  json:"count,omitempty"`
	Offset int `yaml:"offset,omitempty" json:"offset,omitempty"`
}

type IFTCosmosConfig struct {
	Denom      string `yaml:"denom"        json:"denom"`
	MsgTypeURL string `yaml:"msg_type_url" json:"msg_type_url"`
}

type IFTEVMConfig struct {
	ContractAddress string `yaml:"contract_address" json:"contract_address"`
}

type IFTDestinationConfig struct {
	Kind   string                      `yaml:"kind"             json:"kind"`
	Cosmos *IFTDestinationCosmosConfig `yaml:"cosmos,omitempty" json:"cosmos,omitempty"`
	EVM    *IFTDestinationEVMConfig    `yaml:"evm,omitempty"    json:"evm,omitempty"`
}

type IFTDestinationCosmosConfig struct {
	Bech32Prefix string `yaml:"bech32_prefix" json:"bech32_prefix"`
}

type IFTDestinationEVMConfig struct{}

func (c *IFTConfig) Validate(spec LoadTestSpec) error {
	if c == nil {
		return nil
	}

	if c.Recipients.Count < 0 {
		return fmt.Errorf("ift.recipients.count must be greater than or equal to zero")
	}

	if c.Recipients.Offset < 0 {
		return fmt.Errorf("ift.recipients.offset must be greater than or equal to zero")
	}

	if c.ClientID == "" {
		return fmt.Errorf("ift.client_id must be specified")
	}

	if c.Amount == "" {
		return fmt.Errorf("ift.amount must be specified")
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("ift.timeout must be greater than zero")
	}

	if err := c.Destination.Validate(); err != nil {
		return err
	}

	switch spec.Kind {
	case ChainTypeCosmos:
		if err := c.validateCosmos(); err != nil {
			return err
		}
		if c.Destination.Kind != ChainTypeEVM && c.Destination.Kind != ChainTypeCosmos {
			return fmt.Errorf(
				"ift.destination.kind %q is incompatible with source kind %q",
				c.Destination.Kind,
				spec.Kind,
			)
		}
	case ChainTypeETH:
		if err := c.validateEVM(); err != nil {
			return err
		}
		if c.Destination.Kind != ChainTypeCosmos && c.Destination.Kind != ChainTypeEVM {
			return fmt.Errorf(
				"ift.destination.kind %q is incompatible with source kind %q",
				c.Destination.Kind,
				spec.Kind,
			)
		}
	default:
		return fmt.Errorf("unsupported source kind %q for ift mode", spec.Kind)
	}

	return nil
}

func (c *IFTConfig) validateCosmos() error {
	if c.Cosmos == nil {
		return fmt.Errorf("ift.cosmos must be specified for cosmos runners")
	}
	if c.Cosmos.Denom == "" {
		return fmt.Errorf("ift.cosmos.denom must be specified")
	}
	if c.Cosmos.MsgTypeURL == "" {
		return fmt.Errorf("ift.cosmos.msg_type_url must be specified")
	}
	return nil
}

func (c *IFTConfig) validateEVM() error {
	if c.EVM == nil {
		return fmt.Errorf("ift.evm must be specified for eth runners")
	}
	if c.EVM.ContractAddress == "" {
		return fmt.Errorf("ift.evm.contract_address must be specified")
	}
	return nil
}

func (c IFTDestinationConfig) Validate() error {
	switch c.Kind {
	case ChainTypeEVM:
		if c.EVM == nil {
			return fmt.Errorf("ift.destination.evm must be specified for evm destinations")
		}
		return nil
	case ChainTypeCosmos:
		if c.Cosmos == nil {
			return fmt.Errorf("ift.destination.cosmos must be specified for cosmos destinations")
		}
		if c.Cosmos.Bech32Prefix == "" {
			return fmt.Errorf("ift.destination.cosmos.bech32_prefix must be specified")
		}
		return nil
	default:
		return fmt.Errorf("invalid ift.destination.kind %q", c.Kind)
	}
}

func (c *IFTConfig) RecipientCount(defaultCount int) int {
	if c == nil || c.Recipients.Count == 0 {
		return defaultCount
	}
	return c.Recipients.Count
}

func (c *IFTConfig) RecipientOffset(defaultOffset int) int {
	if c == nil || c.Recipients.Offset == 0 {
		return defaultOffset
	}
	return c.Recipients.Offset
}
