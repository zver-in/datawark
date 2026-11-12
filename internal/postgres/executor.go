package postgres

import (
	"context"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/execution"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Executor is the PostgreSQL implementation of execution.Executor.
type Executor struct {
	LookupEnv func(string) (string, bool)
	Connect   func(context.Context, string) (ExecutionConnection, error)
}

// ExecutionRows is the narrow row scanning boundary used by Executor.
type ExecutionRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
	FieldDescriptions() []pgconn.FieldDescription
}

// ExecutionTransaction performs exactly one statement for a request.
type ExecutionTransaction interface {
	Query(context.Context, string, ...any) (ExecutionRows, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

// ExecutionConnection starts explicit read-only transactions.
type ExecutionConnection interface {
	BeginTx(context.Context, pgx.TxOptions) (ExecutionTransaction, error)
	Close(context.Context) error
}

func NewExecutor() Executor { return Executor{LookupEnv: os.LookupEnv, Connect: connectExecutor} }

func connectExecutor(ctx context.Context, dsn string) (ExecutionConnection, error) {
	connection, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return pgxExecutionConnection{connection}, nil
}

type pgxExecutionConnection struct{ connection *pgx.Conn }

func (connection pgxExecutionConnection) BeginTx(ctx context.Context, options pgx.TxOptions) (ExecutionTransaction, error) {
	transaction, err := connection.connection.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return pgxExecutionTransaction{transaction}, nil
}
func (connection pgxExecutionConnection) Close(ctx context.Context) error {
	return connection.connection.Close(ctx)
}

type pgxExecutionTransaction struct{ transaction pgx.Tx }

func (transaction pgxExecutionTransaction) Query(ctx context.Context, sql string, args ...any) (ExecutionRows, error) {
	rows, err := transaction.transaction.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgxExecutionRows{rows}, nil
}
func (transaction pgxExecutionTransaction) Commit(ctx context.Context) error {
	return transaction.transaction.Commit(ctx)
}
func (transaction pgxExecutionTransaction) Rollback(ctx context.Context) error {
	return transaction.transaction.Rollback(ctx)
}

type pgxExecutionRows struct{ pgx.Rows }

// Execute resolves a configured datasource, then executes one compiler-issued
// statement in an explicit read-only transaction. Driver diagnostics are never
// returned to callers.
func (executor Executor) Execute(ctx context.Context, configuration config.Config, datasource string, statement compiler.Statement) (execution.Result, error) {
	if !statement.Valid() {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	if config.Validate(configuration) != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorUnavailable}
	}
	source, found := config.FindDatasource(configuration, datasource)
	if !found {
		return execution.Result{}, execution.Error{Category: execution.ErrorUnavailable}
	}
	lookup := executor.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	dsn, present := lookup(source.DSNEnv)
	if !present || dsn == "" {
		return execution.Result{}, execution.Error{Category: execution.ErrorUnavailable}
	}
	connect := executor.Connect
	if connect == nil {
		connect = connectExecutor
	}
	connection, err := connect(ctx, dsn)
	if err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorUnavailable}
	}
	defer connection.Close(ctx)
	transaction, err := connection.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	defer transaction.Rollback(ctx)
	rows, err := transaction.Query(ctx, statement.SQL(), statement.Args()...)
	if err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	result, err := scanRows(rows)
	if err != nil {
		return execution.Result{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	return result, nil
}

func scanRows(rows ExecutionRows) (execution.Result, error) {
	if rows == nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	result := execution.Result{Columns: make([]string, len(fields)), Rows: make([][]any, 0)}
	for index, field := range fields {
		result.Columns[index] = field.Name
	}
	for rows.Next() {
		values := make([]any, len(fields))
		destinations := make([]any, len(fields))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
		}
		result.Rows = append(result.Rows, values)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	return result, nil
}

var _ execution.Executor = Executor{}
