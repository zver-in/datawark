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
	"sort"
	"strings"
	"testing"
	"time"

	"datawark/internal/audit"
	"datawark/internal/config"
	"datawark/internal/execution"
	"datawark/internal/principal"
	"datawark/internal/schema"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
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
	for _, field := range []string{`"msg":"http request complete"`, `"request_id":"request-safe"`, `"transport":"other"`, `"operation":"other"`, `"outcome":"other"`, `"authentication":"authenticated"`, `"policy":"not_applicable"`, `"execution":"not_applicable"`, `"status_class":"4xx"`, `"status":404`, `"bytes":`, `"duration_ms":`} {
		if !strings.Contains(line, field) {
			t.Fatalf("missing completion field %s in %q", field, line)
		}
	}
	if strings.Count(line, "\n") != 1 || strings.Contains(line, secret) || strings.Contains(line, "hostile/path") || strings.Contains(line, "distinctive-query") {
		t.Fatalf("unsafe or incomplete log: %q", line)
	}
	metrics := httptest.NewRecorder()
	service.Handler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || strings.Contains(metrics.Body.String(), "request-safe") || strings.Contains(metrics.Body.String(), secret) {
		t.Fatalf("unsafe metrics: %d %q", metrics.Code, metrics.Body.String())
	}
	assertOwnedMetricLabels(t, metrics.Body.Bytes())
}

func TestSchemaDetailCompletionUsesSchemaGet(t *testing.T) {
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER"}}, func(string) (string, bool) { return "reader-key", true })
	if err != nil {
		t.Fatal(err)
	}
	service := NewWithGateway("", authenticator, func(principal.Trusted) (Gateway, error) {
		return &restGateway{sources: []string{"visible"}, snapshots: map[string]schema.Snapshot{"visible": schema.NewSnapshot("visible", nil)}}, nil
	})
	var logs bytes.Buffer
	service.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/schema/visible", nil)
	request.Header.Set("Authorization", "Bearer reader-key")
	service.Handler().ServeHTTP(httptest.NewRecorder(), request)
	if line := logs.String(); !strings.Contains(line, `"operation":"schema_get"`) || !strings.Contains(line, `"outcome":"success"`) || strings.Contains(line, "visible") {
		t.Fatalf("schema completion log = %q", line)
	}
}

