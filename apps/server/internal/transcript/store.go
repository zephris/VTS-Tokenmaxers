package transcript

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"vts-tokenmaxers/apps/server/internal/stego"
)

var ErrSequenceConflict = errors.New("conversation transcript changed; refresh and retry")

type Snapshot struct {
	ConversationID   string         `json:"conversationId"`
	StationID        string         `json:"stationId"`
	Records          []stego.Record `json:"records"`
	SyncCode         string         `json:"syncCode"`
	Algorithm        string         `json:"algorithm"`
	ModelFingerprint string         `json:"modelFingerprint,omitempty"`
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
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
	`)
	if err != nil {
		return fmt.Errorf("migrate public transcripts: %w", err)
	}
	return nil
}

func (s *Store) Snapshot(ctx context.Context, conversationID, stationID string) (Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT record_index, sender, sender_sequence, carrier_text, sync_code, model_fingerprint
		FROM steganography_records
		WHERE conversation_id = ? AND station_id = ?
		ORDER BY record_index ASC
	`, conversationID, stationID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("query public transcript: %w", err)
	}
	defer rows.Close()

	snapshot := Snapshot{ConversationID: conversationID, StationID: stationID, Records: []stego.Record{}}
	for rows.Next() {
		var record stego.Record
		var syncCode, fingerprint string
		if err := rows.Scan(&record.Index, &record.From, &record.SenderSequence, &record.CarrierText, &syncCode, &fingerprint); err != nil {
			return Snapshot{}, fmt.Errorf("scan public transcript: %w", err)
		}
		snapshot.Records = append(snapshot.Records, record)
		snapshot.SyncCode = syncCode
		snapshot.ModelFingerprint = fingerprint
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, fmt.Errorf("read public transcript: %w", err)
	}
	return snapshot, nil
}

func (s *Store) Append(
	ctx context.Context,
	conversationID string,
	stationID string,
	record stego.Record,
	syncCode string,
	modelFingerprint string,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transcript append: %w", err)
	}
	defer tx.Rollback()

	var nextIndex uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(record_index) + 1, 0)
		FROM steganography_records
		WHERE conversation_id = ? AND station_id = ?
	`, conversationID, stationID).Scan(&nextIndex); err != nil {
		return fmt.Errorf("read transcript sequence: %w", err)
	}
	if record.Index != nextIndex {
		return ErrSequenceConflict
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO steganography_records (
			conversation_id, station_id, record_index, sender, sender_sequence,
			carrier_text, sync_code, model_fingerprint, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, conversationID, stationID, record.Index, record.From, record.SenderSequence,
		record.CarrierText, syncCode, modelFingerprint, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("append public transcript: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transcript append: %w", err)
	}
	return nil
}
