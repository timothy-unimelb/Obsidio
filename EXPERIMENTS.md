# Experiments log

One entry per optimisation attempt. This file IS the resilience write-up's evidence —
numbers, not adjectives. Log failures and reverts too; before/after numbers come from
`bench/history.jsonl`, never from memory. Newest entry at the bottom.

Entry format (see the /experiment skill):

```markdown
## YYYY-MM-DD Short title
- **SHA:** abc1234
- **Hypothesis:** ...
- **Change:** files + one-liner
- **Result:** work_score X → Y; p95 price a→b ms, stats c→d ms, risk e→f ms; errors g→h
- **Verdict:** kept | reverted | pending
```

## 2026-08-21 Engineered Go v1 vs naive Go baseline (capped)

- **SHA:** 4519449, dirty tree (scaffolding + app/ not yet committed)
- **Hypothesis:** The /risk hash chain gates the closed loop. A Go app with
  GOMAXPROCS=2, /risk bounded to 2 concurrent chains (semaphore; waiters park),
  a zero-allocation sha256/hex kernel, and reflection-free JSON should raise
  work_score and cut all p95s vs the naive Go starter.
- **Change:** new `app/` (main.go, go.mod, Dockerfile). Baseline = untouched
  `starters/go` in the same capped container (2 CPU / 2 GB, cpuset 0,1),
  same-machine k6 (directional).
- **Result:** work_score 745,484 → 1,273,640 (+71%); p95 price 163.4→119.0ms,
  stats 162.7→118.5ms, risk 1011.4→320.7ms; errors 0%→0%; requests 297k→509k.
  (history.jsonl lines 2 and 3; line 1 is an uncapped Node run, not comparable.)
- **Verdict:** kept

## 2026-08-21 Go 1.22 → 1.26 base image (stdlib SHA-NI on amd64)

- **SHA:** 911351a, dirty tree (Dockerfile bump only at bench time)
- **Hypothesis:** Go 1.25+ adds a SHA-NI path to crypto/sha256 on amd64 (release
  notes: ~2x). Our image shipped Go 1.22 (AVX2-only path). On x86 grading hardware
  with SHA extensions (all AMD Zen, Intel Ice Lake+) this should ~double /risk
  throughput; on this arm64 dev host it should be ~flat (ARM sha2 path unchanged).
- **Change:** app/Dockerfile FROM golang:1.22-bookworm → golang:1.26-bookworm.
- **Result:** work_score 1,148,230 → 1,109,499 (−3.4%, inside the ~10% same-machine
  noise floor; history lines 5 vs 8). p95 price 110.7→103.8ms, stats 110.3→104.1ms,
  risk 835.7→672.2ms; errors 0%→0%. Line 7 was a contaminated run (concurrent
  docker build stole CPU mid-bench) — excluded from the comparison.
- **Verdict:** kept — local-neutral as expected; the claimed ~2x is x86-only and
  unmeasurable on this host. Must be verified on an x86 box before submission-day
  claims. /smoke passed (all 35 checks) on the new toolchain.

## 2026-08-21 Boot-calibrated /risk gate (adapted from joel/draft@894f469)

