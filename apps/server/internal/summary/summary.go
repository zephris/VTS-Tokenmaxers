package summary

import (
	"context"
	"fmt"

	"vts-tokenmaxers/apps/server/internal/domain"
)

type Analyzer interface {
	Analyze(context.Context, string, []domain.Broadcast) (domain.IncidentSummaryResponse, error)
}

type StubAnalyzer struct{}

func (StubAnalyzer) Analyze(_ context.Context, senderID string, evidence []domain.Broadcast) (domain.IncidentSummaryResponse, error) {
	return domain.IncidentSummaryResponse{
		SenderID:    senderID,
		Source:      "fallback",
		EvidenceIDs: evidenceIDs(evidence),
		Summary:     fallbackText(senderID, evidence),
	}, nil
}

func fallbackText(senderID string, evidence []domain.Broadcast) string {
	if len(evidence) == 0 {
		return fmt.Sprintf("No evidence was found for %s. Verify the sender identity and data import.", senderID)
	}

	latest := evidence[len(evidence)-1]
	emergencies := 0
	for _, item := range evidence {
		if item.Type == domain.BroadcastEmergency {
			emergencies++
		}
	}

	return fmt.Sprintf(
		"%s needs review. %d related emergency broadcast(s) are present, and the latest evidence is %s from %s. Dispatch a scout before trusting any unverified all-clear.",
		senderID,
		emergencies,
		latest.ID,
		latest.Timestamp,
	)
}

func evidenceIDs(evidence []domain.Broadcast) []string {
	ids := make([]string, 0, len(evidence))
	for _, item := range evidence {
		ids = append(ids, item.ID)
	}
	return ids
}
