package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"datawark/internal/audit"
	"datawark/internal/config"
	"datawark/internal/httpserver"
	"datawark/internal/policy"
	"datawark/internal/runtime"
	"datawark/internal/schema"
	"github.com/spf13/cobra"
)

type fakePinger struct {
	dsn string
	err error
}

func (p *fakePinger) Ping(_ context.Context, dsn string) error { p.dsn = dsn; return p.err }

func TestConfigValidate(t *testing.T) {
	valid := filepath.Join("..", "config", "testdata", "valid.yaml")
	output, err := run(NewRootCommand(Dependencies{}), "config", "validate", "--config", valid)
	if err != nil || !strings.Contains(output, "configuration is valid") {
		t.Fatalf("output=%q err=%v", output, err)
	}
	_, err = run(NewRootCommand(Dependencies{}), "config", "validate", "--config", filepath.Join("..", "config", "testdata", "invalid-driver.yaml"))
	if err == nil || !strings.Contains(err.Error(), "datasources[0].driver") {
		t.Fatalf("error=%v", err)
	}
	example := filepath.Join("..", "..", "configs", "examples", "gateway-v1alpha1.yaml")
	output, err = run(NewRootCommand(Dependencies{}), "config", "validate", "--config", example)
	if err != nil || !strings.Contains(output, "configuration is valid") {
		t.Fatalf("example output=%q err=%v", output, err)
	}
}

func TestConfigValidateDoesNotLeakSecret(t *testing.T) {
	path := writeConfig(t, "version: v1alpha1\nserver:\n  unsafeDsn: postgres://reader:password@example.test/db\n")
	_, err := run(NewRootCommand(Dependencies{}), "config", "validate", "--config", path)
	if err == nil || strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestSourceTest(t *testing.T) {
	pinger := &fakePinger{}
	dependencies := Dependencies{Lookup: func(name string) (string, bool) {
		return "postgres://reader:password@example.test/db", name == "DATAWARK_ANALYTICS_DSN"
	}, Pinger: pinger}
	output, err := run(NewRootCommand(dependencies), "source", "test", "--config", filepath.Join("..", "config", "testdata", "valid.yaml"), "--datasource", "analytics")
	if err != nil || !strings.Contains(output, "analytics") || pinger.dsn == "" {
		t.Fatalf("output=%q err=%v dsn=%q", output, err, pinger.dsn)
	}
}

func TestSourceTestFailuresAreSafe(t *testing.T) {
	path := filepath.Join("..", "config", "testdata", "valid.yaml")
	_, err := run(NewRootCommand(Dependencies{Lookup: func(string) (string, bool) { return "", false }}), "source", "test", "--config", path, "--datasource", "analytics")
	if err == nil || !strings.Contains(err.Error(), "DATAWARK_ANALYTICS_DSN") {
		t.Fatalf("error=%v", err)
	}

	secret := "postgres://reader:password@example.test/db"
	_, err = run(NewRootCommand(Dependencies{Lookup: func(string) (string, bool) { return secret, true }, Pinger: &fakePinger{err: errors.New(secret)}}), "source", "test", "--config", path, "--datasource", "analytics")
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "password") {
		t.Fatalf("unsafe error: %v", err)
	}

	called := false
	_, err = run(NewRootCommand(Dependencies{Load: func(string) (config.Config, error) { return config.Config{}, errors.New("invalid") }, Pinger: pingerFunc(func(context.Context, string) error { called = true; return nil })}), "source", "test", "--config", "invalid.yaml", "--datasource", "analytics")
	if err == nil || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestMCPStdioBootstrapsBeforeServingAndKeepsErrorsOffStdout(t *testing.T) {
	var started bool
	command := NewRootCommand(Dependencies{
		Runtime: func(context.Context, string) (*runtime.Runtime, error) { return mcpRuntime(t), nil },
		RunMCP:  func(context.Context, runtime.Gateway) error { started = true; return nil },
	})
	if output, err := run(command, "mcp", "stdio", "--config", "gateway.yaml"); err != nil || output != "" || !started {
		t.Fatalf("output=%q err=%v started=%v", output, err, started)
	}

	started = false
	command = NewRootCommand(Dependencies{
		Runtime: func(context.Context, string) (*runtime.Runtime, error) { return nil, errors.New("secret") },
		RunMCP:  func(context.Context, runtime.Gateway) error { started = true; return nil },
	})
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"mcp", "stdio", "--config", "gateway.yaml"})
	if err := command.Execute(); err == nil || started || stdout.Len() != 0 || strings.Contains(err.Error(), "secret") {
		t.Fatalf("stdout=%q err=%v started=%v", stdout.String(), err, started)
	}
}

