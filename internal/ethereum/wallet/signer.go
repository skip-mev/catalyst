package wallet

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Signer handles key management and signing for Ethereum transactions
type Signer struct {
	privKey *ecdsa.PrivateKey
	chainID *big.Int
}

// NewSigner creates a new Ethereum signer with the given private key and chain ID
func NewSigner(privKey *ecdsa.PrivateKey, chainID *big.Int) *Signer {
	return &Signer{
		privKey: privKey,
		chainID: chainID,
	}
}

// Address returns the Ethereum address derived from the private key
func (s *Signer) Address() common.Address {
	return crypto.PubkeyToAddress(s.privKey.PublicKey)
}

// FormattedAddress returns the hex-encoded Ethereum address with 0x prefix
func (s *Signer) FormattedAddress() string {
	return s.Address().Hex()
}

// SignTx signs an Ethereum transaction with the private key
func (s *Signer) SignTx(tx *types.Transaction) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(s.chainID)
	return types.SignTx(tx, signer, s.privKey)
}

// SignLegacyTx signs a legacy (pre-EIP155) transaction
func (s *Signer) SignLegacyTx(tx *types.Transaction) (*types.Transaction, error) {
	signer := types.NewLondonSigner(s.chainID)
	return types.SignTx(tx, signer, s.privKey)
}

// SignDynamicFeeTx signs an EIP-1559 dynamic fee transaction
func (s *Signer) SignDynamicFeeTx(tx *types.Transaction) (*types.Transaction, error) {
	signer := types.NewLondonSigner(s.chainID)
	return types.SignTx(tx, signer, s.privKey)
}

// SignMessage signs an arbitrary message hash
func (s *Signer) SignMessage(messageHash []byte) ([]byte, error) {
	return crypto.Sign(messageHash, s.privKey)
}

// SignPersonalMessage signs a message with Ethereum's personal message format
// This prepends "\x19Ethereum Signed Message:\n" + len(message) to the message
func (s *Signer) SignPersonalMessage(message []byte) ([]byte, error) {
	hash := crypto.Keccak256Hash(
		[]byte("\x19Ethereum Signed Message:\n"),
		[]byte(string(rune(len(message)))),
		message,
	)
	return crypto.Sign(hash.Bytes(), s.privKey)
}

// PrivateKey returns the private key
func (s *Signer) PrivateKey() *ecdsa.PrivateKey {
	return s.privKey
}

// PublicKey returns the public key
func (s *Signer) PublicKey() *ecdsa.PublicKey {
	return &s.privKey.PublicKey
}

// ChainID returns the chain ID used for signing
func (s *Signer) ChainID() *big.Int {
	return s.chainID
}

// CreateTransaction creates a new transaction with the given parameters
func (s *Signer) CreateTransaction(to *common.Address, value *big.Int, gas uint64, gasPrice *big.Int, data []byte, nonce uint64) *types.Transaction {
	return types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gas,
		To:       to,
		Value:    value,
		Data:     data,
	})
}

// CreateDynamicFeeTransaction creates a new EIP-1559 transaction with dynamic fees
func (s *Signer) CreateDynamicFeeTransaction(to *common.Address, value *big.Int, gas uint64, gasFeeCap, gasTipCap *big.Int, data []byte, nonce uint64) *types.Transaction {
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   s.chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gas,
		To:        to,
		Value:     value,
		Data:      data,
	})
}
