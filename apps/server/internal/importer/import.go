package importer

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"vts-tokenmaxers/apps/server/internal/domain"
)

// Import atomically replaces source-dataset rows with a validated source.
// Parsing happens before the transaction, so malformed input leaves the
// existing database untouched.
func Import(ctx context.Context, db *sql.DB, sourcePath string) (domain.ImportReport, error) {
	dataset, err := Load(sourcePath)
	if err != nil {
		return domain.ImportReport{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ImportReport{}, fmt.Errorf("begin dataset import: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM broadcasts WHERE source = 'dataset'"); err != nil {
		return domain.ImportReport{}, fmt.Errorf("clear imported broadcasts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM marv_logs WHERE source = 'dataset'"); err != nil {
		return domain.ImportReport{}, fmt.Errorf("clear imported Marv logs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM senders
        WHERE profile_source IN ('dataset', 'derived')
          AND sender_id NOT IN (SELECT sender_id FROM broadcasts)`); err != nil {
		return domain.ImportReport{}, fmt.Errorf("clear imported senders: %w", err)
	}

	ids := make([]string, 0, len(dataset.Profiles))
	for id := range dataset.Profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		profile := dataset.Profiles[id]
		if _, err := tx.ExecContext(ctx, `
INSERT INTO senders (
    sender_id, sender_type, first_seen, last_seen, reported_total_broadcasts,
    verified_accurate, verified_false, reliability_score, current_status, profile_source
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(sender_id) DO UPDATE SET
    sender_type = excluded.sender_type,
    first_seen = excluded.first_seen,
    last_seen = excluded.last_seen,
    reported_total_broadcasts = excluded.reported_total_broadcasts,
    verified_accurate = excluded.verified_accurate,
    verified_false = excluded.verified_false,
    reliability_score = excluded.reliability_score,
    current_status = excluded.current_status,
    profile_source = excluded.profile_source`,
			profile.ID, profile.Type, profile.FirstSeen, profile.LastSeen,
			nullableInt(profile.ReportedTotal), nullableInt(profile.VerifiedAccurate),
			nullableInt(profile.VerifiedFalse), nullableInt(profile.ReliabilityScore),
			profile.CurrentStatus, profile.ProfileSource,
		); err != nil {
			return domain.ImportReport{}, fmt.Errorf("import sender %s: %w", id, err)
		}
	}

	for _, item := range dataset.Broadcasts {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO broadcasts (
    broadcast_id, timestamp, sender_id, location, broadcast_type, message_text,
    signal_strength, cross_check_status, ground_truth_label, carrier_text,
    encryption_status, source
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, 'plain', 'dataset')`,
			item.ID, item.Timestamp, item.SenderID, item.Location, item.Type,
			nullableString(item.MessageText), nullableInt(item.SignalStrength),
			item.CrossCheckStatus, item.GroundTruthLabel,
		); err != nil {
			return domain.ImportReport{}, fmt.Errorf("import broadcast %s: %w", item.ID, err)
		}
	}
	for _, entry := range dataset.MarvLogs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO marv_logs
            (entry_number, title, body, source) VALUES (?, ?, ?, 'dataset')`,
			entry.EntryNumber, entry.Title, entry.Body); err != nil {
			return domain.ImportReport{}, fmt.Errorf("import Marv log %d: %w", entry.EntryNumber, err)
		}
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO import_runs (
        imported_at, source_path, source_fingerprint, station_count,
        broadcast_count, marv_log_count, anomaly_count
    ) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), sourcePath, dataset.Fingerprint,
		len(dataset.Profiles), len(dataset.Broadcasts), len(dataset.MarvLogs), len(dataset.Anomalies))
	if err != nil {
		return domain.ImportReport{}, fmt.Errorf("record import: %w", err)
	}
	importID, err := result.LastInsertId()
	if err != nil {
		return domain.ImportReport{}, fmt.Errorf("read import id: %w", err)
	}
	for _, anomaly := range dataset.Anomalies {
		if _, err := tx.ExecContext(ctx, `INSERT INTO import_anomalies
            (import_id, code, severity, source, record_id, message)
            VALUES (?, ?, ?, ?, ?, ?)`, importID, anomaly.Code, anomaly.Severity,
			anomaly.Source, nullableText(anomaly.RecordID), anomaly.Message); err != nil {
			return domain.ImportReport{}, fmt.Errorf("record import anomaly: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.ImportReport{}, fmt.Errorf("commit dataset import: %w", err)
	}
	return domain.ImportReport{
		ImportStats: domain.ImportStats{
			StationCount: len(dataset.Profiles), BroadcastCount: len(dataset.Broadcasts),
			MarvLogCount: len(dataset.MarvLogs), AnomalyCount: len(dataset.Anomalies),
		},
		SourceFingerprint: dataset.Fingerprint,
		Anomalies:         dataset.Anomalies,
	}, nil
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}
