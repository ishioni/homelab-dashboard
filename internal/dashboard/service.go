package dashboard

import (
	"context"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"homelab-dashboard/internal/buildinfo"
	"homelab-dashboard/internal/config"
	"homelab-dashboard/internal/kube"
	"homelab-dashboard/internal/prom"
)

type Service struct {
	cfg  config.Config
	prom *prom.Client
	kube *kube.Client
}

func NewService(cfg config.Config, promClient *prom.Client, kubeClient *kube.Client) *Service {
	return &Service{
		cfg:  cfg,
		prom: promClient,
		kube: kubeClient,
	}
}

func (s *Service) Hub(ctx context.Context) ViewModel {
	data, errors := s.loadShared(ctx)
	headlines := buildHeadlines(data)

	view := ViewModel{
		AppName:        s.cfg.AppName,
		AppVersion:     buildinfo.Version,
		ClusterName:    s.cfg.ClusterName,
		PageTitle:      "Insight Hub",
		Screen:         "hub",
		DemoMode:       s.cfg.DemoMode,
		GeneratedAt:    time.Now(),
		RefreshSeconds: int(s.cfg.RefreshInterval.Seconds()),
		Navigation:     s.navigation("hub"),
		Errors:         errors,
		Hub: &HubView{
			Headlines: headlines,
			SummaryCards: []StatCard{
				{Label: "Nodes Ready", Value: fmt.Sprintf("%d / %d", int(data.nodesReady), int(data.nodesTotal)), Detail: "Kubernetes node condition", Tone: toneByRatio(data.nodesReady, data.nodesTotal)},
				{Label: "Scrape Targets", Value: fmt.Sprintf("%d / %d", int(data.targetsHealthy), int(data.targetsTotal)), Detail: "Prometheus targets responding", Tone: toneByRatio(data.targetsHealthy, data.targetsTotal)},
				{Label: "Active Namespaces", Value: fmt.Sprintf("%d", int(data.namespaces)), Detail: "Namespaces in Active phase", Tone: "info"},
				{Label: "Flux Resources", Value: fmt.Sprintf("%d", int(data.fluxTotal)), Detail: fmt.Sprintf("%d unready, %d suspended", int(data.fluxNotReady), int(data.fluxSuspended)), Tone: toneByIssueCounts(data.fluxNotReady, data.fluxSuspended)},
			},
			Utilization: []UsageMeter{
				{Label: "Cluster CPU", Value: data.clusterCPU, Display: fmt.Sprintf("%.1f%%", data.clusterCPU), Detail: "Average non-idle CPU across nodes", Tone: toneByThreshold(data.clusterCPU, s.cfg.Thresholds.NodeCPUWarnPercent)},
				{Label: "Cluster Memory", Value: data.clusterMemory, Display: fmt.Sprintf("%.1f%%", data.clusterMemory), Detail: "Allocated working memory footprint", Tone: toneByThreshold(data.clusterMemory, s.cfg.Thresholds.NodeMemoryWarnPercent)},
				{Label: "Running Pods", Value: podRatioValue(data.podsRunning, data.podsRunning+data.podsPending+data.podsFailed), Display: fmt.Sprintf("%.0f", data.podsRunning), Detail: fmt.Sprintf("%.0f pending, %.0f failed/unknown", data.podsPending, data.podsFailed), Tone: toneByIssueCounts(data.podsPending, data.podsFailed)},
			},
			TopCPU:        data.topCPU,
			TopMemory:     data.topMemory,
			WarningEvents: data.warningEvents,
		},
	}

	view.Banner = Banner{
		Label:  hubStatusLabel(data),
		Detail: fmt.Sprintf("%d active anomalies across nodes, targets, restarts, and Flux", len(data.anomalies)),
		Tone:   hubStatusTone(data),
		Actions: []Action{
			{Label: "Open Security Posture", Path: "/security"},
			{Label: "Review Anomalies", Path: "/anomalies"},
		},
	}

	return view
}

