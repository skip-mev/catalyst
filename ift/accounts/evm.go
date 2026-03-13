package accounts

import (
	"fmt"
	"strconv"
	"strings"

	ethhd "github.com/cosmos/evm/crypto/hd"
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
		addr, err := generateEVMAddress(g.mnemonic, offset+i)
		if err != nil {
			return nil, err
		}

		recipients = append(recipients, addr)
	}

	return recipients, nil
}

func generateEVMAddress(mnemonic string, index int) (string, error) {
	passphrase := strconv.Itoa(index)
	if index == 0 {
		passphrase = ""
	}

	derivedPrivKey, err := ethhd.EthSecp256k1.Derive()(mnemonic, passphrase, evmDerivationPath)
	if err != nil {
		return "", fmt.Errorf("derive evm recipient key: %w", err)
	}

	pk, err := crypto.ToECDSA(derivedPrivKey)
	if err != nil {
		return "", fmt.Errorf("parse evm recipient key: %w", err)
	}

	return crypto.PubkeyToAddress(pk.PublicKey).Hex(), nil
}
