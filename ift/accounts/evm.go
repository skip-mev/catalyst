package accounts

import (
	"crypto/ecdsa"
	"fmt"
	"strconv"
	"strings"

	ethhd "github.com/cosmos/evm/crypto/hd"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const evmDerivationPath = "m/44'/60'/0'/0/0"

type evmGenerator struct {
	mnemonic string
}

func newEVMGenerator(mnemonic string) Generator {
	return &evmGenerator{mnemonic: strings.TrimSpace(mnemonic)}
}

func (g *evmGenerator) GenerateRecipients(count, offset int) ([]string, error) {
	recipients := make([]string, 0, count)
	for i := range count {
		addr, err := evmAddressHex(g.mnemonic, offset+i)
		if err != nil {
			return nil, err
		}

		recipients = append(recipients, addr)
	}

	return recipients, nil
}

// DeriveEVMKey derives the secp256k1 key Catalyst uses for wallet index.
// Every index uses path m/44'/60'/0'/0/0. The index is the BIP39 passphrase:
// empty for 0, then "1", "2", and so on.
func DeriveEVMKey(mnemonic string, index int) (*ecdsa.PrivateKey, error) {
	passphrase := strconv.Itoa(index)
	if index == 0 {
		passphrase = ""
	}

	derivedPrivKey, err := ethhd.EthSecp256k1.Derive()(strings.TrimSpace(mnemonic), passphrase, evmDerivationPath)
	if err != nil {
		return nil, fmt.Errorf("derive evm key %d: %w", index, err)
	}

	pk, err := crypto.ToECDSA(derivedPrivKey)
	if err != nil {
		return nil, fmt.Errorf("parse evm key %d: %w", index, err)
	}
	return pk, nil
}

// EVMAddressFromMnemonic is the address of DeriveEVMKey.
func EVMAddressFromMnemonic(mnemonic string, index int) (common.Address, error) {
	pk, err := DeriveEVMKey(mnemonic, index)
	if err != nil {
		return common.Address{}, err
	}
	return crypto.PubkeyToAddress(pk.PublicKey), nil
}

func evmAddressHex(mnemonic string, index int) (string, error) {
	addr, err := EVMAddressFromMnemonic(mnemonic, index)
	if err != nil {
		return "", err
	}
	return addr.Hex(), nil
}
