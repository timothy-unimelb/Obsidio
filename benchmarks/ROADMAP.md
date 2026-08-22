# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Champion:** compact four-at-a-time hex unrolling from implementation commit
  `5bb6824` (evaluated source `c18b148`), on top of the Go server with two permanent risk workers,
  bounded FIFO queue, cheap-path bypass, allocation-free risk loop, and packed
  lowercase-hex feedback.
- **Latest accepted evidence:** `hex-unroll-x86-full-20260822`; candidate scored
  1,987,151, 1.70% above the stronger bracket champion and 2.10% above the
  bracket average, with zero errors. Control drift was only -0.79%.
- **Architecture phase:** the major concurrency and allocation improvements are
  complete. Work has moved into evidence-led compute-kernel and compiler
  optimization.
- **Latest rejected candidates:** Go 1.26.6 PGO regressed the risk kernel by
  19.67% and reintroduced 50,000 allocations per request. Portable fixed-shape
  SHA was then 5.86x slower than Go's accelerated standard-library path. Both
  were stopped at Level 0 without spending screening or AWS time.
- **Outstanding finalist evidence:** interleaved six-run milestone,
  optional-instruction portability run, and rerun after the organizers lock the
  grader.
- **Separated environment:** reproducible AWS CloudFormation and lifecycle
  scripts live under `benchmarks/aws/`. The first x86 comparison was validated
  successfully; all paid resources were destroyed afterward.

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
5. **Completed: exact full bracket.** Candidate scored 2,919,075 versus controls
   at 2,892,463 and 2,783,480. It was +0.92% versus the stronger side and +2.86%
   versus the bracket average, but control drift was -3.77%. Verdict: unresolved;
   `64e38e0` remains champion and `5bb6824` preserves the candidate.
6. **Completed: separated x86 resolution.** The x86 screen showed +2.91% versus
   the stronger control with -0.85% control drift. The exact full bracket showed
   +1.70% versus the stronger control with -0.79% drift. The candidate is kept
   as the current champion.
7. **Completed: Go PGO Level 0.** A 95-second representative profile was applied
   with the pinned Go 1.26.6 toolchain. The candidate remained correct but made
   `BenchmarkRisk` 19.67% slower and changed it from zero allocations to 50,000
   allocations and 6.4 MB per operation. Verdict: reverted before screening.
8. **Completed: portable fixed-shape SHA Level 0.** A two-block SHA-256 path
   reused the constant padding schedule and passed 10,000 randomized vectors,
   complete risk vectors, and the portability run. It was 8.86x slower for one
   SHA operation and 5.86x slower over the complete kernel. Verdict: reverted
   before screening.

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

**Status:** compact hex unrolling remains champion; Go PGO and portable
fixed-shape SHA were rejected at Level 0, and the AWS stack remains destroyed.
Next, evaluate whether a multi-lane or batched risk kernel has a credible
accelerated implementation; otherwise move to small profiled HTTP-path changes.
Re-provision AWS only after local Level 0 and bracketed screening earn an exact
x86 comparison.
