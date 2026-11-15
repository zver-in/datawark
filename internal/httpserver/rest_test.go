package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"datawark/internal/audit"
	"datawark/internal/config"
	"datawark/internal/execution"
	"datawark/internal/policy"
	"datawark/internal/principal"
	"datawark/internal/query"
	"datawark/internal/schema"
	"github.com/go-chi/chi/v5"
	"go.yaml.in/yaml/v3"
)

type restGateway struct {
	executeResult execution.Result
	executeErr    error
	explanation   policy.Explanation
	sources       []string
	snapshots     map[string]schema.Snapshot
	executed      bool
}

func (gateway *restGateway) Datasources() []string { return append([]string(nil), gateway.sources...) }
func (gateway *restGateway) Schema(source string) (schema.Snapshot, error) {
	snapshot, found := gateway.snapshots[source]
	if !found {
		return schema.Snapshot{}, errors.New("schema: metadata is unavailable")
	}
	return snapshot, nil
}
func (gateway *restGateway) Execute(context.Context, query.Plan) (execution.Result, error) {
	gateway.executed = true
	return gateway.executeResult, gateway.executeErr
}
func (gateway *restGateway) Explain(query.Plan) policy.Explanation { return gateway.explanation }

func restHandler(t *testing.T, gateway *restGateway) http.Handler {
	t.Helper()
	identity, err := principal.New("reader")
	if err != nil {
		t.Fatal(err)
	}
	return NewWithGateway("", nil, func(principal.Trusted) (Gateway, error) { return gateway, nil }).requestID(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, identity))
		router := chiRouterForTest(gateway)
		router.ServeHTTP(writer, request)
	}))
}

func chiRouterForTest(gateway *restGateway) http.Handler {
	router := chi.NewRouter()
	newRESTAPI(func(principal.Trusted) (Gateway, error) { return gateway, nil }, nil).routes(router)
	return router
}

func TestRESTQueryPreservesRowsAndMapsSafeErrors(t *testing.T) {
	plan := `{"source":"analytics","table":"public.events","projection":["id"],"filters":[],"order":[],"limit":1}`
	for name, test := range map[string]struct {
		err  error
		want int
		code string
	}{
		"allowed":     {want: http.StatusOK},
		"denied":      {err: errors.New("policy: query is not authorized"), want: http.StatusForbidden, code: "denied"},
		"unavailable": {err: execution.Error{Category: execution.ErrorUnavailable}, want: http.StatusServiceUnavailable, code: "unavailable"},
		"guardrail":   {err: execution.Error{Category: execution.ErrorResultSize}, want: http.StatusUnprocessableEntity, code: "guardrail"},
		"internal":    {err: errors.New("database password=hostile"), want: http.StatusInternalServerError, code: "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			gateway := &restGateway{executeResult: execution.Result{Columns: []string{"id"}, Rows: [][]any{{nil}}}, executeErr: test.err}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(plan))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			restHandler(t, gateway).ServeHTTP(response, request)
			if response.Code != test.want || !strings.Contains(response.Header().Get(RequestIDHeader), "") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if test.code != "" && (!strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) || strings.Contains(response.Body.String(), "hostile")) {
				t.Fatalf("unsafe error=%s", response.Body.String())
			}
			if name == "allowed" && (!strings.Contains(response.Body.String(), `"columns":["id"]`) || !strings.Contains(response.Body.String(), "null")) {
				t.Fatalf("result=%s", response.Body.String())
			}
		})
	}
}

func TestRESTPlanDecodingIsStrictAndBounded(t *testing.T) {
	valid := `{"source":"analytics","table":"public.events","projection":["id"],"filters":[],"order":[],"limit":1}`
	for name, body := range map[string]string{
		"unknown": strings.Replace(valid, "}", `,"unknown":true}`, 1), "legacy": strings.Replace(valid, `"source"`, `"datasource"`, 1), "trailing": valid + valid,
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			restHandler(t, &restGateway{}).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "unknown") {
				t.Fatalf("response=%d %s", response.Code, response.Body.String())
			}
		})
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(valid))
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	restHandler(t, &restGateway{}).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("media status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/query", bytes.NewReader(make([]byte, MaxPlanBodyBytes+1)))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	restHandler(t, &restGateway{}).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("limit status=%d", response.Code)
	}
}

