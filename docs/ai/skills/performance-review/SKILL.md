---
name: performance-review
description: Use when investigating concrete or user-visible runtime performance issues, or reviewing changes for likely performance regressions in request, refresh, polling, or other hot paths in this repo, including slow responses, high CPU or memory, allocation pressure, throughput limits, remote call fan-out, and repeated work.
---

# Performance Review

Use this skill when the task is about concrete or user-visible runtime performance problems in this repository.

Typical triggers:
- a page, route, refresh cycle, or polling path feels slow, heavy, or sluggish
- CPU, memory, allocations, or throughput look worse than expected
- a recent change may have introduced a performance regression
- the user asks for profiling, benchmarking, or regression analysis on a known or suspected hot path
- the app appears to be doing too much repeated work or too many remote calls

Do not use this skill for:
- generic code review with no performance concern
- vague requests like "make this faster" or "make this more efficient" without a symptom or target path
- speculative micro-optimization without evidence
- architecture brainstorming or caching/concurrency design before a bottleneck has been identified

## Goal

Find the actual bottleneck, support the conclusion with measurements where possible, and propose the smallest focused fixes that are likely to improve the result without unnecessary complexity.

## Operating Rules

- Treat this as an evidence-driven skill, not a generic optimization pass.
- Do not optimize vague requests like "make this faster" or "make this more efficient" without a stated symptom, hot path, or regression concern.
- If the request is too vague, ask for the slow path, user-visible symptom, suspected regression, or performance goal before proceeding.
- Diagnose the bottleneck first and propose focused fixes before making code changes.
- Do not implement performance changes unless the user explicitly asks for them after reviewing the findings.
- Prefer the smallest change that addresses the measured bottleneck.
- Do not introduce caching, concurrency, or broad refactors unless the evidence justifies the added complexity.
- Keep behavior unchanged unless the user explicitly accepts a tradeoff.
- If direct measurement is not possible, state that clearly and separate observation, inference, and recommendation.
- When proposing fixes, include expected benefit, tradeoffs, and how the change should be verified.

## Workflow

1. Establish the symptom, complaint, or regression concern.
2. Identify the affected request path, refresh loop, polling path, or operation.
3. Check whether there is enough signal to proceed.
4. If the request is too vague, ask for the slow path, user-visible symptom, suspected regression, or performance goal before analyzing further.
5. Measure or inspect the hot path before suggesting fixes.
6. Separate measured facts from inference and identify the most likely bottleneck.
7. Propose the smallest focused fixes, with expected benefit, tradeoffs, and clear verification steps.
8. Only implement changes if the user explicitly asks after reviewing the findings.
9. If changes are made, re-measure and report before/after results.

## Measurement First

Prefer direct evidence such as:
- request latency before and after
- `go test -bench` results
- `-benchmem` allocation data
- `pprof` CPU or heap profiles
- counts of external calls per request
- concrete timing, allocation, or query-count comparisons

When measurement is possible, prefer it over intuition.

External observability is valid evidence when available. If the environment exposes Prometheus, tracing, ingress or gateway metrics, load balancer metrics, or service-level latency and error data through MCPs or other tools, use those signals to confirm the user-visible symptom and narrow the hot path before proposing fixes.

When direct measurement is not practical:
- inspect the hot path carefully
- identify the repeated work, remote call fan-out, allocation pressure, or contention risk
- state clearly that the conclusion is based on inspection rather than measurement
- separate measured facts, observed code behavior, and inference

Do not assume any specific metric, dashboard, or controller exists. Use the observability available in the current environment.

Do not present guesses as facts.

## Repo-Specific Focus

For `homelab-dashboard`, check these first:
- Prometheus query count, latency, duplicated fetches, and page-specific fan-out
- Kubernetes API calls that happen in request or refresh paths instead of being reduced, filtered, or reused
- repeated data shaping, sorting, formatting, or aggregation before template rendering
- polling and refresh behavior that amplifies backend load or repeats the same work too often
- page assembly work that fetches or computes the same data separately for hub, security, anomaly, or forecasting views
- string building, JSON marshalling, and other allocation-heavy transforms in hot paths
- shared clients, synchronization, or contention that can serialize otherwise independent work

## Review Heuristics

When reviewing code, prefer findings that materially affect user-visible performance:
- repeated remote calls that could be batched, reduced, cached, or reused safely
- N+1 style loops over Prometheus, Kubernetes, or page assembly data
- work repeated on every request, refresh cycle, or polling interval that could be reduced or reused
- allocation-heavy transforms or repeated creation of large temporary slices, maps, or buffers in hot paths
- expensive formatting, sorting, parsing, or aggregation done more often than necessary
- serial work that could dominate latency because independent operations are forced through one path

Prefer findings with a clear path to verification, such as lower latency, fewer remote calls, fewer allocations, or less repeated work.

Ignore cosmetic micro-optimizations unless they sit on a proven hot path.

## Change Discipline

- Do not optimize blindly or propose changes without a clear bottleneck or symptom.
- Prefer the smallest fix that addresses the measured or strongly supported problem.
- Keep behavior unchanged unless the user explicitly accepts a functional tradeoff.
- Prefer simple changes over clever ones, especially in hot paths that are already hard to reason about.
- Do not recommend caching without explaining invalidation, staleness, and why simpler reuse is not enough.
- Do not recommend concurrency without explaining why the work is independent, what latency it improves, and what new complexity it introduces.
- Avoid broad refactors framed as performance work unless the existing structure is itself the bottleneck.
- If implementation is requested, verify the effect after the change rather than assuming the proposal helped.

## Output Expectations

A good performance review should include:
- the observed symptom, complaint, or regression concern
- the affected code path, request path, refresh loop, polling path, or operation
- the measured evidence, or a clear statement that the conclusion is based on inspection rather than direct measurement
- the most likely bottleneck, with measured facts separated from inference
- the proposed fixes, ordered from smallest to most invasive when there is more than one reasonable option
- the expected benefit, tradeoffs, and verification steps for each proposed fix
- before and after evidence if changes were implemented
- any remaining uncertainty, missing measurements, or follow-up work
