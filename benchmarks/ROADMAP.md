# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Champion:** interleaved multi-lane risk batching, commit `b29480e`
  (workers drain up to four queued jobs and hash them as interleaved
  independent chains). Built on two permanent risk workers, bounded FIFO
  queue, cheap-path bypass, allocation-free packed-hex kernel with compact
  unrolling.
- **Latest accepted evidence:** `risk-lanes-x86-full-20260822`; candidate
  2,062,911, +2.21% over the stronger control with −0.75% drift, zero errors,
  all gates passed. Local arm64 bracket was +9.11% with +0.02% drift.
- **Mechanism:** the earlier 58% `encodeDigest` attribution was the core
  stalling on hardware SHA results; interleaving independent chains overlaps
  that latency. Kernel gain ≈25% on Apple Silicon, ≈5% on SHA-NI, neutral with
  `GODEBUG=cpu.all=off`. `RISK_LANES=4` default (pair ≈ quad on x86).
- **Rejected this session:** Go PGO (−19.7% kernel, 50k allocs) and
  fixed-shape scalar SHA-256 (5.9× slower). Both stopped at Level 0.
- **Outstanding finalist evidence:** interleaved six-run milestone,
  optional-instruction portability full-run set, and a rerun after the
  organizers lock the grader.
- **Separated environment:** AWS stack destroyed after the comparison; the
  CloudFormation lifecycle scripts under `benchmarks/aws/` re-create it.

## Active sequence

1. **Completed: multi-lane Level 0, local screen, local full.** See
   `experiments/2026-08-22-risk-lanes.md`.
2. **Completed: separated x86 screen and full bracket.** +1.57% screen, +2.21%
   full; accepted.
3. **Completed: x86 kernel lane-count check.** Pair and quad equivalent on
   SHA-NI; default stays 4.
4. **Next: decide whether more kernel work is worth it.** The remaining
   measured cost is the SHA chain itself; Go-level options are now largely
   exhausted (scalar SHA, PGO, hex encoding, lanes). Candidates below are
   speculative and each needs a Level 0 case before any load run.
5. **Then:** six-run milestone and portability set for the finalist, and a
   final docs pass before submission.

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

**Status:** multi-lane batching (`b29480e`) is the accepted champion on both
local arm64 and separated x86. AWS stack destroyed. Submission docs updated
with the x86 numbers. Next: either a Level 0 case for a further kernel idea
from the candidate queue, or move to finalist validation (six-run milestone,
portability set). Re-provision AWS only for an exact x86 comparison.
