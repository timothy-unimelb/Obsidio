# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Champion:** commit `64e38e0`, Go server with two permanent risk workers,
  bounded FIFO queue, cheap-path bypass, allocation-free risk loop, and packed
  lowercase-hex feedback.
- **Latest accepted evidence:** `hex-packed-full-20260822`; candidate scored
  2,852,984, 2.14% above the stronger bracket champion, with zero errors. This
  remains local Apple Silicon evidence with substantial host drift, not a judge
  prediction.
- **Architecture phase:** the major concurrency and allocation improvements are
  complete. Work has moved into evidence-led compute-kernel and compiler
  optimization.
- **Outstanding finalist evidence:** interleaved six-run milestone, separated
  x86-64 validation, optional-instruction portability run, and rerun after the
  organizers lock the grader.

## Active sequence

1. **Completed: re-profile the packed-hex champion.** `encodeDigest` remains the
   largest flat CPU cost at 57.84%; SHA accounts for 24.18%. See
   `experiments/2026-08-22-packed-champion-profile.md` and the raw profiles.
2. **Active candidate: compact fixed-size hex unrolling.** A fully expanded
   32-store version accelerated the isolated encoder but not the complete risk
   kernel, likely because it stopped inlining. The smaller four-at-a-time form
   improved the complete kernel by 2.50%. PGO remains next if later rejected.
3. **Completed: validate the active candidate at level 0.** Independent endpoint
   and risk tests pass with optional CPU acceleration disabled; focused
   microbenchmarks show a material kernel gain.
4. **Completed: bracketed 90-second screen.** Candidate scored 1,007,901 versus
   controls at 992,877 and 980,888: +1.51% versus the stronger side and +2.13%
   versus the bracket average, with zero errors. It is promoted using the
   protocol's bracketed-evidence path for small changes.
5. **Active: run an exact full bracket.** Keep, reject, or mark
   unresolved with a decision record and update this file.

## Candidate queue after the active sequence

Priority is evidence-dependent, not a promise to implement every item:

1. Go profile-guided optimization (low source complexity; must prove portable
   benefit and reproducible build inputs).
2. Fixed-shape or multi-lane SHA processing, only if the new profile shows SHA
   dominates and a portable implementation can beat Go's selected SHA path.
3. Small HTTP/response-path reductions, only if profiles show they affect score
   rather than merely microbenchmarks.
4. C or Rust kernel integration only after Go-level options are exhausted; the
   FFI/build complexity and cross-architecture risk require a material gain.
5. Optional persistence bonus after the core-score finalist is stable.

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

**Status:** re-profile, Level 0, and screening are complete. Compact hex
unrolling is promoted to a champion -> candidate -> champion exact full
comparison. No post-packed-hex candidate has been accepted yet.
