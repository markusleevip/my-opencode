-- +goose Up
-- +goose StatementBegin
-- Settings table for global configuration key-value pairs
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at INTEGER NOT NULL,  -- Unix timestamp in milliseconds
    created_at INTEGER NOT NULL   -- Unix timestamp in milliseconds
);

CREATE TRIGGER IF NOT EXISTS update_settings_updated_at
AFTER UPDATE ON settings
BEGIN
UPDATE settings SET updated_at = strftime('%s', 'now')
WHERE key = new.key;
END;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS update_settings_updated_at;
DROP TABLE IF EXISTS settings;
-- +goose StatementEnd
