package relayer

import "github.com/prometheus/client_golang/prometheus"

const (
	promNamespace = "catalyst"
	promSubsystem = "relay"
)

type Metrics struct {
	Success  *prometheus.CounterVec
	Failure  *prometheus.CounterVec
	Duration *prometheus.HistogramVec
}

// NewMetrics constructs and registers the relay metric vectors. Pass the
// result to NewGRPCClient; pass nil to disable instrumentation entirely.
func NewMetrics() *Metrics {
	m := &Metrics{
		Success: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "success_total",
			Help:      "Tx hashes successfully submitted to the relayer (terminal, per submission).",
		}, []string{"chain_id"}),
		Failure: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "failure_total",
			Help:      "Tx hashes that failed to be submitted to the relayer after all retries.",
		}, []string{"chain_id"}),
		Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: promNamespace,
			Subsystem: promSubsystem,
			Name:      "duration_seconds",
			Help:      "Duration of a single gRPC relay request (per-attempt, not including retry backoff).",
			Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10},
		}, []string{"chain_id"}),
	}
	prometheus.MustRegister(m.Success, m.Failure, m.Duration)
	return m
}
