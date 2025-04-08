package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	_ "github.com/lib/pq" // PostgreSQL driver
)

const migrationTableQuery = `CREATE TABLE IF NOT EXISTS migrations (
    migration_name  VARCHAR(255) NOT NULL,
    migration_hash  VARCHAR(64),           
    is_applied      BOOLEAN DEFAULT FALSE, 
    primary key (migration_name)
)`

type migrationRow struct {
	MigrationName string `json:"migration_name,omitempty"`
	MigrationHash string `json:"migration_hash,omitempty"`
	IsApplied     bool   `json:"is_applied,omitempty"`
}

func Migrate(db *sql.DB, migrations embed.FS) error {
	var (
		ctx = context.Background() //TODO: Should probably have an optional timeout
	)
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}
	defer conn.Close()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(migrationTableQuery)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	err = fs.WalkDir(migrations, ".", checkIfMigrationsAreAltered(tx, migrations))
	if err != nil {
		return fmt.Errorf("validate migrations: %w", err)
	}

	// We "walk" the migrations directory and execute each migration file
	// if they are not already applied.
	err = fs.WalkDir(migrations, ".", handleMigration(tx, migrations))
	if err != nil {
		return fmt.Errorf("walk migrations: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func checkIfMigrationsAreAltered(tx *sql.Tx, migrations embed.FS) fs.WalkDirFunc {
	knownMigrations, err := getMigrationsKnownToDb(tx)
	if err != nil {
		return func(path string, d fs.DirEntry, err error) error {
			return fmt.Errorf("get known migrations: %w", err)
		}
	}
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk func errored: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		migration, ok := findMigrationByName(knownMigrations, d.Name())
		if !ok {
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

func handleMigration(tx *sql.Tx, migrations embed.FS) fs.WalkDirFunc {
	knownMigrations, err := getMigrationsKnownToDb(tx)
	if err != nil {
		return func(path string, d fs.DirEntry, err error) error {
			return fmt.Errorf("get known migrations: %w", err)
		}
	}

	return func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk func errored: %w", err)
		}

		if dirEntry.IsDir() {
			return nil
		}

		migration, ok := findMigrationByName(knownMigrations, dirEntry.Name())
		if ok && migration.IsApplied {
			return nil
		}

		readBytes, err := migrations.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration file %q: %w", dirEntry.Name(), err)
		}

		migrationHash := hashFile(readBytes)

		_, err = tx.Exec(string(readBytes))
		if err != nil {
			return fmt.Errorf("execute migration %q: %w", dirEntry.Name(), err)
		}

		err = upsertMigration(tx, migrationRow{
			MigrationName: dirEntry.Name(),
			MigrationHash: migrationHash,
			IsApplied:     true,
		})
		if err != nil {
			return err
		}

		return nil
	}
}

func upsertMigration(tx *sql.Tx, migration migrationRow) error {
	var (
		query = `INSERT INTO migrations (migration_name, migration_hash, is_applied)
			VALUES ($1, $2, $3)
			ON CONFLICT(migration_name) DO UPDATE SET
			migration_hash = excluded.migration_hash,
			is_applied = excluded.is_applied`
	)

	_, err := tx.Exec(query, migration.MigrationName, migration.MigrationHash, migration.IsApplied)
	if err != nil {
		return fmt.Errorf("upsert migration: %w", err)
	}

	return nil
}

func getMigrationsKnownToDb(tx *sql.Tx) ([]migrationRow, error) {
	rows, err := tx.Query("SELECT * FROM migrations")
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()

	var appliedMigrations []migrationRow
	for rows.Next() {
		var migration migrationRow
		if err := rows.Scan(&migration.MigrationName, &migration.MigrationHash, &migration.IsApplied); err != nil {
			return nil, fmt.Errorf("scan migration row: %w", err)
		}
		appliedMigrations = append(appliedMigrations, migration)
	}

	return appliedMigrations, nil
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
