CREATE TABLE spaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    author_id INTEGER NOT NULL,
    FOREIGN KEY (author_id) REFERENCES users(id)
);

CREATE TABLE channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    space_uuid TEXT NOT NULL,
    FOREIGN KEY (space_uuid) REFERENCES spaces(uuid) ON DELETE CASCADE
);

CREATE TABLE messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_uuid TEXT NOT NULL,
    content TEXT NOT NULL,
    user_id INTEGER NOT NULL,
    timestamp TEXT,
    FOREIGN KEY (channel_uuid) REFERENCES channels(uuid) ON DELETE CASCADE
    FOREIGN KEY (user_id) REFERENCES users(id)
);




