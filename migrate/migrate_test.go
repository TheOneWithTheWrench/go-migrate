package migrate_test

import (
	"database/sql"
	"embed"
	"migrate/migrate"
	"migrate/migrate/test_data"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed test_data/two_files_no_error/*.sql
var noErrorsMigration embed.FS

var (
	// These two files have the same name, so they will be treated as the same file
	// However, they are different and their hashes are different

	//go:embed test_data/one_file_changing_content/*.sql
	changingMigrations embed.FS // This is the original file
	//go:embed test_data/one_file_changing_content/duplicate-file/*.sql
	changingMigrationsChanged embed.FS // This changes the original file
)

//go:embed test_data/one_file_invalid_sql/*
var invalidMigration embed.FS

func TestMigrate(t *testing.T) {
	var newRepo = func(db *sql.DB) *test_data.TestRepo {
		return test_data.NewRepo(db)
	}

	t.Run("Migrate", func(t *testing.T) {
		var (
			sut = func(db *sql.DB, migrations embed.FS) error {
				return migrate.NewMigrator(db, migrations).Migrate()
			}
		)
		t.Run("successfully migrate", func(t *testing.T) {
			// Arrange
			var (
				db   = migrate.SetupTestDatabase(t)
				repo = newRepo(db)
			)

			// Act
			err := sut(db, noErrorsMigration)

			// Assert
			assert.NoError(t, err)
			migrations, err := repo.GetAllMigrations()
			assert.NoError(t, err)
			assert.Len(t, migrations, 2)
		})

		t.Run("can call migrate multiple times", func(t *testing.T) {
			db := migrate.SetupTestDatabase(t)

			err := sut(db, noErrorsMigration)
			assert.NoError(t, err)

			err = sut(db, noErrorsMigration)
			assert.NoError(t, err)
		})

		t.Run("should error when migration file changed", func(t *testing.T) {
			// Arrange
			var (
				db = migrate.SetupTestDatabase(t)
			)
			err := sut(db, changingMigrations) // Setup the initial state

			// Act
			err = sut(db, changingMigrationsChanged) // Attempt to migrate with changed file

			// Assert
			assert.Error(t, err)
			assert.ErrorIs(t, err, migrate.ErrMigrationFileChanged)
		})

		t.Run("should error when migration has invalid sql", func(t *testing.T) {
			// Arrange
			var (
				db = migrate.SetupTestDatabase(t)
			)

			// Act
			err := sut(db, invalidMigration)

			// Assert
			assert.Error(t, err)
			assert.ErrorIs(t, err, migrate.ErrMigrationFailed)
		})
	})
}
