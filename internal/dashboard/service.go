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
	healthScore := buildClusterHealthIndex(data)

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
				{Label: "Nodes Ready", Value: fmt.Sprintf("%d / %d", int(data.nodesReady), int(data.nodesTotal)), Detail: "kubernetes node condition", Tone: toneByRatio(data.nodesReady, data.nodesTotal)},
				{Label: "GitOps", Value: fmt.Sprintf("%.0f", data.fluxTotal), Detail: fmt.Sprintf("%.0f unready · %.0f suspended", data.fluxNotReady, data.fluxSuspended), Tone: toneByIssueCounts(data.fluxNotReady, data.fluxSuspended)},
				{Label: "Secret Sync", Value: fmt.Sprintf("%.0f ready", data.externalSecretsReady), Detail: fmt.Sprintf("%.0f degraded · %.0f sync errors/24h", data.externalSecretsDegraded, data.externalSecretSyncErrors24h), Tone: toneByIssueCounts(data.externalSecretsDegraded, data.externalSecretSyncErrors24h)},
				{Label: "Backup Integrity", Value: fmt.Sprintf("%.0f protected", data.volsyncSources), Detail: fmt.Sprintf("%.0f drifted · %.0f missed/24h", data.volsyncOutOfSync, data.volsyncMissed24h), Tone: toneByIssueCounts(data.volsyncOutOfSync, data.volsyncMissed24h)},
			},
			Utilization: []UsageMeter{
				{Label: "Cluster CPU", Value: data.clusterCPU, Display: fmt.Sprintf("%.1f%%", data.clusterCPU), Detail: "Average non-idle CPU across nodes", Tone: toneByThreshold(data.clusterCPU, s.cfg.Thresholds.NodeCPUWarnPercent)},
				{Label: "Cluster Memory", Value: data.clusterMemory, Display: fmt.Sprintf("%.1f%%", data.clusterMemory), Detail: "Allocated working memory footprint", Tone: toneByThreshold(data.clusterMemory, s.cfg.Thresholds.NodeMemoryWarnPercent)},
				{Label: "Healthy Targets", Value: podRatioValue(data.targetsHealthy, data.targetsTotal), Display: fmt.Sprintf("%.0f / %.0f", data.targetsHealthy, data.targetsTotal), Detail: fmt.Sprintf("%.0f namespaces · %.0f running pods", data.namespaces, data.podsRunning), Tone: toneByRatio(data.targetsHealthy, data.targetsTotal)},
			},
			HealthScore:   healthScore,
			HealthTone:    clusterHealthTone(healthScore),
			Signals:       buildHubSignals(data.anomalies),
			Paths:         buildHubPaths(data),
			TopCPU:        data.topCPU,
			TopMemory:     data.topMemory,
			WarningEvents: limitEventRows(data.warningEvents, 4),
		},
	}

	view.Banner = Banner{
		Label:  hubStatusLabel(data),
		Detail: fmt.Sprintf("%d active anomalies across compute, network, storage, and operators", len(data.anomalies)),
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
			SummaryCards: []StatCard{
				{Label: "Flux Resources", Value: fmt.Sprintf("%.0f", data.fluxTotal), Detail: fmt.Sprintf("%.0f unready · %.0f suspended", data.fluxNotReady, data.fluxSuspended), Tone: toneByIssueCounts(data.fluxNotReady, data.fluxSuspended)},
				{Label: "Secret Sync", Value: fmt.Sprintf("%.0f ready", data.externalSecretsReady), Detail: fmt.Sprintf("%.0f degraded · %.0f sync errors/24h", data.externalSecretsDegraded, data.externalSecretSyncErrors24h), Tone: toneByIssueCounts(data.externalSecretsDegraded, data.externalSecretSyncErrors24h)},
				{Label: "Backup Sources", Value: fmt.Sprintf("%.0f protected", data.volsyncSources), Detail: fmt.Sprintf("%.0f drifted · %.0f missed/24h", data.volsyncOutOfSync, data.volsyncMissed24h), Tone: toneByIssueCounts(data.volsyncOutOfSync, data.volsyncMissed24h)},
				{Label: "CNPG Clusters", Value: fmt.Sprintf("%.0f clusters", data.cnpgClusters), Detail: fmt.Sprintf("%.0f instances · max lag %s", data.cnpgInstances, formatSeconds(data.cnpgMaxReplicationLag)), Tone: toneByThreshold(data.cnpgMaxReplicationLag, 30)},
			},
			FluxCards:           buildFluxCards(data.fluxKinds),
			FluxKinds:           data.fluxKinds,
			FluxRecent:          data.fluxRecent,
			CNPGCards:           buildCNPGCards(data),
			CNPGClusters:        data.cnpgClusterRows,
			VolSyncCards:        buildVolSyncCards(data),
			VolSyncSources:      data.volsyncSourceRows,
			ExternalSecretCards: buildExternalSecretCards(data),
			ExternalSecrets:     data.externalSecretRows,
			EnvoyCards:          buildEnvoyCards(data),
			EnvoyRoutes:         data.envoyRouteRows,
			ToolhiveCards:       buildToolhiveCards(data),
			ToolhiveBackends:    data.toolhiveBackendRows,
			RenovateCards:       buildRenovateCards(data),
			RenovateProjects:    data.renovateProjectRows,
			SlowReconciles:      data.slowestFlux,
			WarningEvents:       data.warningEvents,
		},
	}

	view.Banner = Banner{
		Label:  securityStatusLabel(data),
		Detail: "GitOps, secret sync, backup replication, database HA, edge traffic, and operator automation health.",
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
				{Label: "Active Signals", Value: fmt.Sprintf("%d", len(data.anomalies)), Detail: "Current rule hits across live telemetry", Tone: anomalyBannerTone(data.anomalies)},
				{Label: "Critical Signals", Value: fmt.Sprintf("%d", countSeverity(data.anomalies, "critical")), Detail: "Immediate remediation required", Tone: "critical"},
				{Label: "Noisiest Domain", Value: anomalyTopCategoryLabel(data.anomalies), Detail: "Primary concentration of live rule hits", Tone: anomalyTopCategoryTone(data.anomalies)},
				{Label: "Primary Resource", Value: anomalyTopResourceLabel(data.anomalies), Detail: "Most repeated resource across active detections", Tone: "warning"},
			},
			Chart:       buildAnomalyChart(data),
			Events:      buildAnomalyEvents(data.anomalies),
			DomainCards: buildAnomalyDomainCards(data),
			Hotspots:    buildAnomalyHotspots(data.anomalies),
			Actions: []Action{
				{Label: "Open Security Posture", Path: "/security"},
				{Label: "Open Insight Hub", Path: "/"},
				{Label: "Open Forecasting", Path: "/forecasting"},
			},
			WarningEvents: data.warningEvents,
		},
	}

	view.Banner = Banner{
		Label:  anomalyBannerLabel(data.anomalies),
		Detail: "Rule-based detection from live Prometheus signals across compute, network, storage, and operators.",
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

	rustfsForecast := projectSeriesHours(data.rustfsCapacityTrend, 24*30)
	cnpgForecast := projectSeriesHours(data.cnpgSizeTrend, 24*30)
	backupForecast := projectSeriesHours(data.volsyncDurationTrend, 24*7)
	trafficForecast := projectSeriesHours(data.envoyTrafficTrend, 24*7)

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
					Label:      "RustFS Capacity",
					Current:    formatBytes(data.rustfsCapacityCurrent),
					Projection: fmt.Sprintf("%s in 30d", formatBytes(rustfsForecast.projected)),
					Trend:      fmt.Sprintf("%s over next 30d", formatSignedBytes(rustfsForecast.delta)),
					Tone:       "info",
				},
				{
					Label:      "CNPG Primary Size",
					Current:    formatBytes(data.cnpgTotalSizeBytes),
					Projection: fmt.Sprintf("%s in 30d", formatBytes(cnpgForecast.projected)),
					Trend:      fmt.Sprintf("%s over next 30d", formatSignedBytes(cnpgForecast.delta)),
					Tone:       "info",
				},
				{
					Label:      "Backup Duration",
					Current:    formatDurationShort(time.Duration(data.volsyncLongestDuration * float64(time.Second))),
					Projection: fmt.Sprintf("%s in 7d", formatDurationShort(time.Duration(backupForecast.projected*float64(time.Second)))),
					Trend:      fmt.Sprintf("%s over next 7d", formatSignedDurationSeconds(backupForecast.delta)),
					Tone:       forecastTone(backupForecast.projected, 60),
				},
				{
					Label:      "Edge Traffic",
					Current:    fmt.Sprintf("%s req/s", formatRate(data.envoyRequestRate)),
					Projection: fmt.Sprintf("%s req/s in 7d", formatRate(trafficForecast.projected)),
					Trend:      fmt.Sprintf("%s req/s over next 7d", formatSignedRate(trafficForecast.delta)),
					Tone:       "good",
				},
			},
			Series: []SparklineCard{
				buildBytesSparkline("RustFS Capacity", data.rustfsCapacityTrend, "object storage footprint over the last 24h"),
				buildBytesSparkline("CNPG Primary Size", data.cnpgSizeTrend, "primary database footprint across clusters"),
				buildDurationSparkline("VolSync Sync Duration", data.volsyncDurationTrend, "longest source sync duration over the last 24h"),
				buildRateSparkline("Envoy Request Rate", data.envoyTrafficTrend, "aggregate upstream requests per second"),
			},
		},
	}

	view.Banner = Banner{
		Label:  forecastLabel(rustfsForecast, cnpgForecast),
		Detail: "Storage, database, backup, and ingress trend projection from Prometheus history.",
		Tone:   forecastBannerTone(rustfsForecast, cnpgForecast),
		Actions: []Action{
			{Label: "Back To Hub", Path: "/"},
			{Label: "Review Anomalies", Path: "/anomalies"},
		},
	}

	return view
}

