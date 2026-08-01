ALTER TABLE steganography_records
    ADD COLUMN broadcast_type TEXT NOT NULL DEFAULT 'routine_check';
