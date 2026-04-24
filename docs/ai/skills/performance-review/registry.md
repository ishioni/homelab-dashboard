# Performance Review — Endpoint Registry

## Endpoints

| Method | Path | Description | Playwright Page | Auth Required |
|--------|------|-------------|-----------------|---------------|
| GET | /healthz | Health check | | No |
| GET | /api/view/hub | Hub dashboard data (JSON) | / | No |
| GET | /api/view/security | Security view data (JSON) | /security | No |
| GET | /api/view/anomalies | Anomalies view data (JSON) | /anomalies | No |
| GET | /api/view/forecasting | Forecasting view data (JSON) | /forecasting | No |

> HTML page routes (`/`, `/security`, `/anomalies`, `/forecasting`) are exercised via Playwright and are not benchmarked directly with curl.

---

## Baseline Snapshot

```json
{}
```
