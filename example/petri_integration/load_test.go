package petri_integration

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/skip-mev/petri/core/v3/util"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/docker"
	petritypes "github.com/skip-mev/petri/core/v3/types"
	"github.com/skip-mev/petri/cosmos/v3/chain"
	"github.com/skip-mev/petri/cosmos/v3/node"
	"go.uber.org/zap"

	loadtest "github.com/skip-mev/catalyst/chains/cosmos"
	cosmoslttypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

var (
	defaultChainConfig = petritypes.ChainConfig{
		Denom:         "stake",
		Decimals:      6,
		NumValidators: 4,
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
			logger.Debug("Successfully created load test wallet", zap.Any("address", w.FormattedAddress()))

			walletsMutex.Lock()
			wallets = append(wallets, w)
			mnemonics = append(mnemonics, w.Mnemonic())
			walletsMutex.Unlock()
		}()
	}

	wg.Wait()
	time.Sleep(20 * time.Second)

	logger.Info("Successfully created wallets asynchronously", zap.Int("count", len(wallets)))

	addresses := make([]string, len(wallets))
	for i, w := range wallets {
		addresses[i] = w.FormattedAddress()
	}

	command := []string{
		defaultChainConfig.BinaryName,
		"tx", "bank", "multi-send",
		faucetWallet.FormattedAddress(),
	}
	command = append(command, addresses...)
	command = append(command, "1000000000stake",
		"--chain-id", defaultChainConfig.ChainId,
		"--keyring-backend", "test",
		"--fees", "3000stake",
		"--gas", "auto",
		"--yes",
		"--home", defaultChainConfig.HomeDir,
	)

	t.Log("command: ", command)
	stdout, stderr, exitCode, err := node.RunCommand(ctx, command)
	t.Log("stdout, stderr", stdout, stderr)
	if err != nil || exitCode != 0 {
		t.Fatal("Failed to fund wallets with MsgMultiSend", zap.Error(err), zap.String("stderr", stderr))
	}
	time.Sleep(5 * time.Second)

	msgs := []loadtesttypes.LoadTestMsg{
		{Weight: 1, Type: cosmoslttypes.MsgMultiSend},
		//{Weight: 1, Type: cosmoslttypes.MsgSend},
	}

	spec := loadtesttypes.LoadTestSpec{
		ChainID:     defaultChainConfig.ChainId,
		NumOfBlocks: 20,
		Mnemonics:   mnemonics,
		Msgs:        msgs,
		TxTimeout:   time.Second * 20,
		ChainCfg: &cosmoslttypes.ChainConfig{
			GasDenom:       defaultChainConfig.Denom,
			Bech32Prefix:   defaultChainConfig.Bech32Prefix,
			UnorderedTxs:   true,
			NodesAddresses: nodeAddresses,
		},
	}

	time.Sleep(10 * time.Second)
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

	var mnemonics []string
	var wallets []petritypes.WalletI
	var walletsMutex sync.Mutex
	var wg sync.WaitGroup

	faucetWallet := c.GetFaucetWallet()
	node := c.GetValidators()[0]

	for i := 0; i < 2; i++ {
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

	addresses := make([]string, len(wallets))
	for i, w := range wallets {
		addresses[i] = w.FormattedAddress()
	}
	t.Log("addresses: ", addresses)

	command := []string{
		defaultChainConfig.BinaryName,
		"tx", "bank", "multi-send",
		faucetWallet.FormattedAddress(),
	}
	command = append(command, addresses...)
	command = append(command, "1000000000stake",
		"--chain-id", defaultChainConfig.ChainId,
		"--keyring-backend", "test",
		"--fees", "500stake",
		"--gas", "auto",
		"--yes",
		"--home", defaultChainConfig.HomeDir,
	)
	t.Log("funding command. ", command)

	stdout, stderr, exitCode, err := node.RunCommand(ctx, command)
	t.Log("stdout, stderr", stdout, stderr)
	if err != nil || exitCode != 0 {
		t.Fatal("Failed to fund wallets with MsgMultiSend", zap.Error(err), zap.String("stderr", stderr))
	}
	time.Sleep(5 * time.Second)

	msgs := []loadtesttypes.LoadTestMsg{
		{Weight: 1, Type: cosmoslttypes.MsgMultiSend},
	}
	spec := loadtesttypes.LoadTestSpec{
		ChainID:     defaultChainConfig.ChainId,
		NumOfBlocks: 20,
		Mnemonics:   mnemonics,
		Msgs:        msgs,
		ChainCfg: cosmoslttypes.ChainConfig{
			GasDenom:       defaultChainConfig.Denom,
			Bech32Prefix:   defaultChainConfig.Bech32Prefix,
			NodesAddresses: nodeAddresses,
		},
	}

	task, err := p.CreateTask(ctx, provider.TaskDefinition{
		Name:          "catalyst",
		ContainerName: "catalyst",
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
