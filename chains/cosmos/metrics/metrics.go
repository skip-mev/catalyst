package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	PromNamespace      = "catalyst"
	TxMetricsNamespace = "tx_metrics"
)

type Metrics struct {
	BroadcastFailure *prometheus.CounterVec
	BroadcastSuccess prometheus.Counter
}

func NewMetrics() *Metrics {
	broadcastFailure := register(prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_failure",
		Help:      "Number of failed tx broadcasts.",
	}, []string{"error_code"})).(*prometheus.CounterVec)
	broadcastSuccess := register(prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_success",
		Help:      "Number of successful tx broadcasts.",
	})).(prometheus.Counter)
	return &Metrics{
		BroadcastFailure: broadcastFailure,
		BroadcastSuccess: broadcastSuccess,
	}
}

func (m *Metrics) RecordBroadcastFailure(code uint32) {
	m.BroadcastFailure.WithLabelValues(fmt.Sprintf("%d", code)).Add(1)
}

// register adds c to the default registry. Two runners in one process are
// expected (catalyst as a library; ibc e2e/load_relayer_test.go starts A→B and
// B→A catalysts in parallel). On AlreadyRegisteredError, return the collector
// already in the registry so every runner's increments are scraped. Do not
// panic and do not keep an unregistered duplicate.
func register(c prometheus.Collector) prometheus.Collector {
	if err := prometheus.Register(c); err != nil {
		are, ok := err.(prometheus.AlreadyRegisteredError)
		if !ok {
			panic(err)
		}
		return are.ExistingCollector
	}
	return c
}
