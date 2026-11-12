package compiler_test

import (
	"datawark/internal/compiler"
	"datawark/internal/config"
	"datawark/internal/policy"
	"datawark/internal/query"
	"reflect"
	"strings"
	"testing"
)

// Compile's concrete input prevents callers from supplying a public Plan or a
// merely validated plan; only policy.Authorize can issue this capability.
var _ func(policy.AuthorizedQuery) (compiler.Statement, error) = compiler.Compile

func TestCompileRendersAuthorizedPlanDeterministically(t *testing.T) {
	authorized := authorize(t, query.Plan{
		Source: "analytics", Table: "sales.Orders", Projection: []string{"id", "total"},
		Filters: []query.Filter{
			{Column: "state", Operator: query.OperatorEqual, Value: query.String("paid")},
			{Column: "total", Operator: query.OperatorGreaterThanOrEqual, Value: query.Number(42.5)},
		},
		Order: []query.Order{{Column: "created_at", Direction: query.DirectionDescending}, {Column: "id", Direction: query.DirectionAscending}},
		Limit: 50,
	})

	got, err := compiler.Compile(authorized)
	if err != nil {
		t.Fatal(err)
	}
	wantSQL := `SELECT "id", "total" FROM "sales"."Orders" WHERE "state" = $1 AND "total" >= $2 ORDER BY "created_at" DESC, "id" ASC LIMIT $3`
	if got.SQL() != wantSQL {
		t.Fatalf("SQL = %q, want %q", got.SQL(), wantSQL)
	}
	if wantArgs := []any{"paid", 42.5, 50}; !reflect.DeepEqual(got.Args(), wantArgs) {
		t.Fatalf("Args = %#v, want %#v", got.Args(), wantArgs)
	}
	again, err := compiler.Compile(authorized)
	if err != nil || !reflect.DeepEqual(got, again) {
		t.Fatalf("second Compile() = %#v, %v; want identical statement", again, err)
	}
}

func TestCompileOmitsOptionalClausesAndQuotesIdentifiers(t *testing.T) {
	got, err := compiler.Compile(authorize(t, query.Plan{
		Source: "analytics", Table: "Sales.Order", Projection: []string{"Select"}, Limit: 1,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if want := `SELECT "Select" FROM "Sales"."Order" LIMIT $1`; got.SQL() != want {
		t.Fatalf("SQL = %q, want %q", got.SQL(), want)
	}
	if !reflect.DeepEqual(got.Args(), []any{1}) {
		t.Fatalf("Args = %#v, want limit argument", got.Args())
	}
}

func TestCompileBindsHostileValues(t *testing.T) {
	hostile := `x'); DROP TABLE users; --`
	got, err := compiler.Compile(authorize(t, query.Plan{
		Source: "analytics", Table: "sales.orders", Projection: []string{"id"},
		Filters: []query.Filter{{Column: "state", Operator: query.OperatorEqual, Value: query.String(hostile)}}, Limit: 5,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.SQL(), hostile) || !strings.Contains(got.SQL(), "$1") || strings.Contains(got.SQL(), "DROP TABLE") {
		t.Fatalf("hostile value interpolated into SQL: %q", got.SQL())
	}
	if !reflect.DeepEqual(got.Args(), []any{hostile, 5}) {
		t.Fatalf("Args = %#v, want hostile value and limit", got.Args())
	}
}

func TestCompileRejectsUnauthorizedAndInvalidInput(t *testing.T) {
	if statement, err := compiler.Compile(policy.AuthorizedQuery{}); err == nil || statement.SQL() != "" || len(statement.Args()) != 0 {
		t.Fatalf("Compile(zero AuthorizedQuery) = %#v, %v; want no SQL and error", statement, err)
	}
	invalid := query.Plan{Source: "analytics", Table: "orders", Projection: []string{"id"}, Limit: 1}
	if _, err := query.Validate(invalid); err == nil {
		t.Fatal("Validate(invalid table) succeeded")
	}
	validated, err := query.Validate(query.Plan{Source: "analytics", Table: "sales.orders", Projection: []string{"id"}, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.Authorize(policy.Input{Principal: "intruder", Snapshot: compilerConfig(), Plan: validated}); err == nil {
		t.Fatal("Authorize(denied input) succeeded")
	}
}

func FuzzCompileDoesNotInterpolateValues(f *testing.F) {
	f.Add("x'); DROP TABLE users; --")
	f.Add("hello")
	f.Fuzz(func(t *testing.T, value string) {
		statement, err := compiler.Compile(authorize(t, query.Plan{
			Source: "analytics", Table: "sales.orders", Projection: []string{"id"},
			Filters: []query.Filter{{Column: "state", Operator: query.OperatorEqual, Value: query.String(value)}}, Limit: 1,
		}))
		if err != nil {
			t.Fatal(err)
		}
		const wantSQL = `SELECT "id" FROM "sales"."orders" WHERE "state" = $1 LIMIT $2`
		if statement.SQL() != wantSQL || !reflect.DeepEqual(statement.Args(), []any{value, 1}) {
			t.Fatalf("unsafe statement: %#v", statement)
		}
	})
}

func authorize(t *testing.T, plan query.Plan) policy.AuthorizedQuery {
	t.Helper()
	validated, err := query.Validate(plan)
	if err != nil {
		t.Fatal(err)
	}
	authorized, err := policy.Authorize(policy.Input{Principal: "reader", Snapshot: compilerConfig(), Plan: validated})
	if err != nil {
		t.Fatal(err)
	}
	return authorized
}

func compilerConfig() *config.Config {
	return &config.Config{
		Version:     config.VersionV1Alpha1,
		Server:      config.Server{DevelopmentPrincipal: "reader"},
		Datasources: []config.Datasource{{Name: "analytics", Driver: "postgres", DSNEnv: "DATAWARK_ANALYTICS_DSN"}},
		Policies: []config.Policy{{
			Name: "read-orders", Principals: []string{"reader"}, Datasource: "analytics",
			Allow: []config.AllowEntry{
				{Table: "sales.orders", Operations: []string{"select"}, Columns: []string{"id", "total", "state", "created_at"}},
				{Table: "sales.Orders", Operations: []string{"select"}, Columns: []string{"id", "total", "state", "created_at"}},
				{Table: "Sales.Order", Operations: []string{"select"}, Columns: []string{"Select"}},
			},
		}},
	}
}
