# Bench results

_Generated from `bench/history.jsonl` by the `/bench` skill — do not edit by
hand (regenerate: `python3 .claude/skills/bench/scripts/render_results.py`)._
_Full hypothesis → verdict narrative per attempt: [EXPERIMENTS.md](EXPERIMENTS.md)._

Score = weighted 200s under the 4.5-min k6 grading load (1×/price + 3×/stats + 10×/risk). All p95 in ms; 4 bars = price<200 · stats<500 · risk<1500 · errors<1%.

| # | When | Commit | Approach | work_score | Δ vs prev | p95 price | p95 stats | p95 risk | Err | Bars |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 2026-08-21 13:43 | [`4519449`](https://github.com/timothy-unimelb/Obsidio/commit/4519449)\* | baseline: naive node starter, UNCAPPED local process (no docker on this machine) — directional only | ~~225,714~~ | excluded | 586.6 | 589.2 | 624.3 | 0.00% | 2/4 |
| 2 | 2026-08-21 14:23 | [`4519449`](https://github.com/timothy-unimelb/Obsidio/commit/4519449)\* | baseline: naive Go starter, capped container (2cpu/2g, cpuset 0,1), same-machine k6 | **745,484** | baseline | 163.4 | 162.7 | 1,011.4 | 0.00% | 4/4 |
| 3 | 2026-08-21 14:28 | [`4519449`](https://github.com/timothy-unimelb/Obsidio/commit/4519449)\* | engineered Go v1: GOMAXPROCS=2, risk semaphore(2), zero-alloc hash kernel, no-reflection JSON | **1,273,640** ★ | +71% | 119.0 | 118.5 | 320.7 | 0.00% | 4/4 |

\* dirty tree — run included uncommitted changes on top of that commit.

Runs on this machine share CPU between k6 and the container — absolute numbers are directional; trust the deltas between back-to-back runs.