func (s *Service) Security(ctx context.Context) ViewModel {
	data, errors := s.loadShared(ctx)

	view := ViewModel{
		AppName:        s.cfg.AppName,
		AppVersion:     buildinfo.Version,
		ClusterName:    s.cfg.ClusterName,
		PageTitle:      "Security Posture",
		Screen:         "security",
		DemoMode:       s.cfg.DemoMode,
		GeneratedAt:    time.Now(),
		RefreshSeconds: int(s.cfg.RefreshInterval.Seconds()),
		Navigation:     s.navigation("security"),
		Errors:         errors,
		Security: &SecurityView{
			FluxCards: []StatCard{
				{Label: "Flux Controllers", Value: fmt.Sprintf("%.0f", data.fluxControllersUp), Detail: "Prometheus scrape targets in flux-system", Tone: toneByThreshold(data.fluxControllersDown, 0.5)},
				{Label: "Ready Resources", Value: fmt.Sprintf("%.0f", data.fluxReady), Detail: "Flux resources reporting ready=True", Tone: "good"},
				{Label: "Not Ready", Value: fmt.Sprintf("%.0f", data.fluxNotReady), Detail: "Flux resources reporting ready!=True", Tone: toneByThreshold(data.fluxNotReady, s.cfg.Thresholds.FluxUnreadyWarnCount)},
				{Label: "Suspended", Value: fmt.Sprintf("%.0f", data.fluxSuspended), Detail: "Flux resources with reconciliation paused", Tone: toneByThreshold(data.fluxSuspended, 0.5)},
			},
			FluxKinds:      data.fluxKinds,
			SlowReconciles: data.slowestFlux,
			WarningEvents:  data.warningEvents,
		},
	}

	view.Banner = Banner{
		Label:  securityStatusLabel(data),
		Detail: fmt.Sprintf("%d Flux objects tracked across %d kinds", int(data.fluxTotal), len(data.fluxKinds)),
		Tone:   securityStatusTone(data),
		Actions: []Action{
			{Label: "Back To Hub", Path: "/"},
			{Label: "Open Forecasting", Path: "/forecasting"},
		},
	}

	return view
}

func (s *Service) Anomalies(ctx context.Context) ViewModel {
	data, errors := s.loadShared(ctx)

	view := ViewModel{
		AppName:        s.cfg.AppName,
		AppVersion:     buildinfo.Version,
		ClusterName:    s.cfg.ClusterName,
		PageTitle:      "Anomaly Explorer",
		Screen:         "anomalies",
		DemoMode:       s.cfg.DemoMode,
		GeneratedAt:    time.Now(),
		RefreshSeconds: int(s.cfg.RefreshInterval.Seconds()),
		Navigation:     s.navigation("anomalies"),
		Errors:         errors,
		Anomalies: &AnomaliesView{
			SummaryCards: []StatCard{
				{Label: "Critical Signals", Value: fmt.Sprintf("%d", countSeverity(data.anomalies, "critical")), Detail: "Immediate remediation required", Tone: "critical"},
				{Label: "Warning Signals", Value: fmt.Sprintf("%d", countSeverity(data.anomalies, "warning")), Detail: "Thresholds crossed but cluster still serving", Tone: "warning"},
				{Label: "Targets Down", Value: fmt.Sprintf("%.0f", data.downTargetCount), Detail: "Prometheus targets failing scrapes", Tone: toneByThreshold(data.downTargetCount, s.cfg.Thresholds.UnavailableTargetsWarn)},
				{Label: "Restart Bursts", Value: fmt.Sprintf("%.0f", data.restartBurstCount), Detail: "Pods restarted in the last 30 minutes", Tone: toneByThreshold(data.restartBurstCount, s.cfg.Thresholds.RestartBurstThreshold)},
			},
			Signals:      data.anomalies,
			NodePressure: append(data.topCPUToMeters(), data.topMemoryToMeters()...),
		},
	}

	view.Banner = Banner{
		Label:  anomalyBannerLabel(data.anomalies),
		Detail: "Rule-based detection from live Prometheus signals. No runtime AI involved.",
		Tone:   anomalyBannerTone(data.anomalies),
		Actions: []Action{
			{Label: "Open Hub", Path: "/"},
			{Label: "Open Security", Path: "/security"},
		},
	}

	return view
}

