//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"datawark/internal/audit"
	"datawark/internal/cli"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/execution"
	"datawark/internal/httpserver"
	"datawark/internal/mcpserver"
	pgadapter "datawark/internal/postgres"
	"datawark/internal/principal"
	"datawark/internal/query"
	gatewayruntime "datawark/internal/runtime"
	"datawark/internal/schema"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/spf13/cobra"
)

func TestServeProcessHelper(t *testing.T) {
	if os.Getenv("DATAWARK_INTEGRATION_SERVE_HELPER") != "1" {
		return
	}
	for index, argument := range os.Args {
		if argument == "--" {
			os.Exit(cli.Execute(os.Args[index+1:], os.Stdout, os.Stderr))
		}
	}
	os.Exit(2)
}

func TestServeProcessOwnsDockerRESTMCPMetricsAndStreams(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	for _, statement := range []string{"CREATE TABLE IF NOT EXISTS public.process_fixture (id integer PRIMARY KEY, secret text)", "TRUNCATE public.process_fixture", "INSERT INTO public.process_fixture VALUES (1, 'process-fixture-secret-9841')"} {
		if _, err := connection.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	configPath := filepath.Join(t.TempDir(), "gateway.yaml")
	apiKey := "process-api-key-9841"
	configuration := "version: v1alpha1\nserver:\n  developmentPrincipal: process-reader\n  http:\n    listenAddress: " + address + "\n    apiKeys:\n      - principal: process-reader\n        keyEnv: DATAWARK_PROCESS_KEY\ndatasources:\n  - name: process_source_9841\n    driver: postgres\n    dsnEnv: DATAWARK_TEST_POSTGRES_DSN\npolicies:\n  - name: process-read\n    principals: [process-reader]\n    datasource: process_source_9841\n    allow:\n      - table: public.process_fixture\n        operations: [select]\n        columns: [id]\n"
	if err := os.WriteFile(configPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=TestServeProcessHelper", "--", "serve", "--config", configPath)
	command.Env = append(os.Environ(), "DATAWARK_INTEGRATION_SERVE_HELPER=1", "DATAWARK_PROCESS_KEY="+apiKey)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := false
	t.Cleanup(func() {
		if !done && command.Process != nil {
			_ = command.Process.Kill()
			_, _ = io.Copy(io.Discard, stdout)
			_, _ = io.Copy(io.Discard, stderr)
			_ = command.Wait()
		}
	})
	stdoutDone, stderrDone := make(chan []byte, 1), make(chan []byte, 1)
	go func() { data, _ := io.ReadAll(stdout); stdoutDone <- data }()
	go func() { data, _ := io.ReadAll(stderr); stderrDone <- data }()
	endpoint := "http://" + address
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, requestErr := http.Get(endpoint + "/api/v1/ready")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve readiness: %v", requestErr)
		}
		time.Sleep(20 * time.Millisecond)
	}
	request, _ := http.NewRequest(http.MethodPost, endpoint+"/api/v1/query", strings.NewReader(`{"source":"process_source_9841","table":"public.process_fixture","projection":["id"],"filters":[],"order":[],"limit":1}`))
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(httpserver.RequestIDHeader, "process-request-9841")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("REST status=%d", response.StatusCode)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "process-test", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: endpoint + "/mcp", HTTPClient: &http.Client{Transport: bearerRoundTripper(apiKey)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: map[string]any{"source": "process_source_9841", "table": "public.process_fixture", "projection": []string{"id"}, "limit": 1}})
	_ = session.Close()
	if err != nil || result.IsError {
		t.Fatalf("MCP result=%#v err=%v", result, err)
	}
	response, err = http.Get(endpoint + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	assertProcessMetricEvidence(t, metrics, []string{`value:"rest"`, `value:"mcp_http"`, `value:"query"`, `value:"authenticated"`})
	for _, secret := range []string{apiKey, "process-reader", "process-request-9841", "process_source_9841", "process_fixture", "process-fixture-secret-9841", dsn, configuration} {
		if strings.Contains(string(metrics), secret) {
			t.Fatalf("metrics leaked %q", secret)
		}
	}
	if err := command.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	done = true
	actualStdout, actualStderr := <-stdoutDone, <-stderrDone
	assertProcessAuditAndOperationalStreams(t, actualStdout, actualStderr)
}

