package runner

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/skip-mev/catalyst/chains/ethereum/metrics"
	"github.com/skip-mev/catalyst/chains/ethereum/txfactory"
	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

type Runner struct {
	logger *zap.Logger

	clients   []*ethclient.Client
	wsClients []*ethclient.Client

	spec        loadtesttypes.LoadTestSpec
	chainConfig inttypes.ChainConfig
	nonces      *sync.Map
	wallets     []*wallet.InteractingWallet

	txFactory *txfactory.TxFactory

	sentTxs         []*inttypes.SentTx
	blocksProcessed uint64

	// senderToWallet maps sender addresses to their assigned wallet
	senderToWallet map[common.Address]*wallet.InteractingWallet
}

func NewRunner(ctx context.Context, logger *zap.Logger, spec loadtesttypes.LoadTestSpec) (*Runner, error) {
	chainCfg := spec.ChainCfg.(*inttypes.ChainConfig)
	clients := make([]*ethclient.Client, 0, len(chainCfg.NodesAddresses))
	wsClients := make([]*ethclient.Client, 0, len(chainCfg.NodesAddresses))
	logger.Info("configuring runner")
	for _, nodeAddress := range chainCfg.NodesAddresses {
		tr := &http.Transport{
			MaxConnsPerHost:     256,  // cap concurrency per host
			MaxIdleConns:        2048, // large idle pool
			MaxIdleConnsPerHost: 1024,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
				// DO NOT set LocalAddr unless you really need to.
			}).DialContext,
			// Keep-alives are on by default; don't disable them.
		}
		hc := &http.Client{
			Transport: tr,
			Timeout:   30 * time.Second, // per-request ceiling
		}
		rpcClient, err := rpc.DialOptions(ctx, nodeAddress.RPC, rpc.WithHTTPClient(hc))
		if err != nil {
			return nil, fmt.Errorf("failed construct RPC client for %s: %w", nodeAddress.RPC, err)
		}
		client := ethclient.NewClient(rpcClient)
		clients = append(clients, client)

		wsClient, err := ethclient.DialContext(ctx, nodeAddress.Websocket)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to ws node %s: %w", nodeAddress, err)
		}
		wsClients = append(wsClients, wsClient)
	}
	logger.Info("built clients", zap.Int("clients", len(clients)))

	wallets, err := wallet.NewWalletsFromSpec(logger, spec, clients)
	if err != nil {
		return nil, err
	}

	var distribution txfactory.TxDistribution
	if spec.InitialWallets > 0 && spec.InitialWallets < spec.NumWallets {
		distribution = txfactory.NewTxDistributionBootstrapped(logger, wallets, spec.InitialWallets)
	} else {
		distribution = txfactory.NewTxDistributionEven(wallets)
	}

	txf := txfactory.NewTxFactory(logger, chainCfg.TxOpts, distribution)
	nonces := sync.Map{}
	for i, wallet := range wallets {
		if i%10000 == 0 {
			logger.Info("Initializing nonces for accounts", zap.Int("progress", i))
		}
		nonce, err := wallet.GetClient().PendingNonceAt(ctx, wallet.Address())
		if err != nil {
			logger.Warn("Failed getting nonce for wallet setting to 0", zap.String("address", wallet.Address().String()))
		}
		nonces.Store(wallet.Address(), nonce)
	}

	// Create sender to wallet mapping for consistent endpoint usage
	senderToWallet := make(map[common.Address]*wallet.InteractingWallet)
	for _, w := range wallets {
		senderToWallet[w.Address()] = w
	}

	logger.Info("runner construction complete")

	r := &Runner{
		logger:          logger,
		clients:         clients,
		wsClients:       wsClients,
		spec:            spec,
		chainConfig:     *chainCfg,
		wallets:         wallets,
		txFactory:       txf,
		sentTxs:         make([]*inttypes.SentTx, 0, 100),
		blocksProcessed: 0,
		nonces:          &nonces,
		senderToWallet:  senderToWallet,
	}

	return r, nil
}

// getWalletForTx returns the appropriate wallet for sending a transaction based on the sender address.
// This ensures each sender consistently uses the same endpoint.
func (r *Runner) getWalletForTx(tx *gethtypes.Transaction) *wallet.InteractingWallet {
	// Extract sender address from the transaction
	signer := gethtypes.NewPragueSigner(tx.ChainId())
	sender, err := signer.Sender(tx)
	if err != nil {
		panic(fmt.Sprintf("failed to extract sender from transaction: %v", err))
	}

	// Get the wallet assigned to this sender
	wallet, exists := r.senderToWallet[sender]
	if !exists {
		panic(fmt.Sprintf("no wallet found for sender %s", sender.Hex()))
	}

	return wallet
}

func (r *Runner) PrintResults(result loadtesttypes.LoadTestResult) {
	metrics.PrintResults(result)
}

// ContractDeployer encapsulates the contract deployment configuration
type ContractDeployer struct {
	msgType        loadtesttypes.MsgType
	setAddressFunc func(...common.Address)
}

