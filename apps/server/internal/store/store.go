package store

import (
	"math"
	"sort"
	"time"

	"vts-tokenmaxers/apps/server/internal/domain"
)

type Sender struct {
	ID               string
	Type             string
	Location         string
	ReliabilityScore int
	CurrentStatus    domain.SenderStatus
}

type Store struct {
	senders    []Sender
	broadcasts []domain.Broadcast
}

const timestampLayout = "2006-01-02 15:04"

func NewSeeded() *Store {
	return &Store{
		senders: []Sender{
			{ID: "Outpost-Alpha", Type: "robot_outpost", Location: "Sunken Garden", ReliabilityScore: 92, CurrentStatus: domain.SenderActive},
			{ID: "Outpost-Beta", Type: "robot_outpost", Location: "Robotics Workshop", ReliabilityScore: 85, CurrentStatus: domain.SenderActive},
			{ID: "Outpost-Gamma", Type: "robot_outpost", Location: "Prescott Court", ReliabilityScore: 88, CurrentStatus: domain.SenderActive},
			{ID: "Outpost-Delta", Type: "robot_outpost", Location: "Guild Village", ReliabilityScore: 90, CurrentStatus: domain.SenderGoneQuiet},
			{ID: "Mini-Marv-01", Type: "junior_scout_group", Location: "Mobile", ReliabilityScore: 88, CurrentStatus: domain.SenderActive},
			{ID: "New Meridian", Type: "relay_identity", Location: "Unknown", ReliabilityScore: 5, CurrentStatus: domain.SenderSuspectedCompromised},
		},
		broadcasts: []domain.Broadcast{
			broadcast("BC-003", "2026-07-10 14:05", "Outpost-Delta", "Guild Village", domain.BroadcastRoutineCheck, text("Guild Village quiet. Supplies stable."), intPtr(55), domain.CrossCheckVerified, "genuine"),
			broadcast("BC-010", "2026-07-13 06:45", "Outpost-Delta", "Guild Village", domain.BroadcastRoutineCheck, text("Guild Village status unchanged. Low peacock sightings."), intPtr(51), domain.CrossCheckVerified, "genuine"),
			broadcast("BC-019", "2026-07-16 10:05", "Mini-Marv-01", "Guild Village", domain.BroadcastEmergency, text("Attempting contact with Outpost-Delta. No response for 48 hours."), nil, domain.CrossCheckUnconfirmed, "genuine"),
			broadcast("BC-021", "2026-07-17 05:50", "Outpost-Gamma", "Prescott Court", domain.BroadcastEmergency, text("Urgent: peacock swarm breaching Prescott Court perimeter."), intPtr(44), domain.CrossCheckNotChecked, "genuine"),
			broadcast("BC-035", "2026-07-22 11:05", "New Meridian", "Guild Village", domain.BroadcastAllClear, text("Guild Village confirmed clear. Outpost-Delta concerns overstated."), intPtr(87), domain.CrossCheckDisputed, "peacock_spoofed"),
			broadcast("BC-036", "2026-07-22 12:00", "Mini-Marv-01", "Guild Village", domain.BroadcastEmergency, text("Still no contact with Outpost-Delta. The all-clear cannot be confirmed."), nil, domain.CrossCheckNotChecked, "genuine"),
			broadcast("BC-038", "2026-07-22 12:30", "Outpost-Alpha", "Sunken Garden", domain.BroadcastRoutineCheck, text("Scheduled check-in complete."), intPtr(61), domain.CrossCheckVerified, "genuine"),
			broadcast("BC-039", "2026-07-22 12:45", "Outpost-Beta", "Robotics Workshop", domain.BroadcastRoutineCheck, text("Workshop operational. Repairs continuing."), intPtr(50), domain.CrossCheckVerified, "genuine"),
		},
	}
}

func (s *Store) Dashboard() domain.DashboardResponse {
	analysisTimestamp := s.analysisTimestamp()
	analysisTime := mustParseTime(analysisTimestamp)

	outposts := make([]domain.OutpostSummary, 0, len(s.senders))
	for _, sender := range s.senders {
		if sender.Type != "robot_outpost" {
			continue
		}
		lastSeen := s.lastSeen(sender.ID)
		hoursSilent := hoursBetween(mustParseTime(lastSeen), analysisTime)
		outposts = append(outposts, domain.OutpostSummary{
			SenderID:         sender.ID,
			Location:         sender.Location,
			LastSeen:         lastSeen,
			HoursSilent:      hoursSilent,
			ReliabilityScore: sender.ReliabilityScore,
			Status:           sender.CurrentStatus,
			RiskLevel:        riskFor(sender.CurrentStatus, hoursSilent),
		})
	}

	sort.Slice(outposts, func(i, j int) bool {
		return outposts[i].HoursSilent > outposts[j].HoursSilent
	})

	recent := append([]domain.Broadcast(nil), s.broadcasts...)
	sort.Slice(recent, func(i, j int) bool {
		return recent[i].Timestamp > recent[j].Timestamp
	})
	if len(recent) > 20 {
		recent = recent[:20]
	}

	return domain.DashboardResponse{
		AnalysisTimestamp: analysisTimestamp,
		Outposts:          outposts,
		RecentBroadcasts:  recent,
	}
}

func (s *Store) IncidentEvidence(senderID string) []domain.Broadcast {
	location := ""
	for _, sender := range s.senders {
		if sender.ID == senderID {
			location = sender.Location
			break
		}
	}
	if location == "" {
		return []domain.Broadcast{}
	}

	evidence := make([]domain.Broadcast, 0)
	for _, item := range s.broadcasts {
		if item.SenderID == senderID || item.Location == location {
			evidence = append(evidence, item)
		}
	}
	sort.Slice(evidence, func(i, j int) bool {
		return evidence[i].Timestamp < evidence[j].Timestamp
	})
	if len(evidence) > 30 {
		evidence = evidence[:30]
	}
	return evidence
}

func (s *Store) analysisTimestamp() string {
	maxTimestamp := ""
	for _, item := range s.broadcasts {
		if item.Timestamp > maxTimestamp {
			maxTimestamp = item.Timestamp
		}
	}
	return maxTimestamp
}

func (s *Store) lastSeen(senderID string) string {
	maxTimestamp := ""
	for _, item := range s.broadcasts {
		if item.SenderID == senderID && item.Timestamp > maxTimestamp {
			maxTimestamp = item.Timestamp
		}
	}
	return maxTimestamp
}

func broadcast(id, timestamp, senderID, location string, broadcastType domain.BroadcastType, messageText *string, signalStrength *int, status domain.CrossCheckStatus, label string) domain.Broadcast {
	return domain.Broadcast{
		ID:               id,
		Timestamp:        timestamp,
		SenderID:         senderID,
		Location:         location,
		Type:             broadcastType,
		MessageText:      messageText,
		SignalStrength:   signalStrength,
		CrossCheckStatus: status,
		Label:            label,
	}
}

func text(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func riskFor(status domain.SenderStatus, hoursSilent float64) domain.RiskLevel {
	if status == domain.SenderGoneQuiet || hoursSilent >= 72 {
		return domain.RiskCritical
	}
	if hoursSilent >= 36 {
		return domain.RiskHigh
	}
	if hoursSilent >= 18 {
		return domain.RiskMedium
	}
	return domain.RiskLow
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(timestampLayout, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func hoursBetween(start, end time.Time) float64 {
	return math.Round(end.Sub(start).Hours()*10) / 10
}
