package accounts

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const cosmosDerivationPath = "44'/118'/0'/0/0"

type cosmosGenerator struct {
	mnemonic     string
	bech32Prefix string
}

func newCosmosGenerator(mnemonic, bech32Prefix string) Generator {
	return &cosmosGenerator{
		mnemonic:     strings.TrimSpace(mnemonic),
		bech32Prefix: bech32Prefix,
	}
}

func (g *cosmosGenerator) GenerateRecipients(count, offset int) ([]string, error) {
	recipients := make([]string, 0, count)
	for i := range count {
		addr, err := generateCosmosAddress(g.mnemonic, g.bech32Prefix, offset+i)
		if err != nil {
			return nil, err
		}

		recipients = append(recipients, addr)
	}

	return recipients, nil
}

func generateCosmosAddress(mnemonic, bech32Prefix string, index int) (string, error) {
	derivedPrivKey, err := hd.Secp256k1.Derive()(mnemonic, strconv.Itoa(index), cosmosDerivationPath)
	if err != nil {
		return "", fmt.Errorf("derive cosmos recipient key: %w", err)
	}

	privKey := &secp256k1.PrivKey{Key: derivedPrivKey}
	return sdk.MustBech32ifyAddressBytes(bech32Prefix, sdk.AccAddress(privKey.PubKey().Address())), nil
}