func TestObservabilityLifecycleWritesOnlySafeStderrLogger(t *testing.T) {
	service := New("")
	var logs bytes.Buffer
	service.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	secret := "postgres://reader:distinctive-secret@example.test/db"
	t.Setenv("DATAWARK_OBSERVABILITY_SECRET", secret)
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
	if got := logs.String(); !strings.Contains(got, `"event":"startup"`) || !strings.Contains(got, `"event":"readiness_withdrawn"`) || !strings.Contains(got, `"event":"shutdown"`) || strings.Contains(got, secret) || strings.Index(got, `"event":"startup"`) > strings.Index(got, `"event":"readiness_withdrawn"`) || strings.Index(got, `"event":"readiness_withdrawn"`) > strings.Index(got, `"event":"shutdown"`) {
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
	assertOwnedMetricLabels(t, response.Body.Bytes())
}

func TestMetricsRetainLabelSetsAcrossHostileHTTPMatrix(t *testing.T) {
	keys := []config.APIKey{{Principal: "reader-9841", KeyEnv: "KEY_A"}, {Principal: "writer-9841", KeyEnv: "KEY_B"}}
	authenticator, err := NewAuthenticator(keys, func(name string) (string, bool) {
		return map[string]string{"KEY_A": "key-a-9841", "KEY_B": "key-b-9841"}[name], true
	})
	if err != nil {
		t.Fatal(err)
	}
	service := NewWithGateway("", authenticator, func(principal.Trusted) (Gateway, error) {
		return &restGateway{executeResult: execution.Result{Columns: []string{"id"}}}, nil
	})
	call := func(index int) {
		token := fmt.Sprintf("hostile-value-%03d-9841", index)
		path := "/api/v1/query"
		if index%3 == 2 {
			path = "/untrusted/" + token
		}
		body := fmt.Sprintf(`{"source":"source-%s","table":"public.table_%s","projection":["column_%s"],"filters":[{"column":"column_%s","operator":"eq","value":"%s"}],"limit":1}`, token, token, token, token, token)
		request := httptest.NewRequest(http.MethodPost, path+"?query="+token+"&status="+token, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+map[bool]string{true: "key-a-9841", false: "unknown-" + token}[index%2 == 0])
		request.Header.Set(RequestIDHeader, "request-"+token)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Untrusted-Header", token)
		service.Handler().ServeHTTP(httptest.NewRecorder(), request)
	}
	// Prime each bounded outcome before taking the baseline, then repeat it with
	// distinctive values that must never become metric labels.
	for i := range 6 {
		call(i)
	}
	// The first scrape itself creates the bounded operational metrics series.
	_, _ = ownedMetricLabelSets(t, service)
	baseline, raw := ownedMetricLabelSets(t, service)
	for i := range 100 {
		call(i + 10)
	}
	after, afterRaw := ownedMetricLabelSets(t, service)
	if !equalLabelSets(baseline, after) {
		t.Fatalf("owned metric label sets changed\nbefore=%v\nafter=%v", baseline, after)
	}
	for _, value := range []string{"hostile-value-010-9841", "source-hostile", "table_hostile", "column_hostile", "request-hostile", "key-a-9841"} {
		if strings.Contains(raw+afterRaw, value) {
			t.Fatalf("metrics leaked submitted value %q", value)
		}
	}
}

func ownedMetricLabelSets(t *testing.T, service *Server) (map[string]map[string]bool, string) {
	t.Helper()
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	data := response.Body.Bytes()
	assertOwnedMetricLabels(t, data)
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	sets := map[string]map[string]bool{}
	for _, name := range []string{"datawark_gateway_requests_total", "datawark_gateway_request_duration_seconds"} {
		sets[name] = map[string]bool{}
		for _, metric := range families[name].Metric {
			labels := make([]string, 0, len(metric.Label))
			for _, label := range metric.Label {
				labels = append(labels, label.GetName()+"="+label.GetValue())
			}
			sort.Strings(labels)
			sets[name][strings.Join(labels, ",")] = true
		}
	}
	return sets, string(data)
}

func equalLabelSets(left, right map[string]map[string]bool) bool {
	for name, values := range left {
		if len(values) != len(right[name]) {
			return false
		}
		for value := range values {
			if !right[name][value] {
				return false
			}
		}
	}
	return true
}

func TestHTTPMetricsCoverBoundedOutcomeCategories(t *testing.T) {
	plan := `{"source":"analytics","table":"public.events","projection":["id"],"limit":1}`
	for _, test := range []struct {
		name, method, path, key, policy, execution string
		err                                        error
	}{
		{"success", http.MethodPost, "/api/v1/query", "reader-key", "allowed", "success", nil},
		{"unauthenticated", http.MethodPost, "/api/v1/query", "", "not_applicable", "not_applicable", nil},
		{"denied", http.MethodPost, "/api/v1/query", "reader-key", "denied", "not_applicable", errors.New("policy: denied")},
		{"timeout", http.MethodPost, "/api/v1/query", "reader-key", "allowed", "timeout", execution.Error{Category: execution.ErrorTimeout}},
		{"capacity", http.MethodPost, "/api/v1/query", "reader-key", "allowed", "capacity", execution.Error{Category: execution.ErrorCapacity}},
		{"row limit", http.MethodPost, "/api/v1/query", "reader-key", "allowed", "row_limit", execution.Error{Category: execution.ErrorRowLimit}},
		{"result size", http.MethodPost, "/api/v1/query", "reader-key", "allowed", "result_size", execution.Error{Category: execution.ErrorResultSize}},
		{"unavailable", http.MethodPost, "/api/v1/query", "reader-key", "allowed", "unavailable", execution.Error{Category: execution.ErrorUnavailable}},
		{"internal", http.MethodPost, "/api/v1/query", "reader-key", "not_applicable", "internal", errors.New("database password=hostile")},
		{"unknown", http.MethodGet, "/untrusted/path", "reader-key", "not_applicable", "not_applicable", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER"}}, func(string) (string, bool) { return "reader-key", true })
			if err != nil {
				t.Fatal(err)
			}
			service := NewWithGateway("", authenticator, func(principal.Trusted) (Gateway, error) {
				return &restGateway{executeResult: execution.Result{Columns: []string{"id"}}, executeErr: test.err}, nil
			})
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(plan))
			if test.key != "" {
				request.Header.Set("Authorization", "Bearer "+test.key)
			}
			request.Header.Set("Content-Type", "application/json")
			service.Handler().ServeHTTP(httptest.NewRecorder(), request)
			metrics := httptest.NewRecorder()
			service.Handler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
			transport, operation, authentication := "rest", "query", "authenticated"
			if test.name == "unknown" {
				transport = "other"
				operation = "other"
			}
			if test.name == "unauthenticated" {
				authentication = "unauthenticated"
			}
			if !metricHasLabels(t, metrics.Body.Bytes(), "datawark_gateway_requests_total", map[string]string{"transport": transport, "operation": operation, "authentication": authentication, "policy": test.policy, "execution": test.execution}) {
				t.Fatalf("missing metric labels for %s: %s", test.name, metrics.Body.String())
			}
		})
	}
}

