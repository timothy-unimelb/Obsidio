# Sprint 3: close the gap to Tim's 4.84M (post-4.30M)

SELF-CONTAINED plan for an executing agent with no chat context. Read this
whole file, then work the Progress checklist in order. Repo: branch `advait`
of github.com/timothy-unimelb/Obsidio, working dir `/Users/advait/Code/Obsidio`.

## Why this sprint exists (evidence)

Tim's branch `origin/draft` (app in `submission/`) recorded a bracketed
head-to-head on his c7i pair (`benchmarks/decisions/advait-vs-tim-x86-full-
20260822.json` on his branch, exact grading script, our frozen build
`c0c8f95` vs his `c6bf541`):

| Run | Score | Errors | Price median | Risk p95 |
| --- | ---: | ---: | ---: | ---: |
| Ours A1 | 4,321,831 | 0.84% | 6.79 ms | 76 ms |
| Tim B1 | 4,835,626 | 0.87% | 3.35 ms | 89 ms |
| Ours A2 | 4,285,200 | 0.85% | 7.06 ms | 74 ms |

+11.9% at the same error budget. His record's interpretation: architectures
converged (he ported OUR shedding governor; kernels equivalent; GOGC=off both)
— the remaining gap is **cheap-path latency** (price median 3.35 vs ~6.9 ms),
which converts to score in the budget-limited closed loop. His named
candidates for the difference:

1. **Yield cadence 256 rounds** (~60 µs chunks, `runtime.Gosched()` between
   kernel chunks) vs our auto-calibrated stride (lands at 8192 ≈ ~1 ms slices
   on SHA-NI x86). His cadence sweep: 2048 was −9.8% vs 256. His yield
   bracket was +53.8% on his build (before his governor port).
2. **4-lane batches** vs our pairs (his quad 2.66 ms/chain vs pair
   3.07 ms/chain, ~13% better per-chain).

**Why our sprint-1 "stride sweep falsified" verdict does NOT cover this:**
that sweep (EXPERIMENTS.md "Yield-stride sweep") tested {8192, 2048, 1024}
only — never 256 — on devloop only, single runs, at champion 09a1879 which
predates the shedding governor. Under the governor, shed VUs recycle into
cheap traffic, which is exactly the loop that cheap-path latency gates.
Tim's head-to-head is direct evidence the conversion happens at ~0.85% shed.

Also worth taking regardless of score: our `shedBudgetAllows`
(app/main.go:773) is check-then-charge — concurrent sheds can burst past
88bp. Tim hit exactly this failure mode (late-queue variant: 1.489% errors at
800 VUs) and fixed it with a CAS reservation in the decision step
(his commit `6808e7d`), verified score-inert. Our margin to the 1% DQ gate is
only 0.15pp; this is cheap insurance.

## Testbed / protocol (same as sprint-2)

- Stack `obsidio-bench-advait`, region **ap-southeast-2**. Target c7i.xlarge
  (private `10.27.1.115` stable; public IP changes on restart), load gen
  c7i.large. `AWS_REGION=ap-southeast-2 AWS_BENCH_STACK=obsidio-bench-advait
  ./benchmarks/aws/status.sh` for state/IPs. SSH:
  `ssh -i benchmarks/aws/.state/obsidio-bench-advait.pem ec2-user@<public-ip>`.
- Boxes carry `shutdown -P` timers — **re-arm before long work**:
  `sudo shutdown -c; sudo shutdown -P +240` on each box.
- A/B bracket (mandatory for keep/revert):
  `AWS_REGION=ap-southeast-2 AWS_BENCH_STACK=obsidio-bench-advait
  BENCH_COMPARISON_SET=<set> RUN_PREFIX=<prefix>
  ./benchmarks/aws/run-comparison.sh /Users/advait/Code/obsidio-champion/app
  /Users/advait/Code/Obsidio/app <screen|full>` — both contexts committed and
  clean; champion worktree at `../obsidio-champion` (move with
  `git -C ../obsidio-champion checkout <sha>`). Screen first (90 s runs),
  full bracket to confirm a keep. Noise floor 0.8%; keep-threshold ≥ +1%.
