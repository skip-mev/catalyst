package wallet

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/ethereum/go-ethereum/crypto"
)

// ReadCachedPrivateKeys reads the cached private keys from the file and returns them as types.PrivKey
// The file is expected to be in the same format as the Ethereum keys cache.
// See ethereum/wallet/cache.go for more details.
func ReadCachedPrivateKeys(filename string, num int) ([]types.PrivKey, error) {
	bz, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open cache file %s: %w", filename, err)
	}

	type cachedKey struct {
		Signer struct {
			PK []byte
		}
	}

	var cachedKeys []cachedKey
	if err := json.Unmarshal(bz, &cachedKeys); err != nil {
		return nil, fmt.Errorf("decoding cached wallets: %w", err)
	}

	if num > 0 && num < len(cachedKeys) {
		cachedKeys = cachedKeys[:num]
	}

	privateKeys := make([]types.PrivKey, len(cachedKeys))
	for i, key := range cachedKeys {
		rawPK := key.Signer.PK

		ecdsaPK, err := crypto.ToECDSA(rawPK)
		if err != nil {
			return nil, fmt.Errorf("parsing cached wallet %d private key: %w", i, err)
		}

		privateKeys[i] = &ethsecp256k1.PrivKey{
			Key: crypto.FromECDSA(ecdsaPK),
		}
	}

	return privateKeys, nil
}