- **SHA:** 911351a, dirty tree (calibration + Go 1.26 Dockerfile in working tree)
- **Hypothesis:** Replacing the unbounded /risk wait queue with boot-derived limits
  (timed median-of-3 chains → wait deadline = 1500ms − chain − 300ms margin; queue
  via Little's Law × 3; overflow 503s; waiters cancel on deadline/disconnect) costs
  ~nothing locally (queue never fills here) and buys self-tuning robustness on
  unknown grading hardware, where hardcoded limits could let /risk rot past the bar.
- **Change:** app/main.go: calibrateRisk() at boot; bounded wait + deadline + shed
  in handleRisk. Boot log on this host: unitCost=11.2ms, maxQueued=636, wait=1.19s.
- **Result:** work_score 1,109,499 → 1,041,111 (−6.2%, inside ~10% noise floor;
  history lines 8 vs 9). p95 price 103.8→96.6ms, stats 104.1→96.5ms, risk
  672.2→1,046.0ms (risk p95 is the noisiest metric across all runs: 321–1,046 for
  similar builds). Errors 0%→0.053% (~220 deadline sheds of 418k reqs — the valve
  engaging at the margin, far under the 1% gate). Mechanically the gate adds only
  an atomic counter + non-blocking select on the hot path (ns), so the −6.2% is
  noise, not cost.
- **Verdict:** kept — protective change, local-flat within noise. If we ever need
  certainty on its local cost, run the BENCHMARKING.md A/B interleave protocol.

## 2026-08-22 Wave 1: GOGC=off + GOMEMLIMIT=1600MiB

- **SHA:** 46894e4, dirty tree
- **Hypothesis:** GC ran ~1.2 cycles/s under load (gctrace, ~6MB heap), a few ms
  CPU each → disabling scheduled GC (with GOMEMLIMIT as the hard net) buys 1-4%.
- **Change:** app/Dockerfile ENV GOGC=off GOMEMLIMIT=1600MiB.
- **Result (devloop, ~3% noise):** work_score 310,723 → 310,320 (−0.1%, FLAT).
  Errors 0.21% → 0.00%. GC cost was already ~invisible at this heap size.
- **Verdict:** kept — score-neutral, mechanically removes STW pauses from tails,
  memory-safe via limit. RSS watched via new peak-RSS field in bench records.

## 2026-08-22 Wave 1: Gosched yield in the hash loop (stride from calibration)

- **SHA:** 46894e4, dirty tree
- **Hypothesis:** /price p95 == /stats p95 in all 9 recorded runs despite ~1500×
  compute gap → fast-path latency is scheduler wait behind hashing goroutines
  (~10ms preemption quantum), fixable by voluntary yields ~every 1ms of hashing.
- **Change:** app/main.go riskChain yields every stride iterations (power-of-2,
  derived at boot: ~1ms slices; RISK_YIELD_STRIDE env override; boot log prints it).
- **Result (devloop):** /price p95 139.9ms → 12.0ms (~12×); work_score 310,320 →
  309,606 (−0.2%, FLAT — score ≈ 25×chains held exactly, as the fleet predicted:
  faster cheap responses do NOT add score, they only add bar margin). Risk p95
  1.0s→1.2s and errors 0.28% (more risk arrivals/s → more shed pressure) —
  absorbed by the Wave-2 LIFO rework.
- **Verdict:** kept — 10× fast-path DQ margin for ~0 score cost.

## 2026-08-22 Wave 1: Step-0 interleave go/no-go microbench (C intrinsics, arm64)

- **SHA:** 46894e4 (bench.c in session scratchpad, not shipped)
- **Hypothesis:** 2 independent SHA-256 chains interleaved instruction-by-
  instruction on ONE core beat 2 sequential chains, by hiding crypto-unit
  instruction latency (mechanism behind the Wave-2 x86 kernel).
- **Change:** none to app. 170-line NEON-crypto microbench: 1-lane vs 2-lane
  50k-iteration 2-block chains; digest self-test matches hashlib exactly.
- **Result:** 1-lane 3.42ms/chain (292.6 chains/s/core); 2-lane 4.15ms/pair
  (482.1 chains/s/core) → **interleave ratio 1.65× on Apple silicon**. Bonus
  signal: C 3.4ms vs Go-in-container 11.9ms per chain → Go kernel carries
  per-call overhead worth attacking in the same Wave-2 kernel.
- **Verdict:** GO for the Wave-2 2-lane kernel. x86 magnitude TBD on real
  silicon (literature: +60-98% Zen, +38% SPR, +4% ICL).

## 2026-08-22 Wave 1: CFS-throttle hypothesis test

- **Result:** cpu.stat over a full devloop: 88 throttle events, 37ms total
  throttled time (~0.05% of one core-second) → CFS throttling is NOT the
  fast-path latency cause at GOMAXPROCS=2; scheduler-quantum theory confirmed
  by the yield result above. GOMAXPROCS=2 pin validated.
- **Verdict:** hypothesis closed; supports the GOMAXPROCS=3 dead-end ruling.
