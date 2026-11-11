// Package postgres contains narrow PostgreSQL infrastructure adapters.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Connector struct{}

// Ping establishes a PostgreSQL protocol connection and pings it. It does not
// inspect schemas or issue queries against application relations.
func (Connector) Ping(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return errors.New("connect to PostgreSQL")
	}
	defer conn.Close(ctx)
	if err := conn.Ping(ctx); err != nil {
		return errors.New("ping PostgreSQL")
	}
	return nil
}
