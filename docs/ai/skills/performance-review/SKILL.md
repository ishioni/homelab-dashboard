---
name: performance-review
description: Use when asked to benchmark API endpoints, detect performance regressions, measure page load times, or improve handler response times in homelab-dashboard
---

# Performance Review

## Overview

Eight-step pipeline: verify the app is up → discover routes → HTTP benchmark → Playwright page timings → regression comparison against stored baseline → report → targeted code suggestions → optional baseline update.

Run all steps in order. Never skip steps. HTTP benchmarking always runs before Playwright.

---

## Step 1 — Ensure app is running

```bash
curl -sf http://localhost:${PORT:-8080}/healthz
```

If the check fails, start the app:
```bash
go run ./cmd/... &
```

If the app cannot be started, **abort** with a clear error message. Do not proceed.

---

## Step 2 — Route discovery

Grep the Go source for route registrations:
```bash
grep -rn "mux\.HandleFunc\|mux\.Handle\|router\.\(GET\|POST\|PUT\|DELETE\|PATCH\)\|http\.HandleFunc" \
  --include="*.go" .
```

Diff the discovered routes against the endpoint table in `registry.md`.

**For any discovered route NOT in the registry table:**
- Pause execution
- List the new routes
- Ask the user: "These routes are not in the registry. Add them before continuing? (y/n)"
- Do not continue until the user responds

---

## Step 3 — HTTP benchmarking

For each registered endpoint:

```bash
for i in $(seq 1 10); do
  curl -sf -o /dev/null -w "%{time_total}\n" \
    -H "Authorization: Bearer ${PERF_AUTH_TOKEN}" \
    "http://localhost:${PORT:-8080}/path"
done
```

- Discard result #1 (cold start)
- Compute **avg_ms** and **p95_ms** from the remaining 9 results (multiply `time_total` by 1000)
- If `PERF_AUTH_TOKEN` is unset and the endpoint requires auth: **warn and skip** — do not fail the run
- If the endpoint returns non-2xx: record as error, skip from timing, note in report

---

## Step 4 — Playwright measurement

For each page listed in `registry.md` (column `Playwright Page`):

1. Navigate to the page using Playwright MCP
2. Intercept all `fetch` / `XHR` network requests — record URL and response time for each
3. Record full page load time: navigation start → `load` event
4. Key results by page path and API endpoint URL for correlation with Step 3

If Playwright MCP is unavailable: **skip this step**, note it in the report, continue with HTTP-only results.

---

## Step 5 — Regression comparison

Compare Step 3 + Step 4 results against the `baseline` JSON block at the bottom of `registry.md`.

| Condition | Label |
|-----------|-------|
| current avg > baseline avg × 1.20 | 🔴 REGRESSION |
| within threshold | ✅ OK |
| no baseline entry | 🟡 NEW |

---

## Step 6 — Report

Print to terminal:

**API Endpoints**

| Endpoint | Baseline avg | Current avg | Delta | p95 | Status |
|----------|-------------|-------------|-------|-----|--------|

**Pages**

| Page | Baseline load | Current load | Delta | Status |
|------|--------------|-------------|-------|--------|

After both tables, list the **top 3 candidates for Step 7**:
- 🔴 Regressions first, ranked by delta %
- Fill remaining slots with slowest by absolute p95

---

## Step 7 — Code suggestions

For each of the top 3 candidates, trace:

```
route definition → handler function → service/repository methods
```

Look for these patterns **in priority order**:

1. **N+1 queries** — loops making DB/API calls per iteration
2. **Missing caching** — repeated identical fetches within a single request
3. **Synchronous blocking** — sequential calls that could run concurrently with `errgroup`
4. **Unindexed queries** — SQL without WHERE clause index coverage
5. **Unnecessary serialization** — large payloads that could be streamed or paginated

For each issue found: show a **specific before/after diff inline**.

**Do NOT apply fixes automatically.** Present each suggestion and wait for user decision.

---

## Step 8 — Baseline update

Ask: `"Update the baseline in registry.md with the current measurements? (y/n)"`

If yes: rewrite only the fenced JSON block at the bottom of `registry.md` with new timings and the current UTC timestamp. Do not append — replace in place.

---

## Error handling summary

| Situation | Action |
|-----------|--------|
| App fails to start | Abort with instructions |
| Endpoint returns non-2xx | Record as error, skip timing, note in report |
| `PERF_AUTH_TOKEN` unset for auth endpoint | Warn, skip endpoint, continue |
| Playwright MCP unavailable | Skip Step 4, note in report, continue |
