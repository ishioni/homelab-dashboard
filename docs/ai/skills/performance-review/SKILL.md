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

1. Establish the symptom.
2. Identify the affected path, feature, or operation.
3. Measure before changing code.
4. Inspect the hot path and supporting dependencies.
5. Propose the smallest defensible fix.
6. Re-measure after the change.
7. Report evidence, tradeoffs, and residual risks.

## Measurement First

Prefer evidence such as:
- request latency before and after
- `go test -bench` results
- `-benchmem` allocation data
- `pprof` CPU or heap profiles
- counts of external calls per request

Avoid presenting guesses as facts. If you cannot measure directly, say what you inferred, why, and how confident you are.

## Repo-Specific Focus

For `homelab-dashboard`, check these first:
- Prometheus query count, latency, and duplicated fetches
- Kubernetes API calls in request paths
- repeated data shaping before template rendering
- unnecessary polling or refresh fan-out
- string building, JSON marshalling, and allocation-heavy transforms
- lock contention or shared client bottlenecks

## Review Heuristics

When reviewing code, prefer findings that materially affect user-visible performance:
- repeated remote calls that could be batched, cached, or reused
- N+1 style loops over Prometheus or Kubernetes data
- work repeated on every request that could be moved or memoized safely
- large temporary allocations in hot paths
- expensive formatting, sorting, or parsing done too often

Ignore cosmetic micro-optimizations unless they sit on a proven hot path.

## Change Discipline

- Do not optimize blindly.
- Keep behavior unchanged unless the user asked for a functional tradeoff.
- Prefer simple fixes over clever ones.
- If caching is introduced, document invalidation and staleness tradeoffs.
- If concurrency is introduced, explain why it is safe and worth the added complexity.

## Output Expectations

A good performance review should include:
- the observed symptom
- the measured or suspected bottleneck
- the code path involved
- the recommended or implemented fix
- before/after evidence when available
- any remaining uncertainty or follow-up work
