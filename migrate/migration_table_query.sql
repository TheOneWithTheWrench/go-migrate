CREATE TABLE IF NOT EXISTS migrations (
    migration_name  VARCHAR(255) NOT NULL,
    migration_hash  VARCHAR(64),           
    is_applied      BOOLEAN DEFAULT FALSE, 
    primary key (migration_name)
)