func assertProcessMetricEvidence(t *testing.T, data []byte, expected []string) {
	t.Helper()
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"datawark_gateway_requests_total", "datawark_gateway_request_duration_seconds"} {
		family := families[name]
		if family == nil {
			t.Fatalf("missing %s", name)
		}
		text := family.String()
		for _, value := range expected {
			if !strings.Contains(text, value) {
				t.Fatalf("%s missing %s: %s", name, value, text)
			}
		}
	}
}

func assertProcessAuditAndOperationalStreams(t *testing.T, stdout, stderr []byte) {
	t.Helper()
	audits := 0
	for _, line := range strings.FieldsFunc(string(stdout), func(r rune) bool { return r == '\n' }) {
		var event audit.Event
		if json.Unmarshal([]byte(line), &event) != nil || event.Validate() != nil {
			t.Fatalf("invalid audit stdout %q", line)
		}
		audits++
	}
	if audits < 4 {
		t.Fatalf("audit stdout=%s", stdout)
	}
	lifecycle := []string{}
	for _, line := range strings.FieldsFunc(string(stderr), func(r rune) bool { return r == '\n' }) {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) != nil || record["msg"] == nil {
			t.Fatalf("invalid operational stderr %q", line)
		}
		if event, ok := record["event"].(string); ok {
			lifecycle = append(lifecycle, event)
		}
	}
	if strings.Join(lifecycle, ",") != "startup,readiness_withdrawn,shutdown" || !strings.Contains(string(stderr), "http request complete") || strings.Contains(string(stderr), `"version":"v1alpha1"`) {
		t.Fatalf("stderr=%s", stderr)
	}
}

func TestPostgreSQLIsReachable(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse integration PostgreSQL DSN: %v", err)
	}

	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("connect to integration PostgreSQL: %v", err)
	}
	defer conn.Close()
}

func TestSourceTestSucceeds(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}

	configPath := filepath.Join(t.TempDir(), "gateway.yaml")
	config := "version: v1alpha1\nserver:\n  developmentPrincipal: local-dev\ndatasources:\n  - name: test-postgres\n    driver: postgres\n    dsnEnv: DATAWARK_TEST_POSTGRES_DSN\npolicies:\n  - name: test-read\n    principals: [local-dev]\n    datasource: test-postgres\n    allow:\n      - table: public.placeholder\n        operations: [select]\n        columns: [id]\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exitCode := cli.Execute([]string{"source", "test", "--config", configPath, "--datasource", "test-postgres"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("source test failed: stderr=%s", stderr.String())
	}
}

