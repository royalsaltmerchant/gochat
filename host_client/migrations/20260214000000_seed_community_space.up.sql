INSERT OR IGNORE INTO spaces (uuid, name, author_id) VALUES ('parch-community', 'Parch Community', 0);
INSERT OR IGNORE INTO channels (uuid, name, space_uuid) VALUES ('parch-community-general', 'general', 'parch-community');
INSERT OR IGNORE INTO channels (uuid, name, space_uuid) VALUES ('parch-community-feedback', 'feedback', 'parch-community');
INSERT OR IGNORE INTO channels (uuid, name, space_uuid) VALUES ('parch-community-announcements', 'announcements', 'parch-community');