func TestRESTExplainDoesNotExecuteAndSchemaHidesMissingSource(t *testing.T) {
	snapshot := schema.NewSnapshot("analytics", []schema.Relation{schema.NewRelation("public", "events", []schema.Column{{Name: "id", TypeName: "integer"}})})
	gateway := &restGateway{explanation: policy.Explanation{Allowed: false, Reason: policy.ReasonDenied}, sources: []string{"analytics"}, snapshots: map[string]schema.Snapshot{"analytics": snapshot}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/query/explain", strings.NewReader(`{"source":"analytics","table":"public.events","projection":["id"],"filters":[],"order":[],"limit":1}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	restHandler(t, gateway).ServeHTTP(response, request)
	if response.Code != http.StatusOK || gateway.executed || !strings.Contains(response.Body.String(), `"allowed":false`) {
		t.Fatalf("response=%d execute=%v body=%s", response.Code, gateway.executed, response.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/schema/hidden-secret", nil)
	response = httptest.NewRecorder()
	restHandler(t, gateway).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "hidden-secret") {
		t.Fatalf("response=%d body=%s", response.Code, response.Body.String())
	}
}

func TestOpenAPIHasExactlyRESTRouterPaths(t *testing.T) {
	data, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	newRESTAPI(func(principal.Trusted) (Gateway, error) { return &restGateway{}, nil }, nil).routes(router)
	routes := map[string]map[string]any{}
	for _, route := range router.Routes() {
		routes[route.Pattern] = map[string]any{}
		for method := range route.Handlers {
			routes[route.Pattern][strings.ToLower(method)] = nil
		}
	}
	want := map[string]string{"/api/v1/query": "post", "/api/v1/query/explain": "post", "/api/v1/schema": "get", "/api/v1/schema/{source}": "get"}
	encoded, _ := json.Marshal(document.Paths)
	for path, method := range want {
		if _, found := document.Paths[path][method]; !found {
			t.Fatalf("missing %s %s in %s", method, path, encoded)
		}
		if _, found := routes[path][method]; !found {
			t.Fatalf("router missing %s %s", method, path)
		}
	}
	if len(document.Paths) != len(want) || len(routes) != len(want) {
		t.Fatalf("paths=%s routes=%v", encoded, routes)
	}
}

type countingRecorder struct {
	calls int
	err   error
}

func (recorder *countingRecorder) Record(audit.Event) error { recorder.calls++; return recorder.err }

func TestRESTOperationAuditPrecedesResponseAndFailsClosed(t *testing.T) {
	identity, _ := principal.New("reader")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(`{"source":"analytics","table":"public.events","projection":["id"],"filters":[],"order":[],"limit":1}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(context.WithValue(contextWithRequestID(request.Context(), "request-1"), principalContextKey{}, identity))
	for _, test := range []struct {
		recorder *countingRecorder
		want     int
	}{{&countingRecorder{}, http.StatusOK}, {&countingRecorder{err: errors.New("unavailable")}, http.StatusServiceUnavailable}} {
		response := httptest.NewRecorder()
		api := newRESTAPI(func(principal.Trusted) (Gateway, error) {
			return &restGateway{executeResult: execution.Result{Columns: []string{"id"}}}, nil
		}, test.recorder)
		api.query(response, request)
		if response.Code != test.want || test.recorder.calls != 1 {
			t.Fatalf("status=%d calls=%d", response.Code, test.recorder.calls)
		}
	}
}

func TestRESTRequestEmitsAuthenticationAndOperationEvents(t *testing.T) {
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER_KEY"}}, func(string) (string, bool) { return "reader-key", true })
	if err != nil {
		t.Fatal(err)
	}
	recorder := &eventRecorder{}
	authenticator.SetRecorder(recorder)
	handler := NewWithGateway("", authenticator, func(principal.Trusted) (Gateway, error) {
		return &restGateway{executeResult: execution.Result{Columns: []string{"id"}}}, nil
	}).Handler()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(`{"source":"analytics","table":"public.events","projection":["id"],"filters":[],"order":[],"limit":1}`))
	request.Header.Set("Authorization", "Bearer reader-key")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(RequestIDHeader, "correlated-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || len(recorder.events) != 2 {
		t.Fatalf("status=%d events=%#v", response.Code, recorder.events)
	}
	if recorder.events[0].Operation != audit.OperationAuthenticate || recorder.events[1].Operation != audit.OperationQuery || recorder.events[0].RequestID != "correlated-1" || recorder.events[1].RequestID != "correlated-1" {
		t.Fatalf("events=%#v", recorder.events)
	}
}

type eventRecorder struct {
	events []audit.Event
	err    error
}

func (recorder *eventRecorder) Record(event audit.Event) error {
	recorder.events = append(recorder.events, event)
	return recorder.err
}
