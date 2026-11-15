package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"datawark/internal/audit"
	"datawark/internal/config"
	"datawark/internal/execution"
	"datawark/internal/mcpserver"
	"datawark/internal/policy"
	"datawark/internal/principal"
	"datawark/internal/query"
	"datawark/internal/schema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpGateway struct{ principal string }

func (gateway mcpGateway) Datasources() []string { return []string{gateway.principal} }
func (gateway mcpGateway) Schema(source string) (schema.Snapshot, error) {
	if source != gateway.principal {
		return schema.Snapshot{}, errors.New("schema: metadata is unavailable")
	}
	return schema.NewSnapshot(source, []schema.Relation{schema.NewRelation("public", "orders", []schema.Column{{Name: "id", TypeName: "integer"}})}), nil
}
func (gateway mcpGateway) Execute(_ context.Context, plan query.Plan) (execution.Result, error) {
	if plan.Source != gateway.principal {
		return execution.Result{}, errors.New("policy: query is not authorized")
	}
	if plan.Limit > 1 {
		return execution.Result{}, execution.Error{Category: execution.ErrorRowLimit}
	}
	return execution.Result{Columns: []string{"id"}, Rows: [][]any{{int64(1)}}}, nil
}
func (gateway mcpGateway) Explain(plan query.Plan) policy.Explanation {
	return policy.Explanation{Allowed: plan.Source == gateway.principal, Reason: policy.ReasonDenied}
}

type recordedEvents struct {
	mu               sync.Mutex
	events           []audit.Event
	err              error
	failingOperation audit.Operation
}

func (recorder *recordedEvents) Record(event audit.Event) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event)
	if recorder.failingOperation == "" || recorder.failingOperation == event.Operation {
		return recorder.err
	}
	return nil
}

type bearerTransport struct {
	key  string
	base http.RoundTripper
}

func (transport bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+transport.key)
	return transport.base.RoundTrip(request)
}

func TestMCPHTTPUsesAuthenticatedPrincipalAndCanonicalTools(t *testing.T) {
	recorder := &recordedEvents{}
	handler := mcpHandler(t, recorder)
	server := httptest.NewServer(handler)
	defer server.Close()

	session := connectMCP(t, server.URL, "reader-key")
	defer session.Close()
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	if strings.Join(names, ",") != "explain_query,list_allowed_schema,query_data" {
		t.Fatalf("tools=%v", names)
	}
	for name, arguments := range map[string]any{
		"list_allowed_schema": map[string]any{},
		"query_data":          plan("reader"),
		"explain_query":       plan("reader"),
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil || result.IsError {
			t.Fatalf("%s result=%#v err=%v", name, result, err)
		}
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: map[string]any{"source": "reader", "table": "public.orders", "projection": []string{"id"}, "limit": 1, "principal": "writer"}})
	if err != nil || !result.IsError || strings.Contains(toolText(t, result), "writer") {
		t.Fatalf("unsafe invalid result=%#v err=%v", result, err)
	}
	writer := connectMCP(t, server.URL, "writer-key")
	defer writer.Close()
	writerResult, err := writer.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_allowed_schema", Arguments: map[string]any{}})
	if err != nil || writerResult.IsError || !strings.Contains(toolText(t, writerResult), `"writer"`) || strings.Contains(toolText(t, writerResult), `"reader"`) {
		t.Fatalf("writer result=%#v err=%v", writerResult, err)
	}
	denied, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: plan("writer")})
	if err != nil || !denied.IsError || strings.Contains(toolText(t, denied), "writer") {
		t.Fatalf("cross-principal result=%#v err=%v", denied, err)
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	for _, event := range recorder.events {
		if event.Operation != audit.OperationAuthenticate && (event.Transport != audit.TransportMCPHTTP || (event.Principal != "reader" && event.Principal != "writer") || event.RequestID == "") {
			t.Fatalf("event=%#v", event)
		}
	}
}

