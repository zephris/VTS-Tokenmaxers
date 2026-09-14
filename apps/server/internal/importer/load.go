package importer

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"vts-tokenmaxers/apps/server/internal/domain"
)

const (
	broadcastFilename = "broadcast_message_log.csv"
	senderFilename    = "sender_history.csv"
	marvFilename      = "marvs_logs.md"
)

// Load reads either the tracked challenge ZIP or an unpacked dataset directory.
func Load(sourcePath string) (Dataset, error) {
	contents, fingerprint, err := readSource(sourcePath)
	if err != nil {
		return Dataset{}, err
	}
	broadcasts, aggregates, analysisTimestamp, err := parseBroadcasts(contents[broadcastFilename])
	if err != nil {
		return Dataset{}, err
	}
	profiles, err := parseProfiles(contents[senderFilename])
	if err != nil {
		return Dataset{}, err
	}
	anomalies := reconcileProfiles(profiles, aggregates, analysisTimestamp)
	logs, err := parseMarvLogs(contents[marvFilename])
	if err != nil {
		return Dataset{}, err
	}
	return Dataset{
		Broadcasts: broadcasts, Profiles: profiles, MarvLogs: logs,
		AnalysisTimestamp: analysisTimestamp, Fingerprint: fingerprint, Anomalies: anomalies,
	}, nil
}

func readSource(sourcePath string) (map[string][]byte, string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, "", fmt.Errorf("inspect dataset source %s: %w", sourcePath, err)
	}
	if info.IsDir() {
		result := make(map[string][]byte, 3)
		hasher := sha256.New()
		for _, name := range []string{broadcastFilename, senderFilename, marvFilename} {
			contents, err := os.ReadFile(filepath.Join(sourcePath, name))
			if err != nil {
				return nil, "", fmt.Errorf("read dataset file %s: %w", name, err)
			}
			result[name] = contents
			_, _ = hasher.Write([]byte(name))
			_, _ = hasher.Write(contents)
		}
		return result, fmt.Sprintf("%x", hasher.Sum(nil)), nil
	}

	archiveBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, "", fmt.Errorf("read dataset archive: %w", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, "", fmt.Errorf("open dataset archive: %w", err)
	}
	result := make(map[string][]byte, 3)
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if base != broadcastFilename && base != senderFilename && base != marvFilename {
			continue
		}
		if strings.Contains(file.Name, "__MACOSX/") {
			continue
		}
		stream, err := file.Open()
		if err != nil {
			return nil, "", fmt.Errorf("open %s in archive: %w", base, err)
		}
		contents, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, "", fmt.Errorf("read %s in archive: %w", base, readErr)
		}
		if closeErr != nil {
			return nil, "", fmt.Errorf("close %s in archive: %w", base, closeErr)
		}
		result[base] = contents
	}
	for _, required := range []string{broadcastFilename, senderFilename, marvFilename} {
		if len(result[required]) == 0 {
			return nil, "", fmt.Errorf("dataset archive does not contain %s", required)
		}
	}
	sum := sha256.Sum256(archiveBytes)
	return result, fmt.Sprintf("%x", sum), nil
}

type senderAggregate struct {
	FirstTimestamp string
	LastTimestamp  string
	Total          int
	Verified       int
	Disputed       int
}

