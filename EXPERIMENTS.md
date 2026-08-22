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

## 2026-08-22 x86 Tier-0: SHA-NI verified, Go 1.26-vs-1.22 bet falsified

- **SHA:** 02e4f99; testbed = c7i.xlarge (Xeon Platinum 8488C, Sapphire
  Rapids; sha_ni+avx+sse4_1+ssse3 all present), Tim's provisioned harness.
- **Hypothesis:** Go 1.25+ added stdlib SHA-NI (~2× /risk) — the basis of our
  golang:1.22→1.26 image bump.
- **Result (BenchmarkRiskChain, 50k-chain, benchtime 20-50x):**
  - Go 1.26 default 6.81ms/chain; GODEBUG=cpu.sha=off 16.96ms (**SHA-NI is
    selected and worth 2.49×** vs the AVX2 path); cpu.all=off 25.78ms.
  - Go 1.22 default 6.56ms/chain — **1.22 already uses SHA-NI**. The
    "added in 1.25" premise was wrong; the amd64 sha_ni path predates it.
  - 1.26 is ~4% SLOWER than 1.22 on this kernel (6.81 vs 6.56, consistent
    across 3×20x runs) — small; watch, don't act yet.
- **Verdict:** the 1.26 bump is NOT a score lever (keep the image for the
  container-aware runtime + toolchain currency unless the 4% proves real
  end-to-end). The true finding: any x86 grader with sha_ni runs our chain at
  ~6.8ms vs ~11.9ms on the local arm64 Docker — hardware, not toolchain.
  Corrects the open verdict of "Go 1.22 → 1.26 base image" above.

## 2026-08-22 x86 testbed baseline (bracketed, full grading.js × 3)

- **SHA:** 02e4f99 (champion == candidate: same build both sides, so the
  bracket measures pure testbed noise). c7i.xlarge target + separate c7i.large
  k6 host, Tim's harness, container capped 2 CPU / 2 GiB, k6 2.2.0 pinned.
- **Result:** work_score 1,953,437 / 1,953,713 / 1,937,842 (spread 0.8%;
  identical builds A1-vs-B1 within 0.01%). Errors 0.562-0.564% (governor
  sitting on its 0.6% target). p95: price 10.6ms, stats 10.6-10.7ms, risk
  148.7-155.9ms. All bars pass every run, k6 exit 0.
- **Reads:**
  1. **x86 headline ≈ 1.95M** = 1.69× our local 1.15M — silicon (SHA-NI),
     not toolchain. Ties Tim's champion (1.95M) on identical hardware.
  2. **Testbed noise floor ~0.8%** (vs ~10% local) — small effects are now
     resolvable; the A/B harness works as advertised.
  3. **True fast-path p95 = 10.6ms** — the local ~100ms pre-yield readings
     were Docker-Desktop-VM artifacts, as suspected in the plan.
  4. Governor + staleness + front-door shed behave identically on fast
     hardware: errors pinned just under budget, risk p95 margin 10×.
- **Verdict:** baseline banked; blocking-list items "true /price p95" and
  "end-to-end grading run" verified. Next: packed-hex A/B on this testbed.

## 2026-08-22 Packed pair-table hex encoder (x86 full A/B — KEPT)

- **SHA:** candidate a446bd0 vs champion 02e4f99, testbed hex-packed-x86-01
  (c7i.xlarge, bracketed full grading.js, 0.8% measured noise floor).
- **Hypothesis:** hex.Encode dominates the risk loop on SHA-NI silicon (62%
  of loop CPU in Tim's SPR profile); a 256-entry uint16 pair table (one load
  + one 2-byte store per byte) removes most of that cost. Portable Go.
- **Change:** app/main.go riskChain: hex.Encode → hexEncode64 (packed table);
  equivalence tests over all 256 byte values + full-chain digests.
