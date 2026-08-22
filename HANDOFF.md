# Handoff — 2026-08-22 (sprint-3 session)

**Best result / current submission build: `ee09171`** — freeze3-x86-01
verified 3× cold boots on the testbed: **4,801,382 / 4,761,262 / 4,805,018**
(spread 0.92%, mean 4,789,220), errors 0.85–0.87%, 4/4 bars every run (risk
p95 ~82ms, price/stats p95 9.7ms). Sprint-3 day: 4.30M → 4.79M (+11.4%).
No-SHA-NI insurance regime re-checked on this build: **3,255,335**, 4/4 bars,
quad gate verified silent (−7.8% vs sprint-2's single-run 3.53M; both single
runs, regime noise unmeasured — first suspect if it ever matters: in-regime
stride A/B).

**What changed this session** (narrative: EXPERIMENTS.md sprint-3 entries;
verdicts: benchmarks/decisions/; plan: PLAN-GAINS.md, complete):
- Sourced from Tim's branch (`origin/draft`): his head-to-head showed our
  frozen build −11.9% vs his 4,835,626, attributed to cheap-path latency.
- Item 1 `RISK_YIELD_STRIDE=256` (c11949b): KEPT +2.0% full bracket →
  4,430,837. Supersedes sprint-1's "stride flat" verdict (never tested 256,
  pre-governor).
- Item 2 atomic CAS shed-error reservation (7eae870, port of his 6808e7d):
  screen INERT, kept on correctness — removes the check-then-charge burst
  race (our biggest DQ risk; 0.15pp margin to the 1% gate). Race-hammered
  unit test TestShedReserveNoOvershoot. 400-VU overdrive with the final
  build: k6 exit 0 @ 0.83% errors.
- Item 3 4-lane SHA-NI quad kernel (ee09171, his generated asm vendored as
  app/sha4lane_amd64.s behind our walls: 640-case boot differential, test
  suite, ≥5% boot race — measured 1.12× on SPR, RISK_X4 kill switch):
  KEPT **+9.4% at 0.036% drift** → 4,814,186. Workers pop up to 4; short
  pops degrade via pair/serial; gate silent off-SHA-NI.
- /smoke 35/35 after the endpoint-logic (shed write path) change.
- Result: parity with Tim's build plus our raw-TCP path (+2.5%, his build
  is stock net/http) and AVX-512 no-SHA-NI insurance (he has none).

**In flight / uncommitted:** nothing after this handoff commit; all pushed
to `advait`. No containers left on target/load/local. EC2 shutdown timers
re-armed to ~12:39 UTC (boxes self-poweroff; restart from EC2 console in
ap-southeast-2, public IPs change, private target 10.27.1.115 stable).
Champion worktree ../obsidio-champion at ee09171.

**Next step:** archive PLAN-GAINS.md → archived-plans/, then draft the
judged deliverables — `/writeup` from EXPERIMENTS.md + benchmarks/decisions
(PLAN.md W4; the sprint-3 story — learning from a teammate's bracketed
evidence in both directions — belongs in the pitch). Submission-morning
launch blocker unchanged: when final thresholds publish, set
RISK_ERR_GATE_BP in app/Dockerfile and re-run one grading verification.
