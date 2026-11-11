package policy

import (
	"fmt"
	"strings"
	"testing"

	"datawark/internal/config"
	"datawark/internal/query"
)

func TestEvaluateFailsClosedForUnavailableConfiguration(t *testing.T) {
	decision := Evaluate(Input{Principal: "reader", Plan: validPlan(t)})
	if decision.Allowed || decision.Reason != ReasonConfigurationUnavailable {
		t.Fatalf("Evaluate() = %+v, want unavailable configuration denial", decision)
	}
}

func TestEvaluateDeniesMissingPrincipal(t *testing.T) {
	decision := Evaluate(Input{Snapshot: validConfig(), Plan: validPlan(t)})
	if decision.Allowed || decision.Reason != ReasonDenied {
		t.Fatalf("Evaluate() = %+v, want default denial", decision)
	}
}

func TestEvaluateMatchesPrincipalSourceTableAndSelectGrant(t *testing.T) {
	tests := []struct {
		name      string
		principal string
		plan      query.Plan
		allowed   bool
	}{
		{"fully allowed", "reader", query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Limit: 1}, true},
		{"unknown principal", "other", query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Limit: 1}, false},
		{"inaccessible source", "reader", query.Plan{Source: "private", Table: "public.orders", Projection: []string{"id"}, Limit: 1}, false},
		{"inaccessible table", "reader", query.Plan{Source: "analytics", Table: "public.payments", Projection: []string{"id"}, Limit: 1}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := query.Validate(test.plan)
			if err != nil {
				t.Fatal(err)
			}
			decision := Evaluate(Input{Principal: test.principal, Snapshot: validConfig(), Plan: plan})
			if decision.Allowed != test.allowed {
				t.Fatalf("Evaluate() = %+v, want allowed=%t", decision, test.allowed)
			}
		})
	}
}

func TestEvaluateRequiresOneGrantToCoverAllReferencedColumns(t *testing.T) {
	tests := []struct {
		name string
		plan query.Plan
		cfg  *config.Config
	}{
		{
			name: "filter-only leakage",
			plan: query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Filters: []query.Filter{{Column: "status", Operator: query.OperatorEqual, Value: query.String("paid")}}, Limit: 1},
			cfg:  validConfig(),
		},
		{
			name: "sort-only leakage",
			plan: query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Order: []query.Order{{Column: "created_at", Direction: query.DirectionAscending}}, Limit: 1},
			cfg:  validConfig(),
		},
		{
			name: "coverage split across grants",
			plan: query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Filters: []query.Filter{{Column: "status", Operator: query.OperatorEqual, Value: query.String("paid")}}, Limit: 1},
			cfg:  splitGrantConfig(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := query.Validate(test.plan)
			if err != nil {
				t.Fatal(err)
			}
			decision := Evaluate(Input{Principal: "reader", Snapshot: test.cfg, Plan: plan})
			if decision.Allowed {
				t.Fatalf("Evaluate() = %+v, want denial", decision)
			}
		})
	}
}

func TestExplainDisclosesMatchingPolicyOnlyForAllowedDecision(t *testing.T) {
	allowed := Explain(Input{Principal: "reader", Snapshot: validConfig(), Plan: validPlan(t)})
	if !allowed.Allowed || allowed.Reason != ReasonAllowed || allowed.PolicyID != "read-orders" {
		t.Fatalf("Explain() = %+v, want allowed policy explanation", allowed)
	}

	deniedPlan, err := query.Validate(query.Plan{
		Source: "hidden_source", Table: "secret.records", Projection: []string{"secret_column"}, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	denied := Explain(Input{Principal: "reader", Snapshot: validConfig(), Plan: deniedPlan})
	if denied.Allowed || denied.Reason != ReasonDenied || denied.PolicyID != "" {
		t.Fatalf("Explain() = %+v, want non-disclosing denial", denied)
	}
	text := fmt.Sprintf("%+v", denied)
	for _, prohibited := range []string{"read-orders", "hidden_source", "secret.records", "secret_column"} {
		if strings.Contains(text, prohibited) {
			t.Fatalf("denial leaks %q: %s", prohibited, text)
		}
	}
}

func TestEvaluateRegressionCases(t *testing.T) {
	completeConfig := validConfig()
	completeConfig.Policies[0].Allow[0].Columns = []string{"id", "status", "created_at"}
	tests := []struct {
		name      string
		principal string
		cfg       *config.Config
		plan      query.Plan
		allowed   bool
	}{
		{
			name: "complete grant authorizes every referenced column", principal: "reader", cfg: completeConfig,
			plan: query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Filters: []query.Filter{{Column: "status", Operator: query.OperatorEqual, Value: query.String("paid")}}, Order: []query.Order{{Column: "created_at", Direction: query.DirectionDescending}}, Limit: 10}, allowed: true,
		},
		{
			name: "unknown principal", principal: "intruder", cfg: completeConfig,
			plan: query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Limit: 1},
		},
		{
			name: "wrong datasource", principal: "reader", cfg: completeConfig,
			plan: query.Plan{Source: "other", Table: "public.orders", Projection: []string{"id"}, Limit: 1},
		},
		{
			name: "wrong table", principal: "reader", cfg: completeConfig,
			plan: query.Plan{Source: "analytics", Table: "public.users", Projection: []string{"id"}, Limit: 1},
		},
		{
			name: "incomplete grant", principal: "reader", cfg: validConfig(),
			plan: query.Plan{Source: "analytics", Table: "public.orders", Projection: []string{"id", "status"}, Limit: 1},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, err := query.Validate(test.plan)
			if err != nil {
				t.Fatal(err)
			}
			decision := Evaluate(Input{Principal: test.principal, Snapshot: test.cfg, Plan: plan})
			if decision.Allowed != test.allowed {
				t.Fatalf("Evaluate() = %+v, want allowed=%t", decision, test.allowed)
			}
			if !test.allowed && decision.Allowed {
				t.Fatalf("denied case reached authorized result: %+v", decision)
			}
		})
	}
}

func validPlan(t *testing.T) query.ValidatedPlan {
	t.Helper()
	plan, err := query.Validate(query.Plan{
		Source: "analytics", Table: "public.orders", Projection: []string{"id"}, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func validConfig() *config.Config {
	return &config.Config{
		Version: config.VersionV1Alpha1,
		Server:  config.Server{DevelopmentPrincipal: "reader"},
		Datasources: []config.Datasource{{
			Name: "analytics", Driver: "postgres", DSNEnv: "DATAWARK_ANALYTICS_DSN",
		}},
		Policies: []config.Policy{{
			Name: "read-orders", Principals: []string{"reader"}, Datasource: "analytics",
			Allow: []config.AllowEntry{{
				Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id"},
			}},
		}},
	}
}

func splitGrantConfig() *config.Config {
	cfg := validConfig()
	cfg.Policies[0].Allow = []config.AllowEntry{
		{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id"}},
		{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"status"}},
	}
	return cfg
}
