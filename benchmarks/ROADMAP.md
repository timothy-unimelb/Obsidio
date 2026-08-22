# Obsidio performance roadmap

This is the durable handoff for ongoing optimization work. Read it after
`PROTOCOL.md` and `history.jsonl`. Keep it current whenever an experiment starts,
reaches a gate, or changes what should happen next.

## Current position

- **Champion:** `8cc47de` — the governor build `6808e7d` plus the raw-TCP
  HTTP/1.1 server and the sixteen-lane AVX-512 insurance kernel (both ported
  from `fork/advait`). `RISK_HTTP=std`, `RISK_X16=off`, `RISK_SHED=0`,
  `RISK_SHANI=0` each restore the plainer build.
- **Milestone (Level 3, `milestone-x86-full-20260822`):** final 4,397,038
  (4,380,394–4,402,197) vs governor-off 4,187,402 (4,177,395–4,204,856),
  +5.0% on medians, 0.85% errors, all bars, spread 0.5%. This instance runs
  the old `6808e7d` build at 4.31–4.34M (4.85M on the previous one); the
  governor-off build is flat across instances, the governor build moves with
  the hardware. Load generator checked at the peak: 23–40% busy, not the cap.
- **Raw server:** +1.90% bracket, repeat pair confirms; pooled medians +1.6%,
  ranges disjoint, errors unchanged; `/price` p95 9.5 → 10.2 ms, `/risk` mean
  92 → 83 ms. See `experiments/2026-08-22-raw-http-x16.md`.
- **Sixteen-lane kernel:** off on SHA-NI processors by default (forced race
  1.20×). Simulated no-SHA-NI grader: 769,042 / 773,118 portable →
  3,311,947 (4.3×), bars pass in both regimes; boot race 7.45×.
- **Rejected / settled:** PGO; scalar fixed-shape SHA; yield every 2,048
  rounds (−9.8%); pure LIFO shedding at the published load; late queue
  (broke both gates at 800 VUs); shedding at 800 VUs.
- **Not re-measured:** the 800-VU stress on the final build (the overload
  policy is unchanged; the raw server's only behavioural difference there is
  that a client that gives up no longer cancels its parked job, which makes the
  budget accounting more conservative, never less).

## Active sequence

Submission frozen at the deadline. If work resumes after the grader locks:

1. Re-run the milestone on the locked grader.
2. If the error bar tightens below 1%, ship `RISK_SHED=0` (zero errors,
   −5 to −17% depending on the instance).
3. Cheap sweeps: yield cadence 64/128 vs 256; `RISK_LANES=2` vs 4.

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

**Status:** `draft` is the submission: champion `8cc47de` plus docs and
evidence commits on top. Six-run milestone, raw-server brackets, and the
no-SHA-NI simulation are recorded; AWS destroyed. See `HANDOVER.md`.
