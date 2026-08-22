# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Champion:** `5c3d681` — SHA-NI kernel with hex encoding and the chain loop
  in assembly, called in 256-round chunks with a yield between them — carried
  forward in `4d1d2ea` with durable `POST /price` (fsynced WAL on `/data`) and
  an opt-in overload gate (`RISK_SHED=1`, off by default).
- **Latest accepted evidence:** `shani-yield-x86-full-20260822`: 3,598,675 vs
  2,338,684 / 2,339,747 (+53.8%, +0.05% drift, 0 errors, price p95 10.4 ms).
  `gap-x86-full-20260822` confirms the additions are inert (−0.26%, 0 errors).
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

**Status:** champion `5c3d681`/`4d1d2ea` at 3.60M on separated x86; gaps
bridged (persistence on, shedding opt-in); visuals and the results artifact
updated; AWS destroyed. Next: profile the HTTP path under load before
choosing between allocation trims and a custom HTTP server.
