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
	txSuccess := register(prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "tx_success",
		Help:      "Number of successfully committed txs.",
	})).(prometheus.Counter)
	txFailure := register(prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "tx_failure",
		Help:      "Number of tracked txs which timed out without getting included in a block.",
	})).(prometheus.Counter)
	txInclusion := register(prometheus.NewHistogram(prometheus.HistogramOpts{
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
	})).(prometheus.Histogram)
	broadcastFailure := register(prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_failure",
		Help:      "Number of failed tx broadcasts.",
	})).(prometheus.Counter)
	broadcastSuccess := register(prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: PromNamespace,
		Subsystem: TxMetricsNamespace,
		Name:      "broadcast_success",
		Help:      "Number of successful tx broadcasts.",
	})).(prometheus.Counter)

	return &Metrics{
		TxSuccess:        txSuccess,
		TxFailure:        txFailure,
		TxInclusion:      txInclusion,
		BroadcastFailure: broadcastFailure,
		BroadcastSuccess: broadcastSuccess,
	}
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
