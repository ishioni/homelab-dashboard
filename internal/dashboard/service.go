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
				{Label: "CNPG Clusters", Value: fmt.Sprintf("%.0f clusters", data.cnpgClusters), Detail: fmt.Sprintf("%.0f replicas streaming · max lag %s", data.cnpgStreamingReplicas, formatSeconds(data.cnpgMaxReplicationLag)), Tone: toneByThreshold(data.cnpgMaxReplicationLag, 30)},
			},
			FluxCards:  buildFluxCards(data.fluxKinds),
			FluxKinds:  data.fluxKinds,
			FluxRecent: data.fluxRecent,
			OperatorRows: []SecurityStatusRow{
				{
					Icon:   "key",
					Name:   "External Secrets",
					State:  fmt.Sprintf("%.0f ready", data.externalSecretsReady),
					Detail: fmt.Sprintf("%.0f degraded · %.0f sync errors in 24h", data.externalSecretsDegraded, data.externalSecretSyncErrors24h),
					Meta:   "provider-backed secret sync",
					Tone:   toneByIssueCounts(data.externalSecretsDegraded, data.externalSecretSyncErrors24h),
				},
				{
					Icon:   "inventory_2",
					Name:   "VolSync",
					State:  fmt.Sprintf("%.0f sources", data.volsyncSources),
					Detail: fmt.Sprintf("%.0f out of sync · %.0f missed in 24h", data.volsyncOutOfSync, data.volsyncMissed24h),
					Meta:   "replicationsource integrity",
					Tone:   toneByIssueCounts(data.volsyncOutOfSync, data.volsyncMissed24h),
				},
				{
					Icon:   "database",
					Name:   "CloudNativePG",
					State:  fmt.Sprintf("%.0f clusters · %.0f replicas", data.cnpgClusters, data.cnpgStreamingReplicas),
					Detail: fmt.Sprintf("max replication lag %s", formatSeconds(data.cnpgMaxReplicationLag)),
					Meta:   "database HA posture",
					Tone:   toneByThreshold(data.cnpgMaxReplicationLag, 30),
				},
				{
					Icon:   "route",
					Name:   "Envoy Gateway",
					State:  fmt.Sprintf("%s req/s", formatRate(data.envoyRequestRate)),
					Detail: fmt.Sprintf("%s 5xx/s · p95 %s", formatRate(data.envoyErrorRate), formatMilliseconds(data.envoyP95Latency)),
					Meta:   "edge traffic integrity",
					Tone:   toneByIssueCounts(data.envoyErrorRate, 0),
				},
				{
					Icon:   "hub",
					Name:   "Toolhive",
					State:  fmt.Sprintf("%.0f active connections", data.toolhiveConnections),
					Detail: fmt.Sprintf("%.0f backend errors in 24h", data.toolhiveBackendErrors24h),
					Meta:   "MCP gateway health",
					Tone:   toneByIssueCounts(data.toolhiveBackendErrors24h, 0),
				},
				{
					Icon:   "auto_awesome",
					Name:   "Renovate",
					State:  fmt.Sprintf("%.0f projects · %.0f runs/24h", data.renovateProjects, data.renovateExecutions24h),
					Detail: fmt.Sprintf("%.0f failed · %.0f dependency issues", data.renovateRunsFailed, data.renovateDependencyIssues),
					Meta:   "platform automation",
					Tone:   toneByIssueCounts(data.renovateRunsFailed, data.renovateDependencyIssues),
				},
			},
			SlowReconciles: data.slowestFlux,
			WarningEvents:  data.warningEvents,
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
				{Label: "Domains Affected", Value: fmt.Sprintf("%d", countDistinctCategories(data.anomalies)), Detail: "Compute, network, storage, and operators", Tone: "info"},
				{Label: "Resources Impacted", Value: fmt.Sprintf("%d", countDistinctResources(data.anomalies)), Detail: "Distinct workloads, routes, or controllers involved", Tone: "warning"},
			},
			Signals: data.anomalies,
			Timeline: []SparklineCard{
				buildAnomalySparkline("Compute", data.computeSignalTrend, "Scrapes, restarts, node and workload state"),
				buildAnomalySparkline("Network", data.networkSignalTrend, "Cilium datapath health and Envoy route behavior"),
				buildAnomalySparkline("Storage", data.storageSignalTrend, "Backups, replication lag, and sync state"),
				buildAnomalySparkline("Operators", data.operatorSignalTrend, "Flux, Toolhive, External Secrets, and Renovate"),
			},
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
	volsyncSources              float64
	volsyncOutOfSync            float64
	volsyncMissed24h            float64
	cnpgClusters                float64
	cnpgStreamingReplicas       float64
	cnpgMaxReplicationLag       float64
	envoyRequestRate            float64
	envoyErrorRate              float64
	envoyP95Latency             float64
	toolhiveConnections         float64
	toolhiveBackendErrors24h    float64
	renovateProjects            float64
	renovateExecutions24h       float64
	renovateRunsFailed          float64
	renovateDependencyIssues    float64
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
	recordScalar(`sum(max by(job) (cnpg_pg_replication_streaming_replicas))`, &data.cnpgStreamingReplicas)
	recordScalar(`max(cnpg_pg_replication_lag)`, &data.cnpgMaxReplicationLag)
	recordScalar(`sum(rate(envoy_cluster_external_upstream_rq[5m]))`, &data.envoyRequestRate)
	recordScalar(`sum(rate(envoy_cluster_external_upstream_rq_xx{envoy_response_code_class="5"}[5m]))`, &data.envoyErrorRate)
	recordScalar(`histogram_quantile(0.95, sum(rate(envoy_cluster_external_upstream_rq_time_bucket[5m])) by (le))`, &data.envoyP95Latency)
	recordScalar(`sum(toolhive_mcp_active_connections)`, &data.toolhiveConnections)
	recordScalar(`sum(increase(toolhive_vmcp_backend_errors_total[24h]))`, &data.toolhiveBackendErrors24h)
	recordScalar(`count(renovate_operator_run_failed)`, &data.renovateProjects)
	recordScalar(`sum(increase(renovate_operator_project_executions_total[24h]))`, &data.renovateExecutions24h)
	recordScalar(`sum(renovate_operator_run_failed)`, &data.renovateRunsFailed)
	recordScalar(`sum(renovate_operator_dependency_issues)`, &data.renovateDependencyIssues)

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

	computeTrendQuery := fmt.Sprintf(`count(max by(job, instance, namespace, pod) (1 - up) > 0) + count(sum by(namespace,pod) (increase(kube_pod_container_status_restarts_total[30m])) > %.0f) + count(max by(namespace,pod,phase) (kube_pod_status_phase{phase=~"Pending|Failed|Unknown"}) > 0)`, s.cfg.Thresholds.RestartBurstThreshold)
	networkTrendQuery := `count(cilium_controllers_failing > 0) + count(cilium_bpf_map_pressure > 0.10) + count(sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_xx{envoy_response_code_class=~"4|5"}[5m])) > 0.02) + count((sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_sum[5m])) / sum by(envoy_cluster_name) (rate(envoy_cluster_external_upstream_rq_time_count[5m]))) > 250)`
	storageTrendQuery := `count(volsync_volume_out_of_sync{role="source"} > 0) + count(increase(volsync_missed_intervals_total{role="source"}[6h]) > 0) + count(cnpg_pg_replication_lag > 30) + count((time() - cnpg_collector_last_failed_backup_timestamp) < 604800 and cnpg_collector_last_failed_backup_timestamp > 0)`
	operatorTrendQuery := `count(flux_resource_info{ready!="True"}) + count(increase(toolhive_vmcp_backend_errors_total[30m]) > 0) + count(increase(renovate_operator_run_failed[24h]) > 0) + count(externalsecret_status_condition{condition="Ready",status!="True"} == 1) + count(increase(externalsecret_sync_calls_error[30m]) > 0)`

	recordRange(computeTrendQuery, &data.computeSignalTrend)
	recordRange(networkTrendQuery, &data.networkSignalTrend)
	recordRange(storageTrendQuery, &data.storageSignalTrend)
	recordRange(operatorTrendQuery, &data.operatorSignalTrend)
	recordRange(`100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[30m])))`, &data.cpuTrend)
	recordRange(`100 * (1 - (sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes)))`, &data.memoryTrend)
	recordRange(`sum(kube_pod_status_phase{phase="Running"})`, &data.podTrend)

	if s.cfg.EnableKubernetes && s.kube != nil {
		resources, err := s.kube.FluxResources(ctx, 0, s.cfg.NamespaceAllowlist)
		if err != nil {
			errors = append(errors, fmt.Sprintf("kubernetes flux resources: %v", err))
		} else {
			data.fluxKinds = buildFluxKindsFromResources(resources)
			data.fluxRecent = buildFluxRecentRows(resources, time.Now())
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

func formatRate(value float64) string {
	return fmt.Sprintf("%.1f", value)
}

func formatMilliseconds(value float64) string {
	return fmt.Sprintf("%.0fms", value)
}

func formatSeconds(value float64) string {
	if value < 1 {
		return fmt.Sprintf("%.0fms", value*1000)
	}
	if value < 60 {
		return fmt.Sprintf("%.0fs", value)
	}
	return (time.Duration(value) * time.Second).Round(time.Minute).String()
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

func maxInt(a, b int) int {
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
		cnpgClusters:                2,
		cnpgStreamingReplicas:       4,
		cnpgMaxReplicationLag:       41,
		envoyRequestRate:            5.2,
		envoyErrorRate:              0.2,
		envoyP95Latency:             132,
		toolhiveConnections:         3,
		toolhiveBackendErrors24h:    6,
		renovateProjects:            3,
		renovateExecutions24h:       74,
		renovateRunsFailed:          1,
		renovateDependencyIssues:    2,
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
		warningEvents: []EventRow{
			{When: now.Add(-7 * time.Minute), Namespace: "monitor", Reason: "BackOff", Object: "Pod/thanos-query-84ccc68499-gcp9q", Message: "Back-off restarting failed container"},
			{When: now.Add(-16 * time.Minute), Namespace: "downloads", Reason: "FailedMount", Object: "Pod/radarr-0", Message: "Unable to attach or mount volumes"},
		},
		anomalies: []AnomalySignal{
			{Category: "Compute", Severity: "critical", Signal: "Target Down", Resource: "thanos-query · 10.0.0.8:10902", Value: "down", Window: "current", Details: "Prometheus scrape failed for this target."},
			{Category: "Operators", Severity: "critical", Signal: "Flux Unready", Resource: "Kustomization/cluster-apps", Value: "HealthCheckFailed", Window: "current", Details: "Flux reports this resource as not ready."},
			{Category: "Storage", Severity: "warning", Signal: "VolSync Missed Schedule", Resource: "security/authentik-rsrc", Value: "1 intervals", Window: "6h", Details: "This replication source has missed a scheduled backup interval."},
			{Category: "Network", Severity: "warning", Signal: "Envoy Latency", Resource: "httproute/monitor/homelab-dashboard/rule/0", Value: "312 ms", Window: "5m", Details: "Average upstream request latency is elevated for this route."},
		},
		computeSignalTrend:  []float64{1, 1, 2, 2, 3, 2, 2, 1, 1, 2, 2, 1},
		networkSignalTrend:  []float64{0, 1, 1, 2, 2, 1, 1, 0, 1, 1, 2, 1},
		storageSignalTrend:  []float64{0, 0, 1, 1, 1, 2, 1, 1, 0, 1, 1, 1},
		operatorSignalTrend: []float64{1, 1, 1, 2, 2, 2, 3, 2, 2, 1, 1, 1},
		cpuTrend:            []float64{22, 24, 27, 26, 29, 31, 34, 36, 33, 35, 39, 38},
		memoryTrend:         []float64{49, 50, 52, 53, 54, 55, 57, 58, 59, 60, 61, 62},
		podTrend:            []float64{128, 129, 130, 132, 133, 134, 136, 137, 139, 140, 141, 142},
	}
}