func TestTelemetryFailuresDoNotAffectAuditOrRequestOutcome(t *testing.T) {
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER"}}, func(string) (string, bool) { return "reader-key", true })
	if err != nil {
		t.Fatal(err)
	}
	recorder := &eventRecorder{}
	authenticator.SetRecorder(recorder)
	service := NewWithGateway("", authenticator, func(principal.Trusted) (Gateway, error) {
		return &restGateway{executeResult: execution.Result{Columns: []string{"id"}}}, nil
	})
	service.logger = slog.New(failingLogHandler{})
	service.metrics = failingMetrics{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(`{"source":"analytics","table":"public.events","projection":["id"],"limit":1}`))
	request.Header.Set("Authorization", "Bearer reader-key")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	service.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(recorder.events) != 2 || recorder.events[0].Operation != audit.OperationAuthenticate || recorder.events[1].Operation != audit.OperationQuery {
		t.Fatalf("response=%d audit=%#v", response.Code, recorder.events)
	}
}

func assertOwnedMetricLabels(t *testing.T, data []byte) {
	t.Helper()
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}
	want := map[string]bool{"transport": true, "operation": true, "authentication": true, "policy": true, "execution": true, "status_class": true}
	for _, name := range []string{"datawark_gateway_requests_total", "datawark_gateway_request_duration_seconds"} {
		family := families[name]
		if family == nil || len(family.Metric) == 0 {
			t.Fatalf("missing owned metric family %s", name)
		}
		for _, metric := range family.Metric {
			if len(metric.Label) != len(want) {
				t.Fatalf("%s labels = %#v", name, metric.Label)
			}
			for _, label := range metric.Label {
				if !want[label.GetName()] {
					t.Fatalf("unexpected %s label %q", name, label.GetName())
				}
			}
		}
	}
}

func metricHasLabels(t *testing.T, data []byte, name string, want map[string]string) bool {
	t.Helper()
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}
	for _, metric := range families[name].Metric {
		found := map[string]string{}
		for _, label := range metric.Label {
			found[label.GetName()] = label.GetValue()
		}
		matches := true
		for key, value := range want {
			matches = matches && found[key] == value
		}
		if matches {
			return true
		}
	}
	return false
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
