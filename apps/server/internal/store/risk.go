package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"vts-tokenmaxers/apps/server/internal/domain"
	"vts-tokenmaxers/apps/server/internal/importer"
)

const (
	defaultCadenceHours = 24.0
	resolutionWindow    = 48 * time.Hour
)

func (s *Store) calculateOutpost(ctx context.Context, senderID, analysisTimestamp string) (domain.OutpostSummary, []domain.Broadcast, error) {
	var outpost domain.OutpostSummary
	var reliability sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT s.sender_id, latest.location,
		latest.timestamp, s.reliability_score, s.current_status,
		(SELECT COUNT(*) FROM broadcasts count_b WHERE count_b.sender_id = s.sender_id)
        FROM senders s
        JOIN broadcasts latest ON latest.broadcast_id = (
            SELECT broadcast_id FROM broadcasts
            WHERE sender_id = s.sender_id
            ORDER BY timestamp DESC, broadcast_id DESC LIMIT 1
        )
        WHERE s.sender_id = ? AND s.sender_type = 'robot_outpost'`, senderID).Scan(
		&outpost.SenderID, &outpost.Location, &outpost.LastSeen,
		&reliability, &outpost.Status, &outpost.BroadcastCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return outpost, nil, ErrStationNotFound
	}
	if err != nil {
		return outpost, nil, fmt.Errorf("query outpost %s: %w", senderID, err)
	}
	if reliability.Valid {
		outpost.ReliabilityScore = int(reliability.Int64)
	}
	analysisTime, err := time.Parse(importer.TimestampLayout, analysisTimestamp)
	if err != nil {
		return outpost, nil, fmt.Errorf("parse analysis timestamp: %w", err)
	}
	lastSeen, err := time.Parse(importer.TimestampLayout, outpost.LastSeen)
	if err != nil {
		return outpost, nil, fmt.Errorf("parse last broadcast for %s: %w", senderID, err)
	}
	outpost.HoursSilent = roundedHours(analysisTime.Sub(lastSeen))
	outpost.ExpectedCadenceHours, err = s.expectedCadence(ctx, senderID)
	if err != nil {
		return outpost, nil, err
	}
	outpost.SilenceRatio = round(outpost.HoursSilent/outpost.ExpectedCadenceHours, 2)

	evidence, err := s.queryBroadcasts(ctx, `SELECT `+publicBroadcastColumns+`
        FROM broadcasts WHERE sender_id = ? OR location = ?
        ORDER BY timestamp, broadcast_id`, senderID, outpost.Location)
	if err != nil {
		return outpost, nil, err
	}
	unresolvedIDs := unresolvedEmergencyIDs(evidence)
	contradictionIDs := contradictionIDs(evidence)
	outpost.UnresolvedEmergencyCount = len(unresolvedIDs)
	outpost.ContradictionCount = len(contradictionIDs)
	outpost.RiskScore, outpost.RiskLevel, outpost.RiskReasons = assessRisk(outpost, unresolvedIDs, contradictionIDs)
	return outpost, evidence, nil
}

func (s *Store) expectedCadence(ctx context.Context, senderID string) (float64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT timestamp FROM broadcasts
        WHERE sender_id = ? ORDER BY timestamp, broadcast_id`, senderID)
	if err != nil {
		return 0, fmt.Errorf("query cadence for %s: %w", senderID, err)
	}
	defer rows.Close()
	times := make([]time.Time, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return 0, fmt.Errorf("scan cadence for %s: %w", senderID, err)
		}
		parsed, err := time.Parse(importer.TimestampLayout, raw)
		if err != nil {
			return 0, fmt.Errorf("parse cadence timestamp for %s: %w", senderID, err)
		}
		times = append(times, parsed)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate cadence for %s: %w", senderID, err)
	}
	// At least three broadcasts (two intervals) are needed for a useful median.
	if len(times) < 3 {
		return defaultCadenceHours, nil
	}
	intervals := make([]float64, 0, len(times)-1)
	for index := 1; index < len(times); index++ {
		hours := times[index].Sub(times[index-1]).Hours()
		if hours > 0 {
			intervals = append(intervals, hours)
		}
	}
	if len(intervals) < 2 {
		return defaultCadenceHours, nil
	}
	sort.Float64s(intervals)
	middle := len(intervals) / 2
	median := intervals[middle]
	if len(intervals)%2 == 0 {
		median = (intervals[middle-1] + intervals[middle]) / 2
	}
	if median < 1 {
		median = 1
	}
	return round(median, 1), nil
}

func unresolvedEmergencyIDs(evidence []domain.Broadcast) []string {
	result := make([]string, 0)
	for index, item := range evidence {
		if item.Type != domain.BroadcastEmergency {
			continue
		}
		started, err := time.Parse(importer.TimestampLayout, item.Timestamp)
		if err != nil {
			continue
		}
		resolved := false
		for _, candidate := range evidence[index+1:] {
			if candidate.Location != item.Location {
				continue
			}
			candidateTime, err := time.Parse(importer.TimestampLayout, candidate.Timestamp)
			if err != nil || candidateTime.Sub(started) > resolutionWindow {
				break
			}
			if candidate.CrossCheckStatus == domain.CrossCheckVerified && isCalmingBroadcast(candidate.Type) {
				resolved = true
				break
			}
		}
		if !resolved {
			result = append(result, item.ID)
		}
	}
	return result
}

