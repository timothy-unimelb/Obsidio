# Experiments log

One entry per optimisation attempt. This file IS the resilience write-up's evidence —
numbers, not adjectives. Log failures and reverts too; before/after numbers come from
`bench/history.jsonl`, never from memory. Newest entry at the bottom.

Entry format (see the /experiment skill):

```markdown
## YYYY-MM-DD Short title
- **SHA:** abc1234
- **Hypothesis:** ...
- **Change:** files + one-liner
- **Result:** work_score X → Y; p95 price a→b ms, stats c→d ms, risk e→f ms; errors g→h
- **Verdict:** kept | reverted | pending
```

## 2026-08-21 Engineered Go v1 vs naive Go baseline (capped)

- **SHA:** 4519449, dirty tree (scaffolding + app/ not yet committed)
- **Hypothesis:** The /risk hash chain gates the closed loop. A Go app with
  GOMAXPROCS=2, /risk bounded to 2 concurrent chains (semaphore; waiters park),
  a zero-allocation sha256/hex kernel, and reflection-free JSON should raise
  work_score and cut all p95s vs the naive Go starter.
- **Change:** new `app/` (main.go, go.mod, Dockerfile). Baseline = untouched
  `starters/go` in the same capped container (2 CPU / 2 GB, cpuset 0,1),
  same-machine k6 (directional).
- **Result:** work_score 745,484 → 1,273,640 (+71%); p95 price 163.4→119.0ms,
  stats 162.7→118.5ms, risk 1011.4→320.7ms; errors 0%→0%; requests 297k→509k.
  (history.jsonl lines 2 and 3; line 1 is an uncapped Node run, not comparable.)
- **Verdict:** kept

## 2026-08-21 Go 1.22 → 1.26 base image (stdlib SHA-NI on amd64)

- **SHA:** 911351a, dirty tree (Dockerfile bump only at bench time)
- **Hypothesis:** Go 1.25+ adds a SHA-NI path to crypto/sha256 on amd64 (release
  notes: ~2x). Our image shipped Go 1.22 (AVX2-only path). On x86 grading hardware
  with SHA extensions (all AMD Zen, Intel Ice Lake+) this should ~double /risk
  throughput; on this arm64 dev host it should be ~flat (ARM sha2 path unchanged).
- **Change:** app/Dockerfile FROM golang:1.22-bookworm → golang:1.26-bookworm.
- **Result:** work_score 1,148,230 → 1,109,499 (−3.4%, inside the ~10% same-machine
  noise floor; history lines 5 vs 8). p95 price 110.7→103.8ms, stats 110.3→104.1ms,
  risk 835.7→672.2ms; errors 0%→0%. Line 7 was a contaminated run (concurrent
  docker build stole CPU mid-bench) — excluded from the comparison.
- **Verdict:** kept — local-neutral as expected; the claimed ~2x is x86-only and
  unmeasurable on this host. Must be verified on an x86 box before submission-day
  claims. /smoke passed (all 35 checks) on the new toolchain.

## 2026-08-21 Boot-calibrated /risk gate (adapted from joel/draft@894f469)

