package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"datawark/internal/config"
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
