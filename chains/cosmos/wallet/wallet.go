package wallet

import (
	"context"
	"fmt"
	"time"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	"github.com/skip-mev/catalyst/chains/cosmos/client"
	"github.com/skip-mev/catalyst/chains/cosmos/types"
)

// InteractingWallet represents a wallet that can interact with the chain
type InteractingWallet struct {
	signer *Signer
	client *client.Chain
}

// NewInteractingWallet creates a new wallet
func NewInteractingWallet(privKey cryptotypes.PrivKey, bech32Prefix string, client *client.Chain) *InteractingWallet {
	return &InteractingWallet{
		signer: NewSigner(privKey, bech32Prefix),
		client: client,
	}
}

func GetTxResponse(ctx context.Context, client types.ChainI, txHash string) (*sdk.TxResponse, error) {
	cometClient := client.GetCometClient()

	clientCtx := sdkclient.Context{}.
		WithClient(cometClient).
		WithTxConfig(client.GetEncodingConfig().TxConfig).
		WithInterfaceRegistry(client.GetEncodingConfig().InterfaceRegistry)

	txResp, err := authtx.QueryTx(clientCtx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction %s: %w", txHash, err)
	}

	return txResp, nil
}

// CreateSignedTx creates and signs a transaction
func (w *InteractingWallet) CreateSignedTx(
	ctx context.Context,
	client types.ChainI,
	gas uint64,
	fees sdk.Coins,
	sequence,
	accountNumber uint64,
	memo string,
	unordered bool,
	timeoutDuration time.Duration,
	msgs ...sdk.Msg,
) (sdk.Tx, error) {
	encodingConfig := client.GetEncodingConfig()

	txBuilder := encodingConfig.TxConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	txBuilder.SetGasLimit(gas)
	txBuilder.SetFeeAmount(fees)
	txBuilder.SetMemo(memo)
	txBuilder.SetUnordered(unordered)
	if unordered {
		txBuilder.SetTimeoutTimestamp(time.Now().Add(timeoutDuration))
	}

	chainID := client.GetChainID()
	if chainID == "" {
		return nil, fmt.Errorf("chain ID cannot be empty")
	}

	pubKey := w.signer.PublicKey()
	sig := signing.SignatureV2{
		PubKey: pubKey,
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
	}
	if !unordered {
		sig.Sequence = sequence
	}
	err := txBuilder.SetSignatures(sig)
	if err != nil {
		return nil, err
	}

	signerData := xauthsigning.SignerData{
		ChainID:       chainID,
		PubKey:        pubKey,
		AccountNumber: accountNumber,
	}
	if !unordered {
		signerData.Sequence = sequence
	}

	sigV2, err := w.signer.SignTx(signerData, txBuilder, encodingConfig.TxConfig)
	if err != nil {
		return nil, err
	}

	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, err
	}

	return txBuilder.GetTx(), nil
}

// FormattedAddress returns the Bech32 formatted address for the wallet
func (w *InteractingWallet) FormattedAddress() string {
	return w.signer.FormattedAddress()
}

// Address returns the raw address bytes for the wallet
func (w *InteractingWallet) Address() []byte {
	return w.signer.Address()
}

func (w *InteractingWallet) GetClient() *client.Chain {
	return w.client
}