func TestServeIntegrationBootstrapsBeforeBindingAndShutsDown(t *testing.T) {
	secret := "postgres://reader:secret@example.test/db"
	for _, bootstrapErr := range []error{errors.New("invalid configuration"), errors.New(secret)} {
		listenerCalled := false
		command := NewRootCommand(Dependencies{
			Bootstrap: func(context.Context, string) (*runtime.Runtime, config.Config, error) {
				return nil, config.Config{}, bootstrapErr
			},
			Listen: func(string, string) (net.Listener, error) {
				listenerCalled = true
				return nil, nil
			},
		})
		if _, err := run(command, "serve", "--config", "invalid.yaml"); err == nil || listenerCalled || strings.Contains(err.Error(), secret) {
			t.Fatalf("error=%v listenerCalled=%v", err, listenerCalled)
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := NewRootCommand(Dependencies{
		Bootstrap: func(context.Context, string) (*runtime.Runtime, config.Config, error) {
			return mcpRuntime(t), config.Config{Server: config.Server{HTTP: &config.HTTP{ListenAddress: listener.Addr().String(), APIKeys: []config.APIKey{{Principal: "local-dev", KeyEnv: "DATAWARK_TEST_API_KEY"}}}}}, nil
		},
		Lookup: func(name string) (string, bool) { return "test-key", name == "DATAWARK_TEST_API_KEY" },
		Listen: func(string, string) (net.Listener, error) { return listener, nil },
	})
	command.SetContext(ctx)
	done := make(chan error, 1)
	go func() {
		_, err := run(command, "serve", "--config", "gateway.yaml")
		done <- err
	}()
	endpoint := "http://" + listener.Addr().String()
	deadline := time.Now().Add(time.Second)
	for {
		response, requestErr := http.Get(endpoint + "/api/v1/health")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("health status = %d", response.StatusCode)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(requestErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	response, err := http.Get(endpoint + "/api/v1/ready")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %d", response.StatusCode)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeWritesAuditEventsOnlyToStdout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := NewRootCommand(Dependencies{
		Bootstrap: func(context.Context, string) (*runtime.Runtime, config.Config, error) {
			return mcpRuntime(t), config.Config{Server: config.Server{HTTP: &config.HTTP{ListenAddress: listener.Addr().String(), APIKeys: []config.APIKey{{Principal: "local-dev", KeyEnv: "KEY"}}}}}, nil
		},
		Lookup: func(string) (string, bool) { return "key", true },
		Listen: func(string, string) (net.Listener, error) { return listener, nil },
	})
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetContext(ctx)
	command.SetArgs([]string{"serve", "--config", "gateway.yaml"})
	done := make(chan error, 1)
	go func() { done <- command.Execute() }()
	endpoint := "http://" + listener.Addr().String()
	deadline := time.Now().Add(time.Second)
	for {
		request, _ := http.NewRequest(http.MethodGet, endpoint+"/unknown", nil)
		request.Header.Set("Authorization", "Bearer key")
		response, requestErr := http.DefaultClient.Do(request)
		if requestErr == nil {
			response.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(requestErr)
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"operation":"authenticate"`) || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// TestServeProcessStreamsKeepAuditAndOperationalRecordsSeparated exercises the
// command boundary: Cobra's stdout becomes the audit sink and the process
// stderr remains the operational logger sink.
func TestServeProcessStreamsKeepAuditAndOperationalRecordsSeparated(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=TestMCPStdioProcessHelper", "--")
	command.Env = append(os.Environ(), "DATAWARK_SERVE_PROCESS_HELPER=1", "DATAWARK_SERVE_ADDRESS="+address, "DATAWARK_SERVE_CONFIG_SECRET=cli-config-secret-9841", "DATAWARK_SERVE_CREDENTIAL_SECRET=cli-credential-secret-9841")
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
	finished := false
	t.Cleanup(func() {
		if !finished && command.Process != nil {
			_ = command.Process.Kill()
			_, _ = io.Copy(io.Discard, stdout)
			_, _ = io.Copy(io.Discard, stderr)
			_ = command.Wait()
		}
	})
	stdoutDone := make(chan []byte, 1)
	stderrDone := make(chan []byte, 1)
	go func() { data, _ := io.ReadAll(stdout); stdoutDone <- data }()
	go func() { data, _ := io.ReadAll(stderr); stderrDone <- data }()
	endpoint := "http://" + address
	deadline := time.Now().Add(5 * time.Second)
	for {
		response, requestErr := http.Get(endpoint + "/api/v1/ready")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("serve helper did not become ready: %v", requestErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	request, err := http.NewRequest(http.MethodPost, endpoint+"/api/v1/query?hostile=cli-request-secret-9841", strings.NewReader(`{"source":"analytics","table":"public.hostile_cli_secret_9841","projection":["id"],"limit":1}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer cli-credential-secret-9841")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(httpserver.RequestIDHeader, "cli-request-id-9841")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("query status = %d", response.StatusCode)
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
	finished = true
	actualStdout, actualStderr := <-stdoutDone, <-stderrDone
	var audits []audit.Event
	for _, line := range strings.FieldsFunc(string(actualStdout), func(r rune) bool { return r == '\n' }) {
		var event audit.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Validate() != nil {
			t.Fatalf("stdout is not a complete audit event: %q (%v)", line, err)
		}
		audits = append(audits, event)
	}
	if len(audits) < 2 || audits[0].Operation != audit.OperationAuthenticate || audits[1].Operation != audit.OperationQuery {
		t.Fatalf("audit events = %#v", audits)
	}
	var lifecycle []string
	for _, line := range strings.FieldsFunc(string(actualStderr), func(r rune) bool { return r == '\n' }) {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil || record["time"] == nil || record["msg"] == nil {
			t.Fatalf("stderr is not operational JSON: %q (%v)", line, err)
		}
		if event, ok := record["event"].(string); ok {
			lifecycle = append(lifecycle, event)
		}
	}
	if strings.Join(lifecycle, ",") != "startup,readiness_withdrawn,shutdown" || !strings.Contains(string(actualStderr), "http request complete") {
		t.Fatalf("operational lifecycle = %v stderr=%s", lifecycle, actualStderr)
	}
	for _, secret := range []string{"cli-config-secret-9841", "cli-credential-secret-9841", "cli-request-secret-9841", "hostile_cli_secret_9841"} {
		if strings.Contains(string(actualStdout), secret) || strings.Contains(string(actualStderr), secret) {
			t.Fatalf("process stream leaked %q", secret)
		}
	}
}

type mcpCatalog struct{}

func (mcpCatalog) Load(context.Context, config.Config, string) (schema.Snapshot, error) {
	return schema.NewSnapshot("analytics", nil), nil
}

type explainCatalog struct{}

func (explainCatalog) Load(context.Context, config.Config, string) (schema.Snapshot, error) {
	return schema.NewSnapshot("analytics", []schema.Relation{schema.NewRelation("public", "orders", []schema.Column{{Name: "id"}})}), nil
}

func mcpRuntime(t *testing.T) *runtime.Runtime {
	t.Helper()
	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev"}, Datasources: []config.Datasource{{Name: "analytics", Driver: "postgres", DSNEnv: "TEST_DSN"}}}
	runtime, err := runtime.NewFromConfig(context.Background(), configuration, runtime.Dependencies{Catalog: mcpCatalog{}})
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func TestMCPStdioProcessKeepsDiagnosticsOffStdout(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestMCPStdioProcessHelper", "--", "mcp", "stdio", "--config", "missing.yaml")
	command.Env = append(os.Environ(), "DATAWARK_MCP_HELPER=1")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err == nil || stdout.Len() != 0 || !strings.Contains(stderr.String(), "MCP runtime bootstrap failed") {
		t.Fatalf("stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
	}
}

func TestMCPStdioProcessHelper(t *testing.T) {
	if os.Getenv("DATAWARK_SERVE_PROCESS_HELPER") == "1" {
		address := os.Getenv("DATAWARK_SERVE_ADDRESS")
		command := NewRootCommand(Dependencies{
			Bootstrap: func(context.Context, string) (*runtime.Runtime, config.Config, error) {
				return mcpRuntime(t), config.Config{Server: config.Server{HTTP: &config.HTTP{ListenAddress: address, APIKeys: []config.APIKey{{Principal: "local-dev", KeyEnv: "DATAWARK_SERVE_CREDENTIAL_SECRET"}}}}}, nil
			},
		})
		command.SetOut(os.Stdout)
		command.SetErr(os.Stderr)
		command.SetArgs([]string{"serve", "--config", "cli-config-secret-9841"})
		os.Exit(boolExit(command.Execute()))
	}
	if os.Getenv("DATAWARK_MCP_PROTOCOL_HELPER") == "1" {
		command := NewRootCommand(Dependencies{Runtime: func(context.Context, string) (*runtime.Runtime, error) { return mcpRuntime(t), nil }})
		command.SetArgs([]string{"mcp", "stdio", "--config", "gateway.yaml"})
		command.SetOut(os.Stdout)
		command.SetErr(os.Stderr)
		os.Exit(boolExit(command.Execute()))
	}
	if os.Getenv("DATAWARK_MCP_HELPER") != "1" {
		return
	}
	arguments := os.Args
	for index, argument := range arguments {
		if argument == "--" {
			os.Exit(Execute(arguments[index+1:], os.Stdout, os.Stderr))
		}
	}
	os.Exit(2)
}

func boolExit(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

func TestMCPStdioProcessEmitsOnlyProtocolMessages(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestMCPStdioProcessHelper", "--")
	command.Env = append(os.Environ(), "DATAWARK_MCP_PROTOCOL_HELPER=1")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(output)
	send := func(message any) { data, _ := json.Marshal(message); _, _ = input.Write(append(data, '\n')) }
	receive := func() string {
		if !scanner.Scan() {
			t.Fatalf("stdout ended: %s", stderr.String())
		}
		return scanner.Text()
	}
	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "1"}}})
	first := receive()
	send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}})
	send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": "query_data", "arguments": map[string]any{"source": "analytics", "table": "bad", "projection": []string{"id"}, "limit": 1}}})
	second := receive()
	_ = input.Close()
	_ = command.Wait()
	for _, line := range []string{first, second} {
		var message map[string]any
		if json.Unmarshal([]byte(line), &message) != nil || message["jsonrpc"] != "2.0" {
			t.Fatalf("stdout=%q", line)
		}
	}
	if strings.Contains(first+second, `"version":"v1alpha1"`) || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", first+second, stderr.String())
	}
}