func contradictionIDs(evidence []domain.Broadcast) []string {
	result := make([]string, 0)
	for _, item := range evidence {
		if item.Type == domain.BroadcastAllClear && item.CrossCheckStatus == domain.CrossCheckDisputed {
			result = append(result, item.ID)
		}
	}
	return result
}

func isCalmingBroadcast(kind domain.BroadcastType) bool {
	return kind == domain.BroadcastAllClear || kind == domain.BroadcastRoutineCheck || kind == domain.BroadcastSituation
}

func assessRisk(outpost domain.OutpostSummary, unresolvedIDs, contradictionIDs []string) (int, domain.RiskLevel, []domain.RiskReason) {
	score := 0
	reasons := make([]domain.RiskReason, 0, 5)
	if outpost.SilenceRatio >= 3 {
		score += 60
		reasons = append(reasons, domain.RiskReason{
			Code: "cadence_severely_overdue", Severity: domain.RiskCritical,
			Message: fmt.Sprintf("No broadcast for %.1f hours (%.1fx the expected %.1f-hour cadence).", outpost.HoursSilent, outpost.SilenceRatio, outpost.ExpectedCadenceHours),
		})
	} else if outpost.SilenceRatio >= 2 {
		score += 40
		reasons = append(reasons, domain.RiskReason{
			Code: "cadence_overdue", Severity: domain.RiskHigh,
			Message: fmt.Sprintf("No broadcast for %.1f hours (%.1fx expected cadence).", outpost.HoursSilent, outpost.SilenceRatio),
		})
	} else if outpost.SilenceRatio >= 1.5 {
		score += 20
		reasons = append(reasons, domain.RiskReason{
			Code: "cadence_late", Severity: domain.RiskMedium,
			Message: fmt.Sprintf("The latest check-in is late relative to the %.1f-hour cadence.", outpost.ExpectedCadenceHours),
		})
	}
	if len(unresolvedIDs) > 0 {
		score += min(30, 15*len(unresolvedIDs))
		reasons = append(reasons, domain.RiskReason{
			Code: "unresolved_emergency", Severity: domain.RiskHigh,
			Message:     fmt.Sprintf("%d emergency broadcast(s) lack a verified calming follow-up within 48 hours.", len(unresolvedIDs)),
			EvidenceIDs: unresolvedIDs,
		})
	}
	if len(contradictionIDs) > 0 {
		score += min(20, 10*len(contradictionIDs))
		reasons = append(reasons, domain.RiskReason{
			Code: "disputed_all_clear", Severity: domain.RiskHigh,
			Message:     fmt.Sprintf("%d all-clear broadcast(s) for this location were disputed.", len(contradictionIDs)),
			EvidenceIDs: contradictionIDs,
		})
	}
	if outpost.Status == domain.SenderGoneQuiet {
		score += 20
		reasons = append(reasons, domain.RiskReason{
			Code: "sender_gone_quiet", Severity: domain.RiskCritical,
			Message: "Sender history marks this outpost as gone quiet.",
		})
	} else if outpost.Status == domain.SenderSuspectedCompromised {
		score += 30
		reasons = append(reasons, domain.RiskReason{
			Code: "sender_compromised", Severity: domain.RiskCritical,
			Message: "Sender history marks this identity as suspected compromised.",
		})
	}
	if outpost.ReliabilityScore > 0 && outpost.ReliabilityScore < 50 {
		score += 20
		reasons = append(reasons, domain.RiskReason{
			Code: "low_reliability", Severity: domain.RiskHigh,
			Message: fmt.Sprintf("Historical reliability is only %d/100.", outpost.ReliabilityScore),
		})
	} else if outpost.ReliabilityScore >= 50 && outpost.ReliabilityScore < 75 {
		score += 10
		reasons = append(reasons, domain.RiskReason{
			Code: "reduced_reliability", Severity: domain.RiskMedium,
			Message: fmt.Sprintf("Historical reliability is %d/100.", outpost.ReliabilityScore),
		})
	}
	if score > 100 {
		score = 100
	}
	level := domain.RiskLow
	if score >= 70 {
		level = domain.RiskCritical
	} else if score >= 45 {
		level = domain.RiskHigh
	} else if score >= 20 {
		level = domain.RiskMedium
	}
	if len(reasons) == 0 {
		reasons = append(reasons, domain.RiskReason{
			Code: "operating_normally", Severity: domain.RiskLow,
			Message: "Broadcast cadence and observable evidence are within normal limits.",
		})
	}
	for index := range reasons {
		if reasons[index].EvidenceIDs == nil {
			reasons[index].EvidenceIDs = []string{}
		}
	}
	return score, level, reasons
}

func sortOutposts(outposts []domain.OutpostSummary) {
	sort.Slice(outposts, func(i, j int) bool {
		if outposts[i].RiskScore != outposts[j].RiskScore {
			return outposts[i].RiskScore > outposts[j].RiskScore
		}
		if outposts[i].HoursSilent != outposts[j].HoursSilent {
			return outposts[i].HoursSilent > outposts[j].HoursSilent
		}
		return outposts[i].SenderID < outposts[j].SenderID
	})
}

func roundedHours(duration time.Duration) float64 { return round(duration.Hours(), 1) }

func round(value float64, decimals int) float64 {
	factor := math.Pow10(decimals)
	return math.Round(value*factor) / factor
}
