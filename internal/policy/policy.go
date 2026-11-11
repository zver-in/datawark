// Package policy authorizes validated query plans against Gateway policy.
package policy

import (
	"datawark/internal/config"
	"datawark/internal/query"
	"errors"
	"strings"
)

// ReasonCode is a stable, machine-readable authorization outcome.
type ReasonCode string

const (
	ReasonAllowed                  ReasonCode = "allowed"
	ReasonConfigurationUnavailable ReasonCode = "configuration_unavailable"
	ReasonDenied                   ReasonCode = "denied"
)

// Decision is the result of evaluating one validated query plan. PolicyID is
// populated only for an allowed decision.
type Decision struct {
	Allowed  bool
	Reason   ReasonCode
	PolicyID string
}

// Explanation is safe to return from a trusted dry-run interface. It reveals
// a matching policy only for allowed decisions.
type Explanation struct {
	Allowed  bool
	Reason   ReasonCode
	PolicyID string
}

// Input contains values supplied by trusted server-side boundaries. Principal
// is never read from the query plan, and Snapshot must be a validated, stable
// configuration snapshot for the duration of evaluation.
type Input struct {
	Principal string
	Snapshot  *config.Config
	Plan      query.ValidatedPlan
}

// AuthorizedQuery is an opaque capability issued only for an allowed policy
// decision. Its contents are deliberately not constructible by callers.
type AuthorizedQuery struct {
	plan query.ValidatedPlan
}

// Plan returns the validated plan carried by this authorization capability.
// The boolean is false for the zero value, which is never authorized.
func (authorized AuthorizedQuery) Plan() (query.ValidatedPlan, bool) {
	return authorized.plan, authorized.plan.Plan.Source != ""
}

// Authorize evaluates input and, only on success, issues the capability that
// downstream compilers require.
func Authorize(input Input) (AuthorizedQuery, error) {
	if decision := Evaluate(input); !decision.Allowed {
		return AuthorizedQuery{}, errors.New("policy: query is not authorized")
	}
	return AuthorizedQuery{plan: input.Plan}, nil
}

// Evaluate authorizes input against exactly its supplied configuration
// snapshot. It fails closed when the trusted caller identity, snapshot, or
// validated-plan boundary is unusable.
func Evaluate(input Input) Decision {
	if input.Snapshot == nil || config.Validate(*input.Snapshot) != nil {
		return Decision{Reason: ReasonConfigurationUnavailable}
	}
	if strings.TrimSpace(input.Principal) == "" {
		return Decision{Reason: ReasonDenied}
	}
	if _, err := query.Validate(input.Plan.Plan); err != nil {
		return Decision{Reason: ReasonDenied}
	}
	for _, policy := range input.Snapshot.Policies {
		if !contains(policy.Principals, input.Principal) || policy.Datasource != input.Plan.Plan.Source {
			continue
		}
		for _, allow := range policy.Allow {
			if allow.Table == input.Plan.Plan.Table && contains(allow.Operations, "select") && covers(allow.Columns, referencedColumns(input.Plan.Plan)) {
				return Decision{Allowed: true, Reason: ReasonAllowed, PolicyID: policy.Name}
			}
		}
	}
	return Decision{Reason: ReasonDenied}
}

// Explain returns the authorization outcome without compiling or executing the
// plan. Denied explanations intentionally omit policy and resource details.
func Explain(input Input) Explanation {
	decision := Evaluate(input)
	explanation := Explanation{Allowed: decision.Allowed, Reason: decision.Reason}
	if decision.Allowed {
		explanation.PolicyID = decision.PolicyID
	}
	return explanation
}

func referencedColumns(plan query.Plan) []string {
	columns := make([]string, 0, len(plan.Projection)+len(plan.Filters)+len(plan.Order))
	columns = append(columns, plan.Projection...)
	for _, filter := range plan.Filters {
		columns = append(columns, filter.Column)
	}
	for _, order := range plan.Order {
		columns = append(columns, order.Column)
	}
	return columns
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func covers(allowed, requested []string) bool {
	for _, column := range requested {
		if !contains(allowed, column) {
			return false
		}
	}
	return true
}
