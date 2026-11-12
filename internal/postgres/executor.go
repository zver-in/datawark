package postgres

import (
	"context"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/execution"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Executor is the PostgreSQL implementation of execution.Executor.
type Executor struct {
	LookupEnv func(string) (string, bool)
	Connect   func(context.Context, string) (ExecutionConnection, error)
	mu        sync.Mutex
	gate      chan struct{}
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
	Exec(context.Context, string, ...any) error
	Query(context.Context, string, ...any) (ExecutionRows, error)
	Commit(context.Context) error
	Rollback(context.Context) error
}

// ExecutionConnection starts explicit read-only transactions.
type ExecutionConnection interface {
	BeginTx(context.Context, pgx.TxOptions) (ExecutionTransaction, error)
	Close(context.Context) error
}

func NewExecutor() *Executor { return &Executor{LookupEnv: os.LookupEnv, Connect: connectExecutor} }

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

func (transaction pgxExecutionTransaction) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := transaction.transaction.Exec(ctx, sql, args...)
	return err
}

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
func (executor *Executor) Execute(ctx context.Context, configuration config.Config, datasource string, statement compiler.Statement) (execution.Result, error) {
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
	runtime := configuration.Server.RuntimeLimits()
	if statement.Limit() > runtime.MaxRows {
		return execution.Result{}, execution.Error{Category: execution.ErrorRowLimit}
	}
	if !executor.acquire(runtime.MaxConcurrentQueries) {
		return execution.Result{}, execution.Error{Category: execution.ErrorCapacity}
	}
	defer executor.release()
	deadline, cancel := context.WithTimeout(ctx, runtime.StatementTimeout)
	defer cancel()
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
	connection, err := connect(deadline, dsn)
	if err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorUnavailable}
	}
	defer connection.Close(context.Background())
	transaction, err := connection.BeginTx(deadline, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	defer transaction.Rollback(context.Background())
	if err := transaction.Exec(deadline, "SET LOCAL statement_timeout = '"+durationMilliseconds(runtime.StatementTimeout)+"ms'"); err != nil {
		return execution.Result{}, deadlineError(deadline)
	}
	rows, err := transaction.Query(deadline, statement.SQL(), statement.Args()...)
	if err != nil {
		return execution.Result{}, deadlineError(deadline)
	}
	result, err := scanRows(rows, runtime.MaxResultBytes)
	if err != nil {
		if deadline.Err() != nil {
			return execution.Result{}, deadlineError(deadline)
		}
		return execution.Result{}, err
	}
	if err := transaction.Commit(deadline); err != nil {
		return execution.Result{}, deadlineError(deadline)
	}
	return result, nil
}

func (executor *Executor) acquire(capacity int) bool {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	if executor.gate == nil || (cap(executor.gate) != capacity && len(executor.gate) == 0) {
		executor.gate = make(chan struct{}, capacity)
	}
	select {
	case executor.gate <- struct{}{}:
		return true
	default:
		return false
	}
}

func (executor *Executor) release() { <-executor.gate }

func durationMilliseconds(duration time.Duration) string {
	return strconv.FormatInt(int64((duration+time.Millisecond-1)/time.Millisecond), 10)
}

func deadlineError(ctx context.Context) error {
	if ctx.Err() == context.DeadlineExceeded {
		return execution.Error{Category: execution.ErrorTimeout}
	}
	return execution.Error{Category: execution.ErrorFailed}
}

func scanRows(rows ExecutionRows, maxBytes int) (execution.Result, error) {
	if rows == nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	result := execution.Result{Columns: make([]string, len(fields)), Rows: make([][]any, 0)}
	usedBytes := 0
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
		encoded, err := json.Marshal(values)
		if err != nil || len(encoded) > maxBytes-usedBytes {
			return execution.Result{}, execution.Error{Category: execution.ErrorResultSize}
		}
		usedBytes += len(encoded)
		result.Rows = append(result.Rows, values)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return execution.Result{}, execution.Error{Category: execution.ErrorFailed}
	}
	return result, nil
}

var _ execution.Executor = (*Executor)(nil)
