//go:build integration

package integration

import (
	"net"
	"net/url"
	"os"
	"testing"
)

func TestPostgreSQLIsReachable(t *testing.T) {
	dsn := os.Getenv("DATAWARK_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("DATAWARK_TEST_POSTGRES_DSN must be set for integration tests")
	}

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse integration PostgreSQL DSN: %v", err)
	}

	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("connect to integration PostgreSQL: %v", err)
	}
	defer conn.Close()
}
