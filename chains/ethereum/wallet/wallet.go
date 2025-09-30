package wallet

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	ethhd "github.com/cosmos/evm/crypto/hd"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"go.uber.org/zap"
)

// InteractingWallet represents a wallet that can interact with the Ethereum chain
type InteractingWallet struct {
	signer *Signer
	client Client
}

// NewWalletsFromSpec builds wallets from the spec. It takes the `BaseMnemonic` and derives all keys from this mnemonic
// by using an increasing bip passphrase. The passphrase value is an integer from [0,spec.NumWallets).
func NewWalletsFromSpec(logger *zap.Logger, spec loadtesttypes.LoadTestSpec, clients []*ethclient.Client) ([]*InteractingWallet, error) {
	if len(clients) == 0 {
		return nil, fmt.Errorf("no clients provided")
	}

	chainIDStr := strings.TrimSpace(spec.ChainID)
	chainID, ok := new(big.Int).SetString(chainIDStr, 0) // allow "9001" or "0x2329"
	if !ok {
		return nil, fmt.Errorf("failed to parse chain id: %q", spec.ChainID)
	}

	// EXACT path used by 'eth_secp256k1' default account in Ethermint-based chains.
	const evmDerivationPath = "m/44'/60'/0'/0/0"

	ws := make([]*InteractingWallet, spec.NumWallets)
	m := strings.TrimSpace(spec.BaseMnemonic)
	if m == "" {
		return nil, errors.New("BaseMnemonic is empty")
	}
	logger.Info("building wallets", zap.Int("num_wallets", spec.NumWallets))
	for i := range spec.NumWallets {
		// derive raw 32-byte private key from mnemonic at ETH path .../0
		derivedPrivKey, err := ethhd.EthSecp256k1.Derive()(m, strconv.Itoa(i), evmDerivationPath)
		if err != nil {
			return nil, fmt.Errorf("mnemonic[%d]: derive failed: %w", i, err)
		}

		pk, err := crypto.ToECDSA(derivedPrivKey)
		if err != nil {
			return nil, fmt.Errorf("mnemonic[%d]: invalid ECDSA key: %w", i, err)
		}

		c := clients[i%len(clients)]
		w := NewInteractingWallet(pk, chainID, c)
		ws[i] = w
		if i%10_000 == 0 {
			logger.Info("wallets built", zap.Int("num_wallets", i))
		}
	}
	logger.Info("completed building wallets")
	return ws, nil
}

// NewInteractingWallet creates a new Ethereum wallet
func NewInteractingWallet(privKey *ecdsa.PrivateKey, chainID *big.Int, client Client) *InteractingWallet {
	return &InteractingWallet{
		signer: NewSigner(privKey, chainID),
		client: client,
	}
}

// GetTxReceipt retrieves a transaction receipt by hash
func GetTxReceipt(ctx context.Context, client Client, txHash common.Hash) (*types.Receipt, error) {
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to find transaction %s: %w", txHash.Hex(), err)
	}
	return receipt, nil
}

// GetTxByHash retrieves a transaction by hash
func GetTxByHash(ctx context.Context, client Client, txHash common.Hash) (*types.Transaction, bool, error) {
	tx, isPending, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, false, fmt.Errorf("failed to find transaction %s: %w", txHash.Hex(), err)
	}
	return tx, isPending, nil
}

// getNonce returns the nonce to use for a transaction, either from the provided value or by querying the client
func (w *InteractingWallet) getNonce(ctx context.Context, nonce *uint64) (uint64, error) {
	if nonce != nil {
		return *nonce, nil
	}

	txNonce, err := w.client.PendingNonceAt(ctx, w.signer.Address())
	if err != nil {
		return 0, fmt.Errorf("failed to get nonce: %w", err)
	}
	return txNonce, nil
}

// estimateGasWithBuffer estimates gas for a transaction and adds a 10% buffer
func (w *InteractingWallet) estimateGasWithBuffer(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	gasLimit, err := w.client.EstimateGas(ctx, msg)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate gas: %w", err)
	}

	// add a 20% buffer to limit (common practice)
	buffer := gasLimit / 50
	return gasLimit + buffer, nil
}

func (w *InteractingWallet) SignerFn() bind.SignerFn {
	return w.signer.SignerFn()
}

func (w *InteractingWallet) SignerFnLegacy() bind.SignerFn {
	return w.signer.SignLegacyTxFn()
}

