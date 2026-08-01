package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vts-tokenmaxers/apps/server/internal/domain"
)

func TestRealDatasetImportIsCompleteAndIdempotent(t *testing.T) {
	ctx := context.Background()
	dataStore, err := OpenDatabase(ctx, filepath.Join(t.TempDir(), "outposts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })

	for run := 1; run <= 2; run++ {
		stats, err := dataStore.ImportDataset(ctx, realArchive(t))
		if err != nil {
			t.Fatalf("import run %d: %v", run, err)
		}
		if stats.StationCount != 14 || stats.BroadcastCount != 300 {
			t.Fatalf("import run %d counts = %#v; want 14 stations and 300 broadcasts", run, stats)
		}
	}

	stations, err := dataStore.Stations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stations.AnalysisTimestamp != "2026-11-20 09:57" || len(stations.Stations) != 14 {
		t.Fatalf("stations response = timestamp %q, count %d", stations.AnalysisTimestamp, len(stations.Stations))
	}
	byID := make(map[string]domain.StationSummary, len(stations.Stations))
	derivedCount := 0
	for _, station := range stations.Stations {
		byID[station.SenderID] = station
		if station.ProfileSource == domain.ProfileDerived {
			derivedCount++
		}
	}
	if derivedCount != 5 {
		t.Fatalf("derived station count = %d; want 5", derivedCount)
	}
	assertStation(t, byID["Outpost-Alpha"], "robot_outpost", domain.ProfileDataset)
	assertStation(t, byID["Outpost-Epsilon"], "robot_outpost", domain.ProfileDerived)
	assertStation(t, byID["Mini-Marv-04"], "junior_scout_group", domain.ProfileDerived)
	if byID["Outpost-Epsilon"].ReliabilityScore == nil {
		t.Fatal("derived reliability score is nil despite assessed labels")
	}
}

func TestOpenDatabaseRebuildsLegacyDatasetSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "outposts.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`
CREATE TABLE senders (
    sender_id TEXT PRIMARY KEY,
    sender_type TEXT NOT NULL,
    location TEXT NOT NULL,
    reliability_score INTEGER NOT NULL,
    current_status TEXT NOT NULL
);
CREATE TABLE broadcasts (
    broadcast_id TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL,
    sender_id TEXT NOT NULL REFERENCES senders(sender_id),
    location TEXT NOT NULL,
    broadcast_type TEXT NOT NULL,
    message_text TEXT,
    signal_strength INTEGER,
    cross_check_status TEXT NOT NULL,
    label TEXT NOT NULL
);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	dataStore, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	stats, err := dataStore.ImportDataset(ctx, realArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	if stats.StationCount != 14 || stats.BroadcastCount != 300 {
		t.Fatalf("import stats = %#v", stats)
	}
}

func TestImportedBroadcastsPreserveCSVDataAndPlainState(t *testing.T) {
	dataStore := openRealDataset(t)
	ctx := context.Background()

	response, err := dataStore.StationBroadcasts(ctx, "Marv Mail")
	if err != nil {
		t.Fatal(err)
	}
	blank := findBroadcast(t, response.Broadcasts, "BC-026")
	if blank.MessageText != nil {
		t.Fatalf("BC-026 message = %q; want null", *blank.MessageText)
	}
	if blank.CarrierText != nil || blank.EncryptionStatus != domain.EncryptionPlain || blank.Label != "" {
		t.Fatalf("BC-026 encryption fields = %#v", blank)
	}
	encoded, err := json.Marshal(blank)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "label") || strings.Contains(string(encoded), "unknown") {
		t.Fatalf("operational broadcast leaked ground truth: %s", encoded)
	}

	scouts, err := dataStore.StationBroadcasts(ctx, "Mini-Marv-01")
	if err != nil {
		t.Fatal(err)
	}
	quoted := findBroadcast(t, scouts.Broadcasts, "BC-004")
	want := "Cross-checked Outpost-Beta's report with eyewitness scouts. Matches."
	if quoted.MessageText == nil || *quoted.MessageText != want {
		t.Fatalf("quoted CSV message = %#v; want %q", quoted.MessageText, want)
	}
	missingSignal := findBroadcast(t, scouts.Broadcasts, "BC-019")
	if missingSignal.SignalStrength != nil {
		t.Fatalf("BC-019 signal = %d; want null", *missingSignal.SignalStrength)
	}
}

func TestDashboardAndIncidentEvidenceQuerySQLite(t *testing.T) {
	dataStore := openRealDataset(t)
	ctx := context.Background()
	dashboard, err := dataStore.Dashboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.AnalysisTimestamp != "2026-11-20 09:57" || len(dashboard.Outposts) != 7 || len(dashboard.RecentBroadcasts) != 20 {
		t.Fatalf("unexpected dashboard: timestamp=%q outposts=%d recent=%d", dashboard.AnalysisTimestamp, len(dashboard.Outposts), len(dashboard.RecentBroadcasts))
	}
	if dashboard.RecentBroadcasts[0].ID != "BC-300" {
		t.Fatalf("latest broadcast = %s; want BC-300", dashboard.RecentBroadcasts[0].ID)
	}
	delta, err := dataStore.Outpost(ctx, "Outpost-Delta")
	if err != nil {
		t.Fatal(err)
	}
	if delta.Outpost.ExpectedCadenceHours != 24 || delta.Outpost.SilenceRatio < 100 || delta.Outpost.RiskLevel != domain.RiskCritical {
		t.Fatalf("Delta risk metrics = %#v", delta.Outpost)
	}
	if delta.Outpost.UnresolvedEmergencyCount != 2 || delta.Outpost.ContradictionCount != 3 || len(delta.Outpost.RiskReasons) == 0 {
		t.Fatalf("Delta evidence metrics = %#v", delta.Outpost)
	}
	filtered, err := dataStore.Broadcasts(ctx, domain.BroadcastFilter{
		SenderID: "Outpost-Delta", Type: domain.BroadcastRoutineCheck, Page: 1, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Pagination.Total != 2 || len(filtered.Broadcasts) != 1 || filtered.Pagination.Limit != 1 {
		t.Fatalf("filtered broadcasts = %#v", filtered)
	}

	evidence, err := dataStore.IncidentEvidence(ctx, "Outpost-Delta")
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) == 0 {
		t.Fatal("incident evidence is empty")
	}
	for index := 1; index < len(evidence); index++ {
		if evidence[index].Timestamp < evidence[index-1].Timestamp {
			t.Fatalf("evidence is not chronological at index %d", index)
		}
	}
	missing, err := dataStore.IncidentEvidence(ctx, "missing-station")
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing evidence = %d rows, %v", len(missing), err)
	}
}

func TestStationBroadcastsReturnsNotFound(t *testing.T) {
	dataStore := openRealDataset(t)
	_, err := dataStore.StationBroadcasts(context.Background(), "missing-station")
	if !errors.Is(err, ErrStationNotFound) {
		t.Fatalf("error = %v; want ErrStationNotFound", err)
	}
}

func openRealDataset(t *testing.T) *Store {
	t.Helper()
	dataStore, stats, err := Open(context.Background(), filepath.Join(t.TempDir(), "outposts.db"), realArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	if stats.StationCount != 14 || stats.BroadcastCount != 300 {
		t.Fatalf("unexpected import stats: %#v", stats)
	}
	t.Cleanup(func() { _ = dataStore.Close() })
	return dataStore
}

func realArchive(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate store test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "../../data/source/sunken-garden-and-cs-building.zip"))
}

func assertStation(t *testing.T, station domain.StationSummary, wantType string, wantSource domain.ProfileSource) {
	t.Helper()
	if station.SenderID == "" || station.SenderType != wantType || station.ProfileSource != wantSource {
		t.Fatalf("station = %#v; want type %s and source %s", station, wantType, wantSource)
	}
	if station.BroadcastCount == 0 || station.LastBroadcastAt == "" || station.Location == "" {
		t.Fatalf("station lacks broadcast summary: %#v", station)
	}
}

func findBroadcast(t *testing.T, broadcasts []domain.Broadcast, id string) domain.Broadcast {
	t.Helper()
	for _, item := range broadcasts {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("broadcast %s not found", id)
	return domain.Broadcast{}
}
