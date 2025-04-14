package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsServer represents a server that exposes Prometheus metrics
type MetricsServer struct {
	server *http.Server
	addr   string
}

// MetricsConfig represents the configuration for the metrics server
type MetricsConfig struct {
	Enabled bool
	Addr    string
}

// DefaultMetricsConfig returns the default metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Enabled: true,
		Addr:    ":9090",
	}
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(config MetricsConfig) *MetricsServer {
	if !config.Enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    config.Addr,
		Handler: mux,
	}

	return &MetricsServer{
		server: server,
		addr:   config.Addr,
	}
}

// Start starts the metrics server
func (s *MetricsServer) Start() error {
	if s == nil {
		return nil
	}
	return s.server.ListenAndServe()
}

// Stop gracefully stops the metrics server
func (s *MetricsServer) Stop() error {
	if s == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// GetAddr returns the address the metrics server is listening on
func (s *MetricsServer) GetAddr() string {
	if s == nil {
		return ""
	}
	return s.addr
}
