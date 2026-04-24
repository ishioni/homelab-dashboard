# Performance Review Trigger Tests

Use this file to validate whether the skill description is tuned correctly.

The goal is not to prove the wording is elegant. The goal is to check whether it triggers on the right requests and stays out of the way on normal work.

## Current Description Under Test

```md
Use when investigating concrete or user-visible runtime performance issues, or reviewing changes for likely performance regressions in request, refresh, polling, or other hot paths in this repo, including slow responses, high CPU or memory, allocation pressure, throughput limits, remote call fan-out, and repeated work.
```

## How To Use This Test

For each prompt below:

1. Read the description only.
2. Decide whether the skill should trigger.
3. Compare your answer to the expected result.
4. Mark any ambiguous prompts.
5. If multiple prompts feel ambiguous, tighten the description and rerun the same set.

Optimize for precision first. This skill is more useful when it stays out of generic implementation and review work.

## Should Trigger

| Prompt | Expected | Why |
| --- | --- | --- |
| Profile the dashboard. Responses feel slow after the last change. | Trigger | Explicit latency investigation. |
| Review this PR for performance regressions in the page refresh path. | Trigger | Performance-focused review of a change. |
| The hub page spikes CPU when several users open it. Find the bottleneck. | Trigger | Explicit runtime symptom. |
| Check this handler for memory and allocation issues. | Trigger | Explicit performance concern. |
| The app is making too many Prometheus calls per request. Optimize it. | Trigger | Remote call fan-out in a hot path. |
| Benchmark this service method and see whether the refactor slowed it down. | Trigger | Benchmarking and regression detection. |
| Review whether this polling loop will create throughput problems under load. | Trigger | Polling behavior and scaling concern. |
| We think template rendering is doing repeated work on every request. Investigate. | Trigger | Repeated hot-path work. |
| Look for a performance regression in Kubernetes event collection after this commit. | Trigger | Regression review against runtime behavior. |
| Run a perf review on the anomaly page. It feels slower than the others. | Trigger | Explicit performance investigation. |

## Should Not Trigger

| Prompt | Expected | Why |
| --- | --- | --- |
| Add a new card for disk pressure to the hub page. | No trigger | Feature work only. |
| Refactor this service to make it easier to read. | No trigger | Maintainability, not performance. |
| Review this PR for correctness and edge cases. | No trigger | General review with no perf scope. |
| Explain how this template works. | No trigger | Code explanation only. |
| Design a caching strategy we might use in the future. | No trigger | Architecture brainstorming, no explicit problem. |
| Clean up these duplicated helper functions. | No trigger | General refactoring. |
| Add logging around the Prometheus client. | No trigger | Observability work, not performance by itself. |
| Help me understand how the Kubernetes client is initialized. | No trigger | Explanation request. |
| Make this code more efficient. | No trigger | Too vague; should require a clearer performance target. |
| Review this PR before I merge it. | No trigger | Broad review request with no performance framing. |

## Borderline Cases

These prompts are useful because they expose whether the description is too broad or too narrow.

| Prompt | Default Expectation | Risk |
| --- | --- | --- |
| Review this polling code. I am worried it might not scale. | Trigger | Could be interpreted as design review or performance review. |
| This page feels heavy. See if anything obvious stands out. | Trigger | "Heavy" is imprecise but still a likely performance concern. |
| Review this change with extra attention to efficiency. | No trigger | "Efficiency" is often too vague and risks over-triggering. |
| We should probably cache some of this data. Thoughts? | No trigger | Solution-first brainstorming without a measured problem. |
| Is this loop too expensive? | No trigger | Too context-free unless tied to a hot path or symptom. |

## What To Change If The Skill Over-Triggers

Tighten the description with words like:
- explicit
- likely regression
- hot path
- slow responses
- high CPU or memory

Remove or avoid words like:
- efficiency
- better
- optimize
- improve
- runtime cost

## What To Change If The Skill Under-Triggers

Add or emphasize:
- regression review
- profiling
- benchmarking
- polling behavior
- repeated work in hot paths
- remote call fan-out
