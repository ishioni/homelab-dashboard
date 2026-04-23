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
	TopCPU        []ResourceStat
	TopMemory     []ResourceStat
	WarningEvents []EventRow
}

type SecurityView struct {
	SummaryCards   []StatCard
	FluxCards      []StatCard
	FluxKinds      []KindStatus
	FluxRecent     []FluxRecentRow
	OperatorRows   []SecurityStatusRow
	SlowReconciles []ResourceStat
	WarningEvents  []EventRow
}

type AnomaliesView struct {
	SummaryCards []StatCard
	Signals      []AnomalySignal
	Timeline     []SparklineCard
}

type ForecastView struct {
	ForecastCards []ForecastCard
	Series        []SparklineCard
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
}
