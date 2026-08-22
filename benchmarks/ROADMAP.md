# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Champion:** two-lane SHA-NI risk kernel, commit `8075efe`
  (`submission/go/risk_amd64.s`): both lanes' fixed 64-byte SHA-256 in one
  interleaved assembly routine, constant padding block precomputed, CPUID
  gated with `RISK_SHANI=0` fallback. On top of multi-lane batching, two
  permanent workers, bounded queue, cheap-path bypass, packed-hex encoder.
- **Latest accepted evidence:** `shani-x86-full-20260822`; candidate
  2,545,521, +21.76% over the stronger control with −0.92% drift, zero
  errors. Kernel: 6.21 → 2.99 ms per chain on Sapphire Rapids (−52%).
- **Origin:** the idea and references came from Advait's `PLAN.md`
  (`fork/advait`, never started there).
- **Cost to watch:** `/price` p95 is now ~40 ms on x86 (was ~24 ms). Bar is
  200 ms. First knobs if the locked grader tightens it: `RISK_LANES=2` or a
  periodic yield in the batch loop.
- **Rejected this session:** Go PGO, fixed-shape scalar SHA-256.
- **Outstanding finalist evidence:** interleaved six-run milestone,
  `RISK_SHANI=0` full-run portability set, rerun after the grader locks.
- **Separated environment:** AWS stack destroyed.

## Active sequence

1. **Completed: SHA-NI kernel Level 0, x86 screen (+23.3%), x86 full
   (+21.8%).** See `experiments/2026-08-22-shani-kernel.md`.
2. **Next candidate (optional): in-register hex via `PSHUFB` inside the
   kernel** so the 64-byte hex input for the next round is produced in
   assembly and the Go `encodeDigest` calls disappear. Level 0 on x86 first.
3. **Then: finalist validation.** Six-run milestone `A B B A A B` on x86,
   `RISK_SHANI=0` full set, update `submission/go` docs from those results.

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

**Status:** SHA-NI kernel (`8075efe`) is the accepted champion on separated
x86. AWS destroyed. Submission docs still describe the Go-lanes champion and
need the kernel section and the new x86 figures. Next: docs update, then
either the in-kernel hex candidate (Level 0 on x86) or finalist validation.