func parseBroadcasts(contents []byte) ([]BroadcastRecord, map[string]senderAggregate, string, error) {
	reader := csv.NewReader(bytes.NewReader(contents))
	header, err := reader.Read()
	if err != nil {
		return nil, nil, "", fmt.Errorf("read broadcast header: %w", err)
	}
	want := []string{"broadcast_id", "timestamp", "sender_id", "location", "broadcast_type", "message_text", "signal_strength", "cross_check_status", "label"}
	if err := requireHeader(header, want); err != nil {
		return nil, nil, "", fmt.Errorf("broadcast dataset: %w", err)
	}

	items := make([]BroadcastRecord, 0, 300)
	aggregates := make(map[string]senderAggregate)
	seen := make(map[string]struct{})
	analysisTimestamp := ""
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, "", fmt.Errorf("read broadcast line %d: %w", line, err)
		}
		if len(record) != len(want) {
			return nil, nil, "", fmt.Errorf("broadcast line %d has %d columns; want %d", line, len(record), len(want))
		}
		for _, index := range []int{0, 1, 2, 3, 4, 7, 8} {
			if strings.TrimSpace(record[index]) == "" {
				return nil, nil, "", fmt.Errorf("broadcast line %d has empty %s", line, want[index])
			}
		}
		if _, exists := seen[record[0]]; exists {
			return nil, nil, "", fmt.Errorf("broadcast line %d duplicates id %s", line, record[0])
		}
		seen[record[0]] = struct{}{}
		if _, err := time.Parse(TimestampLayout, record[1]); err != nil {
			return nil, nil, "", fmt.Errorf("broadcast line %d has invalid timestamp: %w", line, err)
		}
		kind := domain.BroadcastType(record[4])
		if !validBroadcastType(kind) {
			return nil, nil, "", fmt.Errorf("broadcast line %d has invalid type %q", line, kind)
		}
		crossCheck := domain.CrossCheckStatus(record[7])
		if !validCrossCheck(crossCheck) {
			return nil, nil, "", fmt.Errorf("broadcast line %d has invalid cross-check status %q", line, crossCheck)
		}
		if !validLabel(record[8]) {
			return nil, nil, "", fmt.Errorf("broadcast line %d has invalid private label %q", line, record[8])
		}
		item := BroadcastRecord{
			ID: record[0], Timestamp: record[1], SenderID: record[2], Location: record[3],
			Type: kind, MessageText: optionalString(record[5]), CrossCheckStatus: crossCheck,
			GroundTruthLabel: record[8],
		}
		if record[6] != "" {
			signal, err := strconv.Atoi(record[6])
			if err != nil || signal < 0 || signal > 100 {
				return nil, nil, "", fmt.Errorf("broadcast line %d has invalid signal strength %q", line, record[6])
			}
			item.SignalStrength = &signal
		}
		items = append(items, item)
		aggregate := aggregates[item.SenderID]
		aggregate.Total++
		if aggregate.FirstTimestamp == "" || item.Timestamp < aggregate.FirstTimestamp {
			aggregate.FirstTimestamp = item.Timestamp
		}
		if item.Timestamp > aggregate.LastTimestamp {
			aggregate.LastTimestamp = item.Timestamp
		}
		if crossCheck == domain.CrossCheckVerified {
			aggregate.Verified++
		}
		if crossCheck == domain.CrossCheckDisputed {
			aggregate.Disputed++
		}
		aggregates[item.SenderID] = aggregate
		if item.Timestamp > analysisTimestamp {
			analysisTimestamp = item.Timestamp
		}
	}
	if len(items) == 0 {
		return nil, nil, "", errors.New("broadcast dataset is empty")
	}
	return items, aggregates, analysisTimestamp, nil
}

func parseProfiles(contents []byte) (map[string]SenderProfile, error) {
	reader := csv.NewReader(bytes.NewReader(contents))
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read sender header: %w", err)
	}
	want := []string{"sender_id", "sender_type", "first_seen", "last_seen", "total_broadcasts", "broadcasts_verified_accurate", "broadcasts_verified_false", "reliability_score", "current_status"}
	if err := requireHeader(header, want); err != nil {
		return nil, fmt.Errorf("sender dataset: %w", err)
	}
	profiles := make(map[string]SenderProfile)
	for line := 2; ; line++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read sender line %d: %w", line, err)
		}
		if len(record) != len(want) {
			return nil, fmt.Errorf("sender line %d has %d columns; want %d", line, len(record), len(want))
		}
		if _, exists := profiles[record[0]]; exists {
			return nil, fmt.Errorf("sender line %d duplicates id %s", line, record[0])
		}
		values := make([]int, 5)
		for offset, index := range []int{4, 5, 6, 7} {
			value, err := strconv.Atoi(record[index])
			if err != nil || value < 0 {
				return nil, fmt.Errorf("sender line %d has invalid %s %q", line, want[index], record[index])
			}
			values[offset] = value
		}
		if values[3] > 100 {
			return nil, fmt.Errorf("sender line %d reliability exceeds 100", line)
		}
		status := domain.SenderStatus(record[8])
		if status != domain.SenderActive && status != domain.SenderGoneQuiet && status != domain.SenderSuspectedCompromised {
			return nil, fmt.Errorf("sender line %d has invalid status %q", line, status)
		}
		profile := SenderProfile{
			ID: record[0], Type: record[1], FirstSeen: record[2], LastSeen: record[3],
			ReportedTotal: intPtr(values[0]), VerifiedAccurate: intPtr(values[1]),
			VerifiedFalse: intPtr(values[2]), ReliabilityScore: intPtr(values[3]),
			CurrentStatus: status, ProfileSource: domain.ProfileDataset,
		}
		profiles[profile.ID] = profile
	}
	return profiles, nil
}

