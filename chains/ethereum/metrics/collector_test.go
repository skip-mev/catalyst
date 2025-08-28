package metrics

import (
	"testing"
	"time"

	"github.com/skip-mev/catalyst/chains/types"
	"github.com/stretchr/testify/require"
)

func TestGetMaxTPS(t *testing.T) {
	start := time.Now()
	testCases := []struct {
		name           string
		stats          []types.BlockStat
		expectedMaxTPS int
	}{
		{
			name:           "TestGetMaxTPS",
			expectedMaxTPS: 1880,
			stats: []types.BlockStat{
				{
					Timestamp: start.Add(1 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(2 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(3 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(4 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(5 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(6 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(7 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(8 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(9 * time.Second),
					NumTxs:    2000,
				},
				{
					Timestamp: start.Add(10 * time.Second),
					NumTxs:    800,
				},
				{
					Timestamp: start.Add(11 * time.Second),
					NumTxs:    1500,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tps, _, _ := findMaxTPS(tc.stats, 10*time.Second)
			require.Equal(t, tps, tc.expectedMaxTPS)
		})
	}
}
