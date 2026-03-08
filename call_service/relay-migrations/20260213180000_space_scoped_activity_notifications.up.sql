CREATE TABLE IF NOT EXISTS user_space_memberships (
    user_id INTEGER NOT NULL,
    host_uuid TEXT NOT NULL,
    space_uuid TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    first_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, host_uuid, space_uuid),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_space_memberships_host_space_active
ON user_space_memberships (host_uuid, space_uuid, is_active);

CREATE TABLE IF NOT EXISTS user_space_message_counters (
    user_id INTEGER NOT NULL,
    host_uuid TEXT NOT NULL,
    space_uuid TEXT NOT NULL,
    pending_count INTEGER NOT NULL DEFAULT 0,
    last_message_at TEXT,
    last_emailed_at TEXT,
    PRIMARY KEY (user_id, host_uuid, space_uuid),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_space_message_counters_user_host
ON user_space_message_counters (user_id, host_uuid);
