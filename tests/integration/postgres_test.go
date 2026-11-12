//go:build integration

package integration

import (
	"bytes"
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"datawark/internal/cli"
	"datawark/internal/config"
	pgadapter "datawark/internal/postgres"

	"github.com/jackc/pgx/v5"
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
