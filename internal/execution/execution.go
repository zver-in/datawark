// Package execution provides the application-level read-only query path.
package execution

import (
	"context"
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/policy"
	"datawark/internal/query"
	"datawark/internal/schema"
	"errors"
)

// Result is an ordered, transport-neutral tabular result. Values preserve the
// database driver's native representation; nil represents a database NULL.
type Result struct {
	Columns []string
	Rows    [][]any
}

// ErrorCategory is a stable, non-sensitive category for execution failures.
type ErrorCategory string

const (
	ErrorUnavailable ErrorCategory = "unavailable"
	ErrorFailed      ErrorCategory = "failed"
)

// Error deliberately contains no driver details, SQL, arguments, or DSN.
type Error struct{ Category ErrorCategory }

func (error Error) Error() string { return "query execution " + string(error.Category) }

// Executor is the sole boundary that may execute a compiler-produced
// statement. The datasource name comes from the validated plan.
type Executor interface {
	Execute(context.Context, config.Config, string, compiler.Statement) (Result, error)
}

// QueryService validates, authorizes, validates metadata, compiles, then
// executes a plan. Callers cannot skip any of these gates.
type QueryService struct {
	Configuration config.Config
	Snapshot      schema.Snapshot
	Executor      Executor
	Compile       func(policy.AuthorizedQuery) (compiler.Statement, error)
}

func (service QueryService) Execute(ctx context.Context, plan query.Plan) (Result, error) {
	validated, err := query.Validate(plan)
	if err != nil {
		return Result{}, err
	}
	authorized, err := policy.Authorize(policy.Input{
		Principal: service.Configuration.Server.DevelopmentPrincipal,
		Snapshot:  &service.Configuration,
		Plan:      validated,
	})
	if err != nil {
		return Result{}, err
	}
	if err := schema.ValidatePlan(validated, &service.Snapshot); err != nil {
		return Result{}, err
	}
	compile := service.Compile
	if compile == nil {
		compile = compiler.Compile
	}
	statement, err := compile(authorized)
	if err != nil {
		return Result{}, err
	}
	if service.Executor == nil {
		return Result{}, errors.New("query execution unavailable")
	}
	return service.Executor.Execute(ctx, service.Configuration, plan.Source, statement)
}
