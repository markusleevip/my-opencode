-- name: GetSetting :one
SELECT * FROM settings WHERE key = ? LIMIT 1;

-- name: SetSetting :one
INSERT INTO settings (key, value, created_at, updated_at) 
VALUES (?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET 
    value = excluded.value, 
    updated_at = excluded.updated_at
RETURNING *;
