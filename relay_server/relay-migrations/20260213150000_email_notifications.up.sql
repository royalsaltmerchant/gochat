CREATE TABLE IF NOT EXISTS email_preferences (
    user_id INTEGER PRIMARY KEY,
    invite_emails INTEGER NOT NULL DEFAULT 1,
    activity_emails INTEGER NOT NULL DEFAULT 1,
    weekly_emails INTEGER NOT NULL DEFAULT 1,
    unsubscribed_all INTEGER NOT NULL DEFAULT 0,
    unsubscribe_token TEXT NOT NULL UNIQUE,
    last_weekly_sent_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TRIGGER IF NOT EXISTS users_email_preferences_after_insert
AFTER INSERT ON users
BEGIN
    INSERT OR IGNORE INTO email_preferences (user_id, unsubscribe_token)
    VALUES (NEW.id, lower(hex(randomblob(16))));
END;

INSERT OR IGNORE INTO email_preferences (user_id, unsubscribe_token)
SELECT u.id, lower(hex(randomblob(16)))
FROM users u;

CREATE TABLE IF NOT EXISTS user_host_memberships (
    user_id INTEGER NOT NULL,
    host_uuid TEXT NOT NULL,
    first_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, host_uuid),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_host_message_counters (
    user_id INTEGER NOT NULL,
    host_uuid TEXT NOT NULL,
    pending_count INTEGER NOT NULL DEFAULT 0,
    last_message_at TEXT,
    last_emailed_at TEXT,
    PRIMARY KEY (user_id, host_uuid),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
