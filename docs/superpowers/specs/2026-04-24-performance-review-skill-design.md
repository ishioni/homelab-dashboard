# Performance Review Skill — Design Spec

**Date:** 2026-04-24  
**Status:** Approved

## Problem

There is no automated way to detect API performance regressions in homelab-dashboard before they reach the cluster. Changes to Go handlers can silently degrade response times. This skill gives Claude a repeatable, structured process to measure, compare, and improve endpoint performance locally.

## Goal

A Claude Code skill that:
- Benchmarks all registered API endpoints against a stored baseline
- Captures both raw HTTP timings and browser-side network/page-load timings via Playwright MCP
- Reports regressions (>20% slower than baseline)
- Suggests specific Go code fixes for the worst offenders
- Allows the baseline to be updated on demand

---

## Files

| File | Purpose |
|------|---------|
| `docs/ai/skills/performance-review/SKILL.md` | Skill instruction file — guides Claude through the pipeline |
| `docs/ai/skills/performance-review/registry.md` | Endpoint registry + stored baseline snapshot |

---

## Registry Format (`registry.md`)

### Endpoint Table

```markdown
| Method | Path | Description | Playwright Page |
|--------|------|-------------|-----------------|
| GET | /api/v1/nodes | List cluster nodes | /dashboard |
```

- Human-maintained; Claude flags new Go routes not in the table and pauses for confirmation before continuing
- `Playwright Page` column maps each endpoint to the frontend page that triggers it (blank = not triggered via UI)

### Baseline Snapshot

Fenced JSON block at the bottom of `registry.md`, written by the skill when the user approves an update. Only the last accepted baseline is stored — no history appended.

```json
{
  "recorded_at": "2026-04-24T10:00:00Z",
  "endpoints": {
    "GET /api/v1/nodes": { "avg_ms": 45, "p95_ms": 78 }
  },
  "pages": {
    "/dashboard": { "load_ms": 420, "api_requests": [
      { "url": "/api/v1/nodes", "avg_ms": 47 }
    ]}
  }
}
```

---

## Pipeline (single-pass)

### Step 1 — Ensure app is running
Attempt `curl http://localhost:8080/health` (port read from `PORT` env var, default `8080`). If it fails, start the app with `go run ./cmd/...` or the local Docker container. Fail with a clear message if the app cannot be started.

### Step 2 — Route discovery
Grep Go source for route definitions (e.g., `router.GET`, `mux.Handle`, `http.HandleFunc`). Diff discovered routes against the endpoint table in `registry.md`. For any new route not in the registry: pause, list the new routes, and ask the user whether to add them before continuing.

### Step 3 — HTTP benchmarking
For each registered endpoint:
- Run 10 sequential `curl` requests with `-w "%{time_total}"`
- Discard result #1 (cold start)
- Compute avg and p95 from the remaining 9
- Auth: read `PERF_AUTH_TOKEN` env var and inject as `Authorization: Bearer` header. If not set and endpoint requires auth, warn and skip rather than fail.

### Step 4 — Playwright measurement
For each page listed in `registry.md`:
- Navigate to the page using Playwright MCP
- Intercept all `fetch`/`XHR` network requests; record URL + response time for each
- Record full page load time (navigation start → `load` event)
- Key results by page path and API endpoint URL for correlation with Step 3

HTTP runs first (no browser overhead), Playwright second.

### Step 5 — Regression comparison
Compare Step 3 + Step 4 results against the baseline JSON in `registry.md`.

Thresholds:
- **🔴 REGRESSION** — current avg > baseline avg × 1.20 (>20% slower)
- **✅ OK** — within threshold
- **🟡 NEW** — endpoint has no baseline entry

### Step 6 — Report (terminal output)

**API Endpoints**
| Endpoint | Baseline avg | Current avg | Delta | p95 | Status |
|----------|-------------|-------------|-------|-----|--------|

**Pages**
| Page | Baseline load | Current load | Delta | Status |
|------|--------------|-------------|-------|--------|

After the tables: list the top 3 candidates for Step 7, selected by: 🔴 regressions first (ranked by delta %), then slowest by absolute p95 if fewer than 3 regressions exist.

### Step 7 — Code suggestions
For each of the top 3 flagged endpoints, trace `route definition → handler → service/repository methods` and look for these patterns in priority order:

1. **N+1 queries** — loops making DB/API calls per iteration
2. **Missing caching** — repeated identical fetches within a single request
3. **Synchronous blocking** — sequential calls that could run concurrently with `errgroup`
4. **Unindexed queries** — SQL without WHERE clause index coverage (if query is visible)
5. **Unnecessary serialization** — large payloads that could be streamed or paginated

For each issue: show a specific before/after diff inline. Do NOT apply fixes automatically — present and wait for user decision.

### Step 8 — Baseline update prompt
Ask: "Update the baseline in `registry.md` with the current measurements? (y/n)"  
If yes: rewrite the JSON block with the new timings and timestamp.

---

## Error Handling

- App fails to start → abort with instructions
- Endpoint returns non-2xx → record as error, skip from timing calculation, note in report
- `PERF_AUTH_TOKEN` missing for authenticated endpoints → warn + skip, do not fail entire run
- Playwright MCP unavailable → skip page timing phase, note in report, continue with HTTP-only

---

## Verification Checklist

- [ ] App starts locally and health check passes
- [ ] Route discovery finds all Go handlers
- [ ] New routes cause a pause + prompt before continuing
- [ ] HTTP benchmark produces avg + p95 per endpoint
- [ ] Playwright captures per-request timings and page load times
- [ ] Report table renders correctly with correct status flags
- [ ] Regression threshold (>20%) triggers 🔴 correctly
- [ ] Code suggestions are specific (before/after diff), not generic
- [ ] Baseline update rewrites only the JSON block in `registry.md`
