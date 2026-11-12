package postgres

import (
	"context"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/execution"
	"datawark/internal/policy"
	"datawark/internal/query"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestExecutorRejectsUnavailableDatasourcesAndSanitizesConnectionFailures(t *testing.T) {
	statement := compiled(t)
	for name, executor := range map[string]Executor{
		"unknown": {LookupEnv: func(string) (string, bool) { return "", false }},
		"missing": {LookupEnv: func(string) (string, bool) { return "", false }},
		"connect": {LookupEnv: func(string) (string, bool) { return "postgres://reader:secret@example.test/db", true }, Connect: func(context.Context, string) (ExecutionConnection, error) {
			return nil, errors.New("postgres://reader:secret@example.test/db")
		}},
	} {
		t.Run(name, func(t *testing.T) {
			datasource := "analytics"
			if name == "unknown" {
				datasource = "missing"
			}
			_, err := executor.Execute(context.Background(), executorConfig(), datasource, statement)
			if err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "postgres://") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
}

func TestExecutorUsesOneReadOnlyTransactionAndScansRows(t *testing.T) {
	connection := &fakeExecutionConnection{transaction: &fakeExecutionTransaction{rows: &fakeExecutionRows{fields: []pgconn.FieldDescription{{Name: "id"}, {Name: "note"}}, values: [][]any{{int32(1), nil}}}}}
	executor := Executor{LookupEnv: func(string) (string, bool) { return "dsn", true }, Connect: func(context.Context, string) (ExecutionConnection, error) { return connection, nil }}
	result, err := executor.Execute(context.Background(), executorConfig(), "analytics", compiled(t))
	if err != nil {
		t.Fatal(err)
	}
	if connection.options.AccessMode != pgx.ReadOnly || connection.transaction.queries != 1 || !connection.transaction.committed || !connection.transaction.rolledBack {
		t.Fatalf("transaction lifecycle: %#v", connection)
	}
	if !reflect.DeepEqual(result, execution.Result{Columns: []string{"id", "note"}, Rows: [][]any{{int32(1), nil}}}) {
		t.Fatalf("result = %#v", result)
	}
}

func TestExecutorRejectsNonCompilerStatementAndScanFailures(t *testing.T) {
	connection := &fakeExecutionConnection{transaction: &fakeExecutionTransaction{rows: &fakeExecutionRows{fields: []pgconn.FieldDescription{{Name: "id"}}, scanErr: errors.New("driver details")}}}
	executor := Executor{LookupEnv: func(string) (string, bool) { return "dsn", true }, Connect: func(context.Context, string) (ExecutionConnection, error) { return connection, nil }}
	if _, err := executor.Execute(context.Background(), executorConfig(), "analytics", compiler.Statement{}); err == nil || connection.begun {
		t.Fatal("non-compiler statement reached database")
	}
	if _, err := executor.Execute(context.Background(), executorConfig(), "analytics", compiled(t)); err == nil {
		t.Fatal("scan failure succeeded")
	}
}

func TestExecutorRejectsRowCloseFailure(t *testing.T) {
	rows := &fakeExecutionRows{fields: []pgconn.FieldDescription{{Name: "id"}}, err: errors.New("close failed")}
	connection := &fakeExecutionConnection{transaction: &fakeExecutionTransaction{rows: rows}}
	executor := Executor{LookupEnv: func(string) (string, bool) { return "dsn", true }, Connect: func(context.Context, string) (ExecutionConnection, error) { return connection, nil }}
	if _, err := executor.Execute(context.Background(), executorConfig(), "analytics", compiled(t)); err == nil {
		t.Fatal("row close failure succeeded")
	}
}

func compiled(t *testing.T) compiler.Statement {
	t.Helper()
	plan, err := query.Validate(query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := policy.Authorize(policy.Input{Principal: "reader", Snapshot: ptr(executorConfig()), Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	statement, err := compiler.Compile(authorized)
	if err != nil {
		t.Fatal(err)
	}
	return statement
}
func executorConfig() config.Config {
	return config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "reader"}, Datasources: []config.Datasource{{Name: "analytics", Driver: "postgres", DSNEnv: "TEST_DSN"}}, Policies: []config.Policy{{Name: "orders", Principals: []string{"reader"}, Datasource: "analytics", Allow: []config.AllowEntry{{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
}
func ptr(value config.Config) *config.Config { return &value }

type fakeExecutionConnection struct {
	transaction *fakeExecutionTransaction
	options     pgx.TxOptions
	begun       bool
}

func (connection *fakeExecutionConnection) BeginTx(_ context.Context, options pgx.TxOptions) (ExecutionTransaction, error) {
	connection.options = options
	connection.begun = true
	return connection.transaction, nil
}
func (*fakeExecutionConnection) Close(context.Context) error { return nil }

type fakeExecutionTransaction struct {
	rows       *fakeExecutionRows
	queries    int
	committed  bool
	rolledBack bool
}

func (transaction *fakeExecutionTransaction) Query(context.Context, string, ...any) (ExecutionRows, error) {
	transaction.queries++
	return transaction.rows, nil
}
func (transaction *fakeExecutionTransaction) Commit(context.Context) error {
	transaction.committed = true
	return nil
}
func (transaction *fakeExecutionTransaction) Rollback(context.Context) error {
	transaction.rolledBack = true
	return nil
}

type fakeExecutionRows struct {
	fields  []pgconn.FieldDescription
	values  [][]any
	index   int
	scanErr error
	err     error
}

func (rows *fakeExecutionRows) Next() bool {
	return rows.index < len(rows.values) || (rows.scanErr != nil && rows.index == 0)
}
func (rows *fakeExecutionRows) Scan(destinations ...any) error {
	if rows.scanErr != nil {
		rows.index++
		return rows.scanErr
	}
	for index, destination := range destinations {
		*(destination.(*any)) = rows.values[rows.index][index]
	}
	rows.index++
	return nil
}
func (rows *fakeExecutionRows) Err() error                                   { return rows.err }
func (*fakeExecutionRows) Close()                                            {}
func (rows *fakeExecutionRows) FieldDescriptions() []pgconn.FieldDescription { return rows.fields }
