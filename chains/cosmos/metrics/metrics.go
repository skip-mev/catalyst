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
	broadcastFailure := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_failure",
		Help:      "Number of failed tx broadcasts.",
	}, []string{"error_code"})
	broadcastSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_success",
		Help:      "Number of successful tx broadcasts.",
	})
	prometheus.MustRegister(broadcastFailure)
	prometheus.MustRegister(broadcastSuccess)
	return &Metrics{
		BroadcastFailure: broadcastFailure,
		BroadcastSuccess: broadcastSuccess,
	}
}

func (m *Metrics) RecordBroadcastFailure(code uint32) {
	m.BroadcastFailure.WithLabelValues(fmt.Sprintf("%d", code)).Add(1)
}
