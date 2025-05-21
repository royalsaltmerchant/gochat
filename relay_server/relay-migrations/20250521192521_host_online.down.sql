CREATE TABLE hosts_temp (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    author_id TEXT NOT NULL UNIQUE
);

INSERT INTO hosts_temp (id, uuid, name, author_id)
SELECT id, uuid, name, author_id FROM hosts;

DROP TABLE hosts;

ALTER TABLE hosts_temp RENAME TO hosts;