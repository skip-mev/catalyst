package ift

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	iftbindings "github.com/skip-mev/catalyst/chains/ethereum/contracts/load/ift"
	ethwallet "github.com/skip-mev/catalyst/chains/ethereum/wallet"
)

func TestTransferContract(t *testing.T) {
	t.Run("NewTransferContract", func(t *testing.T) {
		for _, tt := range []struct {
			name        string
			address     string
			errContains string
			assert      func(t *testing.T, c *TransferContract)
		}{
			{
				name:    "validHexAddress",
				address: "0x1234567890123456789012345678901234567890",
				assert: func(t *testing.T, c *TransferContract) {
					t.Helper()
					assert.Equal(t, common.HexToAddress("0x1234567890123456789012345678901234567890"), c.Address())
				},
			},
			{
				name:    "validChecksumAddress",
				address: common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Hex(),
				assert: func(t *testing.T, c *TransferContract) {
					t.Helper()
					assert.Equal(t, common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"), c.Address())
				},
			},
			{
				name:        "emptyAddress",
				address:     "",
				errContains: `invalid IFT contract address ""`,
			},
			{
				name:        "nonHexAddress",
				address:     "not-an-address",
				errContains: `invalid IFT contract address "not-an-address"`,
			},
			{
				name:        "tooShortHexAddress",
				address:     "0x1234",
				errContains: `invalid IFT contract address "0x1234"`,
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				// ARRANGE
				addr := tt.address

				// ACT
				contract, err := NewTransferContract(addr)

				// ASSERT
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
					assert.Nil(t, contract)
					return
				}

				require.NoError(t, err)
				require.NotNil(t, contract)
				tt.assert(t, contract)
			})
		}
	})

	t.Run("Address", func(t *testing.T) {
		// ARRANGE
		want := common.HexToAddress("0x1111111111111111111111111111111111111111")
		contract, err := NewTransferContract(want.Hex())
		require.NoError(t, err)

		// ACT
		got := contract.Address()

		// ASSERT
		assert.Equal(t, want, got)
	})

	t.Run("BuildTransferTx", func(t *testing.T) {
		t.Run("buildsGaslimitedSignedTx", func(t *testing.T) {
			// ARRANGE
			wallet := newTestWallet(t)
			contract, err := NewTransferContract(
				common.HexToAddress("0x2222222222222222222222222222222222222222").Hex(),
			)
			require.NoError(t, err)

			ctx := context.Background()
			clientID := "client-0"
			receiver := "cosmos1receiver"
			amount := big.NewInt(42)
			timeoutTimestamp := uint64(1_700_000_000)
			nonce := uint64(7)
			gasFeeCap := big.NewInt(30_000_000_000)
			gasTipCap := big.NewInt(2_000_000_000)
			gasLimit := uint64(100_000)

			expectedData, err := expectedIFTCalldata(clientID, receiver, amount, timeoutTimestamp)
			require.NoError(t, err)

			// ACT
			tx, err := contract.BuildTransferTx(
				ctx,
				wallet,
				clientID,
				receiver,
				amount,
				timeoutTimestamp,
				nonce,
				gasFeeCap,
				gasTipCap,
				gasLimit,
			)

			// ASSERT
			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, uint8(types.DynamicFeeTxType), tx.Type())
			assert.Equal(t, nonce, tx.Nonce())
			assert.Equal(t, gasLimit, tx.Gas())
			assert.Equal(t, 0, tx.GasFeeCap().Cmp(gasFeeCap))
			assert.Equal(t, 0, tx.GasTipCap().Cmp(gasTipCap))
			assert.Equal(t, contract.Address(), *tx.To())
			assert.Equal(t, 0, tx.Value().Sign())
			assert.Equal(t, expectedData, tx.Data())
			assert.NotEqual(t, common.Hash{}, tx.Hash())
		})

		t.Run("buildsGaslessSignedTx", func(t *testing.T) {
			// ARRANGE
			wallet := newTestWallet(t)
			contract, err := NewTransferContract(
				common.HexToAddress("0x3333333333333333333333333333333333333333").Hex(),
			)
			require.NoError(t, err)

			ctx := context.Background()
			clientID := "client-0"
			receiver := "cosmos1receiver"
			amount := big.NewInt(7)
			timeoutTimestamp := uint64(1_800_000_000)
			nonce := uint64(3)
			gasFeeCap := big.NewInt(30_000_000_000)
			gasTipCap := big.NewInt(1_000_000_000)

			expectedData, err := expectedIFTCalldata(clientID, receiver, amount, timeoutTimestamp)
			require.NoError(t, err)
			chainID, err := wallet.GetClient().ChainID(ctx)
			require.NoError(t, err)

			// ACT
			tx, err := contract.BuildTransferTx(
				ctx,
				wallet,
				clientID,
				receiver,
				amount,
				timeoutTimestamp,
				nonce,
				gasFeeCap,
				gasTipCap,
				0,
			)

			// ASSERT
			require.NoError(t, err)
			require.NotNil(t, tx)
			assert.Equal(t, uint8(types.DynamicFeeTxType), tx.Type())
			assert.Equal(t, uint64(0), tx.Gas())
			assert.Equal(t, nonce, tx.Nonce())
			assert.Equal(t, 0, tx.ChainId().Cmp(chainID))
			assert.Equal(t, 0, tx.GasFeeCap().Cmp(gasFeeCap))
			assert.Equal(t, 0, tx.GasTipCap().Cmp(gasTipCap))
			assert.Equal(t, contract.Address(), *tx.To())
			assert.Equal(t, 0, tx.Value().Sign())
			assert.Equal(t, expectedData, tx.Data())
			assert.NotEqual(t, common.Hash{}, tx.Hash())
		})

		t.Run("returnsErrorWhenPackingFails", func(t *testing.T) {
			// ARRANGE
			// Same-package construction with an empty ABI exercises the Pack error
			// wrap; a nil amount panics inside go-ethereum abi.Pack instead of
			// returning an error.
			wallet := newTestWallet(t)
			contract := &TransferContract{
				address: common.HexToAddress("0x4444444444444444444444444444444444444444"),
			}

			// ACT
			tx, err := contract.BuildTransferTx(
				context.Background(),
				wallet,
				"client-0",
				"cosmos1receiver",
				big.NewInt(1),
				1,
				0,
				big.NewInt(1),
				big.NewInt(1),
				100_000,
			)

			// ASSERT
			require.ErrorContains(t, err, "pack iftTransfer calldata")
			assert.Nil(t, tx)
		})

		t.Run("requiresGasCapsForGaslessTransfer", func(t *testing.T) {
			for _, tt := range []struct {
				name      string
				gasFeeCap *big.Int
				gasTipCap *big.Int
			}{
				{
					name:      "nilFeeCap",
					gasFeeCap: nil,
					gasTipCap: big.NewInt(1_000_000_000),
				},
				{
					name:      "nilTipCap",
					gasFeeCap: big.NewInt(30_000_000_000),
					gasTipCap: nil,
				},
				{
					name:      "bothNil",
					gasFeeCap: nil,
					gasTipCap: nil,
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					// ARRANGE
					wallet := newTestWallet(t)
					contract, err := NewTransferContract(
						common.HexToAddress("0x5555555555555555555555555555555555555555").Hex(),
					)
					require.NoError(t, err)

					// ACT
					tx, err := contract.BuildTransferTx(
						context.Background(),
						wallet,
						"client-0",
						"cosmos1receiver",
						big.NewInt(1),
						1,
						0,
						tt.gasFeeCap,
						tt.gasTipCap,
						0,
					)

					// ASSERT
					require.ErrorContains(t, err, "gas tip/fee caps required for gasless ift transfer")
					assert.Nil(t, tx)
				})
			}
		})
	})
}

func newTestWallet(t *testing.T) *ethwallet.InteractingWallet {
	t.Helper()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	addr := crypto.PubkeyToAddress(key.PublicKey)
	sim := simulated.NewBackend(types.GenesisAlloc{
		addr: {Balance: big.NewInt(1_000_000_000_000_000_000)},
	})
	t.Cleanup(func() {
		if err := sim.Close(); err != nil {
			t.Errorf("close simulated backend: %v", err)
		}
	})

	chainID, err := sim.Client().ChainID(context.Background())
	require.NoError(t, err)

	return ethwallet.NewInteractingWallet(key, chainID, sim.Client())
}

func expectedIFTCalldata(clientID, receiver string, amount *big.Int, timeoutTimestamp uint64) ([]byte, error) {
	parsedABI, err := iftbindings.IftMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return parsedABI.Pack("iftTransfer", clientID, receiver, amount, timeoutTimestamp)
}
