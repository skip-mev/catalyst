# Ethereum Load Tester

## Run Types

Load tests for ethereum can be configured to run in one of two ways:

- Block Cadence
  - The defined transactions will be fired off every block until the specified num_blocks has been reached.
  - To use this option, define the num_blocks config value.
- Timed Cadence
  - The defined transactions will be fired off every send_interval until it has done so num_batches times.
  - To use this option, define both the send_interval and num_batches config values.

## Chain Config

The following config can be tuned to alter the behavior of the load tests.

```go
type ChainConfig struct {
	NodesAddresses []NodeAddress `yaml:"nodes_addresses" json:"NodesAddresses"`
	// MaxContracts is the maximum number of contracts that the load test runner will hold in memory.
	// The contracts in memory are used for the other load test message types to interact with.
	NumInitialContracts uint64 `yaml:"num_initial_contracts" json:"NumInitialContracts"`
	// Static gas options for transactions.
	TxOpts TxOpts `yaml:"tx_opts" json:"TxOpts"`
}

type NodeAddress struct {
    RPC       string `yaml:"rpc"`
    Websocket string `yaml:"websocket"`
}

type TxOpts struct {
    GasFeeCap *big.Int `yaml:"gas_fee_cap" json:"gas_fee_cap"`
    GasTipCap *big.Int `yaml:"gas_tip_cap" json:"gas_tip_cap"`
}
```

**NumInitialContracts** - configures how many Loader contracts (see: chains/ethereum/contracts/load/src/Loader.sol) are deployed before the load test begins.

**TxOpts** - allows you to hardcode the dynamic fee transaction values. If these are left unchanged, transactions will use baseline values from ethereum client gas estimation.