- Record: decision JSON via `node benchmarks/record-decision.mjs`, narrative
  in EXPERIMENTS.md (failures and reverts too), commit via /commit-push.
- Baseline champion: HEAD `1f18512` (app/ byte-identical to freeze `5c42e72`,
  4.299M ±0.36% over 3 cold boots).

## Items, strict order

### Item 1 — yield stride 256 (env-only, highest expected value)

Candidate = one-line commit: `ENV RISK_YIELD_STRIDE=256` in `app/Dockerfile`
(the env override already exists, app/main.go:490). Screen bracket, then full
bracket if the screen is ≥ +1%. Expected from Tim's data: most of +11.9%,
price median roughly halves, risk p95 rises slightly (fine vs 1500 ms bar).

If KEPT, decide the shipping form before freeze: hard ENV 256 is simplest but
wrong for a no-SHA-NI grader (30 ms chains → 256-round chunks are still fine;
the auto-calibration exists precisely for slow boxes — check: auto on a slow
box already yields ≤1 ms slices, and 256 rounds on a slow box is ~150 µs, so
hard 256 is safe there too, just slightly more yield overhead). Preferred:
change the calibration target from ~1 ms to ~60–100 µs slices so it stays
adaptive; re-bracket the calibrated form (or ship env if identical).
Also re-run the no-SHA-NI insurance regime check (RISK_KERNEL=avx512 forced
sim, sprint-2 Item C) to confirm no regression there.

### Item 2 — atomic shed-budget reservation (correctness, expect inert)

Port Tim's CAS-reservation semantics into `shedBudgetAllows` callers: reserve
the error slot atomically in the same step as the shed decision (CAS on the
error counter against the budget), instead of check-then-charge. Reference:
his commit `6808e7d` on `origin/draft` (submission/). His measurements: score
inert at published load AND 800 VU stress (1,663,973 @ 0.878%). Verify: unit
test for no-overshoot under concurrent sheds; screen bracket to confirm inert;
800 VU overdrive run (k6/overdrive.js) to confirm errors stay <1%.

### Item 3 — 4-lane batches (ONLY if Items 1–2 leave a gap vs ~4.8M)

Our x16 kernel at k=4 runs at 4/16 efficiency — useless for this; a real
4-lane interleaved SHA-NI kernel is needed (port Tim's `riskChain4x` from
`origin/draft:submission/`, or extend `benchmarks/gen2lane.py` to 4 lanes).
Full correctness walls mandatory (differential vs crypto/sha256, 50k naive
reference, boot self-test + racing). Expected ~13% on risk CPU only —
worth it only if the head-to-head gap persists after Item 1.

### Item 4 — parity check (optional, cheap)

After keeps land: one full bracket of our new champion vs Tim's `c6bf541`
build context (`git worktree` of `origin/draft`, context `submission/`) on
OUR testbed pair, to confirm the gap is closed rather than inferred.

## Endgame reminders (unchanged from PLAN.md)

Write-up + video still owed (PLAN.md W4); submission-morning launch blocker:
set RISK_ERR_GATE_BP when final thresholds publish and re-verify once.
Feature freeze discipline: every keep needs /smoke green and its bracket.

## Progress

- [ ] Item 1: stride-256 candidate commit + screen bracket
- [ ] Item 1: full bracket + keep/revert decision recorded (decision JSON +
      EXPERIMENTS.md entry)
- [ ] Item 1 (if kept): shipping form decided (hard env vs recalibrated
      target), no-SHA-NI regime re-check, /smoke green, committed
- [ ] Item 2: CAS budget port + unit test + screen (expect inert) + 800 VU
      overdrive check, committed with EXPERIMENTS.md entry
- [ ] Item 3 (conditional): 4-lane kernel decision — only if gap vs 4.8M
      persists; correctness walls + bracket if attempted
- [ ] Item 4 (optional): head-to-head vs Tim's c6bf541 on our testbed
- [ ] HANDOFF.md refreshed; new freeze verified (3× cold boots) if champion
      changed
