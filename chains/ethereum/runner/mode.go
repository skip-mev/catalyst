package runner

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"

	ethift "github.com/skip-mev/catalyst/chains/ethereum/ift"
	inttypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	"github.com/skip-mev/catalyst/chains/ethereum/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	iftaccounts "github.com/skip-mev/catalyst/ift/accounts"
	iftrelayer "github.com/skip-mev/catalyst/ift/relayer"
)

type txMode interface {
	Prepare(ctx context.Context) error
	SetBaselines(ctx context.Context, msgs []loadtesttypes.LoadTestMsg) error
	ResetAllocation()
	BuildLoad(msgSpec loadtesttypes.LoadTestMsg, useBaseline bool) ([]*gethtypes.Transaction, error)
	HandlePostBroadcast(ctx context.Context, msgType loadtesttypes.MsgType, txHash common.Hash, sourceErr error) error
}

type localTxMode struct {
	runner *Runner
}

func newLocalTxMode(runner *Runner) txMode {
	return &localTxMode{runner: runner}
}

func (m *localTxMode) Prepare(ctx context.Context) error {
	return m.runner.deployInitialContracts(ctx)
}

func (m *localTxMode) SetBaselines(ctx context.Context, msgs []loadtesttypes.LoadTestMsg) error {
	return m.runner.txFactory.SetBaselines(ctx, msgs)
}

func (m *localTxMode) ResetAllocation() {
	m.runner.txFactory.ResetWalletAllocation()
}

func (m *localTxMode) BuildLoad(msgSpec loadtesttypes.LoadTestMsg, useBaseline bool) ([]*gethtypes.Transaction, error) {
	var fromWallet *wallet.InteractingWallet
	switch msgSpec.Type {
	case inttypes.MsgTransferERC0, inttypes.MsgNativeTransferERC20:
		fromWallet = m.runner.txFactory.GetNextSender()
	case inttypes.MsgDeployERC20, inttypes.MsgCreateContract:
		fromWallet = m.runner.wallets[0]
	default:
		fromWallet = m.runner.wallets[rand.Intn(len(m.runner.wallets))]
	}

	nonce, ok := m.runner.nonces.Load(fromWallet.Address())
	if !ok {
		return nil, fmt.Errorf("nonce for wallet %s not found", fromWallet.Address())
	}

	txs, err := m.runner.txFactory.BuildTxs(msgSpec, fromWallet, nonce.(uint64), useBaseline)
	if err != nil {
		return nil, fmt.Errorf("failed to build tx for %q: %w", msgSpec.Type, err)
	}
	if len(txs) == 0 {
		return nil, nil
	}

	lastTx := txs[len(txs)-1]
	if lastTx == nil {
		return nil, nil
	}

	m.runner.nonces.Store(fromWallet.Address(), lastTx.Nonce()+1)
	return txs, nil
}

func (m *localTxMode) HandlePostBroadcast(context.Context, loadtesttypes.MsgType, common.Hash, error) error {
	return nil
}

type iftTxMode struct {
	runner     *Runner
	cfg        *loadtesttypes.IFTConfig
	recipients []string
	relayer    iftrelayer.Client
	contract   *ethift.TransferContract
	amount     *big.Int
}

func newIFTTxMode(runner *Runner) (txMode, error) {
	recipients, err := iftaccounts.GenerateRecipients(runner.spec)
	if err != nil {
		return nil, fmt.Errorf("generate ift recipients: %w", err)
	}

	relayerClient, err := iftrelayer.NewGRPCClient(runner.spec.IFT.Relayer, runner.spec.ChainID)
	if err != nil {
		return nil, fmt.Errorf("create ift relayer client: %w", err)
	}

	contract, err := ethift.NewTransferContract(runner.spec.IFT.EVM.ContractAddress)
	if err != nil {
		return nil, fmt.Errorf("create ift transfer contract: %w", err)
	}

	amount, ok := new(big.Int).SetString(runner.spec.IFT.Amount, 10)
	if !ok {
		return nil, fmt.Errorf("parse ift.amount %q", runner.spec.IFT.Amount)
	}

	return &iftTxMode{
		runner:     runner,
		cfg:        runner.spec.IFT,
		recipients: recipients,
		relayer:    relayerClient,
		contract:   contract,
		amount:     amount,
	}, nil
}

func (m *iftTxMode) Prepare(context.Context) error {
	return nil
}

func (m *iftTxMode) SetBaselines(context.Context, []loadtesttypes.LoadTestMsg) error {
	return nil
}

func (m *iftTxMode) ResetAllocation() {}

func (m *iftTxMode) BuildLoad(msgSpec loadtesttypes.LoadTestMsg, _ bool) ([]*gethtypes.Transaction, error) {
	if msgSpec.Type != inttypes.MsgIFTTransfer {
		return nil, fmt.Errorf("unsupported message type %s for ift mode", msgSpec.Type)
	}
	if len(m.recipients) == 0 {
		return nil, fmt.Errorf("no ift recipients configured")
	}

	fromWallet := m.runner.wallets[rand.Intn(len(m.runner.wallets))]
	nonce, ok := m.runner.nonces.Load(fromWallet.Address())
	if !ok {
		return nil, fmt.Errorf("nonce for wallet %s not found", fromWallet.Address())
	}

	timeout := uint64(time.Now().Add(m.cfg.Timeout).UnixNano())
	receiver := m.recipients[rand.Intn(len(m.recipients))]
	tx, err := m.contract.BuildTransferTx(
		context.Background(),
		fromWallet,
		m.cfg.ClientID,
		receiver,
		new(big.Int).Set(m.amount),
		timeout,
		nonce.(uint64),
		m.runner.chainConfig.TxOpts.GasFeeCap,
		m.runner.chainConfig.TxOpts.GasTipCap,
	)
	if err != nil {
		return nil, fmt.Errorf("build ift transfer tx: %w", err)
	}

	m.runner.nonces.Store(fromWallet.Address(), tx.Nonce()+1)
	return []*gethtypes.Transaction{tx}, nil
}

func (m *iftTxMode) HandlePostBroadcast(ctx context.Context, msgType loadtesttypes.MsgType, txHash common.Hash, sourceErr error) error {
	if sourceErr != nil || msgType != inttypes.MsgIFTTransfer {
		return nil
	}
	return m.relayer.SubmitTxHash(ctx, txHash.Hex())
}