// deployContracts is a generic function that handles the common deployment logic
func (r *Runner) deployContracts(ctx context.Context, deployer ContractDeployer) error {
	numInitialDeploy := r.chainConfig.NumInitialContracts
	if numInitialDeploy == 0 {
		numInitialDeploy = 5
	}

	contractDeploy := loadtesttypes.LoadTestMsg{Type: deployer.msgType}
	deployedTxs := make([]*gethtypes.Transaction, 0)

	// Deploy contracts
	for range numInitialDeploy {
		txs, err := r.buildLoad(contractDeploy, false)
		if err != nil {
			return fmt.Errorf("failed to deploy contracts in PreRun: %w", err)
		}

		for _, tx := range txs {
			// Use the wallet assigned to this transaction's sender for consistent endpoint usage
			wallet := r.getWalletForTx(tx)
			if err := wallet.SendTransaction(ctx, tx); err != nil {
				return fmt.Errorf("failed to send transaction in PreRun: %w", err)
			}
		}

		// Get the main contract transaction (always the last one)
		mainContractTx := txs[len(txs)-1]
		deployedTxs = append(deployedTxs, mainContractTx)
	}

	// Wait for receipts and collect addresses
	addresses := make([]common.Address, len(deployedTxs))
	wg := sync.WaitGroup{}

	for i, tx := range deployedTxs {
		wg.Add(1)
		go func(index int, transaction *gethtypes.Transaction) {
			defer wg.Done()
			// Use the wallet assigned to this transaction's sender for consistent endpoint usage
			wallet := r.getWalletForTx(transaction)
			rec, err := wallet.WaitForTxReceipt(ctx, transaction.Hash(), 5*time.Second)
			if err == nil {
				addresses[index] = rec.ContractAddress
			} else {
				r.logger.Error("failed to find receipt", zap.String("msg_type", deployer.msgType.String()), zap.Error(err))
			}
		}(i, tx)
	}
	wg.Wait()

	// Set addresses in the factory
	for _, addr := range addresses {
		if addr.Cmp(common.Address{}) != 0 {
			deployer.setAddressFunc(addr)
		}
	}

	return nil
}

// deployWETH deploys WETH contracts using the generic deployment function
func (r *Runner) deployWETH(ctx context.Context) error {
	deployer := ContractDeployer{
		msgType:        inttypes.MsgDeployERC20,
		setAddressFunc: r.txFactory.SetWETHAddresses,
	}
	return r.deployContracts(ctx, deployer)
}

// deployLoader deploys loader contracts using the generic deployment function
func (r *Runner) deployLoader(ctx context.Context) error {
	deployer := ContractDeployer{
		msgType:        inttypes.MsgCreateContract,
		setAddressFunc: r.txFactory.SetLoaderAddresses,
	}
	return r.deployContracts(ctx, deployer)
}

// deployInitialContracts deploys the contracts needed for the messages in the spec.
func (r *Runner) deployInitialContracts(ctx context.Context) error {
	hasLoaderDependencies := slices.ContainsFunc(r.spec.Msgs, func(msg loadtesttypes.LoadTestMsg) bool {
		return slices.Contains(inttypes.LoaderDependencies, msg.Type)
	})
	hasERC20Dependencies := slices.ContainsFunc(r.spec.Msgs, func(msg loadtesttypes.LoadTestMsg) bool {
		return slices.Contains(inttypes.ERC20Dependencies, msg.Type)
	})
	r.logger.Info("deploy loader?", zap.Bool("hasLoaderDependencies", hasLoaderDependencies))
	r.logger.Info("deploy erc20?", zap.Bool("hasERC20Deps", hasERC20Dependencies))
	if hasLoaderDependencies {
		if err := r.deployLoader(ctx); err != nil {
			return err
		}
	}
	if hasERC20Dependencies {
		if err := r.deployWETH(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) Run(ctx context.Context) (loadtesttypes.LoadTestResult, error) {
	// when batches and interval are specified, user wants to run on a timed interval
	if r.spec.NumBatches > 0 && r.spec.SendInterval > 0 {
		r.logger.Info("Running loadtest on interval", zap.Duration("interval", r.spec.SendInterval), zap.Int("num_batches", r.spec.NumBatches))
		return r.runOnInterval(ctx)
	} else if r.spec.NumOfBlocks > 0 {
		r.logger.Info("Running loadtest on blocks", zap.Int("blocks", r.spec.NumOfBlocks))
		return r.runOnBlocks(ctx)
	}
	r.logger.Info("Running loadtest persistently")
	return r.runPersistent(ctx)
}

func (r *Runner) buildLoad(msgSpec loadtesttypes.LoadTestMsg, useBaseline bool) ([]*gethtypes.Transaction, error) {
	// For ERC20 transactions, use optimal sender selection from factory
	var fromWallet *wallet.InteractingWallet
	switch msgSpec.Type {
	case inttypes.MsgTransferERC0, inttypes.MsgNativeTransferERC20:
		fromWallet = r.txFactory.GetNextSender()
	case inttypes.MsgDeployERC20, inttypes.MsgCreateContract:
		fromWallet = r.wallets[0]
	default:
		// For non-ERC20 transactions, keep random selection
		fromWallet = r.wallets[rand.Intn(len(r.wallets))]
	}

	nonce, ok := r.nonces.Load(fromWallet.Address())
	if !ok {
		// this really should not happen ever. better safe than sorry.
		return nil, fmt.Errorf("nonce for wallet %s not found", fromWallet.Address())
	}
	txs, err := r.txFactory.BuildTxs(msgSpec, fromWallet, nonce.(uint64), useBaseline)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %q: %w", msgSpec.Type, err)
	}
	if len(txs) == 0 {
		return nil, nil
	}

	// some cases, like contract creation, will give us more than one tx to send.
	// the tx factory will correctly handle setting the correct nonces for these txs.
	// naturally, the final tx will have the latest nonce that should be set for the account.
	lastTx := txs[len(txs)-1]
	if lastTx == nil {
		return nil, nil
	}
	r.nonces.Store(fromWallet.Address(), lastTx.Nonce()+1)
	return txs, nil
}
