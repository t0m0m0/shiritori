-- Base schema
--
-- Migrations tracking table
CREATE TABLE IF NOT EXISTS migrations (
    migration_number INTEGER PRIMARY KEY,
    migration_name TEXT NOT NULL,
    executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Record execution of this migration
INSERT
OR IGNORE INTO migrations (migration_number, migration_name)
VALUES
    (001, '001-base');
