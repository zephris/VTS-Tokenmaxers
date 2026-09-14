CREATE TABLE IF NOT EXISTS station_accounts (
    station_id TEXT PRIMARY KEY REFERENCES senders(sender_id) ON DELETE CASCADE,
    username TEXT NOT NULL UNIQUE,
    password_salt TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TEXT
);

CREATE TABLE IF NOT EXISTS station_sessions (
    token_hash TEXT PRIMARY KEY,
    station_id TEXT NOT NULL REFERENCES station_accounts(station_id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_station_sessions_station
    ON station_sessions(station_id, expires_at);

CREATE INDEX IF NOT EXISTS idx_station_sessions_expires
    ON station_sessions(expires_at);
