CREATE TABLE IF NOT EXISTS call_password_resets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    used_at TEXT,
    request_ip TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_call_password_resets_user_id
    ON call_password_resets(user_id);

CREATE INDEX IF NOT EXISTS idx_call_password_resets_expires_at
    ON call_password_resets(expires_at);
