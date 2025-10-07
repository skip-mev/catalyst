package petriintegration

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/skip-mev/catalyst/chains"
	cosmoslttypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"github.com/skip-mev/ironbird/petri/core/provider"
	"github.com/skip-mev/ironbird/petri/core/provider/docker"
	petritypes "github.com/skip-mev/ironbird/petri/core/types"
	"github.com/skip-mev/ironbird/petri/cosmos/chain"
	"github.com/skip-mev/ironbird/petri/cosmos/node"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

var (
	defaultChainConfig = petritypes.ChainConfig{
		Denom:         "stake",
		Name:          "iamatest",
		Decimals:      6,
		NumValidators: 1,
		NumNodes:      0,
		BinaryName:    "/usr/bin/simd",
		Image: provider.ImageDefinition{
			Image: "ghcr.io/cosmos/simapp:v0.50",
			UID:   "1000",
			GID:   "1000",
		},
		GasPrices:            "0.0005stake",
		Bech32Prefix:         "cosmos",
		HomeDir:              "/gaia",
		CoinType:             "118",
		ChainId:              "stake-1",
		UseGenesisSubCommand: true,
	}

	defaultChainOptions = petritypes.ChainOptions{
		NodeCreator: node.CreateNode,
		ModifyGenesis: chain.ModifyGenesis([]chain.GenesisKV{
			{
				Key:   "consensus.params.block.max_gas",
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
		AdditionalAccounts: 500,
		BaseMnemonic:       "copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom",
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

	var nodeAddresses []cosmoslttypes.NodeAddress
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
		nodeAddresses = append(nodeAddresses, cosmoslttypes.NodeAddress{
			GRPC: grpcAddress,
			RPC:  "http://" + rpcAddress,
		})
	}

	msgs := []loadtesttypes.LoadTestMsg{
		{NumTxs: 10, Type: cosmoslttypes.MsgMultiSend},
	}

	spec := loadtesttypes.LoadTestSpec{
		ChainID:      defaultChainConfig.ChainId,
		NumBatches:   5,
		SendInterval: 10 * time.Second,
		BaseMnemonic: defaultChainOptions.BaseMnemonic,
		NumWallets:   defaultChainOptions.AdditionalAccounts,
		Msgs:         msgs,
		TxTimeout:    time.Second * 20,
		ChainCfg: &cosmoslttypes.ChainConfig{
			GasDenom:       defaultChainConfig.Denom,
			Bech32Prefix:   defaultChainConfig.Bech32Prefix,
			UnorderedTxs:   false,
			NodesAddresses: nodeAddresses,
		},
		Kind: chains.CosmosKind,
	}

	time.Sleep(10 * time.Second)
	test, err := chains.NewLoadTest(ctx, logger, spec)
	if err != nil {
		t.Fatal("Failed to create test", zap.Error(err))
	}

	result, err := test.Run(ctx, logger)
	if err != nil {
		t.Fatal("Failed to run load test", zap.Error(err))
	}

	fmt.Printf("Load test results: %+v\n", result)
}

func TestPetriDockerfileIntegration(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger, _ := zap.NewDevelopment()

	p, err := docker.CreateProvider(ctx, logger, "docker_provider")
	if err != nil {
		t.Fatal("Provider creation error", zap.Error(err))
		return
	}

	defer func() {
		err := p.Teardown(context.TODO())
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

	var nodeAddresses []cosmoslttypes.NodeAddress
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
		nodeAddresses = append(nodeAddresses, cosmoslttypes.NodeAddress{
			GRPC: grpcAddress,
			RPC:  "http://" + rpcAddress,
		})
	}

	msgs := []loadtesttypes.LoadTestMsg{
		{NumTxs: 10, Type: cosmoslttypes.MsgMultiSend},
	}
	spec := loadtesttypes.LoadTestSpec{
		ChainID:      defaultChainConfig.ChainId,
		NumBatches:   10,
		SendInterval: 5 * time.Second,
		BaseMnemonic: defaultChainOptions.BaseMnemonic,
		NumWallets:   defaultChainOptions.AdditionalAccounts,
		Msgs:         msgs,
		Kind:         chains.CosmosKind,
		ChainCfg: &cosmoslttypes.ChainConfig{
			GasDenom:       defaultChainConfig.Denom,
			Bech32Prefix:   defaultChainConfig.Bech32Prefix,
			NodesAddresses: nodeAddresses,
		},
	}

	task, err := p.CreateTask(ctx, provider.TaskDefinition{
		Name: "catalyst",
		Image: provider.ImageDefinition{
			Image: "ghcr.io/skip-mev/catalyst:latest",
			UID:   "100",
			GID:   "100",
		},
		ProviderSpecificConfig: map[string]string{
			"region":   "ams3",
			"image_id": "177032231",
			"size":     "s-1vcpu-1gb",
		},
		Command: []string{"/tmp/catalyst/loadtest.yml"},
		DataDir: "/tmp/catalyst",
	})
	if err != nil {
		t.Fatal(err)
	}

	configBytes, err := yaml.Marshal(&spec)
	if err != nil {
		t.Fatal("Failed to marshal spec", zap.Error(err))
	}

	if err := task.WriteFile(ctx, "loadtest.yml", configBytes); err != nil {
		t.Fatal("failed to write config file to task", zap.Error(err), zap.Error(err))
	}

	logger.Info("starting load test")
	if err := task.Start(ctx); err != nil {
		t.Fatal("failed to starting load test", zap.Error(err), zap.Error(err))
	}

	time.Sleep(360 * time.Second)
	test, err := chains.NewLoadTest(ctx, logger, spec)
	if err != nil {
		t.Fatal("Failed to create test", zap.Error(err))
	}

	result, err := test.Run(ctx, logger)
	if err != nil {
		t.Fatal("Failed to run load test", zap.Error(err))
	}

	fmt.Printf("Load test results: %+v\n", result)
}
