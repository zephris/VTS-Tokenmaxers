CREATE TABLE IF NOT EXISTS senders (
    sender_id TEXT PRIMARY KEY,
    sender_type TEXT NOT NULL,
    first_seen TEXT,
    last_seen TEXT,
    reported_total_broadcasts INTEGER,
    verified_accurate INTEGER,
    verified_false INTEGER,
    reliability_score INTEGER,
    current_status TEXT NOT NULL,
    profile_source TEXT NOT NULL CHECK (profile_source IN ('dataset', 'derived'))
);

CREATE TABLE IF NOT EXISTS broadcasts (
    broadcast_id TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL,
    sender_id TEXT NOT NULL REFERENCES senders(sender_id),
    location TEXT NOT NULL,
    broadcast_type TEXT NOT NULL,
    message_text TEXT,
    signal_strength INTEGER,
    cross_check_status TEXT NOT NULL,
    ground_truth_label TEXT NOT NULL,
    carrier_text TEXT,
    encryption_status TEXT NOT NULL DEFAULT 'plain'
        CHECK (encryption_status IN ('plain', 'encrypted')),
    source TEXT NOT NULL DEFAULT 'dataset'
);

CREATE INDEX IF NOT EXISTS idx_broadcasts_sender_timestamp
    ON broadcasts(sender_id, timestamp, broadcast_id);
CREATE INDEX IF NOT EXISTS idx_broadcasts_location_timestamp
    ON broadcasts(location, timestamp, broadcast_id);
CREATE INDEX IF NOT EXISTS idx_broadcasts_timestamp
    ON broadcasts(timestamp, broadcast_id);
CREATE INDEX IF NOT EXISTS idx_broadcasts_filters
    ON broadcasts(broadcast_type, cross_check_status, timestamp);

CREATE TABLE IF NOT EXISTS marv_logs (
    entry_number INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'dataset'
);

CREATE TABLE IF NOT EXISTS import_runs (
    import_id INTEGER PRIMARY KEY AUTOINCREMENT,
    imported_at TEXT NOT NULL,
    source_path TEXT NOT NULL,
    source_fingerprint TEXT NOT NULL,
    station_count INTEGER NOT NULL,
    broadcast_count INTEGER NOT NULL,
    marv_log_count INTEGER NOT NULL,
    anomaly_count INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS import_anomalies (
    anomaly_id INTEGER PRIMARY KEY AUTOINCREMENT,
    import_id INTEGER NOT NULL REFERENCES import_runs(import_id) ON DELETE CASCADE,
    code TEXT NOT NULL,
    severity TEXT NOT NULL,
    source TEXT NOT NULL,
    record_id TEXT,
    message TEXT NOT NULL
);

