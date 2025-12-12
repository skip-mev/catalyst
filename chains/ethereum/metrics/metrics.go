package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	PromNamespace      = "catalyst"
	TxMetricsNamespace = "tx_metrics"
)

type Metrics struct {
	TxSuccess        prometheus.Counter
	TxFailure        prometheus.Counter
	TxInclusion      prometheus.Histogram
	BroadcastFailure prometheus.Counter
	BroadcastSuccess prometheus.Counter
}

func NewMetrics() *Metrics {
	txSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "tx_success",
		Help:      "Number of successfully committed txs.",
	})
	txFailure := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "tx_failure",
		Help:      "Number of tracked txs which timed out without getting included in a block.",
	})
	txInclusion := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "tx_inclusion",
		Help:      "Histogram of time between broadcast and block inclusion (measured via websocket subscription).",
		Buckets:   []float64{50, 100, 250, 500, 1000, 1500, 2000, 5000, 10000, 15000, 20000, 30000, 60000, 90000, 120000, 300000},
	})
	broadcastFailure := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_failure",
		Help:      "Number of failed tx broadcasts.",
	})
	broadcastSuccess := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_success",
		Help:      "Number of successful tx broadcasts.",
	})
	prometheus.MustRegister(txSuccess)
	prometheus.MustRegister(txFailure)
	prometheus.MustRegister(txInclusion)
	prometheus.MustRegister(broadcastFailure)
	prometheus.MustRegister(broadcastSuccess)
	return &Metrics{
		TxSuccess:        txSuccess,
		TxFailure:        txFailure,
		TxInclusion:      txInclusion,
		BroadcastFailure: broadcastFailure,
		BroadcastSuccess: broadcastSuccess,
	}
}