type sharedData struct {
	nodesReady                  float64
	nodesTotal                  float64
	namespaces                  float64
	podsRunning                 float64
	podsPending                 float64
	podsFailed                  float64
	targetsHealthy              float64
	targetsTotal                float64
	clusterCPU                  float64
	clusterMemory               float64
	fluxReady                   float64
	fluxNotReady                float64
	fluxSuspended               float64
	fluxTotal                   float64
	fluxControllersUp           float64
	fluxControllersDown         float64
	downTargetCount             float64
	restartBurstCount           float64
	externalSecretsReady        float64
	externalSecretsDegraded     float64
	externalSecretSyncErrors24h float64
	externalSecretTotal         float64
	externalSecretStoreTotal    float64
	externalSecretStoreReady    float64
	externalSecretOldestRefresh float64
	externalSecretRows          []ExternalSecretRow
	volsyncSources              float64
	volsyncOutOfSync            float64
	volsyncMissed24h            float64
	volsyncLongestDuration      float64
	volsyncSlowestSource        string
	volsyncSourceRows           []VolSyncSourceRow
	cnpgClusters                float64
	cnpgInstances               float64
	cnpgStreamingReplicas       float64
	cnpgMaxReplicationLag       float64
	cnpgWALQueue                float64
	cnpgMaxArchivalDelay        float64
	cnpgRecentBackupFailures    float64
	cnpgTotalSizeBytes          float64
	cnpgClusterRows             []CNPGClusterRow
	envoyActiveRoutes           float64
	envoyTopRoute               string
	envoyRouteRows              []EnvoyRouteRow
	envoyRequestRate            float64
	envoyErrorRate              float64
	envoyP95Latency             float64
	toolhiveBackends            float64
	toolhiveSlowestBackend      string
	toolhiveAvgBackendLatency   float64
	toolhiveBackendRows         []ToolhiveBackendRow
	toolhiveConnections         float64
	toolhiveBackendErrors24h    float64
	renovateProjects            float64
	renovateExecutions24h       float64
	renovateRunsFailed          float64
	renovateDependencyIssues    float64
	renovateProjectRows         []RenovateProjectRow
	rustfsCapacityCurrent       float64
	rustfsCapacityTrend         []float64
	cnpgSizeTrend               []float64
	volsyncDurationTrend        []float64
	envoyTrafficTrend           []float64
	topCPU                      []ResourceStat
	topMemory                   []ResourceStat
	slowestFlux                 []ResourceStat
	fluxKinds                   []KindStatus
	fluxRecent                  []FluxRecentRow
	warningEvents               []EventRow
	anomalies                   []AnomalySignal
	computeSignalTrend          []float64
	networkSignalTrend          []float64
	storageSignalTrend          []float64
	operatorSignalTrend         []float64
	cpuTrend                    []float64
	memoryTrend                 []float64
	podTrend                    []float64
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

	cnpgSnapshots := map[string]*cnpgClusterSnapshot{}
	volsyncSnapshots := map[string]*volsyncSourceSnapshot{}
	externalSecretSnapshots := map[string]*externalSecretSnapshot{}
	envoyRouteSnapshots := map[string]*envoyRouteSnapshot{}
	toolhiveBackendSnapshots := map[string]*toolhiveBackendSnapshot{}
	renovateProjectSnapshots := map[string]*renovateProjectSnapshot{}
	ensureCNPGSnapshot := func(metric map[string]string) *cnpgClusterSnapshot {
		cluster := cnpgClusterName(metric)
		if cluster == "" {
			return nil
		}
		namespace := strings.TrimSpace(metric["namespace"])
		key := namespace + "/" + cluster
		snapshot, ok := cnpgSnapshots[key]
		if !ok {
			snapshot = &cnpgClusterSnapshot{
				Name:      cluster,
				Namespace: namespace,
			}
			cnpgSnapshots[key] = snapshot
		}
		return snapshot
	}
	ensureVolsyncSnapshot := func(namespace, name string) *volsyncSourceSnapshot {
		namespace = strings.TrimSpace(namespace)
		name = strings.TrimSpace(name)
		if namespace == "" || name == "" {
			return nil
		}
		key := namespace + "/" + name
		snapshot, ok := volsyncSnapshots[key]
		if !ok {
			snapshot = &volsyncSourceSnapshot{
				Name:      name,
				Namespace: namespace,
			}
			volsyncSnapshots[key] = snapshot
		}
		return snapshot
	}
	ensureExternalSecretSnapshot := func(namespace, name string) *externalSecretSnapshot {
		namespace = strings.TrimSpace(namespace)
		name = strings.TrimSpace(name)
		if namespace == "" || name == "" {
			return nil
		}
		key := namespace + "/" + name
		snapshot, ok := externalSecretSnapshots[key]
		if !ok {
			snapshot = &externalSecretSnapshot{
				Name:      name,
				Namespace: namespace,
			}
			externalSecretSnapshots[key] = snapshot
		}
		return snapshot
	}
	ensureEnvoyRouteSnapshot := func(name string) *envoyRouteSnapshot {
		name = strings.TrimSpace(name)
		if name == "" || name == "prometheus_stats" {
			return nil
		}
		snapshot, ok := envoyRouteSnapshots[name]
		if !ok {
			routeNamespace, routeName := parseEnvoyRoute(name)
			snapshot = &envoyRouteSnapshot{
				Name:      name,
				Namespace: routeNamespace,
				Route:     routeName,
			}
			envoyRouteSnapshots[name] = snapshot
		}
		return snapshot
	}
	ensureToolhiveBackendSnapshot := func(name string) *toolhiveBackendSnapshot {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		snapshot, ok := toolhiveBackendSnapshots[name]
		if !ok {
			snapshot = &toolhiveBackendSnapshot{Name: name}
			toolhiveBackendSnapshots[name] = snapshot
		}
		return snapshot
	}
	ensureRenovateProjectSnapshot := func(name string) *renovateProjectSnapshot {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		snapshot, ok := renovateProjectSnapshots[name]
		if !ok {
			snapshot = &renovateProjectSnapshot{Name: name}
			renovateProjectSnapshots[name] = snapshot
		}
		return snapshot
	}

	recordScalar := func(query string, dest *float64) {
		value, err := s.prom.Scalar(ctx, query)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", query, err))
			return
		}
		if math.IsNaN(value) {
			value = 0
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
	recordScalar(`sum(externalsecret_status_condition{condition="Ready",status="True"})`, &data.externalSecretsReady)
	recordScalar(`sum(externalsecret_status_condition{condition="Ready",status!="True"})`, &data.externalSecretsDegraded)
	recordScalar(`sum(increase(externalsecret_sync_calls_error[24h]))`, &data.externalSecretSyncErrors24h)
	recordScalar(`count(volsync_missed_intervals_total{role="source"})`, &data.volsyncSources)
	recordScalar(`sum(volsync_volume_out_of_sync{role="source"})`, &data.volsyncOutOfSync)
	recordScalar(`sum(increase(volsync_missed_intervals_total{role="source"}[24h]))`, &data.volsyncMissed24h)
	recordScalar(`count(count by(job) (cnpg_pg_replication_streaming_replicas))`, &data.cnpgClusters)
	recordScalar(`sum(count by(job, namespace) (cnpg_collector_up))`, &data.cnpgInstances)
	recordScalar(`sum(max by(job) (cnpg_pg_replication_streaming_replicas))`, &data.cnpgStreamingReplicas)
	recordScalar(`max(cnpg_pg_stat_replication_write_lag_seconds)`, &data.cnpgMaxReplicationLag)
	recordScalar(`sum(rate(envoy_cluster_external_upstream_rq[5m]))`, &data.envoyRequestRate)
	recordScalar(`sum(rate(envoy_cluster_external_upstream_rq_xx{envoy_response_code_class="5"}[5m]))`, &data.envoyErrorRate)
	recordScalar(`histogram_quantile(0.95, sum(rate(envoy_cluster_external_upstream_rq_time_bucket[5m])) by (le))`, &data.envoyP95Latency)
	recordScalar(`sum(toolhive_mcp_active_connections)`, &data.toolhiveConnections)
	recordScalar(`count(count by(target_workload_name) (toolhive_vmcp_backend_requests_total))`, &data.toolhiveBackends)
	recordScalar(`sum(increase(toolhive_vmcp_backend_errors_total[24h]))`, &data.toolhiveBackendErrors24h)
	recordScalar(`sum(rate(toolhive_vmcp_backend_requests_duration_seconds_sum[5m])) / sum(rate(toolhive_vmcp_backend_requests_duration_seconds_count[5m]))`, &data.toolhiveAvgBackendLatency)
	recordScalar(`count(renovate_operator_run_failed)`, &data.renovateProjects)
	recordScalar(`sum(increase(renovate_operator_project_executions_total[24h]))`, &data.renovateExecutions24h)
	recordScalar(`sum(renovate_operator_run_failed)`, &data.renovateRunsFailed)
	recordScalar(`sum(renovate_operator_dependency_issues)`, &data.renovateDependencyIssues)
	recordScalar(`count(sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq[5m])) > 0)`, &data.envoyActiveRoutes)
	recordScalar(`sum(rustfs_capacity_current)`, &data.rustfsCapacityCurrent)

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

	recordVector(`externalsecret_status_condition{condition="Ready"}`, func(values []prom.Sample) {
		for _, sample := range values {
			namespace := sample.Metric["exported_namespace"]
			if namespace == "" {
				namespace = sample.Metric["namespace"]
			}
			snapshot := ensureExternalSecretSnapshot(namespace, sample.Metric["name"])
			if snapshot == nil {
				continue
			}
			status := sample.Metric["status"]
			switch {
			case strings.EqualFold(status, "True") && sample.Value > 0:
				snapshot.Ready = true
			case sample.Value > 0:
				snapshot.Ready = false
				snapshot.Status = "Error"
			}
		}
	})

	recordVector(`increase(externalsecret_sync_calls_error[24h])`, func(values []prom.Sample) {
		for _, sample := range values {
			namespace := sample.Metric["exported_namespace"]
			if namespace == "" {
				namespace = sample.Metric["namespace"]
			}
			snapshot := ensureExternalSecretSnapshot(namespace, sample.Metric["name"])
			if snapshot == nil {
				continue
			}
			snapshot.Errors24h = sample.Value
		}
	})

	recordVector(`topk(24, sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq[5m])))`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureEnvoyRouteSnapshot(sample.Metric["envoy_cluster_name"])
			if snapshot == nil {
				continue
			}
			snapshot.RequestRate = sample.Value
		}
	})

	recordVector(`topk(24, sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_xx{envoy_response_code_class=~"4|5"}[5m])))`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureEnvoyRouteSnapshot(sample.Metric["envoy_cluster_name"])
			if snapshot == nil {
				continue
			}
			snapshot.ErrorRate = sample.Value
		}
	})

	recordVector(`topk(24, sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_sum[5m])) / sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_count[5m])))`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureEnvoyRouteSnapshot(sample.Metric["envoy_cluster_name"])
			if snapshot == nil {
				continue
			}
			snapshot.LatencyMs = sample.Value
		}
	})

	recordVector(`sum by(target_workload_name) (increase(toolhive_vmcp_backend_requests_total[24h]))`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureToolhiveBackendSnapshot(sample.Metric["target_workload_name"])
			if snapshot == nil {
				continue
			}
			snapshot.Requests24h = sample.Value
		}
	})

	recordVector(`sum by(target_workload_name) (increase(toolhive_vmcp_backend_errors_total[24h]))`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureToolhiveBackendSnapshot(sample.Metric["target_workload_name"])
			if snapshot == nil {
				continue
			}
			snapshot.Errors24h = sample.Value
		}
	})

	recordVector(`sum by(target_workload_name) (rate(toolhive_vmcp_backend_requests_duration_seconds_sum[5m])) / sum by(target_workload_name) (rate(toolhive_vmcp_backend_requests_duration_seconds_count[5m]))`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureToolhiveBackendSnapshot(sample.Metric["target_workload_name"])
			if snapshot == nil {
				continue
			}
			snapshot.LatencySeconds = sample.Value
		}
	})

	recordVector(`increase(renovate_operator_project_executions_total[24h])`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureRenovateProjectSnapshot(sample.Metric["project"])
			if snapshot == nil {
				continue
			}
			snapshot.Executions24h += sample.Value
		}
	})

	recordVector(`renovate_operator_dependency_issues`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureRenovateProjectSnapshot(sample.Metric["project"])
			if snapshot == nil {
				continue
			}
			snapshot.DependencyIssues = sample.Value
		}
	})

	recordVector(`renovate_operator_run_failed`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureRenovateProjectSnapshot(sample.Metric["project"])
			if snapshot == nil {
				continue
			}
			snapshot.RunFailed = sample.Value
		}
	})

	recordVector(`max by(job, namespace) (cnpg_pg_replication_streaming_replicas)`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureCNPGSnapshot(sample.Metric)
			if snapshot == nil {
				continue
			}
			snapshot.StreamingReplicas = sample.Value
		}
	})

	recordVector(`count by(job, namespace) (cnpg_collector_up)`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureCNPGSnapshot(sample.Metric)
			if snapshot == nil {
				continue
			}
			snapshot.Instances = sample.Value
		}
	})

	recordVector(`volsync_volume_out_of_sync{role="source"}`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureVolsyncSnapshot(sample.Metric["obj_namespace"], sample.Metric["obj_name"])
			if snapshot == nil {
				continue
			}
			snapshot.OutOfSync = sample.Value > 0
			if snapshot.Method == "" {
				snapshot.Method = sample.Metric["method"]
			}
		}
	})

	recordVector(`increase(volsync_missed_intervals_total{role="source"}[24h])`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureVolsyncSnapshot(sample.Metric["obj_namespace"], sample.Metric["obj_name"])
			if snapshot == nil {
				continue
			}
			snapshot.Missed24h = sample.Value
			if snapshot.Method == "" {
				snapshot.Method = sample.Metric["method"]
			}
		}
	})

	recordVector(`max by(job, namespace) (cnpg_pg_stat_replication_write_lag_seconds)`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureCNPGSnapshot(sample.Metric)
			if snapshot == nil {
				continue
			}
			snapshot.ReplicationLag = sample.Value
		}
	})

	recordVector(`sum by(job, namespace) (
		sum by(job, namespace, pod) (cnpg_pg_database_size_bytes)
		* on(job, namespace, pod) group_left
		max by(job, namespace, pod) (cnpg_pg_replication_in_recovery == bool 0)
	)`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureCNPGSnapshot(sample.Metric)
			if snapshot == nil {
				continue
			}
			snapshot.DatabaseSizeBytes = sample.Value
		}
	})

	recordVector(`max by(job, namespace) (cnpg_collector_last_failed_backup_timestamp)`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureCNPGSnapshot(sample.Metric)
			if snapshot == nil {
				continue
			}
			snapshot.LastFailedBackup = sample.Value
		}
	})

	recordVector(`max by(job, namespace) (cnpg_collector_pg_wal_archive_status{value="ready"})`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureCNPGSnapshot(sample.Metric)
			if snapshot == nil {
				continue
			}
			snapshot.WALReadyQueue = sample.Value
		}
	})

	recordVector(`max by(job, namespace) (cnpg_pg_stat_archiver_seconds_since_last_archival)`, func(values []prom.Sample) {
		for _, sample := range values {
			snapshot := ensureCNPGSnapshot(sample.Metric)
			if snapshot == nil {
				continue
			}
			snapshot.SecondsSinceArchival = sample.Value
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
			entry.Total = entry.Ready + entry.NotReady + entry.Suspended
			switch {
			case entry.NotReady > 0:
				entry.Status = "Drift"
				entry.Tone = "critical"
			case entry.Suspended > 0:
				entry.Status = "Paused"
				entry.Tone = "warning"
			default:
				entry.Status = "Ready"
				entry.Tone = "good"
			}
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
				Category: "Compute",
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
				Category: "Compute",
				Severity: "warning",
				Signal:   "Restart Burst",
				Resource: sample.Metric["namespace"] + "/" + sample.Metric["pod"],
				Value:    fmt.Sprintf("%.0f restarts", sample.Value),
				Window:   "30m",
				Details:  "Repeated restarts exceeded the configured threshold.",
			})
		}
	})

	recordVector(`topk(8, max by(namespace,pod,phase) (kube_pod_status_phase{phase=~"Pending|Failed|Unknown"}) > 0)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < 1 {
				continue
			}
			severity := "warning"
			if sample.Metric["phase"] != "Pending" {
				severity = "critical"
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Compute",
				Severity: severity,
				Signal:   "Workload State",
				Resource: sample.Metric["namespace"] + "/" + sample.Metric["pod"],
				Value:    sample.Metric["phase"],
				Window:   "current",
				Details:  "This workload is not in the running state.",
			})
		}
	})

	recordVector(`topk(8, flux_resource_info{ready!="True"})`, func(values []prom.Sample) {
		for _, sample := range values {
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Operators",
				Severity: "critical",
				Signal:   "Flux Unready",
				Resource: sample.Metric["kind"] + "/" + sample.Metric["name"],
				Value:    sample.Metric["reason"],
				Window:   "current",
				Details:  "Flux reports this resource as not ready.",
			})
		}
	})

	recordVector(`topk(6, cilium_controllers_failing)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 0 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Network",
				Severity: "warning",
				Signal:   "Cilium Controller Failures",
				Resource: sample.Metric["node"],
				Value:    fmt.Sprintf("%.0f failing", sample.Value),
				Window:   "current",
				Details:  "One or more Cilium controllers are failing on this node.",
			})
		}
	})

	recordVector(`topk(6, cilium_bpf_map_pressure)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < 0.10 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Network",
				Severity: "warning",
				Signal:   "Cilium BPF Map Pressure",
				Resource: sample.Metric["node"] + " · " + sample.Metric["map_name"],
				Value:    fmt.Sprintf("%.1f%%", sample.Value*100),
				Window:   "current",
				Details:  "A Cilium BPF map is filling up on this node.",
			})
		}
	})

	recordVector(`topk(6, sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_xx{envoy_response_code_class=~"4|5"}[5m])))`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < 0.02 {
				continue
			}
			severity := "warning"
			if sample.Value >= 0.10 {
				severity = "critical"
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Network",
				Severity: severity,
				Signal:   "Envoy Error Rate",
				Resource: sample.Metric["envoy_cluster_name"],
				Value:    fmt.Sprintf("%.2f req/s", sample.Value),
				Window:   "5m",
				Details:  "Upstream 4xx/5xx responses are elevated for this route.",
			})
		}
	})

	recordVector(`topk(6, sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_sum[5m])) / sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_count[5m])))`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < 250 {
				continue
			}
			severity := "warning"
			if sample.Value >= 500 {
				severity = "critical"
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Network",
				Severity: severity,
				Signal:   "Envoy Latency",
				Resource: sample.Metric["envoy_cluster_name"],
				Value:    fmt.Sprintf("%.0f ms", sample.Value),
				Window:   "5m",
				Details:  "Average upstream request latency is elevated for this route.",
			})
		}
	})

	recordVector(`topk(8, volsync_volume_out_of_sync{role="source"})`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 0 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Storage",
				Severity: "critical",
				Signal:   "VolSync Backup Drift",
				Resource: sample.Metric["obj_namespace"] + "/" + sample.Metric["obj_name"],
				Value:    "out of sync",
				Window:   "current",
				Details:  "This replication source is not synchronized with its backup target.",
			})
		}
	})

	recordVector(`topk(8, increase(volsync_missed_intervals_total{role="source"}[6h]))`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 0 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Storage",
				Severity: "warning",
				Signal:   "VolSync Missed Schedule",
				Resource: sample.Metric["obj_namespace"] + "/" + sample.Metric["obj_name"],
				Value:    fmt.Sprintf("%.0f intervals", sample.Value),
				Window:   "6h",
				Details:  "This replication source has missed scheduled backup intervals.",
			})
		}
	})

	recordVector(`topk(8, cnpg_pg_replication_lag)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 30 {
				continue
			}
			severity := "warning"
			if sample.Value >= 120 {
				severity = "critical"
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Storage",
				Severity: severity,
				Signal:   "CNPG Replication Lag",
				Resource: sample.Metric["pod"],
				Value:    fmt.Sprintf("%.0fs", sample.Value),
				Window:   "current",
				Details:  "A PostgreSQL replica is lagging behind its primary.",
			})
		}
	})

	recordVector(`topk(8, cnpg_collector_last_failed_backup_timestamp)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 0 {
				continue
			}
			age := time.Since(time.Unix(int64(sample.Value), 0))
			if age > 7*24*time.Hour {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Storage",
				Severity: "critical",
				Signal:   "CNPG Backup Failure",
				Resource: sample.Metric["pod"],
				Value:    age.Round(time.Hour).String() + " ago",
				Window:   "7d",
				Details:  "A recent CloudNativePG backup failure was recorded for this cluster.",
			})
		}
	})

	recordVector(`topk(8, externalsecret_status_condition{condition="Ready",status!="True"} == 1)`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value < 1 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Operators",
				Severity: "critical",
				Signal:   "External Secret Not Ready",
				Resource: sample.Metric["exported_namespace"] + "/" + sample.Metric["name"],
				Value:    sample.Metric["status"],
				Window:   "current",
				Details:  "An ExternalSecret is not reporting a Ready status.",
			})
		}
	})

	recordVector(`topk(8, increase(externalsecret_sync_calls_error[30m]))`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 0 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Operators",
				Severity: "warning",
				Signal:   "External Secret Sync Errors",
				Resource: sample.Metric["exported_namespace"] + "/" + sample.Metric["name"],
				Value:    fmt.Sprintf("%.0f errors", sample.Value),
				Window:   "30m",
				Details:  "Recent sync attempts to the provider backend returned errors.",
			})
		}
	})

	recordVector(`topk(8, increase(toolhive_vmcp_backend_errors_total[30m]))`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 0 {
				continue
			}
			target := sample.Metric["target_workload_name"]
			if target == "" {
				target = sample.Metric["server"]
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Operators",
				Severity: "warning",
				Signal:   "Toolhive Backend Errors",
				Resource: target,
				Value:    fmt.Sprintf("%.0f errors", sample.Value),
				Window:   "30m",
				Details:  "Toolhive recorded backend request errors for this MCP workload.",
			})
		}
	})

	recordVector(`topk(8, increase(renovate_operator_run_failed[24h]))`, func(values []prom.Sample) {
		for _, sample := range values {
			if sample.Value <= 0 {
				continue
			}
			data.anomalies = append(data.anomalies, AnomalySignal{
				Category: "Operators",
				Severity: "warning",
				Signal:   "Renovate Failed Runs",
				Resource: sample.Metric["project"],
				Value:    fmt.Sprintf("%.0f failed", sample.Value),
				Window:   "24h",
				Details:  "Recent Renovate executions failed for this repository.",
			})
		}
	})

	computeTrendQuery := fmt.Sprintf(`(count(max by(job, instance, namespace, pod) (1 - up) > 0) + count(sum by(namespace,pod) (increase(kube_pod_container_status_restarts_total[30m])) > %.0f) + count(max by(namespace,pod,phase) (kube_pod_status_phase{phase=~"Pending|Failed|Unknown"}) > 0)) or vector(0)`, s.cfg.Thresholds.RestartBurstThreshold)
	networkTrendQuery := `((count(cilium_controllers_failing > 0) + count(cilium_bpf_map_pressure > 0.10) + count(sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_xx{envoy_response_code_class=~"4|5"}[5m])) > 0.02) + count((sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_sum[5m])) / sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_count[5m]))) > 250))) or vector(0)`
	storageTrendQuery := `((count(volsync_volume_out_of_sync{role="source"} > 0) + count(increase(volsync_missed_intervals_total{role="source"}[6h]) > 0) + count(cnpg_pg_replication_lag > 30) + count((time() - cnpg_collector_last_failed_backup_timestamp) < 604800 and cnpg_collector_last_failed_backup_timestamp > 0))) or vector(0)`
	operatorTrendQuery := `((count(flux_resource_info{ready!="True"}) + count(increase(toolhive_vmcp_backend_errors_total[30m]) > 0) + count(increase(renovate_operator_run_failed[24h]) > 0) + count(externalsecret_status_condition{condition="Ready",status!="True"} == 1) + count(increase(externalsecret_sync_calls_error[30m]) > 0))) or vector(0)`

	recordRange(computeTrendQuery, &data.computeSignalTrend)
	recordRange(networkTrendQuery, &data.networkSignalTrend)
	recordRange(storageTrendQuery, &data.storageSignalTrend)
	recordRange(operatorTrendQuery, &data.operatorSignalTrend)
	recordRange(`100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[30m])))`, &data.cpuTrend)
	recordRange(`100 * (1 - (sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)))`, &data.memoryTrend)
	recordRange(`sum(kube_pod_status_phase{phase="Running"})`, &data.podTrend)
	recordRange(`sum(rustfs_capacity_current) or vector(0)`, &data.rustfsCapacityTrend)
	recordRange(`sum(
		sum by(job, namespace, pod) (cnpg_pg_database_size_bytes)
		* on(job, namespace, pod) group_left
		max by(job, namespace, pod) (cnpg_pg_replication_in_recovery == bool 0)
	) or vector(0)`, &data.cnpgSizeTrend)
	recordRange(`max(volsync_sync_duration_seconds{role="source",quantile="0.99"}) or vector(0)`, &data.volsyncDurationTrend)
	recordRange(`sum(rate(envoy_cluster_external_upstream_rq[5m])) or vector(0)`, &data.envoyTrafficTrend)

	data.cnpgClusterRows = buildCNPGClusterRows(cnpgSnapshots, time.Now())
	for _, row := range data.cnpgClusterRows {
		data.cnpgTotalSizeBytes += row.SizeBytes
		data.cnpgWALQueue += row.WALQueue
		if row.SecondsSinceArchival > data.cnpgMaxArchivalDelay {
			data.cnpgMaxArchivalDelay = row.SecondsSinceArchival
		}
		if row.HasRecentBackupFailure {
			data.cnpgRecentBackupFailures++
		}
	}

	if s.cfg.EnableKubernetes && s.kube != nil {
		resources, err := s.kube.FluxResources(ctx, 0, s.cfg.NamespaceAllowlist)
		if err != nil {
			errors = append(errors, fmt.Sprintf("kubernetes flux resources: %v", err))
		} else {
			data.fluxKinds = buildFluxKindsFromResources(resources)
			data.fluxRecent = buildFluxRecentRows(resources, time.Now())
		}

		sources, err := s.kube.VolsyncSources(ctx, 0, s.cfg.NamespaceAllowlist)
		if err != nil {
			errors = append(errors, fmt.Sprintf("kubernetes volsync sources: %v", err))
		} else {
			for _, source := range sources {
				snapshot := ensureVolsyncSnapshot(source.Namespace, source.Name)
				if snapshot == nil {
					continue
				}
				snapshot.Schedule = source.Schedule
				snapshot.SourcePVC = source.SourcePVC
				if source.Method != "" {
					snapshot.Method = source.Method
				}
				snapshot.Status = source.Status
				snapshot.Message = source.Message
				snapshot.LastResult = source.LastResult
				snapshot.LastSyncTime = source.LastSyncTime
				snapshot.NextSyncTime = source.NextSyncTime
				snapshot.LastSyncDuration = source.LastSyncDuration
			}
		}

		externalSecrets, err := s.kube.ExternalSecrets(ctx, 0, s.cfg.NamespaceAllowlist)
		if err != nil {
			errors = append(errors, fmt.Sprintf("kubernetes external secrets: %v", err))
		} else {
			for _, secret := range externalSecrets {
				snapshot := ensureExternalSecretSnapshot(secret.Namespace, secret.Name)
				if snapshot == nil {
					continue
				}
				snapshot.Store = secret.StoreRef
				snapshot.StoreKind = secret.StoreKind
				snapshot.RefreshInterval = secret.RefreshInterval
				snapshot.RefreshTime = secret.RefreshTime
				snapshot.Status = secret.Status
				snapshot.Message = secret.Message
			}
		}

		stores, err := s.kube.ClusterSecretStores(ctx)
		if err != nil {
			errors = append(errors, fmt.Sprintf("kubernetes cluster secret stores: %v", err))
		} else {
			data.externalSecretStoreTotal = float64(len(stores))
			for _, store := range stores {
				if store.Status == "Ready" {
					data.externalSecretStoreReady++
				}
			}
		}

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

	data.volsyncSourceRows = buildVolSyncSourceRows(volsyncSnapshots, time.Now())
	for _, row := range data.volsyncSourceRows {
		if row.LastSyncDurationSeconds > data.volsyncLongestDuration {
			data.volsyncLongestDuration = row.LastSyncDurationSeconds
			data.volsyncSlowestSource = row.Name
		}
	}

	data.externalSecretRows = buildExternalSecretRows(externalSecretSnapshots, time.Now())
	data.externalSecretTotal = float64(len(data.externalSecretRows))
	for _, row := range data.externalSecretRows {
		if !row.RefreshAt.IsZero() {
			age := time.Since(row.RefreshAt).Seconds()
			if age > data.externalSecretOldestRefresh {
				data.externalSecretOldestRefresh = age
			}
		}
	}

	data.envoyRouteRows = buildEnvoyRouteRows(envoyRouteSnapshots)
	maxEnvoyLatency := -1.0
	for _, row := range data.envoyRouteRows {
		if row.LatencyValue > maxEnvoyLatency {
			maxEnvoyLatency = row.LatencyValue
			data.envoyTopRoute = row.Route
		}
	}
	if data.envoyTopRoute == "" && len(data.envoyRouteRows) > 0 {
		data.envoyTopRoute = data.envoyRouteRows[0].Route
	}

	data.toolhiveBackendRows = buildToolhiveBackendRows(toolhiveBackendSnapshots)
	maxBackendLatency := -1.0
	for _, row := range data.toolhiveBackendRows {
		if row.LatencyValue > maxBackendLatency {
			maxBackendLatency = row.LatencyValue
			data.toolhiveSlowestBackend = row.Name
		}
	}
	if data.toolhiveSlowestBackend == "" && len(data.toolhiveBackendRows) > 0 {
		data.toolhiveSlowestBackend = data.toolhiveBackendRows[0].Name
	}

	data.renovateProjectRows = buildRenovateProjectRows(renovateProjectSnapshots)

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
	switch securityStatusTone(data) {
	case "good":
		return "Platform Integrity Stable"
	case "warning":
		return "Integrity Warnings Present"
	default:
		return "Integrity Signals Require Attention"
	}
}

func securityStatusTone(data sharedData) string {
	if data.fluxControllersUp == 0 {
		return "critical"
	}
	if data.fluxNotReady > 0 || data.externalSecretsDegraded > 0 || data.volsyncOutOfSync > 0 || data.cnpgMaxReplicationLag >= 120 {
		return "critical"
	}
	if data.fluxSuspended > 0 || data.fluxControllersDown > 0 || data.externalSecretSyncErrors24h > 0 || data.volsyncMissed24h > 0 || data.toolhiveBackendErrors24h > 0 || data.renovateRunsFailed > 0 || data.envoyErrorRate > 0 {
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
		fmt.Sprintf("%d of %d nodes are ready and %.0f of %.0f scrape targets are healthy.", int(data.nodesReady), int(data.nodesTotal), data.targetsHealthy, data.targetsTotal),
		fmt.Sprintf("Cluster CPU is at %.1f%% and memory at %.1f%%, with %.0f active namespaces and %.0f running pods.", data.clusterCPU, data.clusterMemory, data.namespaces, data.podsRunning),
	}
	switch {
	case len(data.anomalies) == 0:
		headlines = append(headlines, "No active anomaly rules are firing across compute, network, storage, or operators.")
	case countSeverity(data.anomalies, "critical") > 0:
		headlines = append(headlines, fmt.Sprintf("%d critical signals are active; review the hotspot list before drilling into operator pages.", countSeverity(data.anomalies, "critical")))
	default:
		headlines = append(headlines, fmt.Sprintf("%d active signals are present, concentrated around %s.", len(data.anomalies), strings.ToLower(anomalyTopCategoryLabel(data.anomalies))))
	}
	return headlines
}

func buildClusterHealthIndex(data sharedData) int {
	score := 100.0
	if data.nodesTotal > 0 {
		score -= (1 - (data.nodesReady / data.nodesTotal)) * 30
	}
	if data.targetsTotal > 0 {
		score -= (1 - (data.targetsHealthy / data.targetsTotal)) * 25
	}
	score -= math.Min(15, data.fluxNotReady+(data.fluxSuspended*0.5))
	score -= math.Min(10, data.externalSecretsDegraded+data.externalSecretSyncErrors24h)
	score -= math.Min(10, data.volsyncOutOfSync+data.volsyncMissed24h)
	score -= math.Min(20, float64(countSeverity(data.anomalies, "critical")*8+countSeverity(data.anomalies, "warning")*3))
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int(math.Round(score))
}

func clusterHealthTone(score int) string {
	switch {
	case score >= 90:
		return "good"
	case score >= 75:
		return "warning"
	default:
		return "critical"
	}
}

func buildHubSignals(signals []AnomalySignal) []AnomalySignal {
	if len(signals) <= 4 {
		return signals
	}
	return signals[:4]
}

func buildHubPaths(data sharedData) []HubPath {
	return []HubPath{
		{
			Label:  "Security Posture",
			Icon:   "security",
			Value:  fmt.Sprintf("%.0f suspended · %.0f degraded", data.fluxSuspended, data.externalSecretsDegraded+data.volsyncOutOfSync),
			Detail: "GitOps, secret sync, backup integrity, database HA, and edge health.",
			Path:   "/security",
			Tone:   securityStatusTone(data),
		},
		{
			Label:  "Anomaly Explorer",
			Icon:   "monitoring",
			Value:  fmt.Sprintf("%d active · %d critical", len(data.anomalies), countSeverity(data.anomalies, "critical")),
			Detail: "Cross-domain signals across compute, network, storage, and operators.",
			Path:   "/anomalies",
			Tone:   anomalyBannerTone(data.anomalies),
		},
		{
			Label:  "Forecasting",
			Icon:   "trending_up",
			Value:  fmt.Sprintf("%s RustFS · %s DB", formatBytes(data.rustfsCapacityCurrent), formatBytes(data.cnpgTotalSizeBytes)),
			Detail: "Storage runway, database growth, backup duration, and ingress traffic trends.",
			Path:   "/forecasting",
			Tone:   toneByIssueCounts(data.volsyncMissed24h, data.cnpgRecentBackupFailures),
		},
	}
}

func limitEventRows(rows []EventRow, limit int) []EventRow {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func forecastLabel(rustfs, cnpg projection) string {
	switch {
	case growthRatio(rustfs) >= 0.25 || growthRatio(cnpg) >= 0.25:
		return "Storage Growth Accelerating"
	case growthRatio(rustfs) >= 0.10 || growthRatio(cnpg) >= 0.10:
		return "Storage Growth Increasing"
	default:
		return "Storage Trend Stable"
	}
}

func forecastBannerTone(rustfs, cnpg projection) string {
	if growthRatio(rustfs) >= 0.25 || growthRatio(cnpg) >= 0.25 {
		return "critical"
	}
	if growthRatio(rustfs) >= 0.10 || growthRatio(cnpg) >= 0.10 {
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
	current   float64
	projected float64
	delta     float64
	hours     float64
	slope     float64
	summary   string
}

func projectSeries(values []float64) projection {
	return projectSeriesHours(values, 24)
}

func projectSeriesHours(values []float64, hours float64) projection {
	if len(values) == 0 {
		return projection{hours: hours, summary: "insufficient data"}
	}
	if len(values) == 1 {
		return projection{current: values[0], projected: values[0], hours: hours, summary: "flat sample"}
	}

	first := values[0]
	last := values[len(values)-1]
	slopePerHour := (last - first) / float64(len(values)-1)
	projected := math.Max(0, last+(slopePerHour*hours))
	summary := "stable"
	switch {
	case slopePerHour > 1:
		summary = fmt.Sprintf("rising %.2f units/hour", slopePerHour)
	case slopePerHour < -1:
		summary = fmt.Sprintf("falling %.2f units/hour", math.Abs(slopePerHour))
	default:
		summary = fmt.Sprintf("near-flat %.2f units/hour", slopePerHour)
	}

	return projection{
		current:   last,
		projected: projected,
		delta:     projected - last,
		hours:     hours,
		slope:     slopePerHour,
		summary:   summary,
	}
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

func buildBytesSparkline(label string, values []float64, detail string) SparklineCard {
	path := sparklinePath(values, 180, 56)
	latest := "n/a"
	delta := "insufficient data"
	if len(values) > 0 {
		latest = formatBytes(values[len(values)-1])
	}
	if len(values) > 1 {
		delta = fmt.Sprintf("%s over 24h", formatSignedBytes(values[len(values)-1]-values[0]))
	}

	return SparklineCard{
		Label:  label,
		Path:   path,
		Latest: latest,
		Delta:  delta,
		Detail: detail,
		Tone:   "info",
		Scale:  scaleLabelsForSeries(values, formatBytes),
	}
}

func buildDurationSparkline(label string, values []float64, detail string) SparklineCard {
	path := sparklinePath(values, 180, 56)
	latest := "n/a"
	delta := "insufficient data"
	tone := "good"
	if len(values) > 0 {
		latest = formatDurationShort(time.Duration(values[len(values)-1] * float64(time.Second)))
		tone = forecastTone(values[len(values)-1], 60)
	}
	if len(values) > 1 {
		delta = fmt.Sprintf("%s over 24h", formatSignedDurationSeconds(values[len(values)-1]-values[0]))
	}

	return SparklineCard{
		Label:  label,
		Path:   path,
		Latest: latest,
		Delta:  delta,
		Detail: detail,
		Tone:   tone,
		Scale:  scaleLabelsForSeries(values, formatSeconds),
	}
}

func buildRateSparkline(label string, values []float64, detail string) SparklineCard {
	path := sparklinePath(values, 180, 56)
	latest := "n/a"
	delta := "insufficient data"
	if len(values) > 0 {
		latest = fmt.Sprintf("%s req/s", formatRate(values[len(values)-1]))
	}
	if len(values) > 1 {
		delta = fmt.Sprintf("%s req/s vs start", formatSignedRate(values[len(values)-1]-values[0]))
	}

	return SparklineCard{
		Label:  label,
		Path:   path,
		Latest: latest,
		Delta:  delta,
		Detail: detail,
		Tone:   "good",
		Scale:  scaleLabelsForSeries(values, formatRate),
	}
}

func buildAnomalySparkline(label string, values []float64, detail string) SparklineCard {
	path := sparklinePath(values, 180, 56)
	latest := "0"
	delta := "flat"
	tone := "good"
	if len(values) > 0 {
		current := math.Round(values[len(values)-1])
		latest = fmt.Sprintf("%.0f active", current)
		switch {
		case current >= 4:
			tone = "critical"
		case current > 0:
			tone = "warning"
		}
	}
	if len(values) > 1 {
		delta = fmt.Sprintf("%+.0f vs start", math.Round(values[len(values)-1]-values[0]))
	}

	return SparklineCard{
		Label:  label,
		Path:   path,
		Latest: latest,
		Delta:  delta,
		Detail: detail,
		Tone:   tone,
	}
}

func buildAnomalyChart(data sharedData) AnomalyChart {
	maxValue := maxSeriesValue(
		data.computeSignalTrend,
		data.networkSignalTrend,
		data.storageSignalTrend,
		data.operatorSignalTrend,
	)

	return AnomalyChart{
		Series: []ChartSeries{
			{
				Label: "Compute",
				Path:  sparklinePathRange(data.computeSignalTrend, 100, 100, 0, maxValue),
				Tone:  "info",
				Value: latestTrendLabel(data.computeSignalTrend),
			},
			{
				Label: "Network",
				Path:  sparklinePathRange(data.networkSignalTrend, 100, 100, 0, maxValue),
				Tone:  "critical",
				Value: latestTrendLabel(data.networkSignalTrend),
			},
			{
				Label: "Storage",
				Path:  sparklinePathRange(data.storageSignalTrend, 100, 100, 0, maxValue),
				Tone:  "warning",
				Value: latestTrendLabel(data.storageSignalTrend),
			},
			{
				Label: "Operators",
				Path:  sparklinePathRange(data.operatorSignalTrend, 100, 100, 0, maxValue),
				Tone:  "good",
				Value: latestTrendLabel(data.operatorSignalTrend),
			},
		},
		Labels: []string{"-24h", "-18h", "-12h", "-6h", "now"},
		Scale:  scaleLabelsFromRange(0, maxValue, func(value float64) string { return fmt.Sprintf("%.0f", value) }),
	}
}

func buildAnomalyEvents(signals []AnomalySignal) []AnomalyEvent {
	events := make([]AnomalyEvent, 0, minInt(len(signals), 6))
	for _, signal := range signals {
		events = append(events, AnomalyEvent{
			Label:    signal.Signal,
			Resource: signal.Resource,
			Detail:   signal.Details,
			Meta:     signal.Window + " · " + signal.Value,
			Icon:     anomalyCategoryIcon(signal.Category, signal.Severity),
			Tone:     signal.Severity,
		})
		if len(events) == 6 {
			break
		}
	}
	return events
}

func buildAnomalyDomainCards(data sharedData) []StatCard {
	return []StatCard{
		{
			Label:  "Compute",
			Value:  latestTrendLabel(data.computeSignalTrend),
			Detail: "scrapes, restarts, and workload state",
			Tone:   trendTone(data.computeSignalTrend),
		},
		{
			Label:  "Network",
			Value:  latestTrendLabel(data.networkSignalTrend),
			Detail: "cilium datapath and envoy routes",
			Tone:   trendTone(data.networkSignalTrend),
		},
		{
			Label:  "Storage",
			Value:  latestTrendLabel(data.storageSignalTrend),
			Detail: "replication, backups, and sync drift",
			Tone:   trendTone(data.storageSignalTrend),
		},
		{
			Label:  "Operators",
			Value:  latestTrendLabel(data.operatorSignalTrend),
			Detail: "flux, secrets, toolhive, renovate",
			Tone:   trendTone(data.operatorSignalTrend),
		},
	}
}

func buildAnomalyHotspots(signals []AnomalySignal) []ResourceStat {
	type hotspot struct {
		count      int
		severity   int
		categories map[string]struct{}
	}

	hotspots := map[string]*hotspot{}
	for _, signal := range signals {
		resource := strings.TrimSpace(signal.Resource)
		if resource == "" {
			continue
		}
		item, ok := hotspots[resource]
		if !ok {
			item = &hotspot{categories: map[string]struct{}{}}
			hotspots[resource] = item
		}
		item.count++
		if rank := severityRank(signal.Severity); rank < item.severity || item.count == 1 {
			item.severity = rank
		}
		item.categories[signal.Category] = struct{}{}
	}

	rows := make([]ResourceStat, 0, len(hotspots))
	for resource, item := range hotspots {
		categories := make([]string, 0, len(item.categories))
		for category := range item.categories {
			categories = append(categories, category)
		}
		sort.Strings(categories)
		tone := "good"
		switch item.severity {
		case 0:
			tone = "critical"
		case 1:
			tone = "warning"
		}
		rows = append(rows, ResourceStat{
			Name:   resource,
			Value:  fmt.Sprintf("%d", item.count),
			Detail: strings.Join(categories, " · "),
			Tone:   tone,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		leftCount := parseFloatDefault(rows[i].Value, 0)
		rightCount := parseFloatDefault(rows[j].Value, 0)
		if leftCount != rightCount {
			return leftCount > rightCount
		}
		return rows[i].Name < rows[j].Name
	})
	if len(rows) > 6 {
		rows = rows[:6]
	}
	return rows
}

func latestTrendLabel(values []float64) string {
	if len(values) == 0 {
		return "0"
	}
	return fmt.Sprintf("%.0f", math.Round(values[len(values)-1]))
}

func trendTone(values []float64) string {
	if len(values) == 0 {
		return "good"
	}
	current := values[len(values)-1]
	switch {
	case current >= 4:
		return "critical"
	case current > 0:
		return "warning"
	default:
		return "good"
	}
}

func anomalyTopCategoryLabel(signals []AnomalySignal) string {
	if len(signals) == 0 {
		return "None"
	}
	counts := map[string]int{}
	for _, signal := range signals {
		counts[signal.Category]++
	}
	bestLabel := "None"
	bestCount := -1
	for _, category := range []string{"Compute", "Network", "Storage", "Operators"} {
		if counts[category] > bestCount {
			bestCount = counts[category]
			bestLabel = category
		}
	}
	return bestLabel
}

func anomalyTopCategoryTone(signals []AnomalySignal) string {
	top := anomalyTopCategoryLabel(signals)
	for _, signal := range signals {
		if signal.Category == top {
			if signal.Severity == "critical" {
				return "critical"
			}
			if signal.Severity == "warning" {
				return "warning"
			}
		}
	}
	return "info"
}

func anomalyTopResourceLabel(signals []AnomalySignal) string {
	if len(signals) == 0 {
		return "None"
	}
	counts := map[string]int{}
	best := ""
	bestCount := -1
	for _, signal := range signals {
		resource := strings.TrimSpace(signal.Resource)
		if resource == "" {
			continue
		}
		counts[resource]++
		if counts[resource] > bestCount {
			best = resource
			bestCount = counts[resource]
		}
	}
	if best == "" {
		return "None"
	}
	return best
}

func anomalyCategoryIcon(category, severity string) string {
	switch category {
	case "Compute":
		return "memory"
	case "Network":
		return "network_node"
	case "Storage":
		return "database"
	case "Operators":
		return "sync_problem"
	}
	return anomalySeverityIcon(severity)
}

func anomalySeverityIcon(severity string) string {
	switch severity {
	case "critical":
		return "error"
	case "warning":
		return "warning"
	default:
		return "info"
	}
}

func sparklinePath(values []float64, width, height float64) string {
	if len(values) == 0 {
		return ""
	}

	min, max := seriesMinMax(values)
	return sparklinePathRange(values, width, height, min, max)
}

func sparklinePathRange(values []float64, width, height, min, max float64) string {
	if len(values) == 0 {
		return ""
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

func seriesMinMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
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
	return min, max
}

func maxSeriesValue(series ...[]float64) float64 {
	max := 0.0
	for _, values := range series {
		for _, value := range values {
			if value > max {
				max = value
			}
		}
	}
	if max == 0 {
		return 1
	}
	return max
}

func scaleLabelsForSeries(values []float64, formatter func(float64) string) []string {
	min, max := seriesMinMax(values)
	return scaleLabelsFromRange(min, max, formatter)
}

func scaleLabelsFromRange(min, max float64, formatter func(float64) string) []string {
	if max < min {
		max, min = min, max
	}
	mid := min + ((max - min) / 2)
	return []string{
		formatter(max),
		formatter(mid),
		formatter(min),
	}
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

func countDistinctCategories(signals []AnomalySignal) int {
	seen := make(map[string]struct{}, len(signals))
	for _, signal := range signals {
		if signal.Category == "" {
			continue
		}
		seen[signal.Category] = struct{}{}
	}
	return len(seen)
}

func countDistinctResources(signals []AnomalySignal) int {
	seen := make(map[string]struct{}, len(signals))
	for _, signal := range signals {
		if signal.Resource == "" {
			continue
		}
		seen[signal.Resource] = struct{}{}
	}
	return len(seen)
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

func parseEnvoyRoute(value string) (string, string) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "httproute/") {
		return "", value
	}
	trimmed := strings.TrimPrefix(value, "httproute/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return "", trimmed
	}
	return parts[0], parts[1]
}

func buildFluxCards(kinds []KindStatus) []StatCard {
	byKind := make(map[string]KindStatus, len(kinds))
	for _, kind := range kinds {
		byKind[kind.Kind] = kind
	}

	sourceTotal := KindStatus{Kind: "Sources"}
	for _, sourceKind := range []string{"GitRepository", "OCIRepository"} {
		kind := byKind[sourceKind]
		sourceTotal.Ready += kind.Ready
		sourceTotal.NotReady += kind.NotReady
		sourceTotal.Suspended += kind.Suspended
		sourceTotal.Total += kind.Total
	}
	sourceTotal.Tone = toneByIssueCounts(float64(sourceTotal.NotReady), float64(sourceTotal.Suspended))

	order := []struct {
		key   string
		label string
	}{
		{key: "Kustomization", label: "Kustomizations"},
		{key: "HelmRelease", label: "Helm Releases"},
		{key: "Sources", label: "Sources"},
		{key: "GitRepository", label: "Git Repositories"},
		{key: "OCIRepository", label: "OCI Repositories"},
	}

	cards := make([]StatCard, 0, len(order))
	for _, item := range order {
		kind := sourceTotal
		if item.key != "Sources" {
			kind = byKind[item.key]
		}
		detail := fmt.Sprintf("%d ready", kind.Ready)
		switch {
		case kind.NotReady > 0 && kind.Suspended > 0:
			detail = fmt.Sprintf("%d ready · %d drifted · %d suspended", kind.Ready, kind.NotReady, kind.Suspended)
		case kind.NotReady > 0:
			detail = fmt.Sprintf("%d ready · %d drifted", kind.Ready, kind.NotReady)
		case kind.Suspended > 0:
			detail = fmt.Sprintf("%d ready · %d suspended", kind.Ready, kind.Suspended)
		}
		cards = append(cards, StatCard{
			Label:  item.label,
			Value:  fmt.Sprintf("%d", kind.Total),
			Detail: detail,
			Tone:   toneByIssueCounts(float64(kind.NotReady), float64(kind.Suspended)),
		})
	}
	return cards
}

func buildFluxKindsFromResources(resources []kube.FluxResource) []KindStatus {
	byKind := map[string]*KindStatus{}
	for _, resource := range resources {
		entry, ok := byKind[resource.Kind]
		if !ok {
			entry = &KindStatus{Kind: resource.Kind}
			byKind[resource.Kind] = entry
		}
		entry.Total++
		switch {
		case resource.Suspended:
			entry.Suspended++
		case resource.Ready:
			entry.Ready++
		default:
			entry.NotReady++
		}
	}

	result := make([]KindStatus, 0, len(byKind))
	for _, entry := range byKind {
		switch {
		case entry.NotReady > 0:
			entry.Status = "Drift"
			entry.Tone = "critical"
		case entry.Suspended > 0:
			entry.Status = "Paused"
			entry.Tone = "warning"
		default:
			entry.Status = "Ready"
			entry.Tone = "good"
		}
		result = append(result, *entry)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Kind < result[j].Kind
	})

	return result
}

func buildFluxRecentRows(resources []kube.FluxResource, now time.Time) []FluxRecentRow {
	rows := make([]FluxRecentRow, 0, 16)
	for _, resource := range resources {
		if resource.Kind != "HelmRelease" || resource.LastTransition.IsZero() {
			continue
		}
		rows = append(rows, FluxRecentRow{
			Kind:      resource.Kind,
			Name:      resource.Name,
			Namespace: resource.Namespace,
			Status:    resource.Status,
			Age:       compactDuration(now.Sub(resource.LastTransition)),
			Tone:      fluxStatusTone(resource.Status),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

type cnpgClusterSnapshot struct {
	Name                 string
	Namespace            string
	Instances            float64
	StreamingReplicas    float64
	ReplicationLag       float64
	DatabaseSizeBytes    float64
	LastFailedBackup     float64
	WALReadyQueue        float64
	SecondsSinceArchival float64
}

type volsyncSourceSnapshot struct {
	Name             string
	Namespace        string
	Method           string
	SourcePVC        string
	Schedule         string
	Status           string
	Message          string
	LastResult       string
	LastSyncTime     time.Time
	NextSyncTime     time.Time
	LastSyncDuration time.Duration
	Missed24h        float64
	OutOfSync        bool
}

type externalSecretSnapshot struct {
	Name            string
	Namespace       string
	Store           string
	StoreKind       string
	RefreshInterval time.Duration
	RefreshTime     time.Time
	Status          string
	Message         string
	Ready           bool
	Errors24h       float64
}

type envoyRouteSnapshot struct {
	Name        string
	Namespace   string
	Route       string
	RequestRate float64
	ErrorRate   float64
	LatencyMs   float64
}

type toolhiveBackendSnapshot struct {
	Name           string
	Requests24h    float64
	Errors24h      float64
	LatencySeconds float64
}

type renovateProjectSnapshot struct {
	Name             string
	Executions24h    float64
	DependencyIssues float64
	RunFailed        float64
}

func buildCNPGCards(data sharedData) []StatCard {
	backupTone := "good"
	backupDetail := "No recent backup failures"
	if data.cnpgRecentBackupFailures > 0 {
		backupTone = "warning"
		backupDetail = fmt.Sprintf("%.0f recent backup failures", data.cnpgRecentBackupFailures)
	}
	if data.cnpgWALQueue > 0 {
		backupTone = "critical"
		backupDetail = fmt.Sprintf("%.0f WAL files waiting to archive", data.cnpgWALQueue)
	} else if data.cnpgMaxArchivalDelay > 0 {
		backupDetail = fmt.Sprintf("oldest archive activity %s ago", compactDuration(time.Duration(data.cnpgMaxArchivalDelay)*time.Second))
	}

	return []StatCard{
		{
			Label:  "Clusters",
			Value:  fmt.Sprintf("%.0f", data.cnpgClusters),
			Detail: fmt.Sprintf("%s total database footprint", formatBytes(data.cnpgTotalSizeBytes)),
			Tone:   "info",
		},
		{
			Label:  "Instances",
			Value:  fmt.Sprintf("%.0f", data.cnpgInstances),
			Detail: fmt.Sprintf("pods across %.0f monitored clusters", data.cnpgClusters),
			Tone:   toneByRatio(data.cnpgInstances, maxFloat(data.cnpgClusters*3, 1)),
		},
		{
			Label:  "Replication Lag",
			Value:  formatSeconds(data.cnpgMaxReplicationLag),
			Detail: "max observed replica lag",
			Tone:   toneByThreshold(data.cnpgMaxReplicationLag, 30),
		},
		{
			Label:  "WAL & Backups",
			Value:  fmt.Sprintf("%.0f queued", data.cnpgWALQueue),
			Detail: backupDetail,
			Tone:   backupTone,
		},
	}
}

func buildVolSyncCards(data sharedData) []StatCard {
	slowest := "No recent duration data"
	slowestTone := "good"
	if data.volsyncLongestDuration > 0 {
		slowest = formatDurationShort(time.Duration(data.volsyncLongestDuration * float64(time.Second)))
		if data.volsyncSlowestSource != "" {
			slowest += " · " + data.volsyncSlowestSource
		}
		if data.volsyncLongestDuration >= 60 {
			slowestTone = "warning"
		}
	}

	return []StatCard{
		{
			Label:  "Replication Sources",
			Value:  fmt.Sprintf("%.0f", data.volsyncSources),
			Detail: "scheduled backup sources",
			Tone:   "info",
		},
		{
			Label:  "Drifted Sources",
			Value:  fmt.Sprintf("%.0f", data.volsyncOutOfSync),
			Detail: "out of sync with backup target",
			Tone:   toneByIssueCounts(data.volsyncOutOfSync, 0),
		},
		{
			Label:  "Missed Schedules",
			Value:  fmt.Sprintf("%.0f", data.volsyncMissed24h),
			Detail: "missed intervals in the last 24h",
			Tone:   toneByIssueCounts(data.volsyncMissed24h, 0),
		},
		{
			Label:  "Longest Sync",
			Value:  slowest,
			Detail: "last observed backup duration",
			Tone:   slowestTone,
		},
	}
}

func buildExternalSecretCards(data sharedData) []StatCard {
	oldestRefresh := "no refresh data"
	if data.externalSecretOldestRefresh > 0 {
		oldestRefresh = compactDuration(time.Duration(data.externalSecretOldestRefresh) * time.Second)
	}

	return []StatCard{
		{
			Label:  "External Secrets",
			Value:  fmt.Sprintf("%.0f", data.externalSecretTotal),
			Detail: "managed secret sync resources",
			Tone:   "info",
		},
		{
			Label:  "Ready",
			Value:  fmt.Sprintf("%.0f", data.externalSecretsReady),
			Detail: fmt.Sprintf("%.0f degraded", data.externalSecretsDegraded),
			Tone:   toneByIssueCounts(data.externalSecretsDegraded, 0),
		},
		{
			Label:  "Sync Errors",
			Value:  fmt.Sprintf("%.0f", data.externalSecretSyncErrors24h),
			Detail: "provider sync errors in 24h",
			Tone:   toneByIssueCounts(data.externalSecretSyncErrors24h, 0),
		},
		{
			Label:  "Cluster Stores",
			Value:  fmt.Sprintf("%.0f / %.0f", data.externalSecretStoreReady, data.externalSecretStoreTotal),
			Detail: "oldest refresh " + oldestRefresh + " ago",
			Tone:   toneByIssueCounts(data.externalSecretStoreTotal-data.externalSecretStoreReady, 0),
		},
	}
}

func buildEnvoyCards(data sharedData) []StatCard {
	topRoute := "no active routes"
	if data.envoyTopRoute != "" {
		topRoute = data.envoyTopRoute
	}
	return []StatCard{
		{
			Label:  "Active Routes",
			Value:  fmt.Sprintf("%.0f", data.envoyActiveRoutes),
			Detail: "routes with live traffic in the last 5m",
			Tone:   "info",
		},
		{
			Label:  "Request Rate",
			Value:  fmt.Sprintf("%s req/s", formatRate(data.envoyRequestRate)),
			Detail: "aggregate upstream request rate",
			Tone:   "good",
		},
		{
			Label:  "Error Rate",
			Value:  fmt.Sprintf("%s err/s", formatRate(data.envoyErrorRate)),
			Detail: "upstream 4xx/5xx responses",
			Tone:   toneByIssueCounts(data.envoyErrorRate, 0),
		},
		{
			Label:  "P95 Latency",
			Value:  formatMilliseconds(data.envoyP95Latency),
			Detail: topRoute,
			Tone:   toneByThreshold(data.envoyP95Latency, 250),
		},
	}
}

func buildToolhiveCards(data sharedData) []StatCard {
	slowest := "no backend timings"
	slowestTone := "good"
	if data.toolhiveSlowestBackend != "" {
		slowest = fmt.Sprintf("%s · %s", data.toolhiveSlowestBackend, formatSeconds(data.toolhiveAvgBackendLatency))
		if data.toolhiveAvgBackendLatency >= 0.2 {
			slowestTone = "warning"
		}
	}

	return []StatCard{
		{
			Label:  "Active Connections",
			Value:  fmt.Sprintf("%.0f", data.toolhiveConnections),
			Detail: "gateway-side live MCP sessions",
			Tone:   "info",
		},
		{
			Label:  "Backends",
			Value:  fmt.Sprintf("%.0f", data.toolhiveBackends),
			Detail: "discovered VMCP-backed workloads",
			Tone:   "good",
		},
		{
			Label:  "Backend Errors",
			Value:  fmt.Sprintf("%.0f", data.toolhiveBackendErrors24h),
			Detail: "errors returned in the last 24h",
			Tone:   toneByIssueCounts(data.toolhiveBackendErrors24h, 0),
		},
		{
			Label:  "Avg Backend Latency",
			Value:  formatSeconds(data.toolhiveAvgBackendLatency),
			Detail: slowest,
			Tone:   slowestTone,
		},
	}
}

func buildRenovateCards(data sharedData) []StatCard {
	return []StatCard{
		{
			Label:  "Projects",
			Value:  fmt.Sprintf("%.0f", data.renovateProjects),
			Detail: "repositories under Renovate management",
			Tone:   "info",
		},
		{
			Label:  "Executions",
			Value:  fmt.Sprintf("%.0f", data.renovateExecutions24h),
			Detail: "runs over the last 24h",
			Tone:   "good",
		},
		{
			Label:  "Failed Runs",
			Value:  fmt.Sprintf("%.0f", data.renovateRunsFailed),
			Detail: "currently failing projects",
			Tone:   toneByIssueCounts(data.renovateRunsFailed, 0),
		},
		{
			Label:  "Dependency Issues",
			Value:  fmt.Sprintf("%.0f", data.renovateDependencyIssues),
			Detail: "projects with unresolved dependency issues",
			Tone:   toneByIssueCounts(data.renovateDependencyIssues, 0),
		},
	}
}

func buildCNPGClusterRows(snapshots map[string]*cnpgClusterSnapshot, now time.Time) []CNPGClusterRow {
	rows := make([]CNPGClusterRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		backup := "Healthy"
		hasRecentBackupFailure := false
		backupTone := "good"
		if snapshot.LastFailedBackup > 0 {
			failedAgo := now.Sub(time.Unix(int64(snapshot.LastFailedBackup), 0))
			if failedAgo <= 24*time.Hour {
				backupTone = "critical"
				hasRecentBackupFailure = true
				backup = "Attention"
			} else if failedAgo <= 72*time.Hour {
				backupTone = "warning"
				hasRecentBackupFailure = true
				backup = "Attention"
			}
		}
		if snapshot.WALReadyQueue > 0 {
			backup = fmt.Sprintf("%.0f queued", snapshot.WALReadyQueue)
			backupTone = "critical"
		}

		tone := toneByThreshold(snapshot.ReplicationLag, 30)
		if backupTone == "critical" || snapshot.WALReadyQueue > 0 {
			tone = "critical"
		} else if tone != "critical" && backupTone == "warning" {
			tone = "warning"
		}

		rows = append(rows, CNPGClusterRow{
			Name:                   snapshot.Name,
			Namespace:              snapshot.Namespace,
			Detail:                 "",
			Replicas:               fmt.Sprintf("%.0f", snapshot.Instances),
			Lag:                    formatSeconds(snapshot.ReplicationLag),
			Backup:                 backup,
			Size:                   formatBytes(snapshot.DatabaseSizeBytes),
			Tone:                   tone,
			BackupTone:             backupTone,
			SizeBytes:              snapshot.DatabaseSizeBytes,
			WALQueue:               snapshot.WALReadyQueue,
			SecondsSinceArchival:   snapshot.SecondsSinceArchival,
			HasRecentBackupFailure: hasRecentBackupFailure,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})

	return rows
}

func buildVolSyncSourceRows(snapshots map[string]*volsyncSourceSnapshot, now time.Time) []VolSyncSourceRow {
	rows := make([]VolSyncSourceRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		tone := "good"
		status := snapshot.Status
		if status == "" {
			status = "Healthy"
		}

		switch {
		case snapshot.OutOfSync || strings.EqualFold(snapshot.LastResult, "Failed"):
			tone = "critical"
			if snapshot.OutOfSync {
				status = "Drifted"
			} else {
				status = "Failed"
			}
		case snapshot.Missed24h > 0:
			tone = "warning"
			status = "Missed"
		case !snapshot.NextSyncTime.IsZero() && snapshot.NextSyncTime.Before(now) &&
			(snapshot.LastSyncTime.IsZero() || snapshot.LastSyncTime.Before(snapshot.NextSyncTime)):
			tone = "warning"
			status = "Overdue"
		case status == "Synchronizing":
			tone = "warning"
		}

		schedule := snapshot.Schedule
		if schedule == "" {
			schedule = "manual"
		}

		lastSync := "never"
		if !snapshot.LastSyncTime.IsZero() {
			lastSync = compactDuration(now.Sub(snapshot.LastSyncTime)) + " ago"
		}

		duration := "n/a"
		if snapshot.LastSyncDuration > 0 {
			duration = formatDurationShort(snapshot.LastSyncDuration)
		}

		detailParts := []string{}
		if snapshot.Method != "" {
			detailParts = append(detailParts, snapshot.Method)
		}
		if snapshot.SourcePVC != "" {
			detailParts = append(detailParts, "pvc "+snapshot.SourcePVC)
		}
		if !snapshot.NextSyncTime.IsZero() {
			if snapshot.NextSyncTime.After(now) {
				detailParts = append(detailParts, "next in "+compactDuration(snapshot.NextSyncTime.Sub(now)))
			} else {
				detailParts = append(detailParts, "overdue by "+compactDuration(now.Sub(snapshot.NextSyncTime)))
			}
		}

		rows = append(rows, VolSyncSourceRow{
			Name:                    snapshot.Name,
			Namespace:               snapshot.Namespace,
			Detail:                  strings.Join(detailParts, " · "),
			Schedule:                schedule,
			LastSync:                lastSync,
			Duration:                duration,
			Status:                  status,
			Tone:                    tone,
			NextSync:                snapshot.NextSyncTime,
			LastSyncAt:              snapshot.LastSyncTime,
			LastSyncDurationSeconds: snapshot.LastSyncDuration.Seconds(),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})

	return rows
}

func buildExternalSecretRows(snapshots map[string]*externalSecretSnapshot, now time.Time) []ExternalSecretRow {
	rows := make([]ExternalSecretRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		tone := "good"
		status := snapshot.Status
		if status == "" {
			if snapshot.Ready {
				status = "Ready"
			} else {
				status = "Pending"
			}
		}

		if !snapshot.Ready || status == "Error" {
			tone = "critical"
			status = "Error"
		} else if snapshot.Errors24h > 0 {
			tone = "warning"
			status = "Retries"
		} else if snapshot.RefreshInterval > 0 && !snapshot.RefreshTime.IsZero() && now.Sub(snapshot.RefreshTime) > snapshot.RefreshInterval*2 {
			tone = "warning"
			status = "Stale"
		}

		refresh := "never"
		if !snapshot.RefreshTime.IsZero() {
			refresh = compactDuration(now.Sub(snapshot.RefreshTime)) + " ago"
		}

		interval := "manual"
		if snapshot.RefreshInterval > 0 {
			interval = formatDurationShort(snapshot.RefreshInterval)
		}

		detailParts := []string{}
		if snapshot.Store != "" {
			detailParts = append(detailParts, strings.TrimSpace(snapshot.StoreKind+" "+snapshot.Store))
		}
		if snapshot.Message != "" {
			detailParts = append(detailParts, snapshot.Message)
		}
		if snapshot.Errors24h > 0 {
			detailParts = append(detailParts, fmt.Sprintf("%.0f sync errors/24h", snapshot.Errors24h))
		}

		rows = append(rows, ExternalSecretRow{
			Name:      snapshot.Name,
			Namespace: snapshot.Namespace,
			Store:     snapshot.Store,
			Refresh:   refresh,
			Interval:  interval,
			Status:    status,
			Detail:    strings.Join(detailParts, " · "),
			Tone:      tone,
			RefreshAt: snapshot.RefreshTime,
			Errors24h: snapshot.Errors24h,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func buildEnvoyRouteRows(snapshots map[string]*envoyRouteSnapshot) []EnvoyRouteRow {
	rows := make([]EnvoyRouteRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		tone := "good"
		status := "Healthy"
		if snapshot.ErrorRate > 0 {
			tone = "critical"
			status = "Errors"
		} else if snapshot.LatencyMs >= 100 {
			tone = "warning"
			status = "Slow"
		}
		rows = append(rows, EnvoyRouteRow{
			Name:             snapshot.Name,
			Namespace:        snapshot.Namespace,
			Route:            snapshot.Route,
			RequestRate:      fmt.Sprintf("%s req/s", formatRate(snapshot.RequestRate)),
			ErrorRate:        fmt.Sprintf("%s err/s", formatRate(snapshot.ErrorRate)),
			Latency:          formatMilliseconds(snapshot.LatencyMs),
			Status:           status,
			Detail:           snapshot.Name,
			Tone:             tone,
			RequestRateValue: snapshot.RequestRate,
			ErrorRateValue:   snapshot.ErrorRate,
			LatencyValue:     snapshot.LatencyMs,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Route < rows[j].Route
	})
	return rows
}

func buildToolhiveBackendRows(snapshots map[string]*toolhiveBackendSnapshot) []ToolhiveBackendRow {
	rows := make([]ToolhiveBackendRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		tone := "good"
		status := "Healthy"
		if snapshot.Errors24h > 0 {
			tone = "critical"
			status = "Errors"
		} else if snapshot.LatencySeconds >= 0.2 {
			tone = "warning"
			status = "Slow"
		}

		rows = append(rows, ToolhiveBackendRow{
			Name:             snapshot.Name,
			Requests24h:      fmt.Sprintf("%.0f", math.Round(snapshot.Requests24h)),
			Errors24h:        fmt.Sprintf("%.0f", math.Round(snapshot.Errors24h)),
			Latency:          formatSeconds(snapshot.LatencySeconds),
			Status:           status,
			Detail:           "VMCP backend",
			Tone:             tone,
			Requests24hValue: snapshot.Requests24h,
			Errors24hValue:   snapshot.Errors24h,
			LatencyValue:     snapshot.LatencySeconds,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		leftRank := severityRank(rows[i].Tone)
		rightRank := severityRank(rows[j].Tone)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if rows[i].Errors24hValue != rows[j].Errors24hValue {
			return rows[i].Errors24hValue > rows[j].Errors24hValue
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func buildRenovateProjectRows(snapshots map[string]*renovateProjectSnapshot) []RenovateProjectRow {
	rows := make([]RenovateProjectRow, 0, len(snapshots))
	for _, snapshot := range snapshots {
		tone := "good"
		status := "Healthy"
		if snapshot.RunFailed > 0 {
			tone = "critical"
			status = "Failed"
		} else if snapshot.DependencyIssues > 0 {
			tone = "warning"
			status = "Issues"
		}
		rows = append(rows, RenovateProjectRow{
			Name:               snapshot.Name,
			Executions24h:      fmt.Sprintf("%.0f", math.Round(snapshot.Executions24h)),
			Issues:             fmt.Sprintf("%.0f", math.Round(snapshot.DependencyIssues)),
			Status:             status,
			Detail:             "dependency automation",
			Tone:               tone,
			Executions24hValue: snapshot.Executions24h,
			IssueCount:         snapshot.DependencyIssues,
			Failed:             snapshot.RunFailed,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		leftRank := severityRank(rows[i].Tone)
		rightRank := severityRank(rows[j].Tone)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if rows[i].Executions24hValue != rows[j].Executions24hValue {
			return rows[i].Executions24hValue > rows[j].Executions24hValue
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func parsePercentage(value string) float64 {
	trimmed := strings.TrimSuffix(strings.TrimSpace(value), "%")
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseFloatDefault(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func growthRatio(proj projection) float64 {
	if proj.current <= 0 {
		if proj.projected > 0 {
			return 1
		}
		return 0
	}
	return math.Max(0, (proj.projected-proj.current)/proj.current)
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

func formatRate(value float64) string {
	return fmt.Sprintf("%.1f", value)
}

func formatBytes(value float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	scaled := value
	unit := units[0]
	for i := 0; i < len(units)-1 && scaled >= 1024; i++ {
		scaled /= 1024
		unit = units[i+1]
	}
	if unit == "B" {
		return fmt.Sprintf("%.0f%s", scaled, unit)
	}
	return fmt.Sprintf("%.1f%s", scaled, unit)
}

func formatSignedBytes(value float64) string {
	sign := "+"
	if value < 0 {
		sign = "-"
		value = math.Abs(value)
	}
	return sign + formatBytes(value)
}

func formatMilliseconds(value float64) string {
	return fmt.Sprintf("%.0fms", value)
}

func formatDurationShort(value time.Duration) string {
	if value <= 0 {
		return "0s"
	}
	if value < time.Second {
		return fmt.Sprintf("%.0fms", float64(value)/float64(time.Millisecond))
	}
	if value < time.Minute {
		return fmt.Sprintf("%.0fs", value.Round(time.Second).Seconds())
	}
	if value < time.Hour {
		minutes := int(value / time.Minute)
		seconds := int((value % time.Minute) / time.Second)
		if seconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return compactDuration(value)
}

func formatSignedDurationSeconds(value float64) string {
	sign := "+"
	if value < 0 {
		sign = "-"
		value = math.Abs(value)
	}
	return sign + formatDurationShort(time.Duration(value*float64(time.Second)))
}

func formatSeconds(value float64) string {
	if value < 0.001 {
		return fmt.Sprintf("%.0fµs", value*1000000)
	}
	if value < 1 {
		return fmt.Sprintf("%.1fms", value*1000)
	}
	if value < 60 {
		return fmt.Sprintf("%.0fs", value)
	}
	return (time.Duration(value) * time.Second).Round(time.Minute).String()
}

func formatSignedRate(value float64) string {
	if value == 0 {
		return "0.0"
	}
	return fmt.Sprintf("%+.1f", value)
}

func compactDuration(value time.Duration) string {
	if value < time.Minute {
		return "<1m"
	}
	if value < time.Hour {
		return fmt.Sprintf("%dm", int(value.Round(time.Minute)/time.Minute))
	}
	if value < 24*time.Hour {
		return fmt.Sprintf("%dh", int(value.Round(time.Hour)/time.Hour))
	}
	return fmt.Sprintf("%dd", int(value.Round(24*time.Hour)/(24*time.Hour)))
}

func fluxStatusTone(status string) string {
	switch status {
	case "Ready":
		return "good"
	case "Suspended", "Reconciling", "Pending":
		return "warning"
	default:
		return "critical"
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

func cnpgClusterName(metric map[string]string) string {
	if job := strings.TrimSpace(metric["job"]); job != "" {
		if parts := strings.SplitN(job, "/", 2); len(parts) == 2 {
			return parts[1]
		}
		return job
	}
	pod := strings.TrimSpace(metric["pod"])
	if pod == "" {
		return ""
	}
	return strings.TrimRightFunc(pod, func(r rune) bool {
		return (r >= '0' && r <= '9') || r == '-'
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func demoSharedData() sharedData {
	now := time.Now()
	return sharedData{
		nodesReady:                  3,
		nodesTotal:                  3,
		namespaces:                  17,
		podsRunning:                 142,
		podsPending:                 2,
		podsFailed:                  1,
		targetsHealthy:              96,
		targetsTotal:                98,
		clusterCPU:                  38.4,
		clusterMemory:               61.7,
		fluxReady:                   268,
		fluxNotReady:                3,
		fluxSuspended:               2,
		fluxTotal:                   273,
		fluxControllersUp:           5,
		fluxControllersDown:         1,
		downTargetCount:             2,
		restartBurstCount:           4,
		externalSecretsReady:        67,
		externalSecretsDegraded:     1,
		externalSecretSyncErrors24h: 2,
		volsyncSources:              11,
		volsyncOutOfSync:            1,
		volsyncMissed24h:            2,
		volsyncLongestDuration:      58.030933328,
		volsyncSlowestSource:        "opencloud-rsrc",
		cnpgClusters:                2,
		cnpgInstances:               6,
		cnpgStreamingReplicas:       4,
		cnpgMaxReplicationLag:       0.000673,
		cnpgWALQueue:                0,
		cnpgMaxArchivalDelay:        1560,
		cnpgRecentBackupFailures:    0,
		cnpgTotalSizeBytes:          4002969994,
		envoyRequestRate:            5.2,
		envoyErrorRate:              0.2,
		envoyP95Latency:             132,
		toolhiveConnections:         3,
		toolhiveBackendErrors24h:    6,
		renovateProjects:            3,
		renovateExecutions24h:       74,
		renovateRunsFailed:          1,
		renovateDependencyIssues:    2,
		rustfsCapacityCurrent:       65157096642,
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
			{Kind: "HelmRelease", Ready: 89, NotReady: 1, Suspended: 2, Total: 92, Status: "Drift", Tone: "critical"},
			{Kind: "Kustomization", Ready: 122, NotReady: 2, Suspended: 0, Total: 124, Status: "Drift", Tone: "critical"},
			{Kind: "OCIRepository", Ready: 34, NotReady: 0, Suspended: 0, Total: 34, Status: "Ready", Tone: "good"},
			{Kind: "GitRepository", Ready: 18, NotReady: 0, Suspended: 0, Total: 18, Status: "Ready", Tone: "good"},
		},
		fluxRecent: []FluxRecentRow{
			{Kind: "Kustomization", Name: "cluster-apps", Namespace: "flux-system", Status: "Ready", Age: "2m", Tone: "good"},
			{Kind: "Kustomization", Name: "monitoring-stack", Namespace: "monitor", Status: "Ready", Age: "4m", Tone: "good"},
			{Kind: "HelmRelease", Name: "toolhive-operator", Namespace: "ai", Status: "Reconciling", Age: "7m", Tone: "warning"},
			{Kind: "OCIRepository", Name: "searxng", Namespace: "selfhosted", Status: "Ready", Age: "9m", Tone: "good"},
		},
		cnpgClusterRows: []CNPGClusterRow{
			{Name: "postgres", Namespace: "database", Detail: "last archive 4m ago", Replicas: "3", Lag: "673µs", Backup: "Clean", Size: "1.8GB", Tone: "good", BackupTone: "good", SizeBytes: 1931391478, WALQueue: 0, SecondsSinceArchival: 249},
			{Name: "postgres-immich", Namespace: "database", Detail: "last archive 26m ago", Replicas: "3", Lag: "0ms", Backup: "Last fail 9d", Size: "1.9GB", Tone: "good", BackupTone: "good", SizeBytes: 2071578516, WALQueue: 0, SecondsSinceArchival: 1560},
		},
		volsyncSourceRows: []VolSyncSourceRow{
			{Name: "opencloud-rsrc", Namespace: "selfhosted", Detail: "restic · pvc opencloud-data · next in 3h", Schedule: "0 13 * * *", LastSync: "22h ago", Duration: "58s", Status: "Healthy", Tone: "good", LastSyncDurationSeconds: 58.030933328},
			{Name: "authentik-rsrc", Namespace: "security", Detail: "restic · pvc authentik-data · overdue by 1h", Schedule: "0 2 * * *", LastSync: "2d ago", Duration: "31s", Status: "Missed", Tone: "warning", LastSyncDurationSeconds: 31},
			{Name: "vaultwarden-rsrc", Namespace: "security", Detail: "restic · pvc vaultwarden-data · next in 7h", Schedule: "0 15 * * *", LastSync: "9h ago", Duration: "42s", Status: "Drifted", Tone: "critical", LastSyncDurationSeconds: 42.126634373},
		},
		externalSecretTotal:         6,
		externalSecretStoreTotal:    1,
		externalSecretStoreReady:    1,
		externalSecretOldestRefresh: 5700,
		externalSecretRows: []ExternalSecretRow{
			{Name: "github-mcp-secret", Namespace: "ai", Store: "onepassword", Refresh: "14m ago", Interval: "15m", Status: "Ready", Detail: "ClusterSecretStore onepassword", Tone: "good", RefreshAt: now.Add(-14 * time.Minute)},
			{Name: "vaultwarden-admin", Namespace: "security", Store: "onepassword", Refresh: "41m ago", Interval: "15m", Status: "Stale", Detail: "ClusterSecretStore onepassword · refresh overdue", Tone: "warning", RefreshAt: now.Add(-41 * time.Minute), Errors24h: 0},
			{Name: "immich-db", Namespace: "media", Store: "onepassword", Refresh: "2m ago", Interval: "15m", Status: "Error", Detail: "ClusterSecretStore onepassword · provider access denied · 2 sync errors/24h", Tone: "critical", RefreshAt: now.Add(-2 * time.Minute), Errors24h: 2},
		},
		envoyActiveRoutes: 8,
		envoyTopRoute:     "mcp-gateway-internal",
		envoyRouteRows: []EnvoyRouteRow{
			{Name: "httproute/ai/mcp-gateway-internal/rule/0", Namespace: "ai", Route: "mcp-gateway-internal", RequestRate: "2.8 req/s", ErrorRate: "0.2 err/s", Latency: "182ms", Status: "Errors", Detail: "httproute/ai/mcp-gateway-internal/rule/0", Tone: "critical", RequestRateValue: 2.8, ErrorRateValue: 0.2, LatencyValue: 182},
			{Name: "httproute/github/renovate-operator/rule/0", Namespace: "github", Route: "renovate-operator", RequestRate: "0.7 req/s", ErrorRate: "0.0 err/s", Latency: "118ms", Status: "Slow", Detail: "httproute/github/renovate-operator/rule/0", Tone: "warning", RequestRateValue: 0.7, ErrorRateValue: 0, LatencyValue: 118},
			{Name: "httproute/monitor/homelab-dashboard/rule/0", Namespace: "monitor", Route: "homelab-dashboard", RequestRate: "1.1 req/s", ErrorRate: "0.0 err/s", Latency: "41ms", Status: "Healthy", Detail: "httproute/monitor/homelab-dashboard/rule/0", Tone: "good", RequestRateValue: 1.1, ErrorRateValue: 0, LatencyValue: 41},
		},
		warningEvents: []EventRow{
			{When: now.Add(-7 * time.Minute), Namespace: "monitor", Reason: "BackOff", Object: "Pod/thanos-query-84ccc68499-gcp9q", Message: "Back-off restarting failed container"},
			{When: now.Add(-16 * time.Minute), Namespace: "downloads", Reason: "FailedMount", Object: "Pod/radarr-0", Message: "Unable to attach or mount volumes"},
		},
		toolhiveBackends:          7,
		toolhiveSlowestBackend:    "kubectl",
		toolhiveAvgBackendLatency: 0.12,
		toolhiveBackendRows: []ToolhiveBackendRow{
			{Name: "kubectl", Requests24h: "928", Errors24h: "0", Latency: "303ms", Status: "Slow", Detail: "VMCP backend", Tone: "warning", Requests24hValue: 928, Errors24hValue: 0, LatencyValue: 0.303},
			{Name: "prometheus", Requests24h: "1462", Errors24h: "7", Latency: "56ms", Status: "Errors", Detail: "VMCP backend", Tone: "critical", Requests24hValue: 1462, Errors24hValue: 7, LatencyValue: 0.056},
			{Name: "github", Requests24h: "884", Errors24h: "2", Latency: "106ms", Status: "Errors", Detail: "VMCP backend", Tone: "critical", Requests24hValue: 884, Errors24hValue: 2, LatencyValue: 0.106},
		},
		renovateProjectRows: []RenovateProjectRow{
			{Name: "ishioni/homelab-ops", Executions24h: "27", Issues: "0", Status: "Healthy", Detail: "dependency automation", Tone: "good", Executions24hValue: 27, IssueCount: 0, Failed: 0},
			{Name: "ishioni/resume", Executions24h: "24", Issues: "0", Status: "Healthy", Detail: "dependency automation", Tone: "good", Executions24hValue: 24, IssueCount: 0, Failed: 0},
			{Name: "ishioni/yellingatclouds", Executions24h: "23", Issues: "2", Status: "Issues", Detail: "dependency automation", Tone: "warning", Executions24hValue: 23, IssueCount: 2, Failed: 0},
		},
		anomalies: []AnomalySignal{
			{Category: "Compute", Severity: "critical", Signal: "Target Down", Resource: "thanos-query · 10.0.0.8:10902", Value: "down", Window: "current", Details: "Prometheus scrape failed for this target."},
			{Category: "Operators", Severity: "critical", Signal: "Flux Unready", Resource: "Kustomization/cluster-apps", Value: "HealthCheckFailed", Window: "current", Details: "Flux reports this resource as not ready."},
			{Category: "Storage", Severity: "warning", Signal: "VolSync Missed Schedule", Resource: "security/authentik-rsrc", Value: "1 intervals", Window: "6h", Details: "This replication source has missed a scheduled backup interval."},
			{Category: "Network", Severity: "warning", Signal: "Envoy Latency", Resource: "httproute/monitor/homelab-dashboard/rule/0", Value: "312 ms", Window: "5m", Details: "Average upstream request latency is elevated for this route."},
		},
		computeSignalTrend:   []float64{1, 1, 2, 2, 3, 2, 2, 1, 1, 2, 2, 1},
		networkSignalTrend:   []float64{0, 1, 1, 2, 2, 1, 1, 0, 1, 1, 2, 1},
		storageSignalTrend:   []float64{0, 0, 1, 1, 1, 2, 1, 1, 0, 1, 1, 1},
		operatorSignalTrend:  []float64{1, 1, 1, 2, 2, 2, 3, 2, 2, 1, 1, 1},
		cpuTrend:             []float64{22, 24, 27, 26, 29, 31, 34, 36, 33, 35, 39, 38},
		memoryTrend:          []float64{49, 50, 52, 53, 54, 55, 57, 58, 59, 60, 61, 62},
		podTrend:             []float64{128, 129, 130, 132, 133, 134, 136, 137, 139, 140, 141, 142},
		rustfsCapacityTrend:  []float64{58411555212, 58948392511, 59485229810, 60022067109, 60558904408, 61095741707, 61632579006, 62169416305, 62706253604, 63243090903, 64166447322, 65157096642},
		cnpgSizeTrend:        []float64{3491758080, 3514836992, 3541047296, 3568306176, 3595565056, 3622823936, 3657424896, 3693080576, 3731888128, 3831750656, 3924841472, 4002969994},
		volsyncDurationTrend: []float64{31.4, 32.1, 35.6, 34.9, 36.4, 38.2, 39.1, 41.0, 44.2, 47.8, 52.4, 58.0},
		envoyTrafficTrend:    []float64{3.2, 3.5, 3.8, 4.0, 4.1, 4.4, 4.6, 4.9, 5.0, 5.1, 5.2, 5.2},
	}
}
