//go:build integration

package integration

import (
	"bytes"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"datawark/internal/cli"
)

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
