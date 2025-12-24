package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

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
					require.Equal(t, len(expected.MessageStats), len(result[i].MessageStats), "MessageStats length should match")
				}
			}
		})
	}
}
