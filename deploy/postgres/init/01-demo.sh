#!/bin/sh
set -eu

psql --set=ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=owner="$DATAWARK_DEMO_OWNER" \
  --set=reader="$DATAWARK_GATEWAY_DB_USER" \
  --set=reader_password="$DATAWARK_GATEWAY_DB_PASSWORD" <<'SQL'
CREATE ROLE :"owner" NOLOGIN;
CREATE ROLE :"reader" LOGIN PASSWORD :'reader_password' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;

CREATE SCHEMA sales AUTHORIZATION :"owner";
SET ROLE :"owner";
CREATE TABLE sales.orders (
  id bigint PRIMARY KEY,
  status text NOT NULL,
  total numeric(12, 2) NOT NULL,
  created_at timestamptz NOT NULL,
  internal_note text NOT NULL
);
INSERT INTO sales.orders (id, status, total, created_at, internal_note) VALUES
  (1001, 'paid', 42.50, '2026-11-01T12:00:00Z', 'demo-protected-note');
RESET ROLE;

GRANT USAGE ON SCHEMA sales TO :"reader";
GRANT SELECT (id, status, total, created_at) ON sales.orders TO :"reader";
SQL