func TestReadPlanRejectsUnsafeOrUnsupportedInput(t *testing.T) {
	secret := "hostile-plan-value-9841"
	valid := `{"source":"analytics","table":"public.orders","projection":["id"],"limit":1}`
	for _, test := range []struct {
		name   string
		source string
		input  string
		open   func(string) (io.ReadCloser, error)
	}{
		{"missing", "", valid, nil},
		{"unreadable", "plan.json", valid, func(string) (io.ReadCloser, error) { return nil, errors.New(secret) }},
		{"oversized", "-", strings.Repeat("x", int(maxPlanBytes)+1), nil},
		{"malformed", "-", `{"source":`, nil},
		{"trailing", "-", valid + ` {}`, nil},
		{"alias", "-", `{"source":"analytics","table":"public.orders","columns":["id"],"limit":1}`, nil},
		{"unknown", "-", valid[:len(valid)-1] + `,"principal":"` + secret + `"}`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := readPlan(strings.NewReader(test.input), test.open, test.source)
			if err == nil || strings.Contains(err.Error(), secret) {
				t.Fatalf("readPlan error = %v", err)
			}
		})
	}
}

func TestExplanationDTOIsVersionedAndNonDisclosing(t *testing.T) {
	allowed, err := json.Marshal(explanationDTO(policy.Explanation{Allowed: true, Reason: policy.ReasonAllowed, PolicyID: "orders-read"}))
	if err != nil || string(allowed) != `{"version":"v1alpha1","allowed":true,"reason":"allowed","policyId":"orders-read"}` {
		t.Fatalf("allowed = %s, %v", allowed, err)
	}
	denied, err := json.Marshal(explanationDTO(policy.Explanation{Reason: policy.ReasonDenied, PolicyID: "hidden-policy"}))
	if err != nil || string(denied) != `{"version":"v1alpha1","allowed":false,"reason":"denied"}` {
		t.Fatalf("denied = %s, %v", denied, err)
	}
}

