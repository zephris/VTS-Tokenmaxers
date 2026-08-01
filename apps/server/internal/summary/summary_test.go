package summary

import (
	"context"
	"strings"
	"testing"

	"vts-tokenmaxers/apps/server/internal/domain"
)

func TestStubAnalyzerCitesEvidence(t *testing.T) {
	message := "Outpost did not respond."
	evidence := []domain.Broadcast{{
		ID: "BC-019", Timestamp: "2026-07-16 10:05", SenderID: "Mini-Marv-01",
		Location: "Guild Village", Type: domain.BroadcastEmergency, MessageText: &message,
	}}

	result, err := (StubAnalyzer{}).Analyze(context.Background(), "Outpost-Delta", evidence)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "fallback" || len(result.EvidenceIDs) != 1 || result.EvidenceIDs[0] != "BC-019" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !strings.Contains(result.Summary, "BC-019") {
		t.Fatalf("summary does not cite evidence: %q", result.Summary)
	}
}

func TestStubAnalyzerHandlesUnknownSender(t *testing.T) {
	result, err := (StubAnalyzer{}).Analyze(context.Background(), "Unknown", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.EvidenceIDs) != 0 || !strings.Contains(result.Summary, "No evidence") {
		t.Fatalf("unexpected result: %#v", result)
	}
}
