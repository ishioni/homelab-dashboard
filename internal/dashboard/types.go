package dashboard

import "time"

type ViewModel struct {
	AppName        string
	AppVersion     string
	ClusterName    string
	PageTitle      string
	Screen         string
	DemoMode       bool
	GeneratedAt    time.Time
	RefreshSeconds int
	Navigation     []NavItem
	Banner         Banner
	Errors         []string
	Hub            *HubView
	Security       *SecurityView
	Anomalies      *AnomaliesView
	Forecast       *ForecastView
}

type NavItem struct {
	Label  string
	Path   string
	Icon   string
	Active bool
}

type Banner struct {
	Label   string
	Detail  string
	Tone    string
	Actions []Action
}

type Action struct {
	Label string
	Path  string
}

type HubView struct {
	Headlines     []string
	SummaryCards  []StatCard
	Utilization   []UsageMeter
	HealthScore   int
	HealthTone    string
	Signals       []AnomalySignal
	Paths         []HubPath
	TopCPU        []ResourceStat
	TopMemory     []ResourceStat
	WarningEvents []EventRow
}

type SecurityView struct {
	SummaryCards        []StatCard
	FluxCards           []StatCard
	FluxKinds           []KindStatus
	FluxRecent          []FluxRecentRow
	CNPGCards           []StatCard
	CNPGClusters        []CNPGClusterRow
	VolSyncCards        []StatCard
	VolSyncSources      []VolSyncSourceRow
	ExternalSecretCards []StatCard
	ExternalSecrets     []ExternalSecretRow
	EnvoyCards          []StatCard
	EnvoyRoutes         []EnvoyRouteRow
	ToolhiveCards       []StatCard
	ToolhiveBackends    []ToolhiveBackendRow
	RenovateCards       []StatCard
	RenovateProjects    []RenovateProjectRow
	SlowReconciles      []ResourceStat
	WarningEvents       []EventRow
}

type AnomaliesView struct {
	SummaryCards  []StatCard
	Chart         AnomalyChart
	Events        []AnomalyEvent
	DomainCards   []StatCard
	Hotspots      []ResourceStat
	Actions       []Action
	WarningEvents []EventRow
}

type ForecastView struct {
	ForecastCards []ForecastCard
	Series        []SparklineCard
}

type AnomalyChart struct {
	Series []ChartSeries
	Labels []string
	Scale  []string
}

type ChartSeries struct {
	Label string
	Path  string
	Tone  string
	Value string
}

type AnomalyEvent struct {
	Label    string
	Resource string
	Detail   string
	Meta     string
	Icon     string
	Tone     string
}

type StatCard struct {
	Label  string
	Value  string
	Detail string
	Tone   string
}

type UsageMeter struct {
	Label   string
	Value   float64
	Display string
	Detail  string
	Tone    string
}

type ResourceStat struct {
	Name   string
	Value  string
	Detail string
	Tone   string
}

type HubPath struct {
	Label  string
	Icon   string
	Value  string
	Detail string
	Path   string
	Tone   string
}

type EventRow struct {
	When      time.Time
	Namespace string
	Reason    string
	Object    string
	Message   string
}

type KindStatus struct {
	Kind      string
	Ready     int
	NotReady  int
	Suspended int
	Total     int
	Status    string
	Tone      string
}

type SecurityStatusRow struct {
	Icon   string
	Name   string
	State  string
	Detail string
	Meta   string
	Tone   string
}

type FluxRecentRow struct {
	Kind      string
	Name      string
	Namespace string
	Status    string
	Age       string
	Tone      string
}

type CNPGClusterRow struct {
	Name       string
	Namespace  string
	Detail     string
	Replicas   string
	Lag        string
	Backup     string
	Size       string
	Tone       string
	BackupTone string

	SizeBytes              float64
	WALQueue               float64
	SecondsSinceArchival   float64
	HasRecentBackupFailure bool
}

type VolSyncSourceRow struct {
	Name                    string
	Namespace               string
	Detail                  string
	Schedule                string
	LastSync                string
	Duration                string
	Status                  string
	Tone                    string
	NextSync                time.Time
	LastSyncAt              time.Time
	LastSyncDurationSeconds float64
}

type ExternalSecretRow struct {
	Name      string
	Namespace string
	Store     string
	Refresh   string
	Interval  string
	Status    string
	Detail    string
	Tone      string

	RefreshAt time.Time
	Errors24h float64
}

type EnvoyRouteRow struct {
	Name        string
	Namespace   string
	Route       string
	RequestRate string
	ErrorRate   string
	Latency     string
	Status      string
	Detail      string
	Tone        string

	RequestRateValue float64
	ErrorRateValue   float64
	LatencyValue     float64
}

type ToolhiveBackendRow struct {
	Name        string
	Requests24h string
	Errors24h   string
	Latency     string
	Status      string
	Detail      string
	Tone        string

	Requests24hValue float64
	Errors24hValue   float64
	LatencyValue     float64
}

type RenovateProjectRow struct {
	Name          string
	Executions24h string
	Issues        string
	Status        string
	Detail        string
	Tone          string

	Executions24hValue float64
	IssueCount         float64
	Failed             float64
}

type AnomalySignal struct {
	Category string
	Severity string
	Signal   string
	Resource string
	Value    string
	Window   string
	Details  string
}

type ForecastCard struct {
	Label      string
	Current    string
	Projection string
	Trend      string
	Tone       string
}

type SparklineCard struct {
	Label  string
	Path   string
	Latest string
	Delta  string
	Detail string
	Tone   string
	Scale  []string
}