func reconcileProfiles(profiles map[string]SenderProfile, aggregates map[string]senderAggregate, analysisTimestamp string) []domain.ImportAnomaly {
	anomalies := make([]domain.ImportAnomaly, 0)
	ids := make([]string, 0, len(aggregates))
	for id := range aggregates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		aggregate := aggregates[id]
		profile, exists := profiles[id]
		if !exists {
			profile = deriveProfile(id, aggregate, analysisTimestamp)
			profiles[id] = profile
			anomalies = append(anomalies, domain.ImportAnomaly{
				Code: "missing_sender_history", Severity: "warning", Source: senderFilename, RecordID: id,
				Message: "sender is present in the broadcast log but missing from sender history; profile was derived from observable records",
			})
			continue
		}
		if profile.ReportedTotal != nil && *profile.ReportedTotal != aggregate.Total {
			anomalies = append(anomalies, domain.ImportAnomaly{
				Code: "sender_history_count_mismatch", Severity: "warning", Source: senderFilename, RecordID: id,
				Message: fmt.Sprintf("sender history reports %d broadcasts but the authoritative log contains %d", *profile.ReportedTotal, aggregate.Total),
			})
		}
	}
	return anomalies
}

func deriveProfile(id string, aggregate senderAggregate, analysisTimestamp string) SenderProfile {
	first := strings.Split(aggregate.FirstTimestamp, " ")[0]
	last := strings.Split(aggregate.LastTimestamp, " ")[0]
	status := domain.SenderActive
	analysis, analysisErr := time.Parse(TimestampLayout, analysisTimestamp)
	latest, latestErr := time.Parse(TimestampLayout, aggregate.LastTimestamp)
	if analysisErr == nil && latestErr == nil && analysis.Sub(latest) >= 72*time.Hour {
		status = domain.SenderGoneQuiet
	}
	assessed := aggregate.Verified + aggregate.Disputed
	var reliability *int
	if assessed > 0 {
		score := int(float64(aggregate.Verified)*100/float64(assessed) + 0.5)
		reliability = &score
	}
	return SenderProfile{
		ID: id, Type: inferSenderType(id), FirstSeen: first, LastSeen: last,
		ReportedTotal: intPtr(aggregate.Total), VerifiedAccurate: intPtr(aggregate.Verified),
		VerifiedFalse: intPtr(aggregate.Disputed), ReliabilityScore: reliability,
		CurrentStatus: status, ProfileSource: domain.ProfileDerived,
	}
}

func parseMarvLogs(contents []byte) ([]MarvLog, error) {
	lines := strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
	logs := make([]MarvLog, 0, 8)
	var current *MarvLog
	for _, line := range lines {
		if strings.HasPrefix(line, "## Entry ") {
			if current != nil {
				current.Body = strings.TrimSpace(current.Body)
				logs = append(logs, *current)
			}
			heading := strings.TrimPrefix(line, "## Entry ")
			parts := strings.SplitN(heading, " — ", 2)
			number, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil || len(parts) != 2 {
				return nil, fmt.Errorf("invalid Marv log heading %q", line)
			}
			current = &MarvLog{EntryNumber: number, Title: strings.TrimSpace(parts[1])}
			continue
		}
		if current == nil || strings.Trim(line, "-") == "" {
			continue
		}
		current.Body += line + "\n"
	}
	if current != nil {
		current.Body = strings.TrimSpace(current.Body)
		logs = append(logs, *current)
	}
	if len(logs) == 0 {
		return nil, errors.New("Marv logs contain no entries")
	}
	return logs, nil
}

func requireHeader(got, want []string) error {
	if len(got) != len(want) {
		return fmt.Errorf("header has %d columns; want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			return fmt.Errorf("column %d is %q; want %q", index+1, got[index], want[index])
		}
	}
	return nil
}

func validBroadcastType(value domain.BroadcastType) bool {
	switch value {
	case domain.BroadcastAllClear, domain.BroadcastWarning, domain.BroadcastSupplyRequest,
		domain.BroadcastSituation, domain.BroadcastRoutineCheck, domain.BroadcastEmergency:
		return true
	default:
		return false
	}
}

func validCrossCheck(value domain.CrossCheckStatus) bool {
	switch value {
	case domain.CrossCheckVerified, domain.CrossCheckDisputed, domain.CrossCheckUnconfirmed, domain.CrossCheckNotChecked:
		return true
	default:
		return false
	}
}

func validLabel(value string) bool {
	return value == "genuine" || value == "peacock_spoofed" || value == "outdated" || value == "unknown"
}

func inferSenderType(id string) string {
	switch {
	case strings.HasPrefix(id, "Outpost-"):
		return "robot_outpost"
	case strings.HasPrefix(id, "Mini-Marv-"):
		return "junior_scout_group"
	default:
		return "relay_identity"
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intPtr(value int) *int { return &value }
