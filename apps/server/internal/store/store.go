package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vts-tokenmaxers/apps/server/internal/database"
	"vts-tokenmaxers/apps/server/internal/domain"
	"vts-tokenmaxers/apps/server/internal/importer"
)

var ErrStationNotFound = errors.New("station not found")

type Store struct {
	db *sql.DB
}

// Open opens and migrates the database, importing sourcePath only when the
// dataset tables are empty. This makes startup recoverable without replacing
// already-imported data on every process launch.
func Open(ctx context.Context, databasePath, sourcePath string) (*Store, domain.ImportStats, error) {
	dataStore, err := OpenDatabase(ctx, databasePath)
	if err != nil {
		return nil, domain.ImportStats{}, err
	}
	report, err := dataStore.EnsureDataset(ctx, sourcePath)
	if err != nil {
		_ = dataStore.Close()
		return nil, domain.ImportStats{}, err
	}
	return dataStore, report.ImportStats, nil
}

// OpenDatabase opens SQLite and applies all embedded migrations.
func OpenDatabase(ctx context.Context, databasePath string) (*Store, error) {
	db, err := database.Open(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Database exposes the shared connection to other internal persistence
// packages. Callers must not close it; Store.Close owns its lifetime.
func (s *Store) Database() *sql.DB { return s.db }

// EnsureDataset imports sourcePath only when no dataset broadcasts exist.
func (s *Store) EnsureDataset(ctx context.Context, sourcePath string) (domain.ImportReport, error) {
	empty, err := database.IsDatasetEmpty(ctx, s.db)
	if err != nil {
		return domain.ImportReport{}, err
	}
	if empty {
		return s.ImportSource(ctx, sourcePath)
	}
	stats, err := s.Stats(ctx)
	if err != nil {
		return domain.ImportReport{}, err
	}
	report, err := s.LastImportReport(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return domain.ImportReport{}, err
	}
	report.ImportStats = stats
	return report, nil
}

// ImportSource validates and atomically imports a challenge ZIP or unpacked directory.
func (s *Store) ImportSource(ctx context.Context, sourcePath string) (domain.ImportReport, error) {
	return importer.Import(ctx, s.db, sourcePath)
}

// ImportDataset preserves the original integration surface while using the
// archive-aware importer. New integrations should use ImportSource to receive
// anomaly details and the source fingerprint.
func (s *Store) ImportDataset(ctx context.Context, sourcePath string) (domain.ImportStats, error) {
	report, err := s.ImportSource(ctx, sourcePath)
	return report.ImportStats, err
}

func (s *Store) Stats(ctx context.Context) (domain.ImportStats, error) {
	var stats domain.ImportStats
	queries := []struct {
		query string
		dest  *int
	}{
		{"SELECT COUNT(*) FROM senders", &stats.StationCount},
		{"SELECT COUNT(*) FROM broadcasts WHERE source = 'dataset'", &stats.BroadcastCount},
		{"SELECT COUNT(*) FROM marv_logs WHERE source = 'dataset'", &stats.MarvLogCount},
	}
	for _, item := range queries {
		if err := s.db.QueryRowContext(ctx, item.query).Scan(item.dest); err != nil {
			return stats, fmt.Errorf("query import statistics: %w", err)
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(anomaly_count, 0)
        FROM import_runs ORDER BY import_id DESC LIMIT 1`).Scan(&stats.AnomalyCount); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return stats, fmt.Errorf("query anomaly count: %w", err)
	}
	return stats, nil
}

func (s *Store) LastImportReport(ctx context.Context) (domain.ImportReport, error) {
	var report domain.ImportReport
	var importID int64
	err := s.db.QueryRowContext(ctx, `SELECT import_id, source_fingerprint,
        station_count, broadcast_count, marv_log_count, anomaly_count
        FROM import_runs ORDER BY import_id DESC LIMIT 1`).Scan(
		&importID, &report.SourceFingerprint, &report.StationCount,
		&report.BroadcastCount, &report.MarvLogCount, &report.AnomalyCount,
	)
	if err != nil {
		return report, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT code, severity, source,
        COALESCE(record_id, ''), message FROM import_anomalies
        WHERE import_id = ? ORDER BY anomaly_id`, importID)
	if err != nil {
		return report, fmt.Errorf("query import anomalies: %w", err)
	}
	defer rows.Close()
	report.Anomalies = make([]domain.ImportAnomaly, 0, report.AnomalyCount)
	for rows.Next() {
		var anomaly domain.ImportAnomaly
		if err := rows.Scan(&anomaly.Code, &anomaly.Severity, &anomaly.Source, &anomaly.RecordID, &anomaly.Message); err != nil {
			return report, fmt.Errorf("scan import anomaly: %w", err)
		}
		report.Anomalies = append(report.Anomalies, anomaly)
	}
	if err := rows.Err(); err != nil {
		return report, fmt.Errorf("iterate import anomalies: %w", err)
	}
	return report, nil
}
