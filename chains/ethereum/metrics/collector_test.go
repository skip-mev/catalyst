package metrics

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	ethtypes "github.com/skip-mev/catalyst/chains/ethereum/types"
	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
	"github.com/skip-mev/catalyst/config"
)

func mustSignedTransferTx(
	t *testing.T,
	key *ecdsa.PrivateKey,
	chainID *big.Int,
	gasTipCap *big.Int,
	gasFeeCap *big.Int,
	nonce uint64,
	to common.Address,
) *gethtypes.Transaction {
	t.Helper()
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       21_000,
		To:        &to,
		Value:     big.NewInt(1),
	})
	signedTx, err := gethtypes.SignTx(tx, gethtypes.LatestSignerForChainID(chainID), key)
	require.NoError(t, err)
	return signedTx
}

func TestProcessResultsConcurrent(t *testing.T) {
	ctx := config.WithEnv(context.Background(), config.Env{ConcurrentReceipts: true})
	logger := zaptest.NewLogger(t)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	from := crypto.PubkeyToAddress(key.PublicKey)
	alloc := gethtypes.GenesisAlloc{
		from: {Balance: big.NewInt(9_000_000_000_000_000_000)},
	}

	sim := simulated.NewBackend(alloc)
	client := sim.Client()
	defer sim.Close()

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	header, err := client.HeaderByNumber(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, header.BaseFee)

	gasTipCap := big.NewInt(2_000_000_000)
	gasFeeCap := new(big.Int).Add(new(big.Int).Mul(header.BaseFee, big.NewInt(2)), gasTipCap)
	blocks := 150
	sentTxs := make([]*ethtypes.SentTx, 0, blocks)
	for i := 0; i < blocks; i++ {
		to := common.BigToAddress(big.NewInt(int64(i + 10_000)))
		tx := mustSignedTransferTx(t, key, chainID, gasTipCap, gasFeeCap, uint64(i), to)
		require.NoError(t, client.SendTransaction(ctx, tx))
		sim.Commit()
		sentTxs = append(sentTxs, &ethtypes.SentTx{TxHash: tx.Hash(), MsgType: ethtypes.ContractCall, Tx: tx})
	}

	endBlock, err := client.BlockNumber(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, endBlock, uint64(blocks))

	f := reflect.Indirect(reflect.ValueOf(client)).FieldByName("Client")
	require.True(t, f.IsValid() && f.CanInterface())
	ec, ok := f.Interface().(*ethclient.Client)
	require.True(t, ok)
	require.NotNil(t, ec)
	clients := []*ethclient.Client{ec}
	require.NotPanics(t, func() {
		for run := 0; run < 25; run++ {
			result, err := ProcessResults(ctx, logger, sentTxs, 1, endBlock, clients)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Greater(t, result.Overall.TotalIncludedTransactions, 0)
		}
	})
}

func TestTrimBlocks(t *testing.T) {
	tests := []struct {
		name        string
		blocks      []loadtesttypes.BlockStat
		expected    []loadtesttypes.BlockStat
		expectError bool
	}{
		{
			name: "single block with transactions - preserves previous block",
			blocks: []loadtesttypes.BlockStat{
				{
					BlockHeight:  1,
					Timestamp:    time.Now().Add(-20 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{}, // empty
				},
				{
					BlockHeight: 2,
					Timestamp:   time.Now().Add(-10 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 1}, // has transactions
					},
				},
				{
					BlockHeight:  3,
					Timestamp:    time.Now(),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{}, // empty
				},
			},
			expected: []loadtesttypes.BlockStat{
				{
					BlockHeight:  1,
					Timestamp:    time.Now().Add(-20 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{},
				},
				{
					BlockHeight: 2,
					Timestamp:   time.Now().Add(-10 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 1},
					},
				},
			},
			expectError: false,
		},
		{
			name: "first block has transactions - no previous block to preserve",
			blocks: []loadtesttypes.BlockStat{
				{
					BlockHeight: 1,
					Timestamp:   time.Now().Add(-10 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 1}, // has transactions
					},
				},
				{
					BlockHeight:  2,
					Timestamp:    time.Now(),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{}, // empty
				},
			},
			expected: []loadtesttypes.BlockStat{
				{
					BlockHeight: 1,
					Timestamp:   time.Now().Add(-10 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 1},
					},
				},
			},
			expectError: false,
		},
		{
			name: "multiple blocks with transactions",
			blocks: []loadtesttypes.BlockStat{
				{
					BlockHeight:  1,
					Timestamp:    time.Now().Add(-30 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{}, // empty
				},
				{
					BlockHeight: 2,
					Timestamp:   time.Now().Add(-20 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 1}, // has transactions
					},
				},
				{
					BlockHeight: 3,
					Timestamp:   time.Now().Add(-10 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 2}, // has transactions
					},
				},
				{
					BlockHeight:  4,
					Timestamp:    time.Now(),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{}, // empty
				},
			},
			expected: []loadtesttypes.BlockStat{
				{
					BlockHeight:  1,
					Timestamp:    time.Now().Add(-30 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{},
				},
				{
					BlockHeight: 2,
					Timestamp:   time.Now().Add(-20 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 1},
					},
				},
				{
					BlockHeight: 3,
					Timestamp:   time.Now().Add(-10 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{
						"test": {SuccessfulTxs: 2},
					},
				},
			},
			expectError: false,
		},
		{
			name: "no blocks with transactions - should return error",
			blocks: []loadtesttypes.BlockStat{
				{
					BlockHeight:  1,
					Timestamp:    time.Now().Add(-20 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{}, // empty
				},
				{
					BlockHeight:  2,
					Timestamp:    time.Now().Add(-10 * time.Second),
					MessageStats: map[loadtesttypes.MsgType]loadtesttypes.MessageBlockStats{}, // empty
				},
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := trimBlocks(tt.blocks)

			if tt.expectError {
				require.Error(t, err, "Expected error but got none")
				require.Nil(t, result, "Expected nil result when error occurs")
				require.Contains(
					t,
					err.Error(),
					"no blocks with transactions",
					"Error message should contain expected text",
				)
			} else {
				require.NoError(t, err, "Expected no error but got: %v", err)
				require.Equal(t, len(tt.expected), len(result), "Length should match")

				for i, expected := range tt.expected {
					require.Equal(t, expected.BlockHeight, result[i].BlockHeight, "Block height should match")
					require.Equal(
						t,
						len(expected.MessageStats),
						len(result[i].MessageStats),
						"MessageStats length should match",
					)
				}
			}
		})
	}
}
