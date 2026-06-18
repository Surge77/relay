package api

import (
	"context"
	"expvar"
	"log/slog"
	"net/http"
	"time"
)

// Operational counters exposed at /debug/vars (stdlib expvar — no external
// metrics dependency). A Prometheus exporter can be layered on later.
var (
	metricHTTPRequests = expvar.NewInt("http_requests_total")
	metricHTTPErrors   = expvar.NewInt("http_errors_total")
)

// statusRecorder captures the response status for logging + metrics.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// withObservability counts and structured-logs every request. Health/readiness
// and metrics endpoints are still counted but kept quiet.
func withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		metricHTTPRequests.Add(1)
		if rec.status >= 500 {
			metricHTTPErrors.Add(1)
		}
		slog.Info("http",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"dur_ms", time.Since(start).Milliseconds())
	})
}

// handleReadyz reports readiness: dependencies reachable. Distinct from
// /healthz, which is pure liveness.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "NOT_READY", "database unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ready": true})
}
