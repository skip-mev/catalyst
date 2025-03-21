package petri_integration

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/skip-mev/petri/core/v3/util"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/docker"
	petritypes "github.com/skip-mev/petri/core/v3/types"
	"github.com/skip-mev/petri/cosmos/v3/chain"
	"github.com/skip-mev/petri/cosmos/v3/node"
	"go.uber.org/zap"

	loadtesttypes "github.com/skip-mev/catalyst/internal/types"
	"github.com/skip-mev/catalyst/loadtest"
)

var (
	defaultChainConfig = petritypes.ChainConfig{
		Denom:         "stake",
		Decimals:      6,
		NumValidators: 10,
		NumNodes:      0,
		BinaryName:    "/usr/bin/simd",
		Image: provider.ImageDefinition{
			Image: "ghcr.io/skip-mev/simapp:latest",
			UID:   "1000",
			GID:   "1000",
		},
		GasPrices:            "0.0005stake",
		Bech32Prefix:         "cosmos",
		HomeDir:              "/gaia",
		CoinType:             "118",
		ChainId:              "stake-1",
		UseGenesisSubCommand: false,
	}

	defaultChainOptions = petritypes.ChainOptions{
		NodeCreator: node.CreateNode,
		ModifyGenesis: chain.ModifyGenesis([]chain.GenesisKV{
			{
				Key:   "consensus_params.block.max_gas",
				Value: "75000000",
			},
		}),
		WalletConfig: petritypes.WalletConfig{
			SigningAlgorithm: string(hd.Secp256k1.Name()),
			Bech32Prefix:     "cosmos",
			HDPath:           hd.CreateHDPath(118, 0, 0),
			DerivationFn:     hd.Secp256k1.Derive(),
			GenerationFn:     hd.Secp256k1.Generate(),
		},
	}
)

func TestPetriDockerIntegration(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger, _ := zap.NewDevelopment()

	p, err := docker.CreateProvider(ctx, logger, "docker_provider")
	if err != nil {
		t.Fatal("Provider creation error", zap.Error(err))
		return
	}

	defer func() {
		err := p.Teardown(ctx)
		if err != nil {
			t.Logf("Failed to teardown provider: %v", err)
		}
	}()

	c, err := chain.CreateChain(ctx, logger, p, defaultChainConfig, defaultChainOptions)
	if err != nil {
		t.Fatal("Chain creation error", zap.Error(err))
	}
	err = c.Init(ctx, defaultChainOptions)
	if err != nil {
		t.Fatal("Failed to init chain", zap.Error(err))
	}
	err = c.WaitForStartup(ctx)
	if err != nil {
		t.Fatal("Failed to wait for chain startup", zap.Error(err))
	}

	// Add a delay to ensure the node is fully ready
	time.Sleep(5 * time.Second)

	var nodeAddresses []loadtesttypes.NodeAddress
	for _, n := range c.GetValidators() {
		grpcAddress, err := n.GetExternalAddress(ctx, "9090")
		if err != nil {
			t.Fatal("Failed to get node grpc address", zap.Error(err))
		}
		rpcAddress, err := n.GetExternalAddress(ctx, "26657")
		if err != nil {
			t.Fatal("Failed to get node rpc address", zap.Error(err))
		}
		logger.Info("Node addresses",
			zap.String("grpc", grpcAddress),
			zap.String("rpc", rpcAddress))
		nodeAddresses = append(nodeAddresses, loadtesttypes.NodeAddress{
			GRPC: grpcAddress,
			RPC:  "http://" + rpcAddress,
		})
	}

	var mnemonics []string
	var wallets []petritypes.WalletI
	var walletsMutex sync.Mutex
	var wg sync.WaitGroup

	faucetWallet := c.GetFaucetWallet()
	node := c.GetValidators()[0]

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, err := c.CreateWallet(ctx, util.RandomString(5), defaultChainOptions.WalletConfig)
			if err != nil {
				logger.Error("Failed to create wallet", zap.Error(err))
				return
			}

			walletsMutex.Lock()
			wallets = append(wallets, w)
			walletsMutex.Unlock()
		}()
	}

	wg.Wait()

	logger.Info("Successfully created wallets asynchronously", zap.Int("count", len(wallets)))

	for _, w := range wallets {
		command := []string{
			defaultChainConfig.BinaryName,
			"tx", "bank", "send",
			faucetWallet.FormattedAddress(),
			w.FormattedAddress(),
			"1000000000stake",
			"--chain-id", defaultChainConfig.ChainId,
			"--keyring-backend", "test",
			"--fees", "100stake",
			"--yes",
			"--home", defaultChainConfig.HomeDir,
		}

		_, stderr, exitCode, err := node.RunCommand(ctx, command)
		if err != nil || exitCode != 0 {
			t.Fatal("Failed to fund wallet", zap.Error(err), zap.String("stderr", stderr))
		}

		mnemonics = append(mnemonics, w.Mnemonic())
		time.Sleep(5 * time.Second)
	}

	msgs := []loadtesttypes.LoadTestMsg{
		//{Weight: 0.5, Type: loadtesttypes.MsgSend},
		{Weight: 1, Type: loadtesttypes.MsgMultiSend},
	}
	spec := loadtesttypes.LoadTestSpec{
		ChainID:             defaultChainConfig.ChainId,
		BlockGasLimitTarget: 1,
		NumOfBlocks:         50,
		NodesAddresses:      nodeAddresses,
		Mnemonics:           mnemonics,
		GasDenom:            defaultChainConfig.Denom,
		Bech32Prefix:        defaultChainConfig.Bech32Prefix,
		Msgs:                msgs,
	}

	test, err := loadtest.New(ctx, spec)
	if err != nil {
		t.Fatal("Failed to create test", zap.Error(err))
	}

	result, err := test.Run(ctx, logger)
	if err != nil {
		t.Fatal("Failed to run load test", zap.Error(err))
	}

	fmt.Printf("Load test results: %+v\n", result)
}
