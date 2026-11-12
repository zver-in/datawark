// Package compiler turns an authorized query into one parameterized PostgreSQL
// SELECT statement. It does not connect to or execute against a database.
package compiler

import (
	"datawark/internal/policy"
	"datawark/internal/query"
	"errors"
	"fmt"
	"strings"
)

// Statement is a side-effect-free PostgreSQL statement and its positional
// arguments. Args are ordered by the placeholders in SQL.
type Statement struct {
	sql   string
	args  []any
	valid bool
}

// SQL returns the parameterized SQL text. It is intended for a database
// adapter, not a transport boundary.
func (statement Statement) SQL() string { return statement.sql }

// Args returns a copy of the positional arguments in placeholder order.
func (statement Statement) Args() []any { return append([]any(nil), statement.args...) }

// Valid reports whether this statement was produced by Compile. Its unexported
// marker prevents callers in other packages from constructing executable SQL.
func (statement Statement) Valid() bool { return statement.valid }

// Compile renders a single SELECT from a capability issued by policy.Authorize.
func Compile(authorized policy.AuthorizedQuery) (Statement, error) {
	validated, ok := authorized.Plan()
	if !ok {
		return Statement{}, errors.New("compiler: query is not authorized")
	}
	if _, err := query.Validate(validated.Plan); err != nil {
		return Statement{}, errors.New("compiler: invalid authorized query")
	}
	return compile(validated.Plan)
}

func compile(plan query.Plan) (Statement, error) {
	table := strings.Split(plan.Table, ".")
	if len(table) != 2 || table[0] == "" || table[1] == "" {
		return Statement{}, errors.New("compiler: invalid table identifier")
	}

	columns := make([]string, len(plan.Projection))
	for i, column := range plan.Projection {
		if column == "" {
			return Statement{}, errors.New("compiler: invalid column identifier")
		}
		columns[i] = quoteIdentifier(column)
	}

	var sql strings.Builder
	sql.WriteString("SELECT ")
	sql.WriteString(strings.Join(columns, ", "))
	sql.WriteString(" FROM ")
	sql.WriteString(quoteIdentifier(table[0]))
	sql.WriteByte('.')
	sql.WriteString(quoteIdentifier(table[1]))

	args := make([]any, 0, len(plan.Filters)+1)
	if len(plan.Filters) > 0 {
		filters := make([]string, len(plan.Filters))
		for i, filter := range plan.Filters {
			operator, err := comparisonOperator(filter.Operator, filter.Value.Kind)
			if err != nil || filter.Column == "" {
				return Statement{}, errors.New("compiler: unsupported filter")
			}
			args = append(args, scalarArgument(filter.Value))
			filters[i] = fmt.Sprintf("%s %s $%d", quoteIdentifier(filter.Column), operator, len(args))
		}
		sql.WriteString(" WHERE ")
		sql.WriteString(strings.Join(filters, " AND "))
	}

	if len(plan.Order) > 0 {
		orders := make([]string, len(plan.Order))
		for i, order := range plan.Order {
			direction, err := sortDirection(order.Direction)
			if err != nil || order.Column == "" {
				return Statement{}, errors.New("compiler: unsupported ordering")
			}
			orders[i] = quoteIdentifier(order.Column) + " " + direction
		}
		sql.WriteString(" ORDER BY ")
		sql.WriteString(strings.Join(orders, ", "))
	}

	args = append(args, plan.Limit)
	fmt.Fprintf(&sql, " LIMIT $%d", len(args))
	return Statement{sql: sql.String(), args: args, valid: true}, nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func comparisonOperator(operator query.Operator, kind query.ScalarKind) (string, error) {
	if kind == query.ScalarNull {
		switch operator {
		case query.OperatorEqual:
			return "IS NOT DISTINCT FROM", nil
		case query.OperatorNotEqual:
			return "IS DISTINCT FROM", nil
		}
	}
	switch operator {
	case query.OperatorEqual:
		return "=", nil
	case query.OperatorNotEqual:
		return "<>", nil
	case query.OperatorLessThan:
		return "<", nil
	case query.OperatorLessThanOrEqual:
		return "<=", nil
	case query.OperatorGreaterThan:
		return ">", nil
	case query.OperatorGreaterThanOrEqual:
		return ">=", nil
	default:
		return "", errors.New("unsupported comparison operator")
	}
}

func sortDirection(direction query.Direction) (string, error) {
	switch direction {
	case query.DirectionAscending:
		return "ASC", nil
	case query.DirectionDescending:
		return "DESC", nil
	default:
		return "", errors.New("unsupported sort direction")
	}
}

func scalarArgument(value query.Scalar) any {
	switch value.Kind {
	case query.ScalarString:
		return value.String
	case query.ScalarNumber:
		return value.Number
	case query.ScalarBoolean:
		return value.Bool
	case query.ScalarNull:
		return nil
	default:
		return nil
	}
}
