package postgres

import (
	"context"
	"datawark/internal/config"
	"errors"
	"strings"
	"testing"
)

func TestCatalogReaderRejectsUnknownAndUnsetDatasource(t *testing.T) {
	reader := CatalogReader{LookupEnv: func(string) (string, bool) { return "", false }}
	for _, name := range []string{"missing", "analytics"} {
		_, err := reader.Load(context.Background(), catalogConfig(), name)
		if err == nil || strings.Contains(err.Error(), "postgres://") {
			t.Fatalf("expected safe failure for %q: %v", name, err)
		}
	}
}

func TestCatalogReaderMapsUserRelations(t *testing.T) {
	connection := &fakeConnection{rows: &fakeRows{values: [][]any{
		{"public", "orders", "id", "integer", false},
		{"public", "orders", "total", "numeric", true},
		{"public", "order_view", "id", "integer", false},
	}}}
	reader := CatalogReader{LookupEnv: func(string) (string, bool) { return "postgres://reader:secret@example.test/db", true }, Connect: func(context.Context, string) (CatalogConnection, error) { return connection, nil }}
	snapshot, err := reader.Load(context.Background(), catalogConfig(), "analytics")
	if err != nil {
		t.Fatal(err)
	}
	if connection.query != CatalogSQL {
		t.Fatal("catalog query was not used")
	}
	relation, found := snapshot.Relation("public.orders")
	if !found || len(relation.Columns()) != 2 || relation.Columns()[1].Nullable != true {
		t.Fatalf("unexpected metadata: %#v", relation)
	}
}

func TestCatalogReaderSanitizesConnectionError(t *testing.T) {
	secret := "postgres://reader:very-secret@example.test/db"
	reader := CatalogReader{LookupEnv: func(string) (string, bool) { return secret, true }, Connect: func(context.Context, string) (CatalogConnection, error) { return nil, errors.New(secret) }}
	_, err := reader.Load(context.Background(), catalogConfig(), "analytics")
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("secret leaked: %v", err)
	}
}

func catalogConfig() config.Config {
	return config.Config{Version: config.VersionV1Alpha1, Server: config.Server{DevelopmentPrincipal: "reader"}, Datasources: []config.Datasource{{Name: "analytics", Driver: "postgres", DSNEnv: "TEST_DSN"}}, Policies: []config.Policy{{Name: "read", Principals: []string{"reader"}, Datasource: "analytics", Allow: []config.AllowEntry{{Table: "public.orders", Operations: []string{"select"}, Columns: []string{"id"}}}}}}
}

type fakeConnection struct {
	rows  *fakeRows
	query string
}

func (connection *fakeConnection) Query(_ context.Context, sql string) (CatalogRows, error) {
	connection.query = sql
	return connection.rows, nil
}
func (connection *fakeConnection) Close(context.Context) error { return nil }

type fakeRows struct {
	values [][]any
	index  int
}

func (rows *fakeRows) Next() bool { return rows.index < len(rows.values) }
func (rows *fakeRows) Scan(destinations ...any) error {
	row := rows.values[rows.index]
	rows.index++
	for index, destination := range destinations {
		switch value := destination.(type) {
		case *string:
			*value = row[index].(string)
		case *bool:
			*value = row[index].(bool)
		}
	}
	return nil
}
func (rows *fakeRows) Err() error { return nil }
func (rows *fakeRows) Close()     {}
