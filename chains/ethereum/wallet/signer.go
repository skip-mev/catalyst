package wallet

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// Signer handles key management and signing for Ethereum transactions
type Signer struct {
	PrivKey *ecdsa.PrivateKey
	ChainID *big.Int
}

// NewSigner creates a new Ethereum signer with the given private key and chain ID
func NewSigner(privKey *ecdsa.PrivateKey, chainID *big.Int) *Signer {
	return &Signer{
		PrivKey: privKey,
		ChainID: chainID,
	}
}

func (s *Signer) MarshalJSON() ([]byte, error) {
	pk := crypto.FromECDSA(s.PrivKey)
	chainID := s.ChainID.Int64()

	return json.Marshal(&struct {
		PK      []byte
		ChainID int64
	}{
		PK:      pk,
		ChainID: chainID,
	})
}

func (s *Signer) UnmarshalJSON(data []byte) error {
	aux := &struct {
		PK      []byte
		ChainID int64
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	pk, err := crypto.ToECDSA(aux.PK)
	if err != nil {
		return fmt.Errorf("parsing private key: %w", err)
	}
	s.PrivKey = pk

	s.ChainID = big.NewInt(aux.ChainID)
	return nil
}

// Address returns the Ethereum address derived from the private key
func (s *Signer) Address() common.Address {
	return crypto.PubkeyToAddress(s.PrivKey.PublicKey)
}

// FormattedAddress returns the hex-encoded Ethereum address with 0x prefix
func (s *Signer) FormattedAddress() string {
	return s.Address().Hex()
}

// SignTx signs an Ethereum transaction with the private key
func (s *Signer) SignTx(tx *types.Transaction) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(s.ChainID)
	return types.SignTx(tx, signer, s.PrivKey)
}

// SignLegacyTx signs a legacy (pre-EIP155) transaction
func (s *Signer) SignLegacyTx(tx *types.Transaction) (*types.Transaction, error) {
	signer := types.NewLondonSigner(s.ChainID)
	return types.SignTx(tx, signer, s.PrivKey)
}

func (s *Signer) SignerFn() bind.SignerFn {
	return func(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
		return s.SignTx(tx)
	}
}

func (s *Signer) SignLegacyTxFn() bind.SignerFn {
	return func(address common.Address, transaction *types.Transaction) (*types.Transaction, error) {
		return s.SignLegacyTx(transaction)
	}
}

// SignDynamicFeeTx signs an EIP-1559 dynamic fee transaction
func (s *Signer) SignDynamicFeeTx(tx *types.Transaction) (*types.Transaction, error) {
	signer := types.NewLondonSigner(s.ChainID)
	return types.SignTx(tx, signer, s.PrivKey)
}

// SignMessage signs an arbitrary message hash
func (s *Signer) SignMessage(messageHash []byte) ([]byte, error) {
	return crypto.Sign(messageHash, s.PrivKey)
}

// SignPersonalMessage signs a message with Ethereum's personal message format
// This prepends "\x19Ethereum Signed Message:\n" + len(message) to the message
func (s *Signer) SignPersonalMessage(message []byte) ([]byte, error) {
	hash := crypto.Keccak256Hash(
		[]byte("\x19Ethereum Signed Message:\n"),
		[]byte(string(rune(len(message)))),
		message,
	)
	return crypto.Sign(hash.Bytes(), s.PrivKey)
}

// PrivateKey returns the private key
func (s *Signer) PrivateKey() *ecdsa.PrivateKey {
	return s.PrivKey
}

// PublicKey returns the public key
func (s *Signer) PublicKey() *ecdsa.PublicKey {
	return &s.PrivKey.PublicKey
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
		ChainID:   s.ChainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gas,
		To:        to,
		Value:     value,
		Data:      data,
	})
}
