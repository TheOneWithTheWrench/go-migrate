package test_data

import (
	"database/sql"
)

type migrationRow struct {
	MigrationName string `json:"migration_name,omitempty"`
	MigrationHash string `json:"migration_hash,omitempty"`
	IsApplied     bool   `json:"is_applied,omitempty"`
}

type TestRepo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *TestRepo {
	return &TestRepo{
		db: db,
	}
}

func (r *TestRepo) GetMigrationByName(name string) migrationRow {
	var row migrationRow
	err := r.db.QueryRow("SELECT migration_name, migration_hash, is_applied FROM migrations WHERE migration_name = ?", name).Scan(&row.MigrationName, &row.MigrationHash, &row.IsApplied)
	if err != nil {
		return migrationRow{}
	}
	return row
}

func (r *TestRepo) GetAllMigrations() ([]migrationRow, error) {
	rows, err := r.db.Query("SELECT migration_name, migration_hash, is_applied FROM migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var migrations []migrationRow
	for rows.Next() {
		var row migrationRow
		if err := rows.Scan(&row.MigrationName, &row.MigrationHash, &row.IsApplied); err != nil {
			return nil, err
		}
		migrations = append(migrations, row)
	}
	return migrations, nil
}