// CreateSignedTransaction creates and signs an Ethereum transaction
func (w *InteractingWallet) CreateSignedTransaction(ctx context.Context, to *common.Address, value *big.Int,
	gasLimit uint64, gasPrice *big.Int, data []byte, nonce *uint64,
) (*types.Transaction, error) {
	// Get nonce if not provided
	txNonce, err := w.getNonce(ctx, nonce)
	if err != nil {
		return nil, err
	}

	// Get gas price if not provided
	if gasPrice == nil {
		var err error
		gasPrice, err = w.client.SuggestGasPrice(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get gas price: %w", err)
		}
	}

	// Estimate gas if not provided
	if gasLimit == 0 {
		msg := ethereum.CallMsg{
			From:     w.signer.Address(),
			To:       to, // nil for contract creation
			Value:    value,
			Data:     data,
			GasPrice: gasPrice,
		}
		var err error
		gasLimit, err = w.estimateGasWithBuffer(ctx, msg)
		if err != nil {
			return nil, err
		}
	}

	// Create transaction
	var tx *types.Transaction
	if to != nil {
		tx = types.NewTransaction(txNonce, *to, value, gasLimit, gasPrice, data)
	} else {
		tx = types.NewContractCreation(txNonce, value, gasLimit, gasPrice, data)
	}

	// Sign transaction
	signedTx, err := w.signer.SignTx(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return signedTx, nil
}

// CreateSignedDynamicFeeTx creates and signs an EIP-1559 transaction with dynamic fees
func (w *InteractingWallet) CreateSignedDynamicFeeTx(ctx context.Context, to *common.Address, value *big.Int,
	gasLimit uint64, gasFeeCap, gasTipCap *big.Int, data []byte, nonce *uint64,
) (*types.Transaction, error) {
	// Get nonce if not provided
	txNonce, err := w.getNonce(ctx, nonce)
	if err != nil {
		return nil, err
	}

	// Get suggested gas prices if not provided
	if gasFeeCap == nil || gasTipCap == nil {
		header, err := w.client.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest header: %w", err)
		}

		if gasFeeCap == nil {
			// gasFeeCap = baseFee * 2 + gasTipCap (reasonable default)
			baseFee := header.BaseFee
			if gasTipCap == nil {
				gasTipCap = big.NewInt(2000000000) // 2 gwei default tip
			}
			gasFeeCap = new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), gasTipCap)
		}
		if gasTipCap == nil {
			gasTipCap = big.NewInt(2000000000) // 2 gwei default tip
		}
	}

	// Estimate gas if not provided
	if gasLimit == 0 {
		msg := ethereum.CallMsg{
			From:      w.signer.Address(),
			To:        to, // nil for contract creation
			Value:     value,
			Data:      data,
			GasFeeCap: gasFeeCap,
			GasTipCap: gasTipCap,
		}
		var err error
		gasLimit, err = w.estimateGasWithBuffer(ctx, msg)
		if err != nil {
			return nil, err
		}
	}

	// Create EIP-1559 transaction
	tx := w.signer.CreateDynamicFeeTransaction(to, value, gasLimit, gasFeeCap, gasTipCap, data, txNonce)

	// Sign transaction
	signedTx, err := w.signer.SignDynamicFeeTx(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	return signedTx, nil
}

// SendTransaction broadcasts a signed transaction to the network
func (w *InteractingWallet) SendTransaction(ctx context.Context, signedTx *types.Transaction) error {
	return w.client.SendTransaction(ctx, signedTx)
}

// CreateAndSendTransaction creates, signs, and sends a transaction in one call
func (w *InteractingWallet) CreateAndSendTransaction(ctx context.Context, to *common.Address, value *big.Int,
	gasLimit uint64, gasPrice *big.Int, data []byte, nonce *uint64,
) (common.Hash, error) {
	signedTx, err := w.CreateSignedTransaction(ctx, to, value, gasLimit, gasPrice, data, nonce)
	if err != nil {
		return common.Hash{}, err
	}

	err = w.SendTransaction(ctx, signedTx)
	if err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}

// CreateAndSendDynamicFeeTx creates, signs, and sends an EIP-1559 transaction in one call
func (w *InteractingWallet) CreateAndSendDynamicFeeTx(ctx context.Context, to *common.Address, value *big.Int,
	gasLimit uint64, gasFeeCap, gasTipCap *big.Int, data []byte, nonce *uint64,
) (common.Hash, error) {
	signedTx, err := w.CreateSignedDynamicFeeTx(ctx, to, value, gasLimit, gasFeeCap, gasTipCap, data, nonce)
	if err != nil {
		return common.Hash{}, err
	}

	err = w.SendTransaction(ctx, signedTx)
	if err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}

// WaitForTxReceipt waits for a transaction to be included in a block and returns the receipt
func (w *InteractingWallet) WaitForTxReceipt(ctx context.Context, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := w.client.TransactionReceipt(ctx, txHash)
			if err == nil {
				return receipt, nil
			}
			// Continue if transaction not found yet
		}
	}
}

// FormattedAddress returns the hex-encoded Ethereum address
func (w *InteractingWallet) FormattedAddress() string {
	return w.signer.FormattedAddress()
}

// Address returns the Ethereum address
func (w *InteractingWallet) Address() common.Address {
	return w.signer.Address()
}

// GetClient returns the Ethereum client
func (w *InteractingWallet) GetClient() Client {
	return w.client
}

// GetBalance returns the account balance
func (w *InteractingWallet) GetBalance(ctx context.Context) (*big.Int, error) {
	return w.client.BalanceAt(ctx, w.signer.Address(), nil)
}

// GetNonce returns the current nonce for the account
func (w *InteractingWallet) GetNonce(ctx context.Context) (uint64, error) {
	return w.client.PendingNonceAt(ctx, w.signer.Address())
}

// GetGasPrice returns the current suggested gas price
func (w *InteractingWallet) GetGasPrice(ctx context.Context) (*big.Int, error) {
	return w.client.SuggestGasPrice(ctx)
}
