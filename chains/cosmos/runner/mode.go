package runner

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	cosmosift "github.com/skip-mev/catalyst/chains/cosmos/ift"
	"github.com/skip-mev/catalyst/chains/cosmos/txfactory"
	inttypes "github.com/skip-mev/catalyst/chains/cosmos/types"
	"github.com/skip-mev/catalyst/chains/cosmos/wallet"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	iftaccounts "github.com/skip-mev/catalyst/ift/accounts"
	iftrelayer "github.com/skip-mev/catalyst/ift/relayer"
)

type txMode interface {
	CreateMessages(msgSpec loadtesttypes.LoadTestMsg, fromWallet *wallet.InteractingWallet) ([]sdk.Msg, error)
	HandlePostBroadcast(ctx context.Context, msgType loadtesttypes.MsgType, txHash string) error
}

type localTxMode struct {
	txFactory *txfactory.TxFactory
}

func newLocalTxMode(gasDenom string, wallets []*wallet.InteractingWallet) txMode {
	return &localTxMode{
		txFactory: txfactory.NewTxFactory(gasDenom, wallets),
	}
}

func (m *localTxMode) CreateMessages(
	msgSpec loadtesttypes.LoadTestMsg,
	fromWallet *wallet.InteractingWallet,
) ([]sdk.Msg, error) {
	if msgSpec.Type == inttypes.MsgArr {
		if msgSpec.ContainedType == "" {
			return nil, fmt.Errorf("msgSpec.ContainedType must not be empty")
		}

		return m.txFactory.CreateMsgs(msgSpec, fromWallet)
	}

	msg, err := m.txFactory.CreateMsg(msgSpec, fromWallet)
	if err != nil {
		return nil, err
	}

	return []sdk.Msg{msg}, nil
}

func (m *localTxMode) HandlePostBroadcast(context.Context, loadtesttypes.MsgType, string) error {
	return nil
}

type iftTxMode struct {
	cfg        *loadtesttypes.IFTConfig
	recipients []string
	relayer    iftrelayer.Client
}

func newIFTTxMode(spec loadtesttypes.LoadTestSpec) (txMode, error) {
	recipients, err := iftaccounts.GenerateRecipients(spec)
	if err != nil {
		return nil, fmt.Errorf("generate ift recipients: %w", err)
	}

	relayerClient, err := iftrelayer.NewGRPCClient(spec.IFT.Relayer, spec.ChainID)
	if err != nil {
		return nil, fmt.Errorf("create ift relayer client: %w", err)
	}

	return &iftTxMode{
		cfg:        spec.IFT,
		recipients: recipients,
		relayer:    relayerClient,
	}, nil
}

func (m *iftTxMode) CreateMessages(
	msgSpec loadtesttypes.LoadTestMsg,
	fromWallet *wallet.InteractingWallet,
) ([]sdk.Msg, error) {
	if msgSpec.Type != inttypes.MsgIFTTransfer {
		return nil, fmt.Errorf("unsupported message type %s for ift mode", msgSpec.Type)
	}

	if len(m.recipients) == 0 {
		return nil, fmt.Errorf("no ift recipients configured")
	}

	receiver := m.recipients[rand.Intn(len(m.recipients))]
	timeout := uint64(time.Now().Add(m.cfg.Timeout).Unix())

	return []sdk.Msg{
		&cosmosift.MsgIFTTransfer{
			Signer:           fromWallet.FormattedAddress(),
			Denom:            m.cfg.Cosmos.Denom,
			ClientId:         m.cfg.ClientID,
			Receiver:         receiver,
			Amount:           m.cfg.Amount,
			TimeoutTimestamp: timeout,
		},
	}, nil
}

func (m *iftTxMode) HandlePostBroadcast(ctx context.Context, msgType loadtesttypes.MsgType, txHash string) error {
	if msgType != inttypes.MsgIFTTransfer {
		return nil
	}

	return m.relayer.SubmitTxHash(ctx, txHash)
}
