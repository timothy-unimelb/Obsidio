# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Champion:** `49cfafb` (governor `d52d74b` plus the late-queue tail fix) — the yielding SHA-NI kernel build plus the
  budgeted shedding governor (88bp, on by default; `RISK_SHED=0` restores the
  zero-error build), single-lane fused kernel, `GOGC=off`/512MiB limit,
  durable `POST /price`.
- **Latest accepted evidence:** `governor-x86-full-20260822`: 4,854,704 vs
  4,143,222 / 4,141,819 (+17.2%, −0.03% drift, errors 0.847%, risk p95 80 ms).
  Passes all bars at 800 VUs (0.80% errors). See
  `experiments/2026-08-22-governor.md`.
- **Head-to-head:** ours 4,835,626 vs Advait's frozen build 4,321,831 /
  4,285,200 on the same c7i pair (+11.9%, same error budget); the difference
  is cheap-path latency (yield cadence, 4-lane batches).
- **Tail fixed:** held stale waiters are re-parked on a priority queue after
  2× patience; risk max 2.4 s (was 60 s), score and errors unchanged.
- **Mechanism:** once the kernel made risk cheap, the closed-loop request rate
  was bound by cheap requests waiting ~17 ms behind an unpreemptible asm loop.
  Yielding freed the fast path; score ∝ request rate.
- **Rejected / settled:** PGO; scalar fixed-shape SHA; yield every 2,048
  rounds (−9.8%); pure LIFO shedding at the published load (errors for no
  score); shedding at 800 VUs (+28% score at 3.5% errors vs plain design
  passing every bar).
- **Outstanding finalist evidence:** six-run milestone, `RISK_SHANI=0` full
  set, rerun after the grader locks. AWS stack destroyed.

## Active sequence

1. **Next score lever: the HTTP path.** With risk cheap, `net/http` overhead
   on ~1.4M cheap requests is the largest remaining CPU share. Profile the
   champion under full load first; then either allocation trims or a minimal
   hand-rolled HTTP/1.1 server for the four GET paths (est. +10–25%, a day).
2. **Cheap sweeps in one AWS session:** yield cadence 64/128 vs 256;
   `RISK_LANES=2` vs 4 and `RISK_WORKERS=1` vs 2 under the new regime.
3. **Finalist validation** (six-run milestone, `RISK_SHANI=0` set, fresh-clone
   build), then docs freeze.

## Candidate queue after the active sequence

Priority is evidence-dependent, not a promise to implement every item:

1. Multi-lane or batched risk processing, only if it can preserve accelerated
   SHA and improve complete-kernel throughput at Level 0.
2. Small HTTP/response-path reductions, only if profiles show they affect score
   rather than merely microbenchmarks.
3. C or Rust kernel integration only after Go-level options are exhausted; the
   FFI/build complexity and cross-architecture risk require a material gain.
4. Optional persistence bonus after the core-score finalist is stable.

## Stop and recording rules

- Never edit `k6/grading.js`.
- Keep one measured change per comparison set.
- Do not spend six full runs on routine candidates; reserve milestone validation
  for finalists.
- Do not compare absolute scores across machines or distant sessions.
- Append accepted, rejected, unresolved, and invalid outcomes to the evidence
  trail; raw summaries are never overwritten.
- Update **Current position** and **Active sequence** before ending a session,
  even if the experiment is incomplete.

## Resume marker

**Status:** `49cfafb` accepted: governor +17.2% over the zero-error build,
+11.9% over Advait's frozen build, tail bounded at ~2.4 s. AWS destroyed.
Next: (1) attribute the head-to-head gap (yield cadence vs lanes) in two
screens; (2) update submission README/RESILIENCE, visuals, and the results
artifact for the governor; (3) HTTP-path CPU profile.
