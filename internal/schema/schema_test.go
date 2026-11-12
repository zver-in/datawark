package schema

import (
	"datawark/internal/config"
	"datawark/internal/query"
	"strings"
	"testing"
)

func snapshot() Snapshot {
	return NewSnapshot("analytics", []Relation{NewRelation("public", "orders", []Column{{Name: "id", TypeName: "integer"}, {Name: "total", TypeName: "numeric", Nullable: true}})})
}

func plan(t *testing.T, source, table string) query.ValidatedPlan {
	t.Helper()
	result, err := query.Validate(query.Plan{Source: source, Table: table, Projection: []string{"id"}, Filters: []query.Filter{{Column: "total", Operator: query.OperatorGreaterThan, Value: query.Number(1)}}, Order: []query.Order{{Column: "id", Direction: query.DirectionAscending}}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSnapshotLookupAndCopies(t *testing.T) {
	s := snapshot()
	relation, ok := s.Relation("public.orders")
	if !ok || relation.QualifiedName() != "public.orders" {
		t.Fatal("relation lookup failed")
	}
	columns := relation.Columns()
	columns[0].Name = "changed"
	if column, _ := relation.Column("id"); column.Name != "id" {
		t.Fatal("columns must be copied")
	}
}

func TestValidatePlan(t *testing.T) {
	for name, p := range map[string]query.ValidatedPlan{
		"existing": plan(t, "analytics", "public.orders"),
		"source":   plan(t, "other", "public.orders"),
		"table":    plan(t, "analytics", "public.missing"),
	} {
		t.Run(name, func(t *testing.T) {
			err := ValidatePlan(p, ptr(snapshot()))
			if name == "existing" && err != nil {
				t.Fatal(err)
			}
			if name != "existing" && err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
	missing := plan(t, "analytics", "public.orders")
	missing.Plan.Projection = []string{"missing"}
	if err := ValidatePlan(missing, ptr(snapshot())); err == nil {
		t.Fatal("expected missing column rejection")
	}
}

func TestVisibleIntersectsSelectGrants(t *testing.T) {
	cfg := validConfig()
	visible, err := Visible("reader", &cfg, ptr(snapshot()))
	if err != nil {
		t.Fatal(err)
	}
	relation, ok := visible.Relation("public.orders")
	if !ok {
		t.Fatal("missing visible relation")
	}
	if columns := relation.Columns(); len(columns) != 1 || columns[0].Name != "id" {
		t.Fatalf("got %#v", columns)
	}
	for _, principal := range []string{"intruder", ""} {
		result, err := Visible(principal, &cfg, ptr(snapshot()))
		if principal == "" && err == nil {
			t.Fatal("expected unavailable visibility")
		}
		if err != nil && strings.Contains(err.Error(), "orders") {
			t.Fatal("hidden name leaked")
		}
		if principal == "intruder" && len(result.Relations()) != 0 {
			t.Fatal("ungranted principal must receive no relations")
		}
	}
}

func TestVisibleOmitsUngrantedRelationsAndStaleColumns(t *testing.T) {
	cfg := validConfig()
	snapshot := NewSnapshot("analytics", []Relation{
		NewRelation("public", "orders", []Column{{Name: "id"}}),
		NewRelation("public", "secrets", []Column{{Name: "token"}}),
	})
	visible, err := Visible("reader", &cfg, &snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := visible.Relation("public.secrets"); found {
		t.Fatal("ungranted relation was disclosed")
	}
	if relation, found := visible.Relation("public.orders"); !found || len(relation.Columns()) != 1 {
		t.Fatal("existing granted column was not retained")
	}
}

func TestVisibleFailsClosed(t *testing.T) {
	cfg := validConfig()
	if _, err := Visible("reader", &cfg, nil); err == nil {
		t.Fatal("expected nil snapshot failure")
	}
	other := NewSnapshot("other", nil)
	if _, err := Visible("reader", &cfg, &other); err == nil {
		t.Fatal("expected source mismatch failure")
	}
}

func validConfig() config.Config {
	return config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "reader"}, Datasources: []config.Datasource{{Name: "analytics", Driver: "postgres", DSNEnv: "DSN"}}, Policies: []config.Policy{{Name: "orders", Principals: []string{"reader"}, Datasource: "analytics", Allow: []config.AllowEntry{{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id", "stale"}}}}}}
}
func ptr(s Snapshot) *Snapshot { return &s }
