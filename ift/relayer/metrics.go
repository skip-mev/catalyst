package relayer

import "github.com/prometheus/client_golang/prometheus"

const (
	promNamespace = "catalyst"
	promSubsystem = "relay"
	chainIDLabel  = "chain_id"
)

type Metrics struct {
	Success  *prometheus.CounterVec
	Failure  *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

// NewMetrics constructs and registers the relay metric vectors. Pass the
// result to NewGRPCClient; pass nil to disable instrumentation entirely.
//
// Two runners in one process are expected (catalyst as a library; ibc
// e2e/load_relayer_test.go starts A→B and B→A catalysts in parallel). On
// AlreadyRegisteredError, reuse the collector already in the registry so every
// runner's increments are scraped. Do not MustRegister a second time.
func NewMetrics() *Metrics {
	return &Metrics{
		Success: registerCounterVec(prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "success_total",
			Help:      "Tx hashes successfully submitted to the relayer (terminal, per submission).",
		}, []string{chainIDLabel})),
		Failure: registerCounterVec(prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "failure_total",
			Help:      "Tx hashes that failed to be submitted to the relayer after all retries.",
		}, []string{chainIDLabel})),
		Duration: registerHistogramVec(prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "duration_seconds",
			Help:      "Duration of a single gRPC relay request (per-attempt, not including retry backoff).",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		}, []string{chainIDLabel})),
	}
}

func registerCounterVec(c *prometheus.CounterVec) *prometheus.CounterVec {
	return register(c).(*prometheus.CounterVec)
}

func registerHistogramVec(c *prometheus.HistogramVec) *prometheus.HistogramVec {
	return register(c).(*prometheus.HistogramVec)
}

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
