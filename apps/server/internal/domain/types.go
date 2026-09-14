package domain

type BroadcastType string

const (
	BroadcastAllClear      BroadcastType = "all_clear"
	BroadcastWarning       BroadcastType = "warning"
	BroadcastSupplyRequest BroadcastType = "supply_request"
	BroadcastSituation     BroadcastType = "situation_report"
	BroadcastRoutineCheck  BroadcastType = "routine_check"
	BroadcastEmergency     BroadcastType = "emergency"
)

type CrossCheckStatus string

const (
	CrossCheckVerified    CrossCheckStatus = "verified"
	CrossCheckDisputed    CrossCheckStatus = "disputed"
	CrossCheckUnconfirmed CrossCheckStatus = "unconfirmed"
	CrossCheckNotChecked  CrossCheckStatus = "not_checked"
)

type SenderStatus string

const (
	SenderActive               SenderStatus = "active"
	SenderGoneQuiet            SenderStatus = "gone_quiet"
	SenderSuspectedCompromised SenderStatus = "suspected_compromised"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type EncryptionStatus string

const (
	EncryptionPlain     EncryptionStatus = "plain"
	EncryptionEncrypted EncryptionStatus = "encrypted"
)

type ProfileSource string

const (
	ProfileDataset ProfileSource = "dataset"
	ProfileDerived ProfileSource = "derived"
)

type Broadcast struct {
	ID               string           `json:"id"`
	Timestamp        string           `json:"timestamp"`
	SenderID         string           `json:"senderId"`
	Location         string           `json:"location"`
	Type             BroadcastType    `json:"type"`
	MessageText      *string          `json:"messageText"`
	SignalStrength   *int             `json:"signalStrength"`
	CrossCheckStatus CrossCheckStatus `json:"crossCheckStatus"`
	// Label is retained only for trusted import/test tooling. Operational JSON
	// must never reveal the archive's retrospective ground-truth assessment.
	Label            string           `json:"-"`
	CarrierText      *string          `json:"carrierText"`
	EncryptionStatus EncryptionStatus `json:"encryptionStatus"`
}

type StationSummary struct {
	SenderID           string        `json:"senderId"`
	SenderType         string        `json:"senderType"`
	Location           string        `json:"location"`
	LastBroadcastAt    string        `json:"lastBroadcastAt"`
	LastMessagePreview *string       `json:"lastMessagePreview"`
	BroadcastCount     int           `json:"broadcastCount"`
	ReliabilityScore   *int          `json:"reliabilityScore"`
	Status             SenderStatus  `json:"status"`
	ProfileSource      ProfileSource `json:"profileSource"`
}

type StationsResponse struct {
	AnalysisTimestamp string           `json:"analysisTimestamp"`
	Stations          []StationSummary `json:"stations"`
}

type StationBroadcastsResponse struct {
	Station    StationSummary `json:"station"`
	Broadcasts []Broadcast    `json:"broadcasts"`
}

type OutpostSummary struct {
	SenderID                 string       `json:"senderId"`
	Location                 string       `json:"location"`
	LastSeen                 string       `json:"lastSeen"`
	HoursSilent              float64      `json:"hoursSilent"`
	ExpectedCadenceHours     float64      `json:"expectedCadenceHours"`
	SilenceRatio             float64      `json:"silenceRatio"`
	ReliabilityScore         int          `json:"reliabilityScore"`
	Status                   SenderStatus `json:"status"`
	UnresolvedEmergencyCount int          `json:"unresolvedEmergencyCount"`
	ContradictionCount       int          `json:"contradictionCount"`
	RiskLevel                RiskLevel    `json:"riskLevel"`
	RiskScore                int          `json:"riskScore"`
	RiskReasons              []RiskReason `json:"riskReasons"`
	BroadcastCount           int          `json:"broadcastCount"`
}

type RiskReason struct {
	Code        string    `json:"code"`
	Severity    RiskLevel `json:"severity"`
	Message     string    `json:"message"`
	EvidenceIDs []string  `json:"evidenceIds"`
}

type DashboardResponse struct {
	AnalysisTimestamp string           `json:"analysisTimestamp"`
	Outposts          []OutpostSummary `json:"outposts"`
	RecentBroadcasts  []Broadcast      `json:"recentBroadcasts"`
}

type BroadcastFilter struct {
	SenderID         string
	Location         string
	Type             BroadcastType
	CrossCheckStatus CrossCheckStatus
	Query            string
	Page             int
	Limit            int
}

type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

type BroadcastsResponse struct {
	Broadcasts []Broadcast `json:"broadcasts"`
	Pagination Pagination  `json:"pagination"`
}

type OutpostDetailResponse struct {
	AnalysisTimestamp string         `json:"analysisTimestamp"`
	Outpost           OutpostSummary `json:"outpost"`
	EvidenceTimeline  []Broadcast    `json:"evidenceTimeline"`
}

type IncidentSummaryResponse struct {
	SenderID    string   `json:"senderId"`
	Summary     string   `json:"summary"`
	Source      string   `json:"source"`
	EvidenceIDs []string `json:"evidenceIds"`
}

type ImportStats struct {
	StationCount   int `json:"stationCount"`
	BroadcastCount int `json:"broadcastCount"`
	MarvLogCount   int `json:"marvLogCount"`
	AnomalyCount   int `json:"anomalyCount"`
}

type ImportAnomaly struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
	RecordID string `json:"recordId,omitempty"`
	Message  string `json:"message"`
}

type ImportReport struct {
	ImportStats
	SourceFingerprint string          `json:"sourceFingerprint"`
	Anomalies         []ImportAnomaly `json:"anomalies"`
}