- **Result:** work_score 1,944,994 / **2,098,957** / 1,947,556 (A/B/A) —
  **+7.8% vs both champion sides** (champion drift 0.13%); errors 0.565%;
  p95 price/stats 10.7ms, risk 141.6ms (better than champion's ~149ms);
  all bars pass, k6 exit 0. Decision recorded in benchmarks/history.jsonl.
- **Verdict:** KEPT — new champion is a446bd0. Gain ~3× Tim's equivalent
  (+2-3%) because our chain hex-encodes in the hot 50k loop every iteration.
  New x86 headline: **~2.10M work_score.**

## 2026-08-22 Post-hex CPU profile on x86 (kernel go/no-go evidence)

- **SHA:** a446bd0 (packed-hex champion), c7i.xlarge, 30s pprof at 200-VU
  devloop steady state (profile: benchmarks/profiles/posthex-champion-
  20260822-cpu.pb.gz). Boot receipt: isa=[sha_ni avx sse4_1 ssse3],
  cgroup_cpus=2.00, unitCost 6.34ms idle / 6.30ms contended (real cores —
  no VM noise; Docker-Desktop contended-vs-idle gap gone).
- **Split (flat CPU):** blockSHANI 65.0%; hexEncode64 9.6%; sha256
  Digest-wrapper overhead (New/Reset/Write/Sum/checkSum/memmove +
  fips RecordApproved) ≈ 15-17%; handlers/runtime remainder ~8%.
- **Reads:** hex is solved (62% → 9.6%). The lane order for the kernel is
  now: (1) fixed-64-byte direct 2-block kernel with constant padding block —
  deletes the ~15% wrapper cost, pure Go around vendored stdlib asm;
  (2) 2-lane SHA-NI interleave — attacks the 65% (literature +38% SPR,
  +60-98% Zen) → Amdahl ceiling ~+25% end-to-end; combined ceiling +30-40%.
- **Verdict:** kernel lane GO, in that order, each step behind differential
  tests + testbed A/B.

## 2026-08-22 Direct 2-block kernel (x86 full A/B — KEPT)

- **SHA:** candidate 92f6ecf vs champion a446bd0 (packed hex), set
  direct-kernel-x86-01.
- **Hypothesis:** the /risk input after iteration 1 is always exactly 64
  bytes = one message block + one CONSTANT padding block; driving the
  stdlib compression asm (vendored, unmodified) directly deletes the ~15%
  Digest-wrapper overhead the post-hex profile exposed.
- **Change:** app/sha256block_amd64.s (vendored), shakernel_amd64.go
  (kernelSum64: IV + two block calls + BE encode; ISA gate from
  /proc/cpuinfo = stdlib's own gate; 512-case boot self-test;
  RISK_KERNEL=off kill switch; stdlib fallback otherwise). Tier-0:
  6.34 → 4.81ms/chain (−24%).
- **Result (bracketed full grading.js):** 2,106,457 / **2,707,803** /
  2,102,554 — **+28.6% vs the stronger champion side** (drift 0.19%);
  errors 0.577%; risk p95 111.4ms (champion ~140ms); price p95 10.9ms;
  all bars pass. Chains/s +31% — score tracks chain throughput almost 1:1.
- **Verdict:** KEPT — champion is now the direct-kernel build.
  **x86 headline: ~2.71M work_score** (baseline 1.95M two hours earlier).

## 2026-08-22 Pairing dispatcher + 2-lane SHA-NI kernel (x86 full A/B — KEPT)

- **SHA:** candidate fa6ffbd vs champion f5674e4 (direct kernel), set
  pairing-x86-01.
- **Hypothesis:** sha256rnds2 latency-bounds a single chain; interleaving two
  independent chains per core hides it. Microbench said 1.166×; the in-chain
  boot race said 1.35× (each lane's hex overlaps the other lane's hashing).
- **Change:** app/sha256block2_amd64.s (generated 2-lane interleave of the
  vendored stdlib routine — benchmarks/gen2lane.py), kernelSum64Pair +
  riskChainPair, and the worker-model gate: riskSlots hash workers pop 1-2
  live fresh waiters off the LIFO stack (staleness-skip at take, governor
  and front-door shed unchanged; front door now keyed on idle workers).
  Boot race keeps pairing only on a ≥5% win. Race hammer, stale-take,
  50k-pair differential, full-chain pair equivalence, smoke: all green.
- **Result (bracketed full grading.js):** 2,712,083 / **3,461,856** /
  2,718,186 — **+27.4% vs the stronger champion side** (drift 0.22%);
  errors 0.584%; risk p95 90.6ms (best recorded); price/stats p95 11.0ms;
  all bars pass.
- **Verdict:** KEPT — champion is now fa6ffbd.
  **Day total: 1.95M → 3.46M (+77%) in four evidence-gated steps.**

## 2026-08-22 Final-build safety validation at 2× overload (x86)

- **SHA:** fa6ffbd (pairing champion), manual side run — 400-VU overdrive.js
  from the load box against the capped container on the target.
- **Result:** all four bars pass with wide margins at DOUBLE the grading
  peak: risk p95 146.2ms (bar 1500), price/stats p95 11.1ms, errors 0.54%
  (governor on budget), raw work_score 996,955 in 75s. Peak cgroup RSS
  across the run: 477MiB (GOMEMLIMIT=512MiB, cap 2048MiB). Boot race on this
  boot: pair ratio 1.37×.
- **Verdict:** the gate degrades gracefully at 2× with the 1.37×-faster
  kernel; memory design point holds. Overdrive exhibit (FIFO self-DQ vs
  governor) plus this run = the resilience story's bookends.

## 2026-08-22 Fused pair iteration (x86 full A/B — KEPT, 4M broken)

- **SHA:** candidate 999a87f vs champion fa6ffbd (pairing), set fused-x86-01.
- **Hypothesis:** three fusible costs remained per iteration: the constant
  padding block's message schedule (precomputable at build time), the Go hex
  encode (PSHUFB nibble LUT does it in-register), and 4 Go↔asm call
  crossings (one fused call does everything in place).
- **Change:** pairHashHex in generated asm (benchmarks/gen2lane.py): block 1
  full schedule, block 2 from a compile-time W+K table, byte-swap + hex
  expansion in-register. 143.2 → 114.1ns per pair-iteration (−20%);
  in-chain boot ratio 1.35 → 1.62×. Same wall: 50k differential vs composed
  path, 512-case boot self-test, stdlib fallback.
- **Result (bracketed full grading.js):** 3,466,470 / **4,032,045** /
  3,473,144 — **+16.1% vs the stronger champion side** (drift 0.19%);
  errors 0.589%; risk p95 84.4ms (best recorded); all bars pass.
- **Verdict:** KEPT — champion 999a87f. Day: 1.95M → 4.03M (+107%).

## 2026-08-22 Yield-stride sweep (devloop, hypothesis falsified)

- **SHA:** 09a1879 image on the testbed, RISK_YIELD_STRIDE ∈ {auto/8192,
  2048, 1024}, one devloop each from the load box.
- **Hypothesis:** cheap traffic is now ~60% of score and cheap latency
  (~7ms avg) is scheduler wait behind the hash workers — smaller yield
  slices should convert into more closed-loop iterations and score.
- **Result:** 1,184,468 / 1,168,099 / 1,182,164 — FLAT (within noise)
  while price avg improved 6.85 → 5.11ms. VU cycle time is dominated by
  /risk waits, so cheap-latency savings don't convert into volume.
- **Verdict:** keep auto stride (8192): best score, no new knob. Logged as
  considered-and-measured for the write-up.

## 2026-08-22 Single-lane fused iteration (x86 full A/B — kept, marginal)

- **SHA:** candidate 09a1879 vs champion 999a87f, set single-fused-x86-01.
- **Result:** 4,003,863 / **4,060,125** / 4,022,632 — +0.9% vs stronger
  side (drift 0.47%, noise floor 0.8%): marginal, consistent with the
  mechanism — hashHex1 only touches lone-waiter (ramp) chains; the peak
  path is byte-identical. Tier-0: 92.3 → 76.9ns per single iteration.
- **Verdict:** KEPT (zero structural downside). **Final champion 09a1879:
  4,060,125 — day total 1.95M → 4.06M (+108%).**

## 2026-08-22 Persistence deployment: bars + durability on x86 (final validation)

- **SHA:** 09a1879 image, WAL-active deployment on the testbed (volume +
  PRICE_WAL, grader-equivalent caps).
- **Result:** full grading.js against the WAL-active app: **4,089,803**,
  4/4 bars (risk p95 80.1ms, price/stats 11.2ms, errors 0.58%) — the bonus
  deployment costs nothing (WAL is off the GET path; POSTs are outside the
  graded mix). Durability: value POSTed before the siege survived a hard
  docker kill afterwards (replayed 1 entry, read back exact).
- **Verdict:** persistence bonus SHIP: docker-compose.yml + fsync'd WAL.

## 2026-08-22 Shed-budget sweep + 88bp bracket (x86 — KEPT)

- **SHA:** candidate ee2d73a (RISK_SHED_BUDGET_BP knob, Dockerfile ships
  88bp, stale-skip preemptive error charging) vs champion at 60bp.
- **Pattern that motivated it:** errors pinned at 0.58-0.59% in every one of
  18 recorded runs — the governor is permanently budget-limited, so unspent
  budget is unspent score (each front-door shed recycles a closed-loop VU
  into cheap scoring traffic).
- **Safety change:** stale-skipped waiters (future k6 timeouts we would
  never otherwise count) are charged as errors at skip time — the internal
  counter strictly overestimates k6's failure view. Measured: internal 88bp
  cap → k6 saw 0.80% (devloop) / 0.83% (grading), so real gate margin is
  0.17pp plus the skew.
- **Sweep (devloop, 60/75/88bp):** 1,178,569 / 1,186,923 / 1,203,222.
- **Result (bracketed full grading.js):** 4,110,569 / **4,168,373** /
  4,087,386 — +1.4% vs the stronger champion side (drift 0.57%); errors
  0.83%; all bars pass.
- **Verdict:** KEPT — **final champion ee2d73a @ 4,168,373; day total
  1.95M → 4.17M (+113%)**. Gains are now Amdahl-thin everywhere we've
  measured: kernel at silicon throughput, cheap latency not a lever
  (stride sweep), budget at the safe edge. Improvement day CLOSED.

## 2026-08-22 Late-day sweep: 95bp, AVX2 branch, PGO, boot warmup

- **95bp budget (devloop):** 1,209,050 vs 88bp's 1,203,222 (+0.5%, sub-noise)
  at k6-visible 0.88% — halves the unseen-failure allowance for noise-level
  gain. REJECTED; 88bp stands.
- **AVX2 branch (the non-SHA-NI grader path):** was untested (every dev box
  has SHA-NI, and GODEBUG can't flip OUR /proc/cpuinfo gate). Added forced-
  branch differential (50k cases, PASS on SPR) + bench: 292ns/iter on the
  direct AVX2 kernel vs ~339ns stdlib — a non-SHA-NI box still gets ~15%
  kernel + packed hex + all hardware-independent wins. Pairing/fusion are
  SHA-NI-only by design; worst case is self-test fallback to stdlib.
- **PGO (production profile from the final build under 200-VU load):**
  devloop A/B 1,199,172 (no PGO) vs 1,198,560 (PGO) — FLAT. The hot path is
  hand-written asm PGO cannot touch. REJECTED; default.pgo not shipped.
- **Boot warmup (8 untimed chains before calibration):** motivated by the
  ~2.7% same-code boot-to-boot score spread and ratio jitter. Four boots
  after: unitCost 4.03-4.10ms (±0.8%, was ±15%) and pairing ratio
  1.33-1.36× every boot. KEPT — grading-day constants no longer depend on
  cold-start luck. (Ratio note: the earlier 1.62× compared fused pairs to
  UNfused singles; vs fused singles the honest steady ratio is 1.35×.)

## 2026-08-22 Freeze verification (shipped build ×3, x86)

- **SHA:** 7added4 (the submission build) as both bracket sides — three
  full grading runs across three cold container boots.
- **Result:** 4,166,303 / 4,166,378 / 4,165,599 — **spread 0.019%**;
  errors 0.83-0.84% (88bp governor, k6 view); all bars pass every run.
  The boot-warmup fix is visible end-to-end: same-code boot-to-boot spread
  was ~2.7% this morning.
- **Verdict:** FROZEN. Submission headline: **4,166,000 ± 400 work_score**
  on c7i-class hardware, 2.13× the morning baseline, 4/4 bars with 18×
  /risk p95 margin and 0.16pp error-gate margin (conservatively counted).

## 2026-08-22 Slow-grader simulation (RISK_KERNEL=off + GODEBUG=cpu.sha=off)

- **Purpose:** the one untested regime — a non-SHA-NI grader where chains
  cost ~17ms (measured boot: unitCost 16.6ms, no pairing) and demand
  outstrips capacity at peak. Full grading.js on the testbed.
- **Result:** work_score 812,074; risk p95 408.7ms (bar 1500 — 3.7×
  margin); price/stats p95 10.5ms; errors 0.72%; all bars pass. Failure
  composition stayed shed-dominated (no 60s-timeout p95 poisoning).
- **Verdict:** the governor + staleness + budget architecture holds on
  2.5×-slower silicon with zero retuning — the constants derived themselves
  (calibration 16.6ms → stride 2048, patience 1.18s). Bar-safety story
  complete: fast box 4.17M, slow box 812k, both 4/4 bars.

## 2026-08-22 Co-located k6 + local verification (submission build d8d9999)

- **Co-located** (k6 on the SAME box as the container — the possible grader
  topology we had never tested): 2,879,823 @ 0.80% errors, all bars pass
  (risk p95 110.1ms, price p95 11.5ms). Score drops vs separate-loadgen
  (CPU contention, expected) but NO socket-level error storm — the
  governor's accounting holds without a separate load host.
- **Local Mac verification** (arm64: all x86 kernels inactive — proves the
  fallback + gate + derived budget alone): 1,212,105, best local run ever
  recorded (+5% over the previous local best), 4/4 bars, 0.76% errors,
  peak RSS 483MiB. Recorded in bench/history.jsonl run 13.
- **Also:** shed budget now DERIVES from the declared error gate
  (RISK_ERR_GATE_BP, default 100bp → budget 88bp, override clamped to 95%
  of gate; unit-tested) — a locked-script threshold change is a one-ENV
  update, closing the "#1 could-fuck-us" procedural risk.

## 2026-08-22 Sprint-2 Item A: kernel v3 — chunked in-asm chain loop (NEGATIVE, kill-switched off)

- **Hypothesis:** pairHashHex pays ~10-15ns/iter of removable overhead
  (Go→asm call + 8 stores/8 loads/4 flips round-tripping the digest through
  memory each iteration). Moving the loop inside the asm — epilogue leaves
  ASCII in the dead W registers, back-edge PSHUFB-flips them in place as the
  next message — should cut the 114.1ns pair-iter to ≤105ns (+3-7% score).
- **Change:** `benchmarks/gen2lane.py` now also emits `pairHashHexN`/
  `hashHex1N` (loop-in-asm, DECQ/JNZ, memory touched only at entry/exit;
  flip_mask doubles as the epilogue's bswap32). Chunked Go loop preserves
  the exact v2 yield cadence (stride = riskYieldMask+1). Full walls: boot
  self-test chains N∈{1,2,3,17,256}×128 starts against the composed v2
  routine; differential tests incl. chunk-boundary shapes + n=0; race suite.
- **Measured (dev box c7i.large, Xeon 8488C, count=3, spread <0.1%):**
  pair 114.9ns/iter v3 vs **114.1 v2 (−0.7%)**; single 76.6 vs 76.75 (flat).
  Target ≤105 decisively missed.
- **Why:** the overhead the loop deletes was never on the critical path —
  the OOO core hides call + store-forwarded reload entirely behind the
  second lane's serial SHA256RNDS2 chain. The kernel was already
  latency-bound at the silicon floor; v3's extra in-loop work (IV/state
  reload, per-use hexlut loads) costs slightly more than it saves.
- **Verdict:** REJECTED for default. No testbed bracket run (Tier-0 gate
  failed; a −0.7% kernel cannot produce the ≥+1% keep threshold). Code kept
  behind `RISK_KERNEL_V3=on` (off by default) for A/B on other silicon;
  differential walls run unconditionally in the test suite, so the disabled
  path stays verified. Estimate was +3-7%; reality 0. Measure, don't argue.
