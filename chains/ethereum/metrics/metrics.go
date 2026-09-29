package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	PromNamespace      = "catalyst"
	TxMetricsNamespace = "ethereum_tx_metrics"
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
		Buckets: []float64{
			50,
			100,
			250,
			500,
			1000,
			1500,
			2000,
			5000,
			10000,
			15000,
			20000,
			30000,
			60000,
			90000,
			120000,
			300000,
		},
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

	register(txSuccess)
	register(txFailure)
	register(txInclusion)
	register(broadcastFailure)
	register(broadcastSuccess)

	return &Metrics{
		TxSuccess:        txSuccess,
		TxFailure:        txFailure,
		TxInclusion:      txInclusion,
		BroadcastFailure: broadcastFailure,
		BroadcastSuccess: broadcastSuccess,
	}
}

// register adds c to the default registry. A second in-process runner hits the
// same metric names; keep the first registration and let this runner count locally.
func register(c prometheus.Collector) {
	err := prometheus.Register(c)
	if err == nil {
		return
	}
	if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
		return
	}
	panic(err)
}
