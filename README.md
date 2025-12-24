# Catalyst

A load testing tool for Cosmos SDK chains.

## Usage

### As a Library

```go
import "github.com/skip-mev/catalyst/loadtest"

// Create a load test specification
spec := types.LoadTestSpec{
    ChainID:             "my-chain-1",
    NumOfBlocks:         100,
	NumOfTxs:            100,
    NodesAddresses:      []types.NodeAddress{...},
    BaseMnemonic:        "word1 word2 word3",  // BIP39 mnemonic
	NumWallets:          1500 // number of wallets to derive from the base mnemonic. 
    GasDenom:            "stake",
    Bech32Prefix:        "cosmos",
}

// Create and run the load test
test, err := loadtest.New(ctx, spec)
if err != nil {
    // Handle error
}

result, err := test.Run(ctx)
if err != nil {
    // Handle error
}

fmt.Printf("Total Transactions: %d\n", result.TotalTransactions)
```

### As a Binary

The load test can also be run as a standalone binary using a YAML configuration file.

1. Build the binary:
```bash
make build
```

2. Create a YAML configuration file (see example in `example/loadtest.yml`):
```yaml
chain_id: "my-chain-1"
num_of_blocks: 100  # Process 100 blocks
nodes_addresses:
  - grpc: "localhost:9090"
    rpc: "http://localhost:26657"
base_mnemonic: "word1 word2 word3 ... word24"  # Replace with actual mnemonic
num_wallets: 150000
gas_denom: "stake"
bech32_prefix: "cosmos"
```

3. Run the load test:
```bash
./build/loadtest -config path/to/loadtest.yml
```

The binary will execute the load test according to the configuration and print the results to stdout.

#### Mnemonic Format

The mnemonics in the YAML configuration should be valid BIP39 mnemonics (12 or 24 words). These will be used to derive secp256k1 private keys using the standard Cosmos HD path (44'/118'/0'/0/0).

## Results

The load test will output various metrics including:
- Total transactions sent
- Successful transactions
- Failed transactions
- Average gas per transaction
- Average block gas utilization
- Number of blocks processed
- Total runtime

## Dependencies

- `golangci-lint@v2.7.2`
- `govulncheck`
- [markdownlint-cli](https://github.com/igorshubovych/markdownlint-cli)