func TestPolicyExplainCLIUsesFileAndStdinWithoutExecution(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	if _, err := connection.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS public.cli_policy_explain_fixture (id integer PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(t.TempDir(), "gateway.yaml")
	configuration := "version: v1alpha1\nserver:\n  developmentPrincipal: local-dev\ndatasources:\n  - name: test_postgres\n    driver: postgres\n    dsnEnv: DATAWARK_TEST_POSTGRES_DSN\npolicies:\n  - name: cli-explain-read\n    principals: [local-dev]\n    datasource: test_postgres\n    allow:\n      - table: public.cli_policy_explain_fixture\n        operations: [select]\n        columns: [id]\n"
	if err := os.WriteFile(configPath, []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, []byte(`{"source":"test_postgres","table":"public.cli_policy_explain_fixture","projection":["id"],"limit":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	executor := &countingExecutor{}
	newCommand := func() *cobra.Command {
		return cli.NewRootCommand(cli.Dependencies{Runtime: func(ctx context.Context, path string) (*gatewayruntime.Runtime, error) {
			return gatewayruntime.New(ctx, path, gatewayruntime.Dependencies{Executor: executor})
		}})
	}

	command := newCommand()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"policy", "explain", "--config", configPath, "--plan", planPath})
	if err := command.Execute(); err != nil || stdout.String() != "{\"version\":\"v1alpha1\",\"allowed\":true,\"reason\":\"allowed\",\"policyId\":\"cli-explain-read\"}\n" || stderr.Len() != 0 {
		t.Fatalf("file stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
	}

	command = newCommand()
	stdout.Reset()
	stderr.Reset()
	command.SetIn(strings.NewReader(`{"source":"test_postgres","table":"public.cli_policy_explain_hidden","projection":["id"],"limit":1}`))
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"policy", "explain", "--config", configPath, "--plan", "-"})
	if err := command.Execute(); err != nil || stdout.String() != "{\"version\":\"v1alpha1\",\"allowed\":false,\"reason\":\"denied\"}\n" || stderr.Len() != 0 {
		t.Fatalf("stdin stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
	}
	if executor.calls != 0 {
		t.Fatalf("explain executed SQL %d times", executor.calls)
	}
}

func TestCatalogSnapshotIncludesUserRelationAndView(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	if _, err := connection.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS public.schema_registry_fixture (id integer PRIMARY KEY, note text)"); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Exec(context.Background(), "CREATE OR REPLACE VIEW public.schema_registry_fixture_view AS SELECT id, note FROM public.schema_registry_fixture"); err != nil {
		t.Fatal(err)
	}

	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev"}, Datasources: []config.Datasource{{Name: "test-postgres", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"}}, Policies: []config.Policy{{Name: "fixture-read", Principals: []string{"local-dev"}, Datasource: "test-postgres", Allow: []config.AllowEntry{{Table: "public.schema_registry_fixture", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
	snapshot, err := pgadapter.NewCatalogReader().Load(context.Background(), configuration, "test-postgres")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"public.schema_registry_fixture", "public.schema_registry_fixture_view"} {
		relation, found := snapshot.Relation(name)
		if !found || len(relation.Columns()) != 2 {
			t.Fatalf("snapshot missing fixture metadata for %s", name)
		}
	}
}

func TestMCPQueryUsesSharedRuntimePipeline(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	if _, err := connection.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS public.mcp_runtime_fixture (id integer PRIMARY KEY, secret text)"); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Exec(context.Background(), "INSERT INTO public.mcp_runtime_fixture (id, secret) VALUES (1, 'hidden') ON CONFLICT (id) DO NOTHING"); err != nil {
		t.Fatal(err)
	}
	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev"}, Datasources: []config.Datasource{{Name: "test_postgres", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"}}, Policies: []config.Policy{{Name: "mcp-read", Principals: []string{"local-dev"}, Datasource: "test_postgres", Allow: []config.AllowEntry{{Table: "public.mcp_runtime_fixture", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
	runtime, err := gatewayruntime.NewFromConfig(context.Background(), configuration, gatewayruntime.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	local, err := runtime.LocalGateway()
	if err != nil {
		t.Fatal(err)
	}
	serverSession, err := mcpserver.New(local).Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "query_data", Arguments: map[string]any{"source": "test_postgres", "table": "public.mcp_runtime_fixture", "projection": []string{"id"}, "limit": 1}})
	if err != nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("MCP query result=%#v err=%v", result, err)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(text.Text, `"id"`) || strings.Contains(text.Text, "hidden") {
		t.Fatalf("MCP query content=%#v", result.Content)
	}
}

func TestMCPHTTPUsesSharedRuntimePipeline(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	for _, statement := range []string{"CREATE TABLE IF NOT EXISTS public.mcp_http_fixture (id integer PRIMARY KEY, secret text)", "TRUNCATE public.mcp_http_fixture", "INSERT INTO public.mcp_http_fixture VALUES (1, 'hidden')"} {
		if _, err := connection.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "reader", HTTP: &config.HTTP{ListenAddress: "127.0.0.1:8080", APIKeys: []config.APIKey{{Principal: "reader", KeyEnv: "MCP_HTTP_READER"}}}}, Datasources: []config.Datasource{{Name: "test_postgres", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"}}, Policies: []config.Policy{{Name: "mcp-http-read", Principals: []string{"reader"}, Datasource: "test_postgres", Allow: []config.AllowEntry{{Table: "public.mcp_http_fixture", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
	runtime, err := gatewayruntime.NewFromConfig(context.Background(), configuration, gatewayruntime.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	auth, err := httpserver.NewAuthenticator(configuration.Server.HTTP.APIKeys, func(string) (string, bool) { return "reader-key", true })
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpserver.NewWithGateway("", auth, func(identity principal.Trusted) (httpserver.Gateway, error) { return runtime.Bind(identity) }).Handler())
	defer server.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "v1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerRoundTripper("reader-key")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for name, arguments := range map[string]any{
		"list_allowed_schema": map[string]any{},
		"query_data":          map[string]any{"source": "test_postgres", "table": "public.mcp_http_fixture", "projection": []string{"id"}, "limit": 1},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
		if err != nil || result.IsError || len(result.Content) != 1 {
			t.Fatalf("MCP HTTP %s result=%#v err=%v", name, result, err)
		}
		text := result.Content[0].(*mcp.TextContent).Text
		if strings.Contains(text, "hidden") {
			t.Fatalf("MCP HTTP leaked hidden data: %s", text)
		}
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/query", strings.NewReader(`{"source":"test_postgres","table":"public.mcp_http_fixture","projection":["id"],"filters":[],"order":[],"limit":1}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer reader-key")
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("REST request status = %d", response.StatusCode)
	}
	response, err = http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	metricText, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("metrics status=%d err=%v", response.StatusCode, err)
	}
	if strings.Contains(string(metricText), "reader-key") || strings.Contains(string(metricText), "hidden") || strings.Contains(string(metricText), "mcp_http_fixture") {
		t.Fatalf("metrics leaked protected value: %q", metricText)
	}
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(metricText))
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}
	for _, name := range []string{"datawark_gateway_requests_total", "datawark_gateway_request_duration_seconds"} {
		family := families[name]
		if family == nil || len(family.Metric) < 3 {
			t.Fatalf("metric family %s = %#v", name, family)
		}
		for _, metric := range family.Metric {
			if len(metric.Label) != 6 {
				t.Fatalf("metric labels for %s = %#v", name, metric.Label)
			}
		}
	}
}