func (s *Service) Forecast(ctx context.Context) ViewModel {
	data, errors := s.loadShared(ctx)

	cpuForecast := projectSeries(data.cpuTrend)
	memForecast := projectSeries(data.memoryTrend)
	podForecast := projectSeries(data.podTrend)

	view := ViewModel{
		AppName:        s.cfg.AppName,
		AppVersion:     buildinfo.Version,
		ClusterName:    s.cfg.ClusterName,
		PageTitle:      "Forecasting",
		Screen:         "forecasting",
		DemoMode:       s.cfg.DemoMode,
		GeneratedAt:    time.Now(),
		RefreshSeconds: int(s.cfg.RefreshInterval.Seconds()),
		Navigation:     s.navigation("forecasting"),
		Errors:         errors,
		Forecast: &ForecastView{
			ForecastCards: []ForecastCard{
				{
					Label:      "CPU Projection",
					Current:    fmt.Sprintf("%.1f%%", data.clusterCPU),
					Projection: fmt.Sprintf("%.1f%% in 24h", cpuForecast.projected),
					Trend:      cpuForecast.summary,
					Tone:       forecastTone(cpuForecast.projected, s.cfg.Thresholds.NodeCPUWarnPercent),
				},
				{
					Label:      "Memory Projection",
					Current:    fmt.Sprintf("%.1f%%", data.clusterMemory),
					Projection: fmt.Sprintf("%.1f%% in 24h", memForecast.projected),
					Trend:      memForecast.summary,
					Tone:       forecastTone(memForecast.projected, s.cfg.Thresholds.NodeMemoryWarnPercent),
				},
				{
					Label:      "Running Pods Projection",
					Current:    fmt.Sprintf("%.0f", data.podsRunning),
					Projection: fmt.Sprintf("%.0f in 24h", podForecast.projected),
					Trend:      podForecast.summary,
					Tone:       "info",
				},
			},
			Series: []SparklineCard{
				buildSparkline("Cluster CPU", data.cpuTrend, "24h rolling mean", s.cfg.Thresholds.NodeCPUWarnPercent),
				buildSparkline("Cluster Memory", data.memoryTrend, "24h working set ratio", s.cfg.Thresholds.NodeMemoryWarnPercent),
				buildSparkline("Running Pods", data.podTrend, "24h pod count trend", 0),
			},
		},
	}

	view.Banner = Banner{
		Label:  forecastLabel(cpuForecast, memForecast),
		Detail: "Simple trend projection from Prometheus history over the last 24 hours.",
		Tone:   forecastBannerTone(cpuForecast, memForecast),
		Actions: []Action{
			{Label: "Back To Hub", Path: "/"},
			{Label: "Review Anomalies", Path: "/anomalies"},
		},
	}

	return view
}

type sharedData struct {
	nodesReady        float64
	nodesTotal        float64
	namespaces        float64
	podsRunning       float64
	podsPending       float64
	podsFailed        float64
	targetsHealthy    float64
	targetsTotal      float64
	clusterCPU        float64
	clusterMemory     float64
	fluxReady         float64
	fluxNotReady      float64
	fluxSuspended     float64
	fluxTotal         float64
	fluxControllersUp float64
	fluxControllersDown float64
	downTargetCount   float64
	restartBurstCount float64
	topCPU            []ResourceStat
	topMemory         []ResourceStat
	slowestFlux       []ResourceStat
	fluxKinds         []KindStatus
	warningEvents     []EventRow
	anomalies         []AnomalySignal
	cpuTrend          []float64
	memoryTrend       []float64
	podTrend          []float64
}

func (s *Service) loadShared(ctx context.Context) (sharedData, []string) {
	if s.cfg.DemoMode {
		return demoSharedData(), nil
	}

	return s.collectShared(ctx)
}

