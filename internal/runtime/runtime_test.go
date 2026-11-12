package runtime

import (
	"context"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/execution"
	"datawark/internal/postgres"
	"datawark/internal/query"
	"datawark/internal/schema"
	"errors"
	"strings"
	"testing"
)

type catalogFake struct {
	snapshots map[string]schema.Snapshot
	err       error
}

func (fake catalogFake) Load(_ context.Context, _ config.Config, datasource string) (schema.Snapshot, error) {
	if fake.err != nil {
		return schema.Snapshot{}, fake.err
	}
	snapshot, found := fake.snapshots[datasource]
	if !found {
		return schema.Snapshot{}, errors.New("missing")
	}
	return snapshot, nil
}

type executorFake struct{ calls int }

func (fake *executorFake) Execute(context.Context, config.Config, string, compiler.Statement) (execution.Result, error) {
	fake.calls++
	return execution.Result{}, nil
}

func TestNewBootstrapsCompleteImmutableRuntime(t *testing.T) {
	cfg := testConfig()
	first := schema.NewSnapshot("primary", []schema.Relation{schema.NewRelation("public", "orders", []schema.Column{{Name: "id"}, {Name: "secret"}})})
	second := schema.NewSnapshot("reporting", []schema.Relation{schema.NewRelation("public", "reports", []schema.Column{{Name: "id"}})})
	runtime, err := NewFromConfig(context.Background(), cfg, Dependencies{Catalog: catalogFake{snapshots: map[string]schema.Snapshot{"primary": first, "reporting": second}}})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Policies[0].Allow[0].Columns[0] = "secret"
	firstRelations := first.Relations()
	firstRelations[0] = schema.NewRelation("public", "other", nil)
	if got := runtime.Datasources(); len(got) != 1 || got[0] != "primary" {
		t.Fatalf("datasources = %v", got)
	}
	visible, err := runtime.Schema("primary")
	if err != nil || len(visible.Relations()) != 1 || len(visible.Relations()[0].Columns()) != 1 {
		t.Fatalf("schema = %#v, %v", visible, err)
	}
}

func TestBootstrapFailsClosedWithoutSecrets(t *testing.T) {
	secret := "postgres://user:secret@example.test/db"
	for _, test := range []struct {
		name string
		cfg  config.Config
		fake catalogFake
	}{
		{"invalid", config.Config{}, catalogFake{}},
		{"missing catalog", testConfig(), catalogFake{err: errors.New(secret)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, err := NewFromConfig(context.Background(), test.cfg, Dependencies{Catalog: test.fake})
			if runtime != nil || err == nil || strings.Contains(err.Error(), secret) {
				t.Fatalf("runtime=%v err=%v", runtime, err)
			}
		})
	}
	missingDSN := testConfig()
	missingDSN.Datasources[0].DSNEnv = "DATAWARK_RUNTIME_TEST_MISSING_DSN"
	runtime, err := NewFromConfig(context.Background(), missingDSN, Dependencies{Catalog: postgres.CatalogReader{LookupEnv: func(string) (string, bool) { return "", false }}})
	if runtime != nil || err == nil {
		t.Fatalf("missing DSN exposed runtime=%v err=%v", runtime, err)
	}
}

func TestRuntimeOperationsNeverExecuteRejectedPlans(t *testing.T) {
	executor := &executorFake{}
	runtime, err := NewFromConfig(context.Background(), testConfig(), Dependencies{Catalog: catalogFake{snapshots: map[string]schema.Snapshot{"primary": schema.NewSnapshot("primary", []schema.Relation{schema.NewRelation("public", "orders", []schema.Column{{Name: "id"}, {Name: "secret"}})}), "reporting": schema.NewSnapshot("reporting", nil)}}, Executor: executor})
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []query.Plan{
		{Source: "unknown", Table: "public.orders", Projection: []string{"id"}, Limit: 1},
		{Source: "primary", Table: "public.orders", Projection: []string{"secret"}, Limit: 1},
		{Source: "primary", Table: "public.missing", Projection: []string{"id"}, Limit: 1},
	} {
		if _, err := runtime.Execute(context.Background(), plan); err == nil {
			t.Fatalf("plan accepted: %#v", plan)
		}
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls = %d", executor.calls)
	}
	if explanation := runtime.Explain(query.Plan{Source: "primary", Table: "public.orders", Projection: []string{"id"}, Limit: 1}); !explanation.Allowed {
		t.Fatalf("explanation = %#v", explanation)
	}
	if explanation := runtime.Explain(query.Plan{Source: "primary", Table: "public.missing", Projection: []string{"id"}, Limit: 1}); explanation.Allowed {
		t.Fatalf("stale explanation = %#v", explanation)
	}
	if executor.calls != 0 {
		t.Fatalf("explain executed: %d", executor.calls)
	}
}

func testConfig() config.Config {
	return config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "developer"}, Datasources: []config.Datasource{{Name: "primary", Driver: "postgres", DSNEnv: "PRIMARY_DSN"}, {Name: "reporting", Driver: "postgres", DSNEnv: "REPORTING_DSN"}}, Policies: []config.Policy{{Name: "read-orders", Principals: []string{"developer"}, Datasource: "primary", Allow: []config.AllowEntry{{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
}
