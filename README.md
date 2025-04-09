# Go Migrate

A simple Go library for managing database schema migrations using embedded SQL files. It ensures migrations are applied transactionally and tracks their state in the database, preventing accidental re-application or modification of already applied migrations.

## Features

* **Embed Migrations:** Uses Go's `//go:embed` directive to bundle SQL migration files directly into your application binary.
* **Transactional:** Applies all pending migrations within a single database transaction. If any migration fails, the entire process is rolled back.
* **State Tracking:** Creates and maintains a `migrations` table in your database to track which migrations have been applied.
* **Integrity Check:** Calculates a SHA256 hash of each migration file upon application. Before applying new migrations, it verifies that previously applied migrations haven't been altered by comparing stored hashes with current file hashes.
* **Idempotent:** Ensures migrations are only applied once.
* **Configurable Timeout:** Includes a configurable timeout for the migration process (defaults to 10 seconds).
* **PostgreSQL Focused:** Currently designed with PostgreSQL in mind (uses `lib/pq` driver and potentially PG-specific SQL).

## Installation

```bash
go get github.com/TheOneWithTheWrench/go-migrate
```

## Usage

1.  **Create your migration files:** Place your SQL migration files in a directory (e.g., `migrations/`). It's recommended to name them sequentially for predictable execution order (e.g., `001_initial_schema.sql`, `002_add_users_table.sql`).

    ```
    .
    ├── go.mod
    ├── main.go // Your application code
    └── migrations/
        ├── 001_create_audit_log.sql
        └── 002_create_users_table.sql
    ```

2.  **Embed the migration files in your Go application:**

    ```go
    package main

    import (
        "database/sql"
        "embed"
        // ... other imports
    )

    //go:embed migrations/*.sql
    var migrationFS embed.FS // Embed the migrations directory

    // ... rest of your application setup
    ```

3.  **Initialize and run the migrator:** Once you have your database connection (`*sql.DB`) and the embedded filesystem (`embed.FS`), you can run the migrator like this:

    ```go
    // Assume 'db *sql.DB' is your initialized and connected PostgreSQL database handle.
    // Assume 'migrationFS embed.FS' is the variable holding your embedded migration files (from step 2).

    import (
        "log"
        "time"
        migrate "github.com/TheOneWithTheWrench/go-migrate" 
    )

    func runMigrations(db *sql.DB, migrationFS embed.FS) {
        // Initialize the migrator, optionally overriding the default 10s timeout.
        migrator := migrate.NewMigrator(db, migrationFS, migrate.WithMigrationTimeout(30*time.Second))
        // Or use the default timeout:
        // migrator := migrate.NewMigrator(db, migrationFS)

        // Apply the migrations
        log.Println("Applying database migrations...")
        err := migrator.Migrate()
        if err != nil {
            // Check for specific migration errors if needed
            if err == migrate.ErrMigrationFileChanged {
                 log.Fatalf("CRITICAL: Migration failed because a previously applied migration file has been modified. Manual intervention required.")
            } else {
                 // Handle generic migration errors (connection, SQL syntax, permissions etc.)
                 log.Fatalf("Migration failed: %v", err)
            }
        }

        log.Println("Database migrations applied successfully!")
    }

    // Example call within your application startup:
    // func main() {
    //     db := setupDatabaseConnection() // Your function to get *sql.DB
    //     runMigrations(db, migrationFS)
    //     // ... start your application server etc.
    // }
    ```

## Configuration Options

The `NewMigrator` function uses the functional options pattern for configuration.

* **`WithMigrationTimeout(time.Duration)`**: Sets the maximum time allowed for the entire migration process (including connecting, running all SQL files, and committing). If the timeout is exceeded, the context will be canceled, and the transaction will be rolled back.
    * *Default*: `10 * time.Second`

## How it Works

1.  **Initialization:** The `Migrator` is created with a database connection (`*sql.DB`), an embedded filesystem (`embed.FS`), and any configured options.
2.  **Transaction Start:** The `Migrate()` method starts a database transaction with a context governed by the configured `migrationTimeout`.
3.  **Migration Table:** It ensures a `migrations` table exists (using the embedded `migration_table_query.sql`). This table stores the name, hash, and applied status of each migration.
4.  **Integrity Check:** It fetches the records of already applied migrations from the `migrations` table. It then walks the embedded filesystem, comparing the hash of any file found in the table with its stored hash. If a mismatch occurs, it returns `ErrMigrationFileChanged`.
5.  **Apply Pending Migrations:** It walks the embedded filesystem again. For each file:
    * If the file is not listed in the `migrations` table or is marked as not applied (`is_applied=false`), its SQL content is executed.
    * Upon successful execution, the file's SHA256 hash is calculated, and an entry (or update) is made in the `migrations` table with the filename, hash, and `is_applied=true`.
    * If execution fails, the process stops, returning an error wrapping `ErrMigrationFailed`.
6.  **Commit/Rollback:** If all migrations are applied successfully and integrity checks pass within the timeout period, the transaction is committed. Otherwise, it's rolled back (either due to an error or timeout).

## Current Limitations

* **PostgreSQL Specific:** This library currently uses the `github.com/lib/pq` driver and contains SQL (`migration_table_query.sql`, `upsertMigration` query) that is written for PostgreSQL (e.g., `ON CONFLICT DO UPDATE`). It is **not** database agnostic out-of-the-box.

## Future Work

* Refactor the library to support multiple database systems. This might involve:
    * Accepting a more generic database interface.
    * Being able to configure the Migration's table and it's name
    * Using a more abstract way to handle transactions and query execution.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
