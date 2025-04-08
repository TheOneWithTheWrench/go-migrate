package migrate_test

import (
	"database/sql"
	"embed"
	"migrate/migrate"
	"migrate/migrate/test_data"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed test_data/*.sql
var testMigrations embed.FS

func TestMigrate(t *testing.T) {
	var newRepo = func(db *sql.DB) *test_data.TestRepo {
		return test_data.NewRepo(db)
	}

	t.Run("Migrate", func(t *testing.T) {
		t.Run("successfully migrate", func(t *testing.T) {
			// Arrange
			var (
				db   = migrate.SetupTestDatabase(t)
				repo = newRepo(db)
			)

			// Act
			err := migrate.Migrate(db, testMigrations)

			// Assert
			assert.NoError(t, err)
			migrations, err := repo.GetAllMigrations()
			assert.NoError(t, err)
			assert.Len(t, migrations, 2)
		})

		t.Run("can call migrate multiple times", func(t *testing.T) {
			db := migrate.SetupTestDatabase(t)

			err := migrate.Migrate(db, testMigrations)
			assert.NoError(t, err)

			err = migrate.Migrate(db, testMigrations)
			assert.NoError(t, err)
		})
	})
}
