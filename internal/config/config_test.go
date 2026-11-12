package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseFixtures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file string
		path string
	}{
		{"unknown field", "unknown-field.yaml", "server.unsafeDsn"},
		{"empty dsn env", "empty-dsn-env.yaml", "datasources[0].dsnEnv"},
		{"invalid driver", "invalid-driver.yaml", "datasources[0].driver"},
		{"invalid operation", "invalid-operation.yaml", "policies[0].allow[0].operations[0]"},
		{"duplicate names", "duplicate-names.yaml", "datasources[1].name"},
		{"unknown datasource", "unknown-policy-datasource.yaml", "policies[0].datasource"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := readFixture(t, test.file)
			_, err := Parse(data)
			if err == nil || !strings.Contains(err.Error(), test.path) {
				t.Fatalf("Parse() error = %v, want field path %q", err, test.path)
			}
		})
	}
}

func TestParseValidFixture(t *testing.T) {
	t.Parallel()
	cfg, err := Parse(readFixture(t, "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Datasources[0].DSNEnv != "DATAWARK_ANALYTICS_DSN" {
		t.Fatal("DSN environment name was not decoded")
	}
	if got := cfg.Server.RuntimeLimits(); got != DefaultRuntime() {
		t.Fatalf("runtime defaults = %#v", got)
	}
}

func TestParseRuntimeLimits(t *testing.T) {
	t.Parallel()
	valid := []byte("version: v1alpha1\nserver:\n  developmentPrincipal: reader\n  runtime:\n    statementTimeout: 250ms\n    maxRows: 2\n    maxResultBytes: 3\n    maxConcurrentQueries: 4\ndatasources:\n  - name: analytics\n    driver: postgres\n    dsnEnv: DATAWARK_ANALYTICS_DSN\npolicies:\n  - name: orders\n    principals: [reader]\n    datasource: analytics\n    allow:\n      - table: public.orders\n        operations: [select]\n        columns: [id]\n")
	cfg, err := Parse(valid)
	if err != nil || cfg.Server.Runtime == nil || cfg.Server.Runtime.StatementTimeout != 250*time.Millisecond {
		t.Fatalf("Parse() = %#v, %v", cfg.Server.Runtime, err)
	}
	secret := "postgres://reader:secret@example.test/db"
	for name, test := range map[string]struct{ old, replacement string }{
		"zero":      {"maxRows: 2", "maxRows: 0"},
		"negative":  {"maxResultBytes: 3", "maxResultBytes: -1"},
		"malformed": {"statementTimeout: 250ms", "statementTimeout: nope"},
		"unknown":   {"maxRows: 2", "unbounded: " + secret},
	} {
		t.Run(name, func(t *testing.T) {
			data := strings.Replace(string(valid), test.old, test.replacement, 1)
			_, err := Parse([]byte(data))
			if err == nil || !strings.Contains(err.Error(), "server.runtime") || strings.Contains(err.Error(), secret) {
				t.Fatalf("unsafe runtime error: %v", err)
			}
		})
	}
}

func TestParseRejectsMalformedDuplicateAndMultipleDocuments(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"version: [",
		"version: v1alpha1\nversion: v1alpha1\n",
		"version: v1alpha1\n---\nversion: v1alpha1\n",
	} {
		_, err := Parse([]byte(input))
		if err == nil {
			t.Fatalf("Parse(%q) succeeded", input)
		}
	}
}

func TestParseDoesNotReflectSecretInput(t *testing.T) {
	t.Parallel()
	secret := "postgres://reader:very-secret@example.test/db"
	_, err := Parse([]byte("version: v1alpha1\nserver:\n  unsafe: " + secret + "\n"))
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
