// Package query defines the bounded, transport-neutral read query plan.
package query

// Plan is an unvalidated, single-table read query. Call Validate before using
// it outside an input boundary.
type Plan struct {
	Source     string   `json:"source"`
	Table      string   `json:"table"`
	Projection []string `json:"projection"`
	Filters    []Filter `json:"filters"`
	Order      []Order  `json:"order"`
	Limit      int      `json:"limit"`
}

// Filter compares one column to a scalar value.
type Filter struct {
	Column   string   `json:"column"`
	Operator Operator `json:"operator"`
	Value    Scalar   `json:"value"`
}

// Order specifies the ordering of one column.
type Order struct {
	Column    string    `json:"column"`
	Direction Direction `json:"direction"`
}

type Operator string

const (
	OperatorEqual              Operator = "eq"
	OperatorNotEqual           Operator = "neq"
	OperatorLessThan           Operator = "lt"
	OperatorLessThanOrEqual    Operator = "lte"
	OperatorGreaterThan        Operator = "gt"
	OperatorGreaterThanOrEqual Operator = "gte"
)

type Direction string

const (
	DirectionAscending  Direction = "asc"
	DirectionDescending Direction = "desc"
)

// Scalar is one of string, number, boolean, or null. Exactly the field
// corresponding to Kind is meaningful.
type Scalar struct {
	Kind   ScalarKind
	String string
	Number float64
	Bool   bool
}

type ScalarKind string

const (
	ScalarString  ScalarKind = "string"
	ScalarNumber  ScalarKind = "number"
	ScalarBoolean ScalarKind = "boolean"
	ScalarNull    ScalarKind = "null"
)

func String(value string) Scalar  { return Scalar{Kind: ScalarString, String: value} }
func Number(value float64) Scalar { return Scalar{Kind: ScalarNumber, Number: value} }
func Boolean(value bool) Scalar   { return Scalar{Kind: ScalarBoolean, Bool: value} }
func Null() Scalar                { return Scalar{Kind: ScalarNull} }

// ValidatedPlan is a plan whose grammar has been checked. It contains no
// database metadata guarantee and must still be authorized before execution.
type ValidatedPlan struct {
	Plan Plan
}

// Violation identifies a safe, machine-actionable validation failure.
type Violation struct {
	Path    string
	Message string
}

// ValidationError contains all detected validation failures.
type ValidationError struct {
	Violations []Violation
}
