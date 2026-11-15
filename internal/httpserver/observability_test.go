package httpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"datawark/internal/audit"
	"datawark/internal/config"
)

func TestObservabilityLogsBoundedCompletionFieldsAndMetrics(t *testing.T) {
	secret := "distinctive-secret"
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER"}}, func(string) (string, bool) { return secret, true })
	if err != nil {
		t.Fatal(err)
	}
	service := New("", authenticator)
	var logs bytes.Buffer
	service.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	request := httptest.NewRequest(http.MethodGet, "/hostile/path?token=distinctive-query", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set(RequestIDHeader, "request-safe")
	service.Handler().ServeHTTP(httptest.NewRecorder(), request)
	line := logs.String()
	if strings.Count(line, "\n") != 1 || !strings.Contains(line, `"msg":"http request complete"`) || !strings.Contains(line, `"request_id":"request-safe"`) || strings.Contains(line, secret) || strings.Contains(line, "hostile/path") || strings.Contains(line, "distinctive-query") {
		t.Fatalf("unsafe or incomplete log: %q", line)
	}
	metrics := httptest.NewRecorder()
	service.Handler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || !strings.Contains(metrics.Body.String(), "datawark_gateway_requests_total") || strings.Contains(metrics.Body.String(), "request-safe") || strings.Contains(metrics.Body.String(), secret) {
		t.Fatalf("unsafe metrics: %d %q", metrics.Code, metrics.Body.String())
	}
}

func TestObservabilityLifecycleWritesOnlySafeStderrLogger(t *testing.T) {
	service := New("")
	var logs bytes.Buffer
	service.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = service.Serve(listener) }()
	for !service.ready.Load() {
		time.Sleep(time.Millisecond)
	}
	if err := service.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := logs.String(); !strings.Contains(got, `"event":"startup"`) || !strings.Contains(got, `"event":"readiness_withdrawn"`) || !strings.Contains(got, `"event":"shutdown"`) {
		t.Fatalf("lifecycle logs = %q", got)
	}
}

func TestObservabilityCollapsesHostileValuesIntoBoundedMetricSeries(t *testing.T) {
	service := New("")
	for i := 0; i < 100; i++ {
		request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/untrusted/%d?principal=p%d&table=t%d", i, i, i), nil)
		request.Header.Set(RequestIDHeader, fmt.Sprintf("request-%d", i))
		service.Handler().ServeHTTP(httptest.NewRecorder(), request)
	}
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	lines := strings.Split(response.Body.String(), "\n")
	requests := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "datawark_gateway_requests_total{") {
			requests++
		}
		if strings.Contains(line, "principal=") || strings.Contains(line, "request-") || strings.Contains(line, "untrusted") {
			t.Fatalf("unbounded metric label: %q", line)
		}
	}
	if requests != 1 {
		t.Fatalf("request series = %d, want one collapsed unknown-traffic series", requests)
	}
}

func TestTelemetryFailuresDoNotAffectAuditOrRequestOutcome(t *testing.T) {
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER"}}, func(string) (string, bool) { return "reader-key", true })
	if err != nil {
		t.Fatal(err)
	}
	recorder := &eventRecorder{}
	authenticator.SetRecorder(recorder)
	service := New("", authenticator)
	service.logger = slog.New(failingLogHandler{})
	service.metrics = failingMetrics{}
	request := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	request.Header.Set("Authorization", "Bearer reader-key")
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || len(recorder.events) != 1 || recorder.events[0].Operation != audit.OperationAuthenticate {
		t.Fatalf("response=%d audit=%#v", response.Code, recorder.events)
	}
}

type failingMetrics struct{}

func (failingMetrics) Record(context.Context, int, time.Duration) error {
	return errors.New("telemetry unavailable")
}

type failingLogHandler struct{}

func (failingLogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (failingLogHandler) Handle(context.Context, slog.Record) error {
	return errors.New("telemetry unavailable")
}
func (failingLogHandler) WithAttrs([]slog.Attr) slog.Handler { return failingLogHandler{} }
func (failingLogHandler) WithGroup(string) slog.Handler      { return failingLogHandler{} }
