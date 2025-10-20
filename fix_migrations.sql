-- Create migrations tracking table
CREATE TABLE IF NOT EXISTS schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(255) NOT NULL UNIQUE,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Mark initial schema as already applied (tables exist from manual setup)
INSERT INTO schema_migrations (version) VALUES ('001_initial_schema.sql')
ON CONFLICT (version) DO NOTHING;

-- Show current migrations
SELECT version, applied_at FROM schema_migrations ORDER BY applied_at;
