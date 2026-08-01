package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"vts-tokenmaxers/apps/server/internal/domain"
)

const publicBroadcastColumns = `broadcast_id, timestamp, sender_id, location,
    broadcast_type, message_text, signal_strength, cross_check_status,
    carrier_text, encryption_status`

func (s *Store) Dashboard(ctx context.Context) (domain.DashboardResponse, error) {
	analysisTimestamp, err := s.analysisTimestamp(ctx)
	if err != nil {
		return domain.DashboardResponse{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT sender_id FROM senders
        WHERE sender_type = 'robot_outpost' ORDER BY sender_id`)
	if err != nil {
		return domain.DashboardResponse{}, fmt.Errorf("query dashboard outposts: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return domain.DashboardResponse{}, fmt.Errorf("scan dashboard outpost: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return domain.DashboardResponse{}, fmt.Errorf("close dashboard outposts: %w", err)
	}

	outposts := make([]domain.OutpostSummary, 0, len(ids))
	for _, id := range ids {
		outpost, _, err := s.calculateOutpost(ctx, id, analysisTimestamp)
		if err != nil {
			return domain.DashboardResponse{}, err
		}
		outposts = append(outposts, outpost)
	}
	sortOutposts(outposts)
	recent, err := s.queryBroadcasts(ctx, `SELECT `+publicBroadcastColumns+`
        FROM broadcasts ORDER BY timestamp DESC, broadcast_id DESC LIMIT 20`)
	if err != nil {
		return domain.DashboardResponse{}, err
	}
	return domain.DashboardResponse{
		AnalysisTimestamp: analysisTimestamp, Outposts: outposts, RecentBroadcasts: recent,
	}, nil
}

// Broadcasts returns an operational, paginated view. The private ground-truth
// column is intentionally absent from the SELECT list.
func (s *Store) Broadcasts(ctx context.Context, filter domain.BroadcastFilter) (domain.BroadcastsResponse, error) {
	page, limit := filter.Page, filter.Limit
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 6)
	if value := strings.TrimSpace(filter.SenderID); value != "" {
		clauses = append(clauses, "sender_id = ?")
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.Location); value != "" {
		clauses = append(clauses, "location = ?")
		args = append(args, value)
	}
	if filter.Type != "" {
		clauses = append(clauses, "broadcast_type = ?")
		args = append(args, filter.Type)
	}
	if filter.CrossCheckStatus != "" {
		clauses = append(clauses, "cross_check_status = ?")
		args = append(args, filter.CrossCheckStatus)
	}
	if value := strings.TrimSpace(filter.Query); value != "" {
		clauses = append(clauses, "(message_text LIKE ? OR sender_id LIKE ? OR location LIKE ?)")
		pattern := "%" + value + "%"
		args = append(args, pattern, pattern, pattern)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM broadcasts"+where, args...).Scan(&total); err != nil {
		return domain.BroadcastsResponse{}, fmt.Errorf("count filtered broadcasts: %w", err)
	}
	queryArgs := append(append([]any{}, args...), limit, (page-1)*limit)
	items, err := s.queryBroadcasts(ctx, `SELECT `+publicBroadcastColumns+`
        FROM broadcasts`+where+` ORDER BY timestamp DESC, broadcast_id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return domain.BroadcastsResponse{}, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return domain.BroadcastsResponse{
		Broadcasts: items,
		Pagination: domain.Pagination{Page: page, Limit: limit, Total: total, TotalPages: totalPages},
	}, nil
}

// Outpost returns risk metrics and the complete related evidence timeline.
func (s *Store) Outpost(ctx context.Context, senderID string) (domain.OutpostDetailResponse, error) {
	analysisTimestamp, err := s.analysisTimestamp(ctx)
	if err != nil {
		return domain.OutpostDetailResponse{}, err
	}
	outpost, evidence, err := s.calculateOutpost(ctx, senderID, analysisTimestamp)
	if err != nil {
		return domain.OutpostDetailResponse{}, err
	}
	return domain.OutpostDetailResponse{
		AnalysisTimestamp: analysisTimestamp, Outpost: outpost, EvidenceTimeline: evidence,
	}, nil
}

func (s *Store) IncidentEvidence(ctx context.Context, senderID string) ([]domain.Broadcast, error) {
	var senderType string
	err := s.db.QueryRowContext(ctx, "SELECT sender_type FROM senders WHERE sender_id = ?", senderID).Scan(&senderType)
	if errors.Is(err, sql.ErrNoRows) {
		return []domain.Broadcast{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find incident sender: %w", err)
	}
	if senderType == "robot_outpost" {
		detail, err := s.Outpost(ctx, senderID)
		if err != nil {
			return nil, err
		}
		return detail.EvidenceTimeline, nil
	}
	return s.queryBroadcasts(ctx, `SELECT `+publicBroadcastColumns+`
        FROM broadcasts WHERE sender_id = ? ORDER BY timestamp, broadcast_id`, senderID)
}

func (s *Store) Stations(ctx context.Context) (domain.StationsResponse, error) {
	analysisTimestamp, err := s.analysisTimestamp(ctx)
	if err != nil {
		return domain.StationsResponse{}, err
	}
	rows, err := s.db.QueryContext(ctx, stationSelect+` ORDER BY latest.timestamp DESC, s.sender_id`)
	if err != nil {
		return domain.StationsResponse{}, fmt.Errorf("query stations: %w", err)
	}
	defer rows.Close()
	stations := make([]domain.StationSummary, 0)
	for rows.Next() {
		station, err := scanStation(rows)
		if err != nil {
			return domain.StationsResponse{}, err
		}
		stations = append(stations, station)
	}
	if err := rows.Err(); err != nil {
		return domain.StationsResponse{}, fmt.Errorf("iterate stations: %w", err)
	}
	return domain.StationsResponse{AnalysisTimestamp: analysisTimestamp, Stations: stations}, nil
}

func (s *Store) StationBroadcasts(ctx context.Context, senderID string) (domain.StationBroadcastsResponse, error) {
	station, err := scanStation(s.db.QueryRowContext(ctx, stationSelect+" WHERE s.sender_id = ?", senderID))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StationBroadcastsResponse{}, ErrStationNotFound
	}
	if err != nil {
		return domain.StationBroadcastsResponse{}, err
	}
	broadcasts, err := s.queryBroadcasts(ctx, `SELECT `+publicBroadcastColumns+`
        FROM broadcasts WHERE sender_id = ? ORDER BY timestamp, broadcast_id`, senderID)
	if err != nil {
		return domain.StationBroadcastsResponse{}, err
	}
	return domain.StationBroadcastsResponse{Station: station, Broadcasts: broadcasts}, nil
}

const stationSelect = `SELECT s.sender_id, s.sender_type, latest.location,
    latest.timestamp, latest.message_text,
    (SELECT COUNT(*) FROM broadcasts count_b WHERE count_b.sender_id = s.sender_id),
    s.reliability_score, s.current_status, s.profile_source
FROM senders s
JOIN broadcasts latest ON latest.broadcast_id = (
    SELECT latest_b.broadcast_id FROM broadcasts latest_b
    WHERE latest_b.sender_id = s.sender_id
    ORDER BY latest_b.timestamp DESC, latest_b.broadcast_id DESC LIMIT 1
)`

type rowScanner interface{ Scan(...any) error }

func scanStation(scanner rowScanner) (domain.StationSummary, error) {
	var station domain.StationSummary
	var preview sql.NullString
	var reliability sql.NullInt64
	if err := scanner.Scan(
		&station.SenderID, &station.SenderType, &station.Location, &station.LastBroadcastAt,
		&preview, &station.BroadcastCount, &reliability, &station.Status, &station.ProfileSource,
	); err != nil {
		return station, err
	}
	station.LastMessagePreview = nullStringPointer(preview)
	station.ReliabilityScore = nullIntPointer(reliability)
	return station, nil
}

func (s *Store) queryBroadcasts(ctx context.Context, query string, args ...any) ([]domain.Broadcast, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query broadcasts: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Broadcast, 0)
	for rows.Next() {
		var item domain.Broadcast
		var message, carrier sql.NullString
		var signal sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.Timestamp, &item.SenderID, &item.Location, &item.Type,
			&message, &signal, &item.CrossCheckStatus, &carrier, &item.EncryptionStatus,
		); err != nil {
			return nil, fmt.Errorf("scan broadcast: %w", err)
		}
		item.MessageText = nullStringPointer(message)
		item.SignalStrength = nullIntPointer(signal)
		item.CarrierText = nullStringPointer(carrier)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate broadcasts: %w", err)
	}
	return items, nil
}

func (s *Store) analysisTimestamp(ctx context.Context) (string, error) {
	var timestamp string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(timestamp), '')
        FROM broadcasts WHERE source = 'dataset'`).Scan(&timestamp); err != nil {
		return "", fmt.Errorf("query analysis timestamp: %w", err)
	}
	if timestamp == "" {
		return "", errors.New("broadcast dataset is empty")
	}
	return timestamp, nil
}

func nullStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullIntPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}
