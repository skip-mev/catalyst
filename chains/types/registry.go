package types

import "fmt"

type ChainConfig interface {
	Validate(LoadTestSpec) error
	IsChainConfig()
}

// Factory returns a fresh concrete ChainConfig for a given kind (e.g. "cosmos", "evm").
type Factory func() ChainConfig

var registry = map[Kind]Factory{}

func Register(kind Kind, fn Factory) {
	// if _, exists := registry[kind]; exists {
	//	panic("duplicate ChainConfig kind: " + kind)
	//}
	// overwriting is fine...
	registry[kind] = fn
}

func NewForKind(kind Kind) (ChainConfig, error) {
	fn, ok := registry[kind]
	if !ok {
		return nil, fmt.Errorf("unknown chain_config kind %q", kind)
	}
	return fn(), nil
}