func TestPolicyExplainUsesLocalRuntimeAndProducesOneSafeJSONResult(t *testing.T) {
	plan := `{"source":"analytics","table":"public.orders","projection":["id"],"limit":1}`
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planPath, []byte(plan), 0o600); err != nil {
		t.Fatal(err)
	}
	command := NewRootCommand(Dependencies{Runtime: func(context.Context, string) (*runtime.Runtime, error) { return explainRuntime(t), nil }})
	output, err := run(command, "policy", "explain", "--config", "gateway.yaml", "--plan", planPath)
	if err != nil || output != "{\"version\":\"v1alpha1\",\"allowed\":true,\"reason\":\"allowed\",\"policyId\":\"orders-read\"}\n" {
		t.Fatalf("output=%q err=%v", output, err)
	}

	command = NewRootCommand(Dependencies{Runtime: func(context.Context, string) (*runtime.Runtime, error) { return explainRuntime(t), nil }})
	var stdout, stderr bytes.Buffer
	command.SetIn(strings.NewReader(`{"source":"analytics","table":"public.hidden","projection":["id"],"limit":1}`))
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"policy", "explain", "--config", "gateway.yaml", "--plan", "-"})
	if err := command.Execute(); err != nil || stdout.String() != "{\"version\":\"v1alpha1\",\"allowed\":false,\"reason\":\"denied\"}\n" || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
	}
}

