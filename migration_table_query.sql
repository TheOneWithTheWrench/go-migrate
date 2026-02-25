CREATE TABLE IF NOT EXISTS migrations (
    migration_name  VARCHAR(255) NOT NULL,
    migration_hash  VARCHAR(64),
    is_applied      BOOLEAN NOT NULL DEFAULT FALSE,
    is_dirty        BOOLEAN NOT NULL DEFAULT FALSE,
    primary key (migration_name)
)
