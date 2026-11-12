package postgres

import (
	"context"
	"datawark/internal/config"
	"datawark/internal/schema"
	"errors"
	"os"

	"github.com/jackc/pgx/v5"
)

// CatalogSQL reads only PostgreSQL catalog metadata; it never reads from an
// application relation.
const CatalogSQL = `SELECT n.nspname, c.relname, a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod), NOT a.attnotnull
FROM pg_catalog.pg_class AS c
JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_attribute AS a ON a.attrelid = c.oid
WHERE c.relkind IN ('r', 'p', 'v', 'm')
  AND a.attnum > 0
  AND NOT a.attisdropped
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY n.nspname, c.relname, a.attnum`

// CatalogRows is the narrow result-set boundary used by the catalog reader.
type CatalogRows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
}

// CatalogConnection is a short-lived connection capable of the catalog query.
type CatalogConnection interface {
	Query(context.Context, string) (CatalogRows, error)
	Close(context.Context) error
}

// CatalogReader loads schema snapshots for configured PostgreSQL datasources.
type CatalogReader struct {
	LookupEnv func(string) (string, bool)
	Connect   func(context.Context, string) (CatalogConnection, error)
}

func NewCatalogReader() CatalogReader {
	return CatalogReader{LookupEnv: os.LookupEnv, Connect: connectCatalog}
}

func connectCatalog(ctx context.Context, dsn string) (CatalogConnection, error) {
	connection, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return pgxCatalogConnection{connection}, nil
}

type pgxCatalogConnection struct{ connection *pgx.Conn }

func (connection pgxCatalogConnection) Query(ctx context.Context, sql string) (CatalogRows, error) {
	rows, err := connection.connection.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	return pgxCatalogRows{rows}, nil
}
func (connection pgxCatalogConnection) Close(ctx context.Context) error {
	return connection.connection.Close(ctx)
}

type pgxCatalogRows struct{ pgx.Rows }

// Load resolves a datasource only through its configured environment variable,
// then returns a complete snapshot or a deliberately non-sensitive error.
func (reader CatalogReader) Load(ctx context.Context, configuration config.Config, datasource string) (schema.Snapshot, error) {
	if config.Validate(configuration) != nil {
		return schema.Snapshot{}, errors.New("schema: datasource is unavailable")
	}
	source, found := config.FindDatasource(configuration, datasource)
	if !found {
		return schema.Snapshot{}, errors.New("schema: datasource is unavailable")
	}
	lookup := reader.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	dsn, present := lookup(source.DSNEnv)
	if !present || dsn == "" {
		return schema.Snapshot{}, errors.New("schema: datasource is unavailable")
	}
	connect := reader.Connect
	if connect == nil {
		connect = connectCatalog
	}
	connection, err := connect(ctx, dsn)
	if err != nil {
		return schema.Snapshot{}, errors.New("schema: metadata connection failed")
	}
	defer connection.Close(ctx)

	rows, err := connection.Query(ctx, CatalogSQL)
	if err != nil {
		return schema.Snapshot{}, errors.New("schema: metadata lookup failed")
	}
	defer rows.Close()
	relations := make([]schema.Relation, 0)
	var currentSchema, currentName string
	var columns []schema.Column
	flush := func() {
		if currentName != "" {
			relations = append(relations, schema.NewRelation(currentSchema, currentName, columns))
		}
		columns = nil
	}
	for rows.Next() {
		var namespace, relation, name, typeName string
		var nullable bool
		if err := rows.Scan(&namespace, &relation, &name, &typeName, &nullable); err != nil {
			return schema.Snapshot{}, errors.New("schema: metadata lookup failed")
		}
		if namespace != currentSchema || relation != currentName {
			flush()
			currentSchema, currentName = namespace, relation
		}
		columns = append(columns, schema.Column{Name: name, TypeName: typeName, Nullable: nullable})
	}
	if err := rows.Err(); err != nil {
		return schema.Snapshot{}, errors.New("schema: metadata lookup failed")
	}
	flush()
	return schema.NewSnapshot(datasource, relations), nil
}
