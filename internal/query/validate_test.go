package query

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestValidateAcceptsEachFilterOperator(t *testing.T) {
	t.Parallel()
	for _, operator := range []Operator{OperatorEqual, OperatorNotEqual, OperatorLessThan, OperatorLessThanOrEqual, OperatorGreaterThan, OperatorGreaterThanOrEqual} {
		t.Run(string(operator), func(t *testing.T) {
			plan := validPlan()
			plan.Filters = []Filter{{Column: "amount", Operator: operator, Value: Number(42)}}
			if _, err := Validate(plan); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidateRejectsInvalidFilters(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		filter Filter
		path   string
	}{
		{"empty column", Filter{Operator: OperatorEqual, Value: String("a")}, "filters[0].column"},
		{"operator", Filter{Column: "name", Operator: "like", Value: String("a")}, "filters[0].operator"},
		{"array or object shape", Filter{Column: "name", Operator: OperatorEqual, Value: Scalar{Kind: "array"}}, "filters[0].value"},
		{"non-finite number", Filter{Column: "amount", Operator: OperatorEqual, Value: Scalar{Kind: ScalarNumber, Number: math.NaN()}}, "filters[0].value"},
		{"null comparison", Filter{Column: "amount", Operator: OperatorLessThan, Value: Null()}, "filters[0].value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := validPlan()
			plan.Filters = []Filter{test.filter}
			_, err := Validate(plan)
			if err == nil || !strings.Contains(err.Error(), test.path) {
				t.Fatalf("Validate() error = %v, want path %q", err, test.path)
			}
		})
	}
}

func validPlan() Plan {
	return Plan{Source: "analytics", Table: "reporting.orders", Projection: []string{"id"}, Limit: 1}
}

func TestValidateRejectsInvalidFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		plan func() Plan
		path string
	}{
		{"source", func() Plan { p := validPlan(); p.Source = ""; return p }, "source"},
		{"table parts", func() Plan { p := validPlan(); p.Table = "orders"; return p }, "table"},
		{"table identifier", func() Plan { p := validPlan(); p.Table = "public.orders;drop"; return p }, "table"},
		{"empty projection", func() Plan { p := validPlan(); p.Projection = nil; return p }, "projection"},
		{"empty column", func() Plan { p := validPlan(); p.Projection = []string{""}; return p }, "projection[0]"},
		{"duplicate column", func() Plan { p := validPlan(); p.Projection = []string{"id", "id"}; return p }, "projection[1]"},
		{"order column", func() Plan {
			p := validPlan()
			p.Order = []Order{{Column: "", Direction: DirectionAscending}}
			return p
		}, "order[0].column"},
		{"direction", func() Plan { p := validPlan(); p.Order = []Order{{Column: "id", Direction: "sideways"}}; return p }, "order[0].direction"},
		{"zero limit", func() Plan { p := validPlan(); p.Limit = 0; return p }, "limit"},
		{"negative limit", func() Plan { p := validPlan(); p.Limit = -1; return p }, "limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Validate(test.plan())
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || !strings.Contains(err.Error(), test.path) {
				t.Fatalf("Validate() error = %v, want path %q", err, test.path)
			}
		})
	}
}

func TestValidateAcceptsOrdering(t *testing.T) {
	t.Parallel()
	plan := validPlan()
	plan.Order = []Order{{Column: "created_at", Direction: DirectionDescending}, {Column: "id", Direction: DirectionAscending}}
	validated, err := Validate(plan)
	if err != nil {
		t.Fatal(err)
	}
	if got := validated.Plan.Order; len(got) != 2 || got[0].Column != "created_at" || got[1].Direction != DirectionAscending {
		t.Fatalf("ordering was not preserved: %#v", got)
	}
}
