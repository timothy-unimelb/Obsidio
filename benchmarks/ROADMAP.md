# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Local champion:** interleaved multi-lane risk batching, commit `b29480e`
  (workers drain up to four queued jobs and hash them as interleaved
  independent chains). Built on the compact hex-unrolling server with two
  permanent risk workers, bounded FIFO queue, cheap-path bypass, and
  allocation-free packed-hex kernel.
- **Accepted (separated x86) champion:** compact hex unrolling, evaluated at
  `c18b148`, now carried at `45ce2c7` with only documentation changes.
- **Latest local evidence:** `risk-lanes-full-20260822`; candidate 3,316,347
  versus controls 3,038,794 / 3,039,407 (+9.11% vs stronger, +0.02% drift,
  zero errors, `/risk` p95 −88 ms, `/price` p95 +4.5 ms).
- **Mechanism:** the previous profile's 58% `encodeDigest` attribution was a
  stall waiting on hardware SHA results; interleaving independent chains lets
  the core overlap that latency. Neutral with `GODEBUG=cpu.all=off`.
- **Rejected this session:** Go PGO (−19.7% kernel, 50k allocs) and
  fixed-shape scalar SHA-256 (5.9× slower than Go's hardware path). Both stopped
  at Level 0.
- **Outstanding finalist evidence:** separated x86 confirmation of multi-lane,
  interleaved six-run milestone, optional-instruction portability run, and a
  rerun after the organizers lock the grader.
- **Separated environment:** AWS stack under `benchmarks/aws/` is destroyed;
  re-provision only for the x86 confirmation below.

## Active sequence

1. **Completed: Level 0 multi-lane prototype.** Per-chain kernel −14.7% (pair)
   and −24.6% (quad) on arm64; neutral on the scalar path; correct for batch
   sizes 1–4 in both CPU modes. See `experiments/2026-08-22-risk-lanes.md`.
2. **Completed: bracketed screen.** +7.57% vs stronger control, +0.08% drift.
3. **Completed: exact full bracket.** +9.11% vs stronger control, +0.02% drift.
   Verdict: kept as local champion.
4. **Next: separated x86 confirmation.** Provision AWS, run the bracketed screen
   then exact full bracket, candidate `b29480e` against champion `45ce2c7`. If
   the x86 gain is materially smaller, run `RISK_LANES=2` vs `4` as a secondary
   set. Destroy the stack afterwards.
5. **Then:** update `submission/go/README.md` and `RESILIENCE.md` from the x86
   result, and schedule the six-run milestone for the finalist.

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

**Status:** multi-lane batching (`b29480e`) is the local champion after a clean
+9.11% full bracket; it is not yet the accepted submission champion because the
gain depends on hardware SHA latency behaviour and needs separated x86
confirmation. Next action: provision AWS (paid; requires `AWS_CONFIRM`), run
screen then full bracket of `b29480e` vs `45ce2c7`, record the decision, and
destroy the stack.
