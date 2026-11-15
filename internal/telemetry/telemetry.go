// Package telemetry provides bounded operational request annotations.
package telemetry

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	TransportREST = "rest"
	TransportMCP  = "mcp_http"
	TransportOps  = "operational"
	Other         = "other"
)

type values struct {
	Transport string
	Operation string
	Auth      string
	Policy    string
	Execution string
}

type contextKey struct{}

// WithRequest installs mutable, bounded metadata at the HTTP boundary.
func WithRequest(ctx context.Context, transport, operation string) context.Context {
	return context.WithValue(ctx, contextKey{}, &values{Transport: enum(transport, TransportREST, TransportMCP, TransportOps), Operation: operationEnum(operation), Auth: "not_applicable", Policy: "not_applicable", Execution: "not_applicable"})
}

func SetAuthentication(ctx context.Context, outcome string) {
	value(ctx).Auth = enum(outcome, "authenticated", "unauthenticated", "not_applicable")
}
func SetPolicy(ctx context.Context, outcome string) {
	value(ctx).Policy = enum(outcome, "allowed", "denied", "not_applicable")
}
func SetExecution(ctx context.Context, outcome string) {
	value(ctx).Execution = enum(outcome, "success", "timeout", "capacity", "row_limit", "result_size", "unavailable", "internal", "not_applicable")
}
func SetOperation(ctx context.Context, operation string) {
	value(ctx).Operation = operationEnum(operation)
}

func Snapshot(ctx context.Context) (transport, operation, auth, policy, execution string) {
	v := value(ctx)
	return v.Transport, v.Operation, v.Auth, v.Policy, v.Execution
}

func value(ctx context.Context) *values {
	if v, ok := ctx.Value(contextKey{}).(*values); ok && v != nil {
		return v
	}
	return &values{Transport: Other, Operation: Other, Auth: "not_applicable", Policy: "not_applicable", Execution: "not_applicable"}
}

func enum(value string, allowed ...string) string {
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return Other
}
func operationEnum(value string) string {
	return enum(value, "query", "explain", "schema_list", "schema_get", "authenticate", "health", "ready", "metrics", "mcp", Other)
}
func StatusClass(status int) string {
	if status < 100 || status > 599 {
		return Other
	}
	return strconv.Itoa(status/100) + "xx"
}

// Metrics owns only DataWark collectors in a dedicated registry.
type Metrics struct {
	Registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	labels := []string{"transport", "operation", "authentication", "policy", "execution", "status_class"}
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "datawark_gateway_requests_total", Help: "Completed Gateway HTTP requests."}, labels)
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "datawark_gateway_request_duration_seconds", Help: "Gateway HTTP request duration."}, labels)
	registry.MustRegister(requests, duration, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return &Metrics{Registry: registry, requests: requests, duration: duration}
}

func (metrics *Metrics) Record(ctx context.Context, status int, elapsed time.Duration) error {
	if metrics == nil {
		return nil
	}
	transport, operation, auth, policy, execution := Snapshot(ctx)
	labels := []string{transport, operation, auth, policy, execution, StatusClass(status)}
	metrics.requests.WithLabelValues(labels...).Inc()
	metrics.duration.WithLabelValues(labels...).Observe(elapsed.Seconds())
	return nil
}

// NewLogger returns the process-wide secret-safe JSON operational logger.
func NewLogger() *slog.Logger { return slog.New(slog.NewJSONHandler(os.Stderr, nil)) }