type bearerRoundTripper string

func (key bearerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+string(key))
	return http.DefaultTransport.RoundTrip(request)
}

// TestRESTDataAPIExercises the HTTP adapter against the same Docker PostgreSQL
// runtime used by MCP. It covers allowed, denied, explain, schema filtering,
// authenticated principal isolation, and fail-closed audit handoff.
func TestRESTDataAPIExercisesSharedRuntime(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	for _, statement := range []string{"CREATE TABLE IF NOT EXISTS public.rest_api_fixture (id integer PRIMARY KEY, secret text)", "TRUNCATE public.rest_api_fixture", "INSERT INTO public.rest_api_fixture VALUES (1, 'hidden'), (2, 'hidden')"} {
		if _, err := connection.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
	}
	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "reader", Runtime: &config.Runtime{StatementTimeout: time.Second, MaxRows: 1, MaxResultBytes: 1024, MaxConcurrentQueries: 1}, HTTP: &config.HTTP{ListenAddress: "127.0.0.1:8080", APIKeys: []config.APIKey{{Principal: "reader", KeyEnv: "REST_READER"}, {Principal: "blocked", KeyEnv: "REST_BLOCKED"}}}}, Datasources: []config.Datasource{{Name: "test_postgres", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"}}, Policies: []config.Policy{{Name: "reader", Principals: []string{"reader"}, Datasource: "test_postgres", Allow: []config.AllowEntry{{Table: "public.rest_api_fixture", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
	runtime, err := gatewayruntime.NewFromConfig(context.Background(), configuration, gatewayruntime.Dependencies{})
	if err != nil {
		t.Fatal(err)
	}
	auth, err := httpserver.NewAuthenticator(configuration.Server.HTTP.APIKeys, func(name string) (string, bool) {
		return map[string]string{"REST_READER": "reader-key", "REST_BLOCKED": "blocked-key"}[name], true
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpserver.NewWithGateway("", auth, func(identity principal.Trusted) (httpserver.Gateway, error) { return runtime.Bind(identity) }).Handler())
	defer server.Close()
	call := func(key, path string, body any) (int, map[string]any) {
		var reader *strings.Reader
		if body != nil {
			data, _ := json.Marshal(body)
			reader = strings.NewReader(string(data))
		} else {
			reader = strings.NewReader("")
		}
		request, _ := http.NewRequest(http.MethodPost, server.URL+path, reader)
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		request.Header.Set("Authorization", "Bearer "+key)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		decoded := map[string]any{}
		_ = json.NewDecoder(response.Body).Decode(&decoded)
		return response.StatusCode, decoded
	}
	plan := map[string]any{"source": "test_postgres", "table": "public.rest_api_fixture", "projection": []string{"id"}, "filters": []any{}, "order": []any{}, "limit": 1}
	if status, result := call("reader-key", "/api/v1/query", plan); status != http.StatusOK || len(result["rows"].([]any)) != 1 {
		t.Fatalf("allowed REST result: %d %#v", status, result)
	}
	guarded := map[string]any{"source": "test_postgres", "table": "public.rest_api_fixture", "projection": []string{"id"}, "filters": []any{}, "order": []any{}, "limit": 2}
	if status, result := call("reader-key", "/api/v1/query", guarded); status != http.StatusUnprocessableEntity || result["error"].(map[string]any)["code"] != "guardrail" {
		t.Fatalf("guardrail REST result: %d %#v", status, result)
	}
	if status, result := call("blocked-key", "/api/v1/query", plan); status != http.StatusForbidden || result["error"].(map[string]any)["code"] != "denied" {
		t.Fatalf("denied REST result: %d %#v", status, result)
	}
	if status, result := call("blocked-key", "/api/v1/query/explain", plan); status != http.StatusOK || result["allowed"] != false {
		t.Fatalf("explain REST result: %d %#v", status, result)
	}
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v1/schema", nil)
	request.Header.Set("Authorization", "Bearer blocked-key")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var schemaBody map[string]any
	_ = json.NewDecoder(response.Body).Decode(&schemaBody)
	if response.StatusCode != http.StatusOK || len(schemaBody["sources"].([]any)) != 0 {
		t.Fatalf("schema isolation: %d %#v", response.StatusCode, schemaBody)
	}

	failingRecorder := &operationFailingRecorder{operation: audit.OperationQuery}
	auditAuth, err := httpserver.NewAuthenticator(configuration.Server.HTTP.APIKeys, func(name string) (string, bool) {
		return map[string]string{"REST_READER": "reader-key", "REST_BLOCKED": "blocked-key"}[name], true
	})
	if err != nil {
		t.Fatal(err)
	}
	auditAuth.SetRecorder(failingRecorder)
	auditServer := httptest.NewServer(httpserver.NewWithGateway("", auditAuth, func(identity principal.Trusted) (httpserver.Gateway, error) { return runtime.Bind(identity) }).Handler())
	defer auditServer.Close()
	request, _ = http.NewRequest(http.MethodPost, auditServer.URL+"/api/v1/query", strings.NewReader(`{"source":"test_postgres","table":"public.rest_api_fixture","projection":["id"],"filters":[],"order":[],"limit":1}`))
	request.Header.Set("Authorization", "Bearer reader-key")
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var auditBody map[string]any
	_ = json.NewDecoder(response.Body).Decode(&auditBody)
	if response.StatusCode != http.StatusServiceUnavailable || auditBody["error"].(map[string]any)["code"] != "unavailable" || failingRecorder.calls != 2 {
		t.Fatalf("audit failure response: %d %#v events=%d", response.StatusCode, auditBody, failingRecorder.calls)
	}
}

type operationFailingRecorder struct {
	operation audit.Operation
	calls     int
}

func (recorder *operationFailingRecorder) Record(event audit.Event) error {
	recorder.calls++
	if event.Operation == recorder.operation {
		return errors.New("audit writer unavailable")
	}
	return nil
}

func TestRuntimeBootstrapIsAllOrNothing(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev"}, Datasources: []config.Datasource{
		{Name: "primary", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"},
		{Name: "replica", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"},
	}, Policies: []config.Policy{
		{Name: "primary-read", Principals: []string{"local-dev"}, Datasource: "primary", Allow: []config.AllowEntry{{Table: "public.schema_registry_fixture", Operations: []string{"select"}, Columns: []string{"id"}}}},
		{Name: "replica-read", Principals: []string{"local-dev"}, Datasource: "replica", Allow: []config.AllowEntry{{Table: "public.schema_registry_fixture", Operations: []string{"select"}, Columns: []string{"id"}}}},
	}}
	runtime, err := gatewayruntime.NewFromConfig(context.Background(), configuration, gatewayruntime.Dependencies{})
	if err != nil || runtime == nil {
		t.Fatalf("multi-datasource bootstrap = %v, %v", runtime, err)
	}
	local, err := runtime.LocalGateway()
	if err != nil {
		t.Fatal(err)
	}
	if got := local.Datasources(); len(got) != 2 || got[0] != "primary" || got[1] != "replica" {
		t.Fatalf("visible datasources = %v", got)
	}

	t.Setenv("DATAWARK_RUNTIME_UNAVAILABLE_DSN", "postgres://datawark_test:datawark_test@127.0.0.1:1/datawark_test?sslmode=disable")
	configuration.Datasources = append(configuration.Datasources, config.Datasource{Name: "unavailable", Driver: "postgres", DSNEnv: "DATAWARK_RUNTIME_UNAVAILABLE_DSN"})
	configuration.Policies = append(configuration.Policies, config.Policy{Name: "unavailable-read", Principals: []string{"local-dev"}, Datasource: "unavailable", Allow: []config.AllowEntry{{Table: "public.schema_registry_fixture", Operations: []string{"select"}, Columns: []string{"id"}}}})
	runtime, err = gatewayruntime.NewFromConfig(context.Background(), configuration, gatewayruntime.Dependencies{})
	if runtime != nil || err == nil {
		t.Fatalf("unavailable datasource exposed runtime=%v err=%v", runtime, err)
	}
}

func TestReadOnlyQueryExecution(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	for _, sql := range []string{
		"CREATE TABLE IF NOT EXISTS public.read_only_query_fixture (id integer PRIMARY KEY, note text, secret text)",
		"CREATE TABLE IF NOT EXISTS public.read_only_query_inaccessible (id integer PRIMARY KEY)",
		"TRUNCATE public.read_only_query_fixture, public.read_only_query_inaccessible",
		"INSERT INTO public.read_only_query_fixture (id, note, secret) VALUES (1, NULL, 'not-granted')",
		"INSERT INTO public.read_only_query_inaccessible (id) VALUES (1)",
	} {
		if _, err := connection.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}

	configuration := readOnlyConfig()
	snapshot, err := pgadapter.NewCatalogReader().Load(context.Background(), configuration, "test_postgres")
	if err != nil {
		t.Fatal(err)
	}
	service := execution.QueryService{Configuration: configuration, Snapshot: snapshot, Executor: pgadapter.NewExecutor(), Principal: trusted(t, "local-dev")}
	result, err := service.Execute(context.Background(), query.Plan{Source: "test_postgres", Table: "public.read_only_query_fixture", Projection: []string{"id", "note"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Columns) != 2 || result.Columns[0] != "id" || len(result.Rows) != 1 || result.Rows[0][1] != nil {
		t.Fatalf("unexpected result: %#v", result)
	}

	for name, plan := range map[string]query.Plan{
		"policy": {Source: "test_postgres", Table: "public.read_only_query_fixture", Projection: []string{"secret"}, Limit: 1},
		"schema": {Source: "test_postgres", Table: "public.read_only_query_inaccessible", Projection: []string{"id"}, Limit: 1},
	} {
		t.Run(name, func(t *testing.T) {
			executor := &countingExecutor{}
			service.Executor = executor
			if _, err := service.Execute(context.Background(), plan); err == nil || executor.calls != 0 {
				t.Fatalf("failure executed a statement: %v, calls=%d", err, executor.calls)
			}
		})
	}
}

func TestQueryServiceRejectsWriteAndDDLShapes(t *testing.T) {
	configuration := readOnlyConfig()
	snapshot := schema.NewSnapshot("test_postgres", []schema.Relation{schema.NewRelation("public", "read_only_query_fixture", []schema.Column{{Name: "id"}})})
	for _, table := range []string{"public.read_only_query_fixture; INSERT INTO x VALUES (1)", "public.read_only_query_fixture; UPDATE x SET id = 1", "public.read_only_query_fixture; DELETE FROM x", "public.read_only_query_fixture; DROP TABLE x"} {
		executor := &countingExecutor{}
		service := execution.QueryService{Configuration: configuration, Snapshot: snapshot, Executor: executor, Principal: trusted(t, "local-dev")}
		if _, err := service.Execute(context.Background(), query.Plan{Source: "test_postgres", Table: table, Projection: []string{"id"}, Limit: 1}); err == nil || executor.calls != 0 {
			t.Fatalf("write shape reached executor: %q", table)
		}
	}
}

func TestPostgresRuntimeGuardrails(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}
	connection, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(context.Background())
	for _, sql := range []string{
		"CREATE TABLE IF NOT EXISTS public.runtime_guard_fixture (id integer PRIMARY KEY, note text)",
		"TRUNCATE public.runtime_guard_fixture",
		"INSERT INTO public.runtime_guard_fixture (id, note) VALUES (1, repeat('x', 128))",
		"CREATE OR REPLACE VIEW public.runtime_guard_sleep AS SELECT id FROM public.runtime_guard_fixture CROSS JOIN LATERAL pg_sleep(0.2)",
	} {
		if _, err := connection.Exec(context.Background(), sql); err != nil {
			t.Fatal(err)
		}
	}

	plan := func(table string, columns []string, limit int) query.Plan {
		return query.Plan{Source: "test_postgres", Table: table, Projection: columns, Limit: limit}
	}
	for name, test := range map[string]struct {
		runtime config.Runtime
		plan    query.Plan
		want    string
		after   query.Plan
	}{
		"timeout": {
			runtime: config.Runtime{StatementTimeout: 100 * time.Millisecond, MaxRows: 10, MaxResultBytes: 1024, MaxConcurrentQueries: 1},
			plan:    plan("public.runtime_guard_sleep", []string{"id"}, 1), want: "query execution timeout", after: plan("public.runtime_guard_fixture", []string{"id"}, 1),
		},
		"row limit": {
			runtime: config.Runtime{StatementTimeout: time.Second, MaxRows: 1, MaxResultBytes: 1024, MaxConcurrentQueries: 1},
			plan:    plan("public.runtime_guard_fixture", []string{"id"}, 2), want: "query execution row_limit", after: plan("public.runtime_guard_fixture", []string{"id"}, 1),
		},
		"result size": {
			runtime: config.Runtime{StatementTimeout: time.Second, MaxRows: 10, MaxResultBytes: 3, MaxConcurrentQueries: 1},
			plan:    plan("public.runtime_guard_fixture", []string{"note"}, 1), want: "query execution result_size", after: plan("public.runtime_guard_fixture", []string{"id"}, 1),
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := runtimeService(test.runtime)
			result, err := service.Execute(context.Background(), test.plan)
			if err == nil || err.Error() != test.want || len(result.Rows) != 0 {
				t.Fatalf("guardrail = %#v, %v", result, err)
			}
			if result, err := service.Execute(context.Background(), test.after); err != nil || len(result.Rows) != 1 {
				t.Fatalf("subsequent query = %#v, %v", result, err)
			}
		})
	}

	t.Run("capacity", func(t *testing.T) {
		service := runtimeService(config.Runtime{StatementTimeout: time.Second, MaxRows: 10, MaxResultBytes: 1024, MaxConcurrentQueries: 1})
		var once sync.Once
		started := make(chan struct{})
		go func() {
			once.Do(func() { close(started) })
			_, _ = service.Execute(context.Background(), plan("public.runtime_guard_sleep", []string{"id"}, 1))
		}()
		<-started
		// Give the first request time to enter PostgreSQL and acquire the gate.
		time.Sleep(25 * time.Millisecond)
		result, err := service.Execute(context.Background(), plan("public.runtime_guard_fixture", []string{"id"}, 1))
		if err == nil || err.Error() != "query execution capacity" || len(result.Rows) != 0 {
			t.Fatalf("capacity = %#v, %v", result, err)
		}
		time.Sleep(250 * time.Millisecond)
		if result, err := service.Execute(context.Background(), plan("public.runtime_guard_fixture", []string{"id"}, 1)); err != nil || len(result.Rows) != 1 {
			t.Fatalf("subsequent query = %#v, %v", result, err)
		}
	})
}

func runtimeService(runtime config.Runtime) execution.QueryService {
	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev", Runtime: &runtime}, Datasources: []config.Datasource{{Name: "test_postgres", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"}}, Policies: []config.Policy{
		{Name: "runtime-fixture", Principals: []string{"local-dev"}, Datasource: "test_postgres", Allow: []config.AllowEntry{{Table: "public.runtime_guard_fixture", Operations: []string{"select"}, Columns: []string{"id", "note"}}}},
		{Name: "runtime-sleep", Principals: []string{"local-dev"}, Datasource: "test_postgres", Allow: []config.AllowEntry{{Table: "public.runtime_guard_sleep", Operations: []string{"select"}, Columns: []string{"id"}}}},
	}}
	snapshot := schema.NewSnapshot("test_postgres", []schema.Relation{
		schema.NewRelation("public", "runtime_guard_fixture", []schema.Column{{Name: "id"}, {Name: "note"}}),
		schema.NewRelation("public", "runtime_guard_sleep", []schema.Column{{Name: "id"}}),
	})
	return execution.QueryService{Configuration: configuration, Snapshot: snapshot, Executor: pgadapter.NewExecutor(), Principal: trustedPrincipal("local-dev")}
}

func trusted(t *testing.T, name string) principal.Trusted {
	t.Helper()
	return trustedPrincipal(name)
}

func trustedPrincipal(name string) principal.Trusted {
	identity, err := principal.New(name)
	if err != nil {
		panic(err)
	}
	return identity
}

func readOnlyConfig() config.Config {
	return config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev"}, Datasources: []config.Datasource{{Name: "test_postgres", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"}}, Policies: []config.Policy{{Name: "fixture-read", Principals: []string{"local-dev"}, Datasource: "test_postgres", Allow: []config.AllowEntry{{Table: "public.read_only_query_fixture", Operations: []string{"select"}, Columns: []string{"id", "note"}}}}}}
}

type countingExecutor struct{ calls int }

func (executor *countingExecutor) Execute(context.Context, config.Config, string, compiler.Statement) (execution.Result, error) {
	executor.calls++
	return execution.Result{}, nil
}
