//go:build integration

package integration

import (
	"bytes"
	"context"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"datawark/internal/cli"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/execution"
	pgadapter "datawark/internal/postgres"
	"datawark/internal/query"
	"datawark/internal/schema"

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
	service := execution.QueryService{Configuration: configuration, Snapshot: snapshot, Executor: pgadapter.NewExecutor()}
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
		service := execution.QueryService{Configuration: configuration, Snapshot: snapshot, Executor: executor}
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
			runtime: config.Runtime{StatementTimeout: 20 * time.Millisecond, MaxRows: 10, MaxResultBytes: 1024, MaxConcurrentQueries: 1},
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
	return execution.QueryService{Configuration: configuration, Snapshot: snapshot, Executor: pgadapter.NewExecutor()}
}

func readOnlyConfig() config.Config {
	return config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "local-dev"}, Datasources: []config.Datasource{{Name: "test_postgres", Driver: "postgres", DSNEnv: "DATAWARK_TEST_POSTGRES_DSN"}}, Policies: []config.Policy{{Name: "fixture-read", Principals: []string{"local-dev"}, Datasource: "test_postgres", Allow: []config.AllowEntry{{Table: "public.read_only_query_fixture", Operations: []string{"select"}, Columns: []string{"id", "note"}}}}}}
}

type countingExecutor struct{ calls int }

func (executor *countingExecutor) Execute(context.Context, config.Config, string, compiler.Statement) (execution.Result, error) {
	executor.calls++
	return execution.Result{}, nil
}
