package relayer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
)

// Two in-process runners construct relay metrics the way ibc
// e2e/load_relayer_test.go starts A→B and B→A catalysts in parallel. Before
// the AlreadyRegisteredError reuse fix, the second NewMetrics() panics via
// MustRegister.
func TestNewMetrics_SecondCallerReusesRegisteredCollectors(t *testing.T) {
	var m1, m2 *Metrics
	require.NotPanics(t, func() {
		m1 = NewMetrics()
		m2 = NewMetrics()
	})
	require.Same(t, m1.Success, m2.Success)
	require.Same(t, m1.Failure, m2.Failure)
	require.Same(t, m1.Duration, m2.Duration)

	m1.Success.WithLabelValues("chain-a").Inc()
	m2.Success.WithLabelValues("chain-a").Inc()

	body := scrapeDefaultRegistry(t)
	require.Contains(t, body, `catalyst_relay_success_total{chain_id="chain-a"} 2`)
}

func scrapeDefaultRegistry(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(promhttp.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(raw)
}
