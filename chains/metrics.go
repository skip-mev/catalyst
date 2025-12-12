package chains

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// startPrometheusServer starts a Prometheus HTTP server, listening for metrics
// collectors on addr.
func startPrometheusServer(addr string, logger *zap.Logger) *http.Server {
	srv := &http.Server{
		Addr: addr,
		Handler: promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer, promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{},
			),
		),
		ReadHeaderTimeout: time.Minute,
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			logger.Error("Prometheus HTTP server ListenAndServe", zap.Error(err))
		}
	}()
	return srv
}
