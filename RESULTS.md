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
| 4 | 2026-08-21 18:38 | [`6b9a3f4`](https://github.com/timothy-unimelb/Obsidio/commit/6b9a3f4) | Tim's submission/go (origin/draft) under our capped harness (cpuset 0,1) — cross-impl comparison; first seconds possibly noisy from another session's aborting k6 | **999,169** | -22% | 90.6 | 90.9 | 917.8 | 0.00% | 4/4 |
| 5 | 2026-08-21 18:43 | [`6b9a3f4`](https://github.com/timothy-unimelb/Obsidio/commit/6b9a3f4)\* | re-run of engineered Go v1, no code changes (post port-conflict rerun) | **1,148,230** | +15% | 110.7 | 110.3 | 835.7 | 0.00% | 4/4 |
| 6 | 2026-08-21 21:13 | [`911351a`](https://github.com/timothy-unimelb/Obsidio/commit/911351a)\* | joel/draft @894f469: go starter + GOMAXPROCS(2), boot-calibrated /risk semaphore (2 conc, queue 462, 1.18s wait timeout) | **698,519** | -39% | 7.3 | 7.3 | 1,201.6 | 0.62% | 4/4 |
| 7 | 2026-08-21 22:16 | [`911351a`](https://github.com/timothy-unimelb/Obsidio/commit/911351a)\* | Go 1.26 base image (was 1.22): stdlib SHA-NI on amd64; expect ~flat on this arm64 host, 2x payoff is x86-only | **941,593** | +35% | 68.4 | 67.7 | 1,169.8 | 0.00% | 4/4 |
| 8 | 2026-08-21 22:21 | [`911351a`](https://github.com/timothy-unimelb/Obsidio/commit/911351a)\* | Go 1.26 bump CLEAN re-run (prev line was contaminated by concurrent docker build; compare vs run 5, engineered v1 on Go 1.22) | **1,109,499** | +18% | 103.8 | 104.1 | 672.2 | 0.00% | 4/4 |
| 9 | 2026-08-21 22:27 | [`911351a`](https://github.com/timothy-unimelb/Obsidio/commit/911351a)\* | boot-calibrated /risk gate (adapted from joel/draft): timed chains at boot derive queue depth + wait deadline + 503 shed; on Go 1.26 image | **1,041,111** | -6% | 96.6 | 96.5 | 1,046.0 | 0.05% | 4/4 |
| 10 | 2026-08-22 00:30 | [`46894e4`](https://github.com/timothy-unimelb/Obsidio/commit/46894e4)\* | Wave 1 keeper build: GOGC=off+GOMEMLIMIT + calibrated Gosched yield + safety bundle (test-in-build, defer release, go.mod 1.26) | **1,154,625** | +11% | 12.0 | 12.1 | 955.5 | 0.01% | 4/4 |
| 11 | 2026-08-22 10:21 | [`a8ec6c0`](https://github.com/timothy-unimelb/Obsidio/commit/a8ec6c0)\* | LIFO governor v2: staleness-skip at grant + front-door budgeted shed | **1,097,306** | -5% | 11.9 | 11.9 | 243.6 | 0.55% | 4/4 |

\* dirty tree — run included uncommitted changes on top of that commit.

Runs on this machine share CPU between k6 and the container — absolute numbers are directional; trust the deltas between back-to-back runs.
