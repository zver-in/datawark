package query

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, violation := range e.Violations {
		parts = append(parts, fmt.Sprintf("%s: %s", violation.Path, violation.Message))
	}
	return strings.Join(parts, "; ")
}

// Validate checks that plan belongs to the deliberately small query grammar.
func Validate(plan Plan) (ValidatedPlan, error) {
	var violations []Violation
	add := func(path, message string) { violations = append(violations, Violation{Path: path, Message: message}) }

	if !identifier(plan.Source) {
		add("source", "must be a non-empty identifier")
	}
	table := strings.Split(plan.Table, ".")
	if len(table) != 2 || !identifierPart(table, 0) || !identifierPart(table, 1) {
		add("table", "must be schema.table")
	}
	if len(plan.Projection) == 0 {
		add("projection", "must contain at least one column")
	}
	projection := make(map[string]struct{}, len(plan.Projection))
	for i, column := range plan.Projection {
		path := fmt.Sprintf("projection[%d]", i)
		if !identifier(column) {
			add(path, "must be a non-empty identifier")
			continue
		}
		if _, exists := projection[column]; exists {
			add(path, "must be unique")
			continue
		}
		projection[column] = struct{}{}
	}
	for i, order := range plan.Order {
		path := fmt.Sprintf("order[%d]", i)
		if !identifier(order.Column) {
			add(path+".column", "must be a non-empty identifier")
		}
		if order.Direction != DirectionAscending && order.Direction != DirectionDescending {
			add(path+".direction", "must be asc or desc")
		}
	}
	for i, filter := range plan.Filters {
		path := fmt.Sprintf("filters[%d]", i)
		if !identifier(filter.Column) {
			add(path+".column", "must be a non-empty identifier")
		}
		if !supportedOperator(filter.Operator) {
			add(path+".operator", "is not supported")
		}
		if !supportedScalar(filter.Value) {
			add(path+".value", "must be a scalar")
		} else if filter.Value.Kind == ScalarNull && filter.Operator != OperatorEqual && filter.Operator != OperatorNotEqual {
			add(path+".value", "null is only allowed with eq or neq")
		}
	}
	if plan.Limit <= 0 {
		add("limit", "must be a positive integer")
	}
	if len(violations) > 0 {
		return ValidatedPlan{}, &ValidationError{Violations: violations}
	}
	return ValidatedPlan{Plan: plan}, nil
}

func supportedOperator(operator Operator) bool {
	return operator == OperatorEqual || operator == OperatorNotEqual ||
		operator == OperatorLessThan || operator == OperatorLessThanOrEqual ||
		operator == OperatorGreaterThan || operator == OperatorGreaterThanOrEqual
}

func supportedScalar(value Scalar) bool {
	if value.Kind == ScalarNumber {
		return !math.IsNaN(value.Number) && !math.IsInf(value.Number, 0)
	}
	return value.Kind == ScalarString || value.Kind == ScalarBoolean || value.Kind == ScalarNull
}

func identifierPart(parts []string, index int) bool {
	return len(parts) > index && identifier(parts[index])
}

func identifier(value string) bool {
	if value == "" {
		return false
	}
	for i, character := range value {
		if character == '_' || unicode.IsLetter(character) || (i > 0 && unicode.IsDigit(character)) {
			continue
		}
		return false
	}
	return true
}
