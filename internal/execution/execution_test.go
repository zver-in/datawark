package execution

import (
	"context"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/policy"
	"datawark/internal/query"
	"datawark/internal/schema"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestQueryServiceStopsBeforeExecutorForEachFailedGate(t *testing.T) {
	valid := testPlan()
	for name, service := range map[string]QueryService{
		"validation": {Configuration: testConfig(), Snapshot: testSnapshot(), Executor: &recordingExecutor{}},
		"policy":     {Configuration: config.Config{}, Snapshot: testSnapshot(), Executor: &recordingExecutor{}},
		"schema":     {Configuration: testConfig(), Snapshot: schema.NewSnapshot("analytics", nil), Executor: &recordingExecutor{}},
		"compile": {Configuration: testConfig(), Snapshot: testSnapshot(), Executor: &recordingExecutor{}, Compile: func(policy.AuthorizedQuery) (compiler.Statement, error) {
			return compiler.Statement{}, errors.New("compile failed")
		}},
	} {
		t.Run(name, func(t *testing.T) {
			executor := service.Executor.(*recordingExecutor)
			plan := valid
			if name == "validation" {
				plan.Table = "orders"
			}
			if _, err := service.Execute(context.Background(), plan); err == nil {
				t.Fatal("expected failure")
			}
			if executor.called {
				t.Fatal("executor called after failed gate")
			}
		})
	}
}

func TestQueryServiceExecutesOnlyAfterAllGates(t *testing.T) {
	executor := &recordingExecutor{result: Result{Columns: []string{"id"}, Rows: [][]any{{1}}}}
	service := QueryService{Configuration: testConfig(), Snapshot: testSnapshot(), Executor: executor}
	result, err := service.Execute(context.Background(), testPlan())
	if err != nil || !executor.called || !reflect.DeepEqual(result, executor.result) {
		t.Fatalf("Execute() = %#v, %v", result, err)
	}
}

func TestResultPreservesColumnsEmptyRowsAndNulls(t *testing.T) {
	result := Result{Columns: []string{"id", "note"}, Rows: [][]any{{1, nil}}}
	if result.Columns[1] != "note" || result.Rows[0][1] != nil {
		t.Fatalf("result lost labels or NULL: %#v", result)
	}
	empty := Result{Columns: []string{"id"}, Rows: [][]any{}}
	if len(empty.Columns) != 1 || len(empty.Rows) != 0 {
		t.Fatalf("empty result = %#v", empty)
	}
	for _, err := range []error{Error{Category: ErrorUnavailable}, Error{Category: ErrorFailed}} {
		if strings.Contains(err.Error(), "postgres://") || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
}

type recordingExecutor struct {
	called bool
	result Result
	err    error
}

func (executor *recordingExecutor) Execute(context.Context, config.Config, string, compiler.Statement) (Result, error) {
	executor.called = true
	return executor.result, executor.err
}

func testPlan() query.Plan {
	return query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id", "note"}, Limit: 10}
}
func testConfig() config.Config {
	return config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "reader"}, Datasources: []config.Datasource{{Name: "analytics", Driver: "postgres", DSNEnv: "TEST_DSN"}}, Policies: []config.Policy{{Name: "orders", Principals: []string{"reader"}, Datasource: "analytics", Allow: []config.AllowEntry{{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id", "note"}}}}}}
}
func testSnapshot() schema.Snapshot {
	return schema.NewSnapshot("analytics", []schema.Relation{schema.NewRelation("public", "orders", []schema.Column{{Name: "id"}, {Name: "note", Nullable: true}})})
}
