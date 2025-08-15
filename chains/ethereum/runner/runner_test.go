package runner

import (
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"github.com/stretchr/testify/require"
)

func TestBuildWallets(t *testing.T) {
	mnems := []string{
		"copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom",
		"maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual",
		"will wear settle write dance topic tape sea glory hotel oppose rebel client problem era video gossip glide during yard balance cancel file rose",
		"doll midnight silk carpet brush boring pluck office gown inquiry duck chief aim exit gain never tennis crime fragile ship cloud surface exotic patch",
	}
	expectedAddrs := []string{
		"0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101",
		"0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17",
		"0x40a0cb1C63e026A81B55EE1308586E21eec1eFa9",
		"0x498B5AeC5D439b733dC2F58AB489783A23FB26dA",
	}
	spec := loadtesttypes.LoadTestSpec{
		Mnemonics: mnems,
		ChainID:   "262144",
	}

	client := &ethclient.Client{}

	wallets, err := buildWallets(spec, []*ethclient.Client{client})
	require.NoError(t, err)

	for i, wallet := range wallets {
		require.Equal(t, expectedAddrs[i], wallet.Address().String())
	}
}
