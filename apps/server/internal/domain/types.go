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

type Broadcast struct {
	ID               string           `json:"id"`
	Timestamp        string           `json:"timestamp"`
	SenderID         string           `json:"senderId"`
	Location         string           `json:"location"`
	Type             BroadcastType    `json:"type"`
	MessageText      *string          `json:"messageText"`
	SignalStrength   *int             `json:"signalStrength"`
	CrossCheckStatus CrossCheckStatus `json:"crossCheckStatus"`
	Label            string           `json:"-"`
}

type OutpostSummary struct {
	SenderID         string       `json:"senderId"`
	Location         string       `json:"location"`
	LastSeen         string       `json:"lastSeen"`
	HoursSilent      float64      `json:"hoursSilent"`
	ReliabilityScore int          `json:"reliabilityScore"`
	Status           SenderStatus `json:"status"`
	RiskLevel        RiskLevel    `json:"riskLevel"`
}

type DashboardResponse struct {
	AnalysisTimestamp string           `json:"analysisTimestamp"`
	Outposts          []OutpostSummary `json:"outposts"`
	RecentBroadcasts  []Broadcast      `json:"recentBroadcasts"`
}

type IncidentSummaryResponse struct {
	SenderID    string   `json:"senderId"`
	Summary     string   `json:"summary"`
	Source      string   `json:"source"`
	EvidenceIDs []string `json:"evidenceIds"`
}
