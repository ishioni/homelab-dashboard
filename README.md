# homelab-dashboard

`homelab-dashboard` is a small Go web app for live homelab visibility using:

- Prometheus for metrics, Flux posture, and rule-based anomaly detection
- Kubernetes API for recent warning events
- a Nord-inspired server-rendered UI with no frontend build step

## Features

- Insight Hub with node readiness, scrape health, pod state, and cluster utilization
- Security Posture with Flux resource readiness, suspended resources, and slow reconciliations
- Anomaly Explorer driven by Prometheus thresholds instead of runtime AI
- Forecasting cards based on the last 24 hours of Prometheus history

## Configuration

All configuration is environment-driven.

- `PORT` default `8080`
- `APP_NAME` default `Homelab Dashboard`
- `CLUSTER_NAME` default `Homelab Cluster`
- `PROMETHEUS_URL` default `http://thanos-query.monitor.svc.cluster.local:10902`
- `PROMETHEUS_TIMEOUT` default `10s`
- `REFRESH_INTERVAL` default `45s`
- `ENABLE_KUBERNETES` default `true`
- `KUBECONFIG` optional for local runs
- `NAMESPACE_ALLOWLIST` optional comma-separated filter for Kubernetes warning events
- `ANOMALY_NODE_CPU_WARN_PERCENT` default `85`
- `ANOMALY_NODE_MEMORY_WARN_PERCENT` default `90`
- `ANOMALY_RESTART_BURST_THRESHOLD` default `3`
- `ANOMALY_TARGETS_DOWN_WARN_COUNT` default `1`
- `ANOMALY_FLUX_UNREADY_WARN_COUNT` default `1`

## Local run

```bash
go run ./cmd/homelab-dashboard
```

If you are outside the cluster, set `KUBECONFIG` and point `PROMETHEUS_URL` at a reachable Prometheus or Thanos endpoint.

## Container build

Build from the repository root:

```bash
docker build -f .docker/Dockerfile -t homelab-dashboard:local .
docker run --rm -p 8080:8080 \
  -e CLUSTER_NAME="Homelab Cluster" \
  -e PROMETHEUS_URL="http://host.docker.internal:10902" \
  homelab-dashboard:local
```

## Kubernetes deployment

- `deploy/kubernetes/rbac.yaml` contains the service account and read-only RBAC
- `deploy/kubernetes/deployment.example.yaml` contains a hardened deployment example with a read-only root filesystem
