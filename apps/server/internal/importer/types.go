package importer

import (
	"vts-tokenmaxers/apps/server/internal/domain"
)

const TimestampLayout = "2006-01-02 15:04"

type BroadcastRecord struct {
	ID               string
	Timestamp        string
	SenderID         string
	Location         string
	Type             domain.BroadcastType
	MessageText      *string
	SignalStrength   *int
	CrossCheckStatus domain.CrossCheckStatus
	GroundTruthLabel string
}

type SenderProfile struct {
	ID               string
	Type             string
	FirstSeen        string
	LastSeen         string
	ReportedTotal    *int
	VerifiedAccurate *int
	VerifiedFalse    *int
	ReliabilityScore *int
	CurrentStatus    domain.SenderStatus
	ProfileSource    domain.ProfileSource
}

type MarvLog struct {
	EntryNumber int
	Title       string
	Body        string
}

type Dataset struct {
	Broadcasts        []BroadcastRecord
	Profiles          map[string]SenderProfile
	MarvLogs          []MarvLog
	AnalysisTimestamp string
	Fingerprint       string
	Anomalies         []domain.ImportAnomaly
}