func TestMCPHTTPRejectsUnauthenticatedAndFailsClosedOnAudit(t *testing.T) {
	recorder := &recordedEvents{}
	server := httptest.NewServer(mcpHandler(t, recorder))
	defer server.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("response=%v err=%v", response, err)
	}
	if body := readBody(t, response); strings.Contains(body, "reader-key") || strings.Contains(body, "initialize") {
		t.Fatalf("unsafe unauthenticated response=%s", body)
	}
	response.Body.Close()

	session := connectMCP(t, server.URL, "reader-key")
	defer session.Close()
	recorder.err = errors.New("writer unavailable")
	recorder.failingOperation = audit.OperationQuery
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: plan("reader")})
	if err != nil || !result.IsError || strings.Contains(toolText(t, result), "columns") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestMCPHTTPStdioAndRESTAgreeOnOutcomeCategory(t *testing.T) {
	server := httptest.NewServer(mcpHandler(t, &recordedEvents{}))
	defer server.Close()
	remote := connectMCP(t, server.URL, "reader-key")
	defer remote.Close()
	for _, test := range []struct {
		name      string
		arguments any
		isError   bool
		status    int
		category  string
	}{
		{"allowed", plan("reader"), false, http.StatusOK, ""},
		{"denied", plan("writer"), true, http.StatusForbidden, "denied"},
		{"guardrail", guardedPlan("reader"), true, http.StatusUnprocessableEntity, "guardrail"},
	} {
		t.Run(test.name, func(t *testing.T) {
			remoteResult, err := remote.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: test.arguments})
			if err != nil || remoteResult.IsError != test.isError {
				t.Fatalf("MCP HTTP result=%#v err=%v", remoteResult, err)
			}
			if test.category != "" && !strings.Contains(toolText(t, remoteResult), test.category) {
				t.Fatalf("MCP HTTP category=%q", toolText(t, remoteResult))
			}

			serverTransport, clientTransport := mcp.NewInMemoryTransports()
			serverSession, err := mcpserver.New(mcpGateway{principal: "reader"}).Connect(context.Background(), serverTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer serverSession.Close()
			stdio, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil).Connect(context.Background(), clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer stdio.Close()
			stdioResult, err := stdio.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: test.arguments})
			if err != nil || stdioResult.IsError != test.isError {
				t.Fatalf("MCP stdio result=%#v err=%v", stdioResult, err)
			}
			if test.category != "" && !strings.Contains(toolText(t, stdioResult), test.category) {
				t.Fatalf("MCP stdio category=%q", toolText(t, stdioResult))
			}

			data, _ := json.Marshal(test.arguments)
			request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/query", strings.NewReader(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Authorization", "Bearer reader-key")
			request.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(request)
			if err != nil || response.StatusCode != test.status {
				t.Fatalf("REST response=%v err=%v", response, err)
			}
			if test.category != "" {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.NewDecoder(response.Body).Decode(&body); err != nil || body.Error.Code != test.category {
					t.Fatalf("REST category=%q err=%v", body.Error.Code, err)
				}
			}
			response.Body.Close()
		})
	}
}

func mcpHandler(t *testing.T, recorder audit.Recorder) http.Handler {
	t.Helper()
	authenticator, err := NewAuthenticator([]config.APIKey{{Principal: "reader", KeyEnv: "READER_KEY"}, {Principal: "writer", KeyEnv: "WRITER_KEY"}}, func(name string) (string, bool) {
		return map[string]string{"READER_KEY": "reader-key", "WRITER_KEY": "writer-key"}[name], true
	})
	if err != nil {
		t.Fatal(err)
	}
	authenticator.SetRecorder(recorder)
	return NewWithGateway("", authenticator, func(identity principal.Trusted) (Gateway, error) { return mcpGateway{principal: identity.Name()}, nil }).Handler()
}

func connectMCP(t *testing.T, endpoint, key string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: endpoint + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{key: key, base: http.DefaultTransport}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func plan(source string) map[string]any {
	return map[string]any{"source": source, "table": "public.orders", "projection": []string{"id"}, "filters": []any{}, "order": []any{}, "limit": 1}
}

func guardedPlan(source string) map[string]any {
	value := plan(source)
	value["limit"] = 2
	return value
}

func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content=%#v", result.Content)
	}
	content, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content=%#v", result.Content[0])
	}
	return content.Text
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