func (s *Service) collectShared(ctx context.Context) (sharedData, []string) {
	var (
		data   sharedData
		errors []string
	)

	recordScalar := func(query string, dest *float64) {
		value, err := s.prom.Scalar(ctx, query)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", query, err))
			return
		}
		*dest = value
	}

	recordVector := func(query string, apply func([]prom.Sample)) {
		values, err := s.prom.Query(ctx, query)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", query, err))
			return
		}
		apply(values)
	}

	recordRange := func(query string, dest *[]float64) {
		end := time.Now().UTC()
		start := end.Add(-24 * time.Hour)
		series, err := s.prom.QueryRange(ctx, query, start, end, time.Hour)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", query, err))
			return
		}
		*dest = flattenSeries(series)
	}

	recordScalar(`sum(kube_node_status_condition{condition="Ready",status="true"})`, &data.nodesReady)
	recordScalar(`count(kube_node_info)`, &data.nodesTotal)
	recordScalar(`count(kube_namespace_status_phase{phase="Active"})`, &data.namespaces)
	recordScalar(`sum(kube_pod_status_phase{phase="Running"})`, &data.podsRunning)
	recordScalar(`sum(kube_pod_status_phase{phase="Pending"})`, &data.podsPending)
	recordScalar(`sum(kube_pod_status_phase{phase=~"Failed|Unknown"})`, &data.podsFailed)
	recordScalar(`sum(up)`, &data.targetsHealthy)
	recordScalar(`count(up)`, &data.targetsTotal)
	recordScalar(`100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[5m])))`, &data.clusterCPU)
	recordScalar(`100 * (1 - (sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)))`, &data.clusterMemory)
	recordScalar(`sum(flux_resource_info{ready="True"})`, &data.fluxReady)
	recordScalar(`sum(flux_resource_info{ready!="True"})`, &data.fluxNotReady)
	recordScalar(`sum(flux_resource_info{suspended="True"})`, &data.fluxSuspended)
	recordScalar(`sum(flux_resource_info)`, &data.fluxTotal)
	recordScalar(`sum(up{namespace="flux-system"})`, &data.fluxControllersUp)
	recordScalar(`count(up{namespace="flux-system"}) - sum(up{namespace="flux-system"})`, &data.fluxControllersDown)
	recordScalar(`count(up == 0)`, &data.downTargetCount)
	recordScalar(`count(sum by(namespace,pod) (increase(kube_pod_container_status_restarts_total[30m])) > 0)`, &data.restartBurstCount)

	recordVector(`topk(5, ((1 - avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))) * 100) * on(instance) group_left(nodename,kubernetes_node) node_uname_info)`, func(values []prom.Sample) {
		data.topCPU = make([]ResourceStat, 0, len(values))
		for _, sample := range values {
			data.topCPU = append(data.topCPU, ResourceStat{
				Name:   nodeDisplayName(sample.Metric),
				Value:  fmt.Sprintf("%.1f%%", sample.Value),
				Detail: "5m CPU saturation",
				Tone:   toneByThreshold(sample.Value, s.cfg.Thresholds.NodeCPUWarnPercent),
			})
		}
	})

	recordVector(`topk(5, ((1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100) * on(instance) group_left(nodename,kubernetes_node) node_uname_info)`, func(values []prom.Sample) {
		data.topMemory = make([]ResourceStat, 0, len(values))
		for _, sample := range values {
			data.topMemory = append(data.topMemory, ResourceStat{
				Name:   nodeDisplayName(sample.Metric),
				Value:  fmt.Sprintf("%.1f%%", sample.Value),
				Detail: "Memory saturation",
				Tone:   toneByThreshold(sample.Value, s.cfg.Thresholds.NodeMemoryWarnPercent),
			})
		}
	})

	recordVector(`topk(6, gotk_reconcile_duration_seconds_sum / gotk_reconcile_duration_seconds_count)`, func(values []prom.Sample) {
		data.slowestFlux = make([]ResourceStat, 0, len(values))
		for _, sample := range values {
			name := sample.Metric["kind"] + "/" + sample.Metric["name"]
			data.slowestFlux = append(data.slowestFlux, ResourceStat{
				Name:   name,
				Value:  fmt.Sprintf("%.2fs", sample.Value),
				Detail: sample.Metric["namespace"],
				Tone:   toneByThreshold(sample.Value, 2.5),
			})
		}
	})

	recordVector(`sum by(kind, ready, suspended) (flux_resource_info)`, func(values []prom.Sample) {
		byKind := map[string]*KindStatus{}
		for _, sample := range values {
			kind := sample.Metric["kind"]
			entry, ok := byKind[kind]
			if !ok {
				entry = &KindStatus{Kind: kind}
				byKind[kind] = entry
			}

			switch {
			case sample.Metric["suspended"] == "True":
				entry.Suspended += int(sample.Value)
			case sample.Metric["ready"] == "True":
				entry.Ready += int(sample.Value)
			default:
				entry.NotReady += int(sample.Value)
			}
		}

		data.fluxKinds = make([]KindStatus, 0, len(byKind))
		for _, entry := range byKind {
			data.fluxKinds = append(data.fluxKinds, *entry)
		}
		sort.Slice(data.fluxKinds, func(i, j int) bool {
			return data.fluxKinds[i].Kind < data.fluxKinds[j].Kind
		})
	})

	recordVector(`topk(8, max by(job, instance, namespace, pod) (1 - up)) > 0`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < 1 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Severity: "critical",
				Signal:   "Target Down",
				Resource: metricResource(sample.Metric, []string{"job", "instance"}),
				Value:    "down",
				Window:   "current",
				Details:  "Prometheus scrape failed for this target.",
			})
		}
	})

	recordVector(`topk(8, sum by(namespace,pod) (increase(kube_pod_container_status_restarts_total[30m])) > 0)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < s.cfg.Thresholds.RestartBurstThreshold {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Severity: "warning",
				Signal:   "Restart Burst",
				Resource: sample.Metric["namespace"] + "/" + sample.Metric["pod"],
				Value:    fmt.Sprintf("%.0f restarts", sample.Value),
				Window:   "30m",
				Details:  "Repeated restarts exceeded the configured threshold.",
			})
		}
	})

	recordVector(`topk(5, ((1 - avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))) * 100) * on(instance) group_left(nodename,kubernetes_node) node_uname_info)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < s.cfg.Thresholds.NodeCPUWarnPercent {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Severity: "warning",
				Signal:   "High Node CPU",
				Resource: nodeDisplayName(sample.Metric),
				Value:    fmt.Sprintf("%.1f%%", sample.Value),
				Window:   "5m",
				Details:  "CPU saturation is elevated on this node.",
			})
		}
	})

	recordVector(`topk(5, ((1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100) * on(instance) group_left(nodename,kubernetes_node) node_uname_info)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < s.cfg.Thresholds.NodeMemoryWarnPercent {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Severity: "warning",
				Signal:   "High Node Memory",
				Resource: nodeDisplayName(sample.Metric),
				Value:    fmt.Sprintf("%.1f%%", sample.Value),
				Window:   "current",
				Details:  "Memory headroom is below the configured threshold.",
			})
		}
	})

	recordVector(`topk(8, flux_resource_info{ready!="True"})`, func(values []prom.Sample) {
		for _, sample := range values {
			data.anomalies = append(data.anomalies, AnomalySignal{
				Severity: "critical",
				Signal:   "Flux Unready",
				Resource: sample.Metric["kind"] + "/" + sample.Metric["name"],
				Value:    sample.Metric["reason"],
				Window:   "current",
				Details:  "Flux reports this resource as not ready.",
			})
		}
	})

	recordRange(`100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[30m])))`, &data.cpuTrend)
	recordRange(`100 * (1 - (sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)))`, &data.memoryTrend)
	recordRange(`sum(kube_pod_status_phase{phase="Running"})`, &data.podTrend)

	if s.cfg.EnableKubernetes && s.kube != nil {
		events, err := s.kube.WarningEvents(ctx, 8)
		if err != nil {
			errors = append(errors, fmt.Sprintf("kubernetes warning events: %v", err))
		} else {
			for _, event := range events {
				if len(s.cfg.NamespaceAllowlist) > 0 && !containsString(s.cfg.NamespaceAllowlist, event.Namespace) {
					continue
				}
				data.warningEvents = append(data.warningEvents, EventRow{
					When:      event.When,
					Namespace: event.Namespace,
					Reason:    event.Reason,
					Object:    event.Object,
					Message:   event.Message,
				})
			}
		}
	}

	sort.Slice(data.anomalies, func(i, j int) bool {
		return severityRank(data.anomalies[i].Severity) < severityRank(data.anomalies[j].Severity)
	})

	return data, errors
}

func (s *Service) navigation(active string) []NavItem {
	return []NavItem{
		{Label: "Insight Hub", Path: "/", Icon: "hub", Active: active == "hub"},
		{Label: "Forecasting", Path: "/forecasting", Icon: "trending_up", Active: active == "forecasting"},
		{Label: "Anomaly Explorer", Path: "/anomalies", Icon: "monitoring", Active: active == "anomalies"},
		{Label: "Security Posture", Path: "/security", Icon: "security", Active: active == "security"},
	}
}

func (d sharedData) topCPUToMeters() []UsageMeter {
	meters := make([]UsageMeter, 0, len(d.topCPU))
	for _, item := range d.topCPU {
		meters = append(meters, UsageMeter{
			Label:   item.Name,
			Value:   parsePercentage(item.Value),
			Display: item.Value,
			Detail:  item.Detail,
			Tone:    item.Tone,
		})
	}
	return meters
}

func (d sharedData) topMemoryToMeters() []UsageMeter {
	meters := make([]UsageMeter, 0, len(d.topMemory))
	for _, item := range d.topMemory {
		meters = append(meters, UsageMeter{
			Label:   item.Name,
			Value:   parsePercentage(item.Value),
			Display: item.Value,
			Detail:  item.Detail,
			Tone:    item.Tone,
		})
	}
	return meters
}

func toneByRatio(ready, total float64) string {
	if total == 0 {
		return "info"
	}
	if ready < total {
		if ready == 0 {
			return "critical"
		}
		return "warning"
	}
	return "good"
}

func toneByIssueCounts(primary, secondary float64) string {
	if primary > 0 {
		return "critical"
	}
	if secondary > 0 {
		return "warning"
	}
	return "good"
}

func toneByThreshold(value, warn float64) string {
	if warn <= 0 {
		return "info"
	}
	if value >= warn {
		if value >= warn*1.1 {
			return "critical"
		}
		return "warning"
	}
	return "good"
}

func hubStatusLabel(data sharedData) string {
	switch hubStatusTone(data) {
	case "critical":
		return "Cluster Requires Attention"
	case "warning":
		return "Cluster Stable With Warnings"
	default:
		return "Cluster Operating Normally"
	}
}

func hubStatusTone(data sharedData) string {
	if data.nodesReady < data.nodesTotal || data.downTargetCount > 0 || data.fluxNotReady > 0 {
		if data.nodesReady == 0 || data.targetsHealthy == 0 {
			return "critical"
		}
		return "warning"
	}
	return "good"
}

func securityStatusLabel(data sharedData) string {
	if securityStatusTone(data) == "good" {
		return "Flux Reconciliations Healthy"
	}
	if securityStatusTone(data) == "warning" {
		return "Flux Drift Signals Present"
	}
	return "Flux Health Degraded"
}

func securityStatusTone(data sharedData) string {
	if data.fluxNotReady > 0 || data.fluxControllersDown > 0 {
		if data.fluxControllersUp == 0 {
			return "critical"
		}
		return "warning"
	}
	return "good"
}

func anomalyBannerLabel(signals []AnomalySignal) string {
	if len(signals) == 0 {
		return "No Active Anomalies"
	}
	if countSeverity(signals, "critical") > 0 {
		return "Critical Signals Active"
	}
	return "Warnings Active"
}

func anomalyBannerTone(signals []AnomalySignal) string {
	if len(signals) == 0 {
		return "good"
	}
	if countSeverity(signals, "critical") > 0 {
		return "critical"
	}
	return "warning"
}

func buildHeadlines(data sharedData) []string {
	headlines := []string{
		fmt.Sprintf("%d of %d nodes are ready.", int(data.nodesReady), int(data.nodesTotal)),
		fmt.Sprintf("%.0f of %.0f scrape targets are healthy.", data.targetsHealthy, data.targetsTotal),
		fmt.Sprintf("Cluster CPU is at %.1f%% and memory at %.1f%%.", data.clusterCPU, data.clusterMemory),
	}
	if data.fluxNotReady > 0 {
		headlines = append(headlines, fmt.Sprintf("%d Flux resources are reporting unready.", int(data.fluxNotReady)))
	} else {
		headlines = append(headlines, "Flux resources are currently ready.")
	}
	return headlines
}

func forecastLabel(cpu, memory projection) string {
	if cpu.projected >= 90 || memory.projected >= 90 {
		return "Capacity Pressure Building"
	}
	return "Capacity Trend Stable"
}

func forecastBannerTone(cpu, memory projection) string {
	if cpu.projected >= 95 || memory.projected >= 95 {
		return "critical"
	}
	if cpu.projected >= 90 || memory.projected >= 90 {
		return "warning"
	}
	return "good"
}

func forecastTone(projected, warn float64) string {
	if warn > 0 && projected >= warn {
		return "warning"
	}
	return "good"
}

type projection struct {
	projected float64
	summary   string
}

func projectSeries(values []float64) projection {
	if len(values) == 0 {
		return projection{summary: "insufficient data"}
	}
	if len(values) == 1 {
		return projection{projected: values[0], summary: "flat sample"}
	}

	first := values[0]
	last := values[len(values)-1]
	slopePerHour := (last - first) / float64(len(values)-1)
	projected := math.Max(0, last+(slopePerHour*24))
	summary := "stable"
	switch {
	case slopePerHour > 1:
		summary = fmt.Sprintf("rising %.2f units/hour", slopePerHour)
	case slopePerHour < -1:
		summary = fmt.Sprintf("falling %.2f units/hour", math.Abs(slopePerHour))
	default:
		summary = fmt.Sprintf("near-flat %.2f units/hour", slopePerHour)
	}

	return projection{projected: projected, summary: summary}
}

func buildSparkline(label string, values []float64, detail string, warn float64) SparklineCard {
	path := sparklinePath(values, 180, 56)
	proj := projectSeries(values)
	latest := "n/a"
	delta := "insufficient data"
	tone := "info"
	if len(values) > 0 {
		latest = fmt.Sprintf("%.1f", values[len(values)-1])
		tone = forecastTone(values[len(values)-1], warn)
	}
	if len(values) > 1 {
		delta = fmt.Sprintf("%+.1f over 24h", values[len(values)-1]-values[0])
	}

	return SparklineCard{
		Label:  label,
		Path:   path,
		Latest: latest,
		Delta:  delta,
		Detail: fmt.Sprintf("%s, %s", detail, proj.summary),
		Tone:   tone,
	}
}

func sparklinePath(values []float64, width, height float64) string {
	if len(values) == 0 {
		return ""
	}

	min := values[0]
	max := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}
	if max == min {
		max = min + 1
	}

	step := width / float64(maxInt(len(values)-1, 1))
	var builder strings.Builder
	for idx, value := range values {
		x := float64(idx) * step
		y := height - ((value-min)/(max-min))*height
		if idx == 0 {
			builder.WriteString(fmt.Sprintf("M %.2f %.2f ", x, y))
			continue
		}
		builder.WriteString(fmt.Sprintf("L %.2f %.2f ", x, y))
	}

	return strings.TrimSpace(builder.String())
}

func flattenSeries(series []prom.RangeSeries) []float64 {
	if len(series) == 0 {
		return nil
	}
	values := make([]float64, 0, len(series[0].Points))
	for _, point := range series[0].Points {
		if math.IsNaN(point.Value) {
			continue
		}
		values = append(values, point.Value)
	}
	return values
}

func countSeverity(signals []AnomalySignal, severity string) int {
	count := 0
	for _, signal := range signals {
		if signal.Severity == severity {
			count++
		}
	}
	return count
}

func compactResourceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	host := value
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	return host
}

func nodeDisplayName(metric map[string]string) string {
	instance := compactResourceName(metric["instance"])
	if instance != "" && net.ParseIP(instance) == nil {
		return instance
	}

	for _, key := range []string{"nodename", "kubernetes_node", "node"} {
		if value := strings.TrimSpace(metric[key]); value != "" {
			return value
		}
	}

	return instance
}

func metricResource(metric map[string]string, keys []string) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if value := strings.TrimSpace(metric[key]); value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, " · ")
}

func parsePercentage(value string) float64 {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "%")
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func podRatioValue(part, total float64) float64 {
	if total == 0 {
		return 0
	}
	return (part / total) * 100
}

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func demoSharedData() sharedData {
	now := time.Now()
	return sharedData{
		nodesReady:          3,
		nodesTotal:          3,
		namespaces:          17,
		podsRunning:         142,
		podsPending:         2,
		podsFailed:          1,
		targetsHealthy:      96,
		targetsTotal:        98,
		clusterCPU:          38.4,
		clusterMemory:       61.7,
		fluxReady:           268,
		fluxNotReady:        3,
		fluxSuspended:       2,
		fluxTotal:           273,
		fluxControllersUp:   5,
		fluxControllersDown: 1,
		downTargetCount:     2,
		restartBurstCount:   4,
		topCPU: []ResourceStat{
			{Name: "talos-1", Value: "72.4%", Detail: "5m CPU saturation", Tone: "warning"},
			{Name: "talos-2", Value: "54.1%", Detail: "5m CPU saturation", Tone: "good"},
			{Name: "talos-3", Value: "42.8%", Detail: "5m CPU saturation", Tone: "good"},
		},
		topMemory: []ResourceStat{
			{Name: "talos-2", Value: "83.6%", Detail: "Memory saturation", Tone: "warning"},
			{Name: "talos-1", Value: "61.0%", Detail: "Memory saturation", Tone: "good"},
			{Name: "talos-3", Value: "40.5%", Detail: "Memory saturation", Tone: "good"},
		},
		slowestFlux: []ResourceStat{
			{Name: "Kustomization/searxng", Value: "2.93s", Detail: "selfhosted", Tone: "warning"},
			{Name: "Kustomization/cluster-apps", Value: "4.29s", Detail: "flux-system", Tone: "critical"},
			{Name: "HelmRelease/toolhive-operator", Value: "1.84s", Detail: "ai", Tone: "good"},
		},
		fluxKinds: []KindStatus{
			{Kind: "HelmRelease", Ready: 89, NotReady: 1, Suspended: 2},
			{Kind: "Kustomization", Ready: 122, NotReady: 2, Suspended: 0},
			{Kind: "OCIRepository", Ready: 34, NotReady: 0, Suspended: 0},
			{Kind: "GitRepository", Ready: 18, NotReady: 0, Suspended: 0},
		},
		warningEvents: []EventRow{
			{When: now.Add(-7 * time.Minute), Namespace: "monitor", Reason: "BackOff", Object: "Pod/thanos-query-84ccc68499-gcp9q", Message: "Back-off restarting failed container"},
			{When: now.Add(-16 * time.Minute), Namespace: "downloads", Reason: "FailedMount", Object: "Pod/radarr-0", Message: "Unable to attach or mount volumes"},
		},
		anomalies: []AnomalySignal{
			{Severity: "critical", Signal: "Target Down", Resource: "thanos-query · 10.0.0.8:10902", Value: "down", Window: "current", Details: "Prometheus scrape failed for this target."},
			{Severity: "critical", Signal: "Flux Unready", Resource: "Kustomization/cluster-apps", Value: "HealthCheckFailed", Window: "current", Details: "Flux reports this resource as not ready."},
			{Severity: "warning", Signal: "Restart Burst", Resource: "monitor/alertmanager-kube-prometheus-stack-0", Value: "4 restarts", Window: "30m", Details: "Repeated restarts exceeded the configured threshold."},
			{Severity: "warning", Signal: "High Node Memory", Resource: "talos-2", Value: "83.6%", Window: "current", Details: "Memory headroom is below the configured threshold."},
		},
		cpuTrend:    []float64{22, 24, 27, 26, 29, 31, 34, 36, 33, 35, 39, 38},
		memoryTrend: []float64{49, 50, 52, 53, 54, 55, 57, 58, 59, 60, 61, 62},
		podTrend:    []float64{128, 129, 130, 132, 133, 134, 136, 137, 139, 140, 141, 142},
	}
}
