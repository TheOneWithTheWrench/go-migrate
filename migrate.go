package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

//go:embed migration_table_query.sql
var migrationTableQuery string

var (
	ErrMigrationFileChanged = fmt.Errorf("migration file has changed")
	ErrMigrationFailed      = fmt.Errorf("migration failed")
	ErrDirtyMigration       = fmt.Errorf("dirty migration state")
)

type migrationRow struct {
	MigrationName string `json:"migration_name,omitempty"`
	MigrationHash string `json:"migration_hash,omitempty"`
	IsApplied     bool   `json:"is_applied,omitempty"`
	IsDirty       bool   `json:"is_dirty,omitempty"`
}

type Migrator struct {
	options    *options
	db         *sql.DB
	migrations embed.FS
}

func NewMigrator(db *sql.DB, migrations embed.FS, opts ...func(*options)) *Migrator {
	opt := &options{
		migrationTimeout: 10 * time.Second,
	}
	for _, o := range opts {
		o(opt)
	}

	return &Migrator{
		options:    opt,
		db:         db,
		migrations: migrations,
	}
}

func (m *Migrator) Migrate() error {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), m.options.migrationTimeout)
	defer cancel()

	conn, err := m.db.Conn(timeoutCtx)
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	_, err = conn.ExecContext(timeoutCtx, migrationTableQuery)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	knownMigrations, err := getMigrationsKnownToDb(conn, timeoutCtx)
	if err != nil {
		return err
	}
	if hasDirtyMigration(knownMigrations) {
		return ErrDirtyMigration
	}

	// We check if any of the migration files have been altered.
	// It is currently undefined what to do if so
	err = fs.WalkDir(m.migrations, ".", checkIfMigrationsAreAltered(m.migrations, knownMigrations))
	if err != nil {
		return ErrMigrationFileChanged
	}

	// We "walk" the migrations directory and execute each migration file
	// if they are not already applied.
	err = fs.WalkDir(m.migrations, ".", handleMigration(conn, timeoutCtx, m.migrations, knownMigrations))
	if err != nil {
		return fmt.Errorf("walk migrations: %w", err)
	}

	return nil
}

func checkIfMigrationsAreAltered(migrations embed.FS, knownMigrations []migrationRow) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk func errored: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		migration, ok := findMigrationByName(knownMigrations, d.Name())
		if !ok || !migration.IsApplied {
			return nil
		}

		readBytes, err := migrations.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %q: %w", d.Name(), err)
		}

		migrationHash := hashFile(readBytes)

		if migrationHash != migration.MigrationHash {
			return fmt.Errorf("migration %q has been altered", d.Name())
		}

		return nil
	}
}

func handleMigration(conn *sql.Conn, ctx context.Context, migrations embed.FS, knownMigrations []migrationRow) fs.WalkDirFunc {
	return func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk func errored: %w", err)
		}

		if dirEntry.IsDir() {
			return nil
		}

		migration, ok := findMigrationByName(knownMigrations, dirEntry.Name())
		if ok {
			if migration.IsDirty {
				return ErrDirtyMigration
			}
			if migration.IsApplied {
				return nil
			}
		}

		readBytes, err := migrations.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %q: %w", dirEntry.Name(), err)
		}

		migrationHash := hashFile(readBytes)

		err = upsertMigration(conn, ctx, migrationRow{
			MigrationName: dirEntry.Name(),
			MigrationHash: migrationHash,
			IsApplied:     false,
			IsDirty:       true,
		})
		if err != nil {
			return err
		}

		_, err = conn.ExecContext(ctx, string(readBytes))
		if err != nil {
			return fmt.Errorf("execute migration %q: %w: %w", dirEntry.Name(), err, ErrMigrationFailed)
		}

		err = upsertMigration(conn, ctx, migrationRow{
			MigrationName: dirEntry.Name(),
			MigrationHash: migrationHash,
			IsApplied:     true,
			IsDirty:       false,
		})
		if err != nil {
			return err
		}

		return nil
	}
}

func upsertMigration(conn *sql.Conn, ctx context.Context, migration migrationRow) error {
	var (
		query = `INSERT INTO migrations (migration_name, migration_hash, is_applied, is_dirty)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT(migration_name) DO UPDATE SET
			migration_hash = excluded.migration_hash,
			is_applied = excluded.is_applied,
			is_dirty = excluded.is_dirty`
	)

	_, err := conn.ExecContext(ctx, query, migration.MigrationName, migration.MigrationHash, migration.IsApplied, migration.IsDirty)
	if err != nil {
		return fmt.Errorf("upsert migration: %w", err)
	}

	return nil
}

func getMigrationsKnownToDb(conn *sql.Conn, ctx context.Context) ([]migrationRow, error) {
	rows, err := conn.QueryContext(ctx, "SELECT migration_name, migration_hash, is_applied, is_dirty FROM migrations")
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()

	var appliedMigrations []migrationRow
	for rows.Next() {
		var migration migrationRow
		if err := rows.Scan(&migration.MigrationName, &migration.MigrationHash, &migration.IsApplied, &migration.IsDirty); err != nil {
			return nil, fmt.Errorf("scan migration row: %w", err)
		}
		appliedMigrations = append(appliedMigrations, migration)
	}

	return appliedMigrations, nil
}

func hasDirtyMigration(migrations []migrationRow) bool {
	for _, migration := range migrations {
		if migration.IsDirty {
			return true
		}
	}
	return false
}

func findMigrationByName(migrations []migrationRow, name string) (migrationRow, bool) {
	for _, migration := range migrations {
		if migration.MigrationName == name {
			return migration, true
		}
	}
	return migrationRow{}, false
}

func hashFile(value any) string {
	var (
		sha = sha256.New()
	)

	sha.Write(fmt.Appendf(nil, "%v", value)) // Should this be marshaled instead?
	return fmt.Sprintf("%x", sha.Sum(nil))
}
