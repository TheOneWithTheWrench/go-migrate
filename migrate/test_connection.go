package migrate

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type TestingT interface {
	Logf(format string, args ...any)
	FailNow()
	Cleanup(fn func())
	Name() string
}

func SetupTestDatabase(t TestingT) *sql.DB {
	var (
		unixTime = time.Now().Unix()
		schema   = strings.ReplaceAll(fmt.Sprintf("test_%s_%d", t.Name(), unixTime), "/", "_")
		connUrl  = "postgres://testuser:testpassword@localhost:5432/testapp_db?sslmode=disable"
	)

	conn, err := sql.Open("postgres", connUrl)
	if err != nil {
		t.Logf("failed to connect to database. Is your local database running?: %v", err)
		t.FailNow()
	}

	_, err = conn.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	if err != nil {
		t.Logf("failed to create schema %q: %v", schema, err)
		t.FailNow()
	}

	_, err = conn.Exec(fmt.Sprintf("SET search_path TO %s", schema))
	if err != nil {
		t.Logf("failed to set search_path: %v", err)
		t.FailNow()
	}

	t.Cleanup(func() {
		// We could drop the schema on cleanup... But it's actually quite
		// nice to have the left over schema for debugging purposes.
		// _, err := conn.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
		// if err != nil {
		// 	t.Logf("failed to drop schema: %v", err)
		// }
		_ = conn.Close()
	})

	return conn
}