func TestPolicyExplainDoesNotAcceptIdentityFlags(t *testing.T) {
	for _, flag := range []string{"principal", "role"} {
		t.Run(flag, func(t *testing.T) {
			command := NewRootCommand(Dependencies{})
			command.SetArgs([]string{"policy", "explain", "--config", "gateway.yaml", "--plan", "-", "--" + flag, "other"})
			if err := command.Execute(); err == nil || strings.Contains(err.Error(), "other") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestPolicyExplainFailuresAreSafeAndDoNotBootstrap(t *testing.T) {
	secret := "postgres://reader:secret@example.test/db"
	called := false
	command := NewRootCommand(Dependencies{Runtime: func(context.Context, string) (*runtime.Runtime, error) {
		called = true
		return nil, errors.New(secret)
	}})
	var stdout, stderr bytes.Buffer
	command.SetIn(strings.NewReader(`{"source":"analytics","table":"public.orders","projection":["id"],"limit":1,"principal":"` + secret + `"}`))
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"policy", "explain", "--config", "gateway.yaml", "--plan", "-"})
	if err := command.Execute(); err == nil || called || stdout.Len() != 0 || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "principal") {
		t.Fatalf("stdout=%q err=%v called=%v", stdout.String(), err, called)
	}
}

func TestPolicyExplainValidPlanFailsSafelyDuringBootstrapAndBinding(t *testing.T) {
	plan := `{"source":"analytics","table":"public.orders","projection":["id"],"limit":1}`
	secret := "postgres://reader:distinctive-secret@example.test/db"
	for _, test := range []struct {
		name string
		deps Dependencies
	}{
		{"bootstrap", Dependencies{Runtime: func(context.Context, string) (*runtime.Runtime, error) { return nil, errors.New(secret) }}},
		{"binding", Dependencies{Runtime: func(context.Context, string) (*runtime.Runtime, error) { return explainRuntime(t), nil }, LocalGateway: func(*runtime.Runtime) (runtime.Gateway, error) { return nil, errors.New(secret) }}},
	} {
		t.Run(test.name, func(t *testing.T) {
			command := NewRootCommand(test.deps)
			var stdout, stderr bytes.Buffer
			command.SetIn(strings.NewReader(plan))
			command.SetOut(&stdout)
			command.SetErr(&stderr)
			command.SetArgs([]string{"policy", "explain", "--config", "gateway.yaml", "--plan", "-"})
			if err := command.Execute(); err == nil || stdout.Len() != 0 || stderr.Len() != 0 || strings.Contains(err.Error(), secret) {
				t.Fatalf("stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
			}
		})
	}
}

func TestPolicyExplainCLIWritesOnlySafeDiagnostics(t *testing.T) {
	secret := "postgres://reader:secret@example.test/db"
	t.Setenv("DATAWARK_CLI_EXPLAIN_SECRET", secret)
	planPath := filepath.Join(t.TempDir(), "hostile-plan.json")
	if err := os.WriteFile(planPath, []byte(`{"source":"analytics","table":"private.`+secret+`","projection":["id"],"limit":1,"rawSql":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exitCode := Execute([]string{"policy", "explain", "--config", "missing.yaml", "--plan", planPath}, &stdout, &stderr)
	if exitCode == 0 || stdout.Len() != 0 || strings.Contains(stdout.String()+stderr.String(), secret) {
		t.Fatalf("stdout=%q stderr=%q exit=%d", stdout.String(), stderr.String(), exitCode)
	}
}

func TestPolicyExplainOutputFailureIsSafe(t *testing.T) {
	command := NewRootCommand(Dependencies{Runtime: func(context.Context, string) (*runtime.Runtime, error) { return explainRuntime(t), nil }})
	command.SetIn(strings.NewReader(`{"source":"analytics","table":"public.orders","projection":["id"],"limit":1}`))
	command.SetOut(errorWriter{})
	command.SetArgs([]string{"policy", "explain", "--config", "gateway.yaml", "--plan", "-"})
	if err := command.Execute(); err == nil || strings.Contains(err.Error(), "writer secret") {
		t.Fatalf("error = %v", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("writer secret") }

func explainRuntime(t *testing.T) *runtime.Runtime {
	t.Helper()
	configuration := config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev"}, Datasources: []config.Datasource{{Name: "analytics", Driver: "postgres", DSNEnv: "TEST_DSN"}}, Policies: []config.Policy{{Name: "orders-read", Principals: []string{"local-dev"}, Datasource: "analytics", Allow: []config.AllowEntry{{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
	runtime, err := runtime.NewFromConfig(context.Background(), configuration, runtime.Dependencies{Catalog: explainCatalog{}})
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

type pingerFunc func(context.Context, string) error

func (fn pingerFunc) Ping(ctx context.Context, dsn string) error { return fn(ctx, dsn) }

func run(command *cobra.Command, args ...string) (string, error) {
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs(args)
	err := command.Execute()
	return output.String(), err
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