- **SHA:** 911351a, dirty tree (calibration + Go 1.26 Dockerfile in working tree)
- **Hypothesis:** Replacing the unbounded /risk wait queue with boot-derived limits
  (timed median-of-3 chains → wait deadline = 1500ms − chain − 300ms margin; queue
  via Little's Law × 3; overflow 503s; waiters cancel on deadline/disconnect) costs
  ~nothing locally (queue never fills here) and buys self-tuning robustness on
  unknown grading hardware, where hardcoded limits could let /risk rot past the bar.
- **Change:** app/main.go: calibrateRisk() at boot; bounded wait + deadline + shed
  in handleRisk. Boot log on this host: unitCost=11.2ms, maxQueued=636, wait=1.19s.
- **Result:** work_score 1,109,499 → 1,041,111 (−6.2%, inside ~10% noise floor;
  history lines 8 vs 9). p95 price 103.8→96.6ms, stats 104.1→96.5ms, risk
  672.2→1,046.0ms (risk p95 is the noisiest metric across all runs: 321–1,046 for
  similar builds). Errors 0%→0.053% (~220 deadline sheds of 418k reqs — the valve
  engaging at the margin, far under the 1% gate). Mechanically the gate adds only
  an atomic counter + non-blocking select on the hot path (ns), so the −6.2% is
  noise, not cost.
- **Verdict:** kept — protective change, local-flat within noise. If we ever need
  certainty on its local cost, run the BENCHMARKING.md A/B interleave protocol.

## 2026-08-22 Wave 1: GOGC=off + GOMEMLIMIT=1600MiB

- **SHA:** 46894e4, dirty tree
- **Hypothesis:** GC ran ~1.2 cycles/s under load (gctrace, ~6MB heap), a few ms
  CPU each → disabling scheduled GC (with GOMEMLIMIT as the hard net) buys 1-4%.
- **Change:** app/Dockerfile ENV GOGC=off GOMEMLIMIT=1600MiB.
- **Result (devloop, ~3% noise):** work_score 310,723 → 310,320 (−0.1%, FLAT).
  Errors 0.21% → 0.00%. GC cost was already ~invisible at this heap size.
- **Verdict:** kept — score-neutral, mechanically removes STW pauses from tails,
  memory-safe via limit. RSS watched via new peak-RSS field in bench records.

## 2026-08-22 Wave 1: Gosched yield in the hash loop (stride from calibration)

- **SHA:** 46894e4, dirty tree
- **Hypothesis:** /price p95 == /stats p95 in all 9 recorded runs despite ~1500×
  compute gap → fast-path latency is scheduler wait behind hashing goroutines
  (~10ms preemption quantum), fixable by voluntary yields ~every 1ms of hashing.
- **Change:** app/main.go riskChain yields every stride iterations (power-of-2,
  derived at boot: ~1ms slices; RISK_YIELD_STRIDE env override; boot log prints it).
- **Result (devloop):** /price p95 139.9ms → 12.0ms (~12×); work_score 310,320 →
  309,606 (−0.2%, FLAT — score ≈ 25×chains held exactly, as the fleet predicted:
  faster cheap responses do NOT add score, they only add bar margin). Risk p95
  1.0s→1.2s and errors 0.28% (more risk arrivals/s → more shed pressure) —
  absorbed by the Wave-2 LIFO rework.
- **Verdict:** kept — 10× fast-path DQ margin for ~0 score cost.

## 2026-08-22 Wave 1: Step-0 interleave go/no-go microbench (C intrinsics, arm64)

- **SHA:** 46894e4 (bench.c in session scratchpad, not shipped)
- **Hypothesis:** 2 independent SHA-256 chains interleaved instruction-by-
  instruction on ONE core beat 2 sequential chains, by hiding crypto-unit
  instruction latency (mechanism behind the Wave-2 x86 kernel).
- **Change:** none to app. 170-line NEON-crypto microbench: 1-lane vs 2-lane
  50k-iteration 2-block chains; digest self-test matches hashlib exactly.
- **Result:** 1-lane 3.42ms/chain (292.6 chains/s/core); 2-lane 4.15ms/pair
  (482.1 chains/s/core) → **interleave ratio 1.65× on Apple silicon**. Bonus
  signal: C 3.4ms vs Go-in-container 11.9ms per chain → Go kernel carries
  per-call overhead worth attacking in the same Wave-2 kernel.
- **Verdict:** GO for the Wave-2 2-lane kernel. x86 magnitude TBD on real
  silicon (literature: +60-98% Zen, +38% SPR, +4% ICL).

## 2026-08-22 Wave 1: CFS-throttle hypothesis test

- **Result:** cpu.stat over a full devloop: 88 throttle events, 37ms total
  throttled time (~0.05% of one core-second) → CFS throttling is NOT the
  fast-path latency cause at GOMAXPROCS=2; scheduler-quantum theory confirmed
  by the yield result above. GOMAXPROCS=2 pin validated.
- **Verdict:** hypothesis closed; supports the GOMAXPROCS=3 dead-end ruling.

## 2026-08-22 Wave 2B: risk-gate redesign trilogy (devloop A/Bs, ~3% noise)

- **SHA:** f1379c4 + working tree
- **Hypothesis (fleet):** FIFO+deadline sheds storm on slow hardware; adaptive
  LIFO fixes it at zero throughput cost.
- **Three measured variants (vs Wave-1 keeper devloop 309,606 / 0.29% err):**
  1. Pure LIFO, no deadline: 240,228 (−22%). Starved stack-bottom waiters hold
     closed-loop VUs hostage up to k6's 60s request timeout → active VU
     population shrinks → hash slots go hungry. Fleet's zero-cost claim WRONG
     for closed-loop scoring.
  2. LIFO + calibrated deadline (1.18s): 740,401 (+139%!!) but 8.38% errors =
     DISQUALIFIED. Mechanism discovery: shedding a stale risk waiter recycles
     its VU into ~5 fast cheap iterations (~50ms each round-trip) — cheap
     VOLUME scores. **Corrects the "score ≈ 25×chains" law: it only holds at
     zero shed.** The error gate, not chain throughput, is what caps score on
     overloaded-but-fast systems.
  3. LIFO + error-budget-governed patience (shed only while total error rate
     ≤0.6%, else park): 324,772 (+4.9%, above noise), 0.45% errors — but risk
     p95 3,403ms (devloop scale): parked stragglers served late blow the bar.
- **Verdict:** resolved by governor v2 (staleness-skip + front-door shed, next
  entries) — variant 3's p95 breach fixed, gate now shippable.

## 2026-08-22 Gotcha: boot calibration jitter under Docker Desktop VM

- unitCost median-of-3 across four boots of near-identical builds: 11.9, 12.7,
  15.8, 12.8 ms — ±15% boot-to-boot on this Mac. Derived knobs (patience,
  yield stride) inherit the jitter. Contended/EWMA calibration (planned) or
  more samples would stabilise; on real x86 hardware expect less VM noise.

## 2026-08-22 Governor v2a: staleness rule at grant time (devloop A/B)

- **SHA:** a8ec6c0, dirty tree
- **Hypothesis:** the trilogy's variant-3 p95 breach comes from parked
  stragglers SERVED at 2-4s; skipping any waiter older than the calibrated
  patience window (1500ms − unitCost − 300ms margin) at grant time removes
  those samples at 1/60-err/s cost each.
- **Change:** app/main.go — `enqueued` timestamp on riskWaiter;
  releaseRiskSlot unlinks-and-skips waiters aged past riskWaitTimeout.
  New unit test TestStaleWaiterSkippedAtGrant; race hammer still green.
- **Result (devloop, vs trilogy variant 3):** work_score 324,772 → 300,213;
  risk p95 3,403 → 2,300ms (still over devloop bar); errors 0.45% → 0.48%.
- **Verdict:** kept as a component, but insufficient alone — the remaining
  p95 damage is the SHED samples themselves: the governor spends its budget
  on waiters that already parked ≥1.19s, and k6 folds those 1.2-2.3s failed
  durations into the graded percentile stream (~5% of the /risk tier ⇒ p95
  lands on an error sample).

## 2026-08-22 Governor v2b: front-door budgeted shed (devloop + grading)

- **SHA:** a8ec6c0, dirty tree
- **Hypothesis:** spend the same error budget at ARRIVAL instead (503
  immediately when slots busy + stack non-empty + budget open): the error
  sample costs ~1ms instead of 1.2s+, and the closed-loop VU recycles into
  cheap scoring traffic ~1.2s sooner. Patience-shed retained only as a
  backstop drain (a 1.2s error sample beats a 60s timeout sample).
- **Change:** app/main.go riskAcquire — instant shed branch before parking;
  patience-shed comment updated to backstop role.
- **Result (devloop, vs v2a):** work_score 300,213 → 323,808; risk p95
  2,300 → 335ms (all devloop thresholds pass); errors 0.48% → 0.52%.
- **Result (grading.js, vs Wave-1 keeper line):** work_score 1,154,625 →
  1,097,306 (−5.0%, inside the ~10% grading noise floor; devloop read +4.6%,
  so score-neutral is the honest call); p95 price 12.0 → 11.9ms, stats
  12.0 → 11.9ms, risk 955.5 → 243.6ms; errors 0.009% → 0.551%; 4/4 bars,
  peak RSS 480MiB (GOMEMLIMIT=512MiB verified safe).
- **Verdict:** KEPT — this is the shippable gate. Score within noise of the
  keeper, /risk p95 margin 4× wider (244ms vs 1500ms bar), and the
  FIFO-shed cliff (self-DQ on slow hardware) is structurally gone: sheds are
  budget-capped at 0.6% by construction, overflow parks instead of storming.

## 2026-08-22 400-VU overdrive exhibit: FIFO+deadline vs governor v2

- **SHA:** efeec33 (governor) vs f1379c4 app/ (FIFO+deadline), new k6/overdrive.js
  (devloop stages at 400 VUs = 2× grading peak; side runs, NOT in history.jsonl)
- **Hypothesis:** the fleet's FIFO-shed-cliff prediction — on hardware/load where
  demand exceeds the deadline-shed equilibrium, the FIFO gate's 503 storm feeds
  itself (closed loop: rejected VU returns in ~50ms) and blows the 1% error gate,
  while the budget-governed gate degrades instead of collapsing.
- **Result (back-to-back, same machine):**
  - FIFO+deadline: **http_req_failed 5.03% → DISQUALIFIED** (5× the gate);
    raw work_score 436,143 (worthless — shed volume scores until the error gate
    voids the run); risk p95 1.21s.
  - Governor v2: **0.59% errors, all four thresholds pass**; work_score 289,108
    (−11% vs its own 200-VU devloop = graceful degradation); risk p95 611.7ms;
    price/stats p95 12.3ms.
- **Verdict:** exhibit banked for the write-up/video — same machine, same 2×
  overload, old gate disqualifies itself, new gate sheds exactly its budget
  (0.59% ≈ the 0.6% target) and keeps all bars green.

## 2026-08-22 Contended calibration (c=2) + live chain-cost EWMA

- **SHA:** efeec33, dirty tree
- **Hypothesis:** boot unitCost jitters ±15% across boots (recorded gotcha) and
  idle cost understates the contended cost the gate actually experiences; a
  contended boot measurement + live EWMA of real chain times should stabilise
  the patience/staleness window at zero score cost.
- **Change:** app/main.go — calibrateRisk adds a c=2 contended round (median
  seeds the EWMA); observeChainCost (α=1/8, samples clamped 1-500ms) re-derives
  patience after every chain; patience/staleness now atomic (riskPatienceNs),
  read per acquire/grant. Boot log confirms the gap: idle 12.8ms vs contended
  15.8ms (+23%).
- **Result (devloop, vs governor v2):** work_score 323,808 → 320,401 (−1.1%,
  noise); risk p95 335 → 386ms; errors 0.52% → 0.54%; all thresholds pass.
- **Result (grading.js):** work_score 1,097,306 → 1,154,460 — back to the
  Wave-1 keeper's exact level (1,154,625), confirming governor v2's −5% was
  noise; p95 price 11.9ms, stats 12.0ms, risk 233.2ms; errors 0.547%;
  4/4 bars; peak RSS 484MiB.
- **Verdict:** KEPT — score-flat robustness: gate constants now track real
  contended cost on unknown grading hardware instead of a one-shot idle boot
  sample. Boot-jitter gotcha closed.

## 2026-08-22 W3 hardening bundle (no perf claim)

- **SHA:** 161db98, dirty tree
- **Hypothesis:** none — robustness/receipts bundle, expected score-flat.
- **Change:** app/main.go — riskSlots sized from the cgroup CPU quota
  (cpu.max, v1 fallback; RISK_SLOTS env override; clamp [1,4]; GOMAXPROCS
  follows), boot fingerprint log (arch, host cores vs cgroup budget, cpu
  model, SHA-ISA flags — the "runtime detection, not hard-coded ISA" receipt),
  cpu.stat throttle logging (shutdown + OBSIDIO_TELEMETRY=1 periodic),
  ReadHeaderTimeout 5s + MaxHeaderBytes 8KB, SIGTERM graceful drain,
  env-gated pprof on :6060 (OBSIDIO_PPROF=1, off the graded port).
- **Result (devloop):** 320,401 → 338,810 (+5.7% with no mechanism → noise,
  not claimed); errors 0.53%; risk p95 332.7ms; all thresholds pass;
  /smoke 35/35. Boot log verified: cgroup_cpus=2.00 read correctly inside
  the capped container while the host advertises more cores.
- **Verdict:** kept — flame-graph-under-load still owed (needs a pprof run,
  ideally on the x86 box).
