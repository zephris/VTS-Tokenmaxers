CREATE TABLE IF NOT EXISTS steganography_records (
    conversation_id TEXT NOT NULL,
    station_id TEXT NOT NULL,
    record_index INTEGER NOT NULL,
    sender TEXT NOT NULL,
    sender_sequence INTEGER NOT NULL,
    carrier_text TEXT NOT NULL,
    sync_code TEXT NOT NULL DEFAULT '',
    model_fingerprint TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (conversation_id, station_id, record_index)
);

CREATE INDEX IF NOT EXISTS idx_steganography_records_station
    ON steganography_records (conversation_id, station_id, record_index);
