package cli

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"datawark/internal/config"
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

type mcpCatalog struct{}

func (mcpCatalog) Load(context.Context, config.Config, string) (schema.Snapshot, error) {
	return schema.NewSnapshot("analytics", nil), nil
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
