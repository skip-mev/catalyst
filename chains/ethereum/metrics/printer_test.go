package metrics

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	loadtesttypes "github.com/skip-mev/catalyst/chains/types"
)

func TestPrintResultsMarksReceiptStatsUnavailable(t *testing.T) {
	result := loadtesttypes.LoadTestResult{
		Overall: loadtesttypes.OverallStats{
			TotalTransactions: 3,
			BroadcastFailures: 1,
		},
		ReceiptCollectionSkipped: true,
	}

	output := captureStdout(t, func() {
		PrintResults(result)
	})

	require.Contains(t, output, "Receipt-derived statistics: unavailable (receipt collection skipped)")
	require.Contains(t, output, "Broadcast Failures: 1")
	require.NotContains(t, output, "Transactions Not Found")
	require.NotContains(t, output, "Successful Transactions")

	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.Contains(t, string(data), `"ReceiptCollectionSkipped":true`)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	original := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = original
	}()

	fn()
	require.NoError(t, writer.Close())
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	return strings.TrimSpace(string(output))
}
