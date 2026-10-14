package metrics

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

// Two in-process runners construct ethereum metrics the way ibc
// e2e/load_relayer_test.go starts A→B and B→A catalysts in parallel. Before
// the ExistingCollector reuse fix, the second NewMetrics() kept an
// unregistered collector whose increments never reached the scrape registry.
func TestNewMetrics_SecondCallerReusesRegisteredCollectors(t *testing.T) {
	var m1, m2 *Metrics
	require.NotPanics(t, func() {
		m1 = NewMetrics()
		m2 = NewMetrics()
	})
	require.Same(t, m1.TxSuccess, m2.TxSuccess)
	require.Same(t, m1.BroadcastSuccess, m2.BroadcastSuccess)

	before := counterValue(t, "catalyst_ethereum_tx_metrics_tx_success")
	m1.TxSuccess.Inc()
	m2.TxSuccess.Inc()
	after := counterValue(t, "catalyst_ethereum_tx_metrics_tx_success")
	require.InDelta(t, 2, after-before, 0.001)
}

func counterValue(t *testing.T, metricName string) float64 {
	t.Helper()
	server := httptest.NewServer(promhttp.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	prefix := metricName + " "
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		var value float64
		_, err := fmt.Sscanf(line, metricName+" %f", &value)
		require.NoError(t, err)
		return value
	}
	return 0
}
