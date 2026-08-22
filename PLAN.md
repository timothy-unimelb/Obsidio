# Obsidio endgame: ~35 hours to maximum work_score

Self-contained plan. Sources: 10-agent research fleet synthesis (raw: `scratchpad/synthesis.json` in session scratchpad, full transcripts in the wf_40fa9c71-28b workflow dir), our bench history (9 runs), EXPERIMENTS.md, BENCHMARKING.md.

## Context

Hackathon submission (Dockerfile + resilience write-up + video) due **Sunday 10:00**; it is now ~midnight Friday. Score = weighted 200s under the published closed-loop k6 siege (60/30/10, weights 1/3/10) in a 2-CPU/2GB container; the four latency/error bars are a pass/fail qualifying gate — miss one = disqualified. Current standing: our engineered Go app ~1.11M work_score locally vs rivals' 699k/999k.

**The one number that matters (fleet-verified against all our runs): work_score ≈ 25 × completed /risk chains.** In closed loop, total score is pinned to chain throughput; making /price//stats faster does NOT raise score (VUs just re-queue for /risk sooner). Every hour goes to (a) more chains/sec, or (b) not getting disqualified. The fast path's only job is bar safety.

## Locked user decisions

- **Stay clean**: no seed-precompute even if possible (moot — see dead ends: k6's PRNG is crypto-seeded per VU at runtime; verified in k6 source. Write-up gets an "identified, verified closed, declined in principle" paragraph).
- **Multi-lane kernel is first-class**, not day-3 filler — built Saturday behind runtime detection + stdlib fallback.
- **Deliverables generous (~6-7h)**, feature freeze Saturday ~22:00.
- **Persistence bonus**: user said attempt; fleet says skip (competes with kernel + deliverables for the same hours). **Resolution: go/no-go at Saturday 18:00** — attempt the cheap version (append-only file + compose volume, ~2h) ONLY if Waves 1-3 are green and ahead of schedule; otherwise the write-up frames skipping as a deliberate trade-off. 
- All subagents on Fable 5 (no model overrides).
- x86 access: cloud VM (user creates, Claude drives) + Joel's PC when awake; uni box backup.

## Findings that reorder everything (fleet, high confidence)

1. **Two silent zero-score hazards exist in the shipped code**: the Docker build never runs the digest-guard test, and `handleRisk` releases the semaphore without `defer`/recover — two panics between acquire and release would permanently kill /risk (40%+ of score). Also `go.mod` still says `go 1.22`, which gates Go 1.25+'s container-aware runtime (our manual pin is currently the only guard) and possibly codegen.
2. **FIFO-shed cliff**: our deadline-503 design enters a sustained 503 storm (closed loop: rejected VU returns in ~50ms) the moment contended chain cost exceeds ~12ms at 200 VUs — we measured 11.2ms *idle* on our own box. On any non-SHA-NI grader or higher locked VU peak this self-DQs on the 1% error gate. **Adaptive LIFO waiter stack fixes it at zero throughput cost — highest-stakes item in the plan.**
3. **/price p95 == /stats p95 within 0.5% in all nine runs** despite ~1500× different compute → fast-path latency is 100% scheduler queueing (the ~10ms preemption quantum), not handler cost. The fix is a `runtime.Gosched()` yield every ~N iterations inside the hash loop (N derived from calibrated unit cost, target ~1ms slices), not more handler golf.
4. **The kernel port is guided, not research**: Go stdlib's SHA-NI asm is avo-generated and forkable; Linux 6.18 merged the exact 2-lane interleaved kernel (finup2x) with per-silicon numbers (+98% Zen4, +38% Sapphire Rapids, +4% Ice Lake); the constant second padding block deletes half the message-schedule work in-asm.
5. **Nothing shipped since engineered-v1 is distinguishable from the ~10% local noise floor** — the entire Go 1.26 ~2× bet is unverified. Two hours on a rented x86 VM outrank any local optimisation.

## Dead ends — settled, do not re-litigate

k6 seed precompute (PRNG crypto-seeded per VU in k6 source; grading.js never calls randomSeed; keep a one-line re-check when the locked script drops) · early-seed/speculative digest work (chain strictly serial; warmup covers ~2^-36 of seed space) · GOMAXPROCS=3+ (CFS quota freeze ~33ms/period — tail bomb) · riskSem=3 or =1 (no gap to fill; only timeslices) · fasthttp/prefork/SO_REUSEPORT/router swaps (plumbing ~1-2%; latency is scheduler, not parser) · /risk worker-pool refactor (measured negative on two sibling branches) · /stats caching (forbidden) · GOAMD64=v3 (forbidden hard ISA dep) · intra-chain SIMD/midstate (mathematically serial) · minio/sha256-simd + AVX-512 16-lane (archived / needs 16 msgs + coin-flip ISA) · Netflix adaptive limiters as the gate (capacity is known =2; estimators only mis-tune it; considered-and-rejected §for write-up) · CoDel/fast-503-as-primary-valve (503 storm in closed loop) · GODEBUG micro-knobs, automaxprocs (obsolete/≤0.01%) · nice/LockOSThread priority splits (joel measured no benefit) · further cheap-handler golf as a score lever (score ≈ 25×chains, proven) · 4-lane SHA-NI (register pressure; Intel saturated at 2) · /risk LRU memo (~1e-7 hit rate + integrity smell).

## Build waves

### Wave 1 — tonight, Fri ~00:00-03:00 (~4h, all local, all high-confidence)
1. `/commit-push` current tree (Go 1.26 bump + calibration are uncommitted!), refresh HANDOFF.md.
2. **Safety bundle** (edit `app/main.go`, `app/Dockerfile`, `app/go.mod`): `RUN go test ./...` in the Docker build (digest guard becomes a build gate); `defer` + recover around semaphore release; `go.mod` → `go 1.26`. `/smoke` after.
3. **Measurement upgrades**: record chains/sec + per-tier counts + RSS in bench history; fix Tier-0 bench image → golang:1.26 (BENCHMARKING.md + any scripts).
4. **GC**: gctrace baseline devloop run → `GOGC=off` + `GOMEMLIMIT=1600MiB` (Dockerfile ENV) → devloop A/B + RSS watch across a full run. Never GOGC=off alone. (+1-4% expected)
5. **Gosched yield in riskChain** every N iterations (N from calibrated unitCost, ~1ms slices; A/B N∈{256,1024,4096} on devloop). Expect fast-path p95 collapse (bar safety), possibly +5-15% score. Must land with/before LIFO (faster fast path → more /risk arrivals → more shed pressure).
6. **Step-0 kernel go/no-go**: ~60-line C-intrinsics microbench on the Mac — NEON 1-lane vs 2-lane interleaved 50k chain ratio. Green (≥~+30%) ⇒ Wave-2 kernel proceeds. 
7. cpu.stat throttle sampling during one devloop (settles the CFS hypothesis for the ~100ms fast-path p95).

### Wave 2 — Sat ~09:00-15:00 (two parallel lanes)
- **Lane A (kernel)**: x86 VM first — verify stdlib picks SHA-NI (gate = AVX&&SHA&&SSE4.1&&SSSE3 — print fingerprint), Tier-0 1.22-vs-1.26 ratio, /smoke, one full grading run (headline numbers). Then: **2-lane interleaved SHA-NI kernel** — fork stdlib's avo generator, Linux finup2x register map, constant-block-2 W+K table, chunked calls (~256 iters/call), v1 hex-in-Go. Differential tests (k∈1..1000 + full chains + pair==2×single + fuzz) from the start.
- **Lane B (resilience)**: **adaptive-LIFO waiter stack** replacing FIFO+deadline (newest-first grant, park till served or ctx-cancel, depth cap only as memory backstop ~10× peak VUs; race-detector hammer test). Then **400-VU overdrive A/B** (doubled-stages copy of grading.js, non-history side run): demonstrate FIFO self-DQ vs LIFO pass — the judged exhibit. Then **contended calibration + EWMA** (sweep c=1/2/3 at boot for contended unit cost; live EWMA of chain times updating gate telemetry, clamped).

### Wave 3 — Sat ~15:00-21:00
1. **Pairing dispatcher** for the 2-lane kernel (2 workers each opportunistically grab a partner from the wait stack; never wait >~1-2ms, never dummy-lane; degrade to single-lane; recalibrate gate in paired mode). Ships only behind **boot-time kernel racing**: time ~10 chains stdlib vs 2-lane, verify digests, require >10% win to switch (function-pointer install) — caps downside at exactly 0%.
2. **Full bench matrix on x86**: kernel on/off × LIFO on/off × GC — recorded, /experiment-logged. 
3. **Hardening bundle**: cgroup cpu.max/memory.max read (v1 fallback) sizing the gate; x/sys/cpu + /proc/cpuinfo boot fingerprint log; cpu.stat telemetry; HTTP polish (ReadHeaderTimeout, MaxHeaderBytes 8KB, SIGTERM graceful); env-gated pprof + one flame graph under load.
4. If green & ahead: kernel v2 in-register hex via PSHUFB (+10-25%), micro-trim bundle (accepted on mechanism, one /smoke), RISK_SLOTS env knob for submission-day bar insurance.
5. **18:00: persistence-bonus go/no-go** (user call, default no if any slippage).

### Wave 4 — Sat ~21:00 → Sun 10:00 (FREEZE ~22:00)
1. Freeze: final /smoke + full grading.js local AND x86, RSS across the run, fresh-clone `docker build` (grader-identical).
2. `/writeup`: bottleneck→fix→measured-delta per claim; prior-art framing (adaptive-LIFO/Fail-at-Scale, Little's Law audit, Tail at Scale); considered-and-rejected §(adaptive limiters, GOMAXPROCS=3, worker pool, seed-exploit declined); boot fingerprint + throttle receipts.
3. Video: architecture + trade-offs, one number per claim; Tier-3 interleaved A/B for headline numbers.
4. Standing item: the moment the LOCKED grading script publishes, diff it and re-derive constants (VU peak, thresholds, randomSeed).
5. Final /commit-push + submission checklist.

## Must verify on real x86 (blocking list — updated session 4, c7i.xlarge testbed live)
~~SHA-NI actually selected~~ **VERIFIED 2026-08-22**: Xeon Platinum 8488C (SPR), all four gate flags; 6.81ms/chain vs 16.96ms with cpu.sha=off (2.49×). **Also: Go 1.22 already has SHA-NI (6.56ms) — the 1.26-bump 2× bet is falsified; 1.26 ~4% slower on the kernel, watch.** Still open: contended chain cost (grading run in flight) · 2-lane correctness on real silicon · interleave ratio on ≥1 Zen (c7a spin-up) and 1 Intel · GC/Gosched deltas carry over · true /price p95 off Docker-Desktop-VM · end-to-end grading run with final build. **NEW (Tim's committed x86 profile, benchmarks/experiments/): hex.Encode = 62% of risk-loop CPU on SHA-NI silicon, SHA block only 22% — re-evaluate the 2-lane kernel lane against a hex-first optimization (packed pair table / PSHUFB) after OUR pprof confirms the split.**

## Verification protocol (every wave)
Kernel change ⇒ differential test + Tier-0 + /smoke (full 50k digests) before any bench. Scheduling/gate change ⇒ devloop A/B (≥5% resolvable), Tier-2 recorded before claiming, /experiment always. One bench at a time; nothing CPU-heavy during a bench. Deltas <10% = noise; 10-30% = A/B protocol; >30% = one confirming re-run. Write-up cites medians+ranges only.

## SESSION-2 HANDOFF (2026-08-22 ~01:15, written for a fresh context after /clear)

**TREE STATE (updated session 3, 2026-08-22 ~10:30): SHIPPABLE — governor v2
green.** The staleness rule alone was insufficient (devloop risk p95 still 2.3s:
the governor's own shed samples, emitted after ≥1.19s of parking, dominated the
tier's percentile stream). The fix that landed is **staleness-skip at grant time
PLUS front-door budgeted shed** (instant 503 on arrival when slots busy + stack
non-empty + error budget open → ~1ms error samples + instant VU recycle;
patience-shed kept only as a backstop drain). Measured: devloop 323,808 @ 0.52%
err, risk p95 335ms (all devloop thresholds pass); grading.js 1,097,306 @ 0.551%
err, risk p95 243.6ms, price/stats p95 11.9ms, 4/4 bars, peak RSS 480MiB
(GOMEMLIMIT=512MiB verified). Score is −5.0% vs the keeper run on grading
(inside the ~10% noise floor; devloop read +4.6%) → call it score-neutral with
4× more /risk p95 margin and the FIFO self-DQ cliff structurally removed.
Full data: EXPERIMENTS.md "Governor v2a/v2b" entries.

**What Wave 2B discovered (full data in EXPERIMENTS.md trilogy entry):**
- Score law CORRECTION: "score ≈ 25×chains" holds only at zero shed. Shedding a
  stale risk waiter recycles its VU into cheap traffic that also scores —
  unthrottled shedding hit 740k devloop (+139%) but at 8.4% errors (DQ). The
  error gate, not chain throughput, caps score under overload.
- Pure LIFO parking (no deadline) idles hash slots via 60s VU hostages: −22%.
- The governor (shed while error-rate ≤0.6%, park beyond) is the right shape:
  +4.9% at 0.45% errors. Remaining defect: parked stragglers eventually get
  SERVED at 2-4s, and those samples blow the risk p95 bar.
- Parking economics: a parked VU removes ~1.3 risk-arrivals/s at an error cost
  of only 1/60 err/s (its eventual k6 timeout); shedding the same demand costs
  ~78× more errors. Park to reduce demand, shed for freshness, is the right mix.

**[DONE session 3] Staleness rule built as designed** (enqueued timestamp +
grant-time skip; unit test TestStaleWaiterSkippedAtGrant) — but it alone left
devloop risk p95 at 2.3s, which forced the real insight: k6 grades failed-
request DURATIONS too, so WHERE the error budget is spent decides the p95. The
front-door shed (see TREE STATE above) completed the fix. Fallback remains
`git checkout f1379c4 -- app/main.go` if anything regresses.

**Other gotchas found:**
- Boot calibration jitter: unitCost median-of-3 swung 11.9→15.8ms across boots
  on this Mac (Docker VM noise). Contended/EWMA calibration will stabilise.
- Grading-run RSS climbs to GOMEMLIMIT by design with GOGC=off; limit now
  512MiB (was 1600 — peak-RSS tripwire caught 1489MiB). Verify on next Tier-2.
- k6 http_req_duration INCLUDES failed requests (a 123µs 503 and a 60s timeout
  both land in the tier's percentile stream) — reason about p95 accordingly.
- The devloop reads p95 ~2× the grading script and error-rate higher (peak-only
  sampling); bars in devloop are advisory, confirm on grading.js.

**State of Wave-2 items:** Step-0 GO (1.65× on arm64, digest-verified);
x86 VM session NOT STARTED (user provides box/credentials — Joel's Windows PC
and/or cloud VM); 2-lane kernel NOT STARTED (references: Go stdlib avo
generator `sha256block_amd64.go` fork, Linux finup2x register map, constant
block-2 W+K table); overdrive exhibit DONE session 3 (see Progress);
contended-calibration/EWMA NOT STARTED.

**Recommended next-session order:** (1) staleness rule → trilogy re-run →
grading run → commit green; (2) overdrive exhibit FIFO-vs-governor;
(3) x86 session the moment a box exists (verifies the entire 2× Go 1.26 bet +
kernel ratios); (4) 2-lane kernel behind boot racing; (5) hardening bundle;
(6) freeze Sat ~22:00 → write-up + video (evidence base already strong:
12 recorded runs, trilogy narrative, Step-0 microbench, throttle receipts).

## Progress
- [x] W1: commit + HANDOFF refresh (46894e4 pushed)
- [x] W1: safety bundle (test-in-build, defer/recover, go.mod 1.26) + /smoke 35/35
- [x] W1: measurement upgrades (chains/sec + peak-RSS in record.py) + Tier-0 image fix
- [x] W1: GOGC=off + GOMEMLIMIT — devloop FLAT (−0.1%), kept for tail/STW hygiene
- [x] W1: Gosched yield — /price p95 140→12ms devloop (~12×), score-flat, kept; auto-stride from calibration
- [x] W1: Step-0 microbench → **GO: 1.65× interleave ratio measured on arm64** (digest verified vs hashlib)
- [x] W1: cpu.stat — 37ms total throttled/run: CFS hypothesis dead, GOMAXPROCS=2 validated
- [x] W2A: x86 verification session — SHA-NI confirmed (2.49×; 1.26-bump bet falsified, 1.22 already had it); baseline 3×grading on c7i.xlarge: **1.95M, 4/4 bars, 0.8% noise floor, true /price p95 10.6ms**; testbed = Tim's harness, stack obsidio-bench-advait
- [x] W2A (reprioritized): packed pair-table hex encoder — **+7.8% full A/B on testbed, new champion a446bd0, x86 headline ~2.10M** (hex was 62% of loop CPU on SHA-NI silicon; Tim's profile finding, our port)
- [x] W2A: post-hex pprof (blockSHANI 65% / wrapper ~15% / hex 9.6%) → kernel lane executed in profile order:
- [x] W2A: direct 2-block kernel (vendored stdlib asm, fixed-64B path) — **+28.6% full A/B, champion 92f6ecf, headline ~2.71M** (Tier-0 −24%/chain)
- [x] W2A: 2-lane interleaved SHA-NI kernel (generated from stdlib asm by benchmarks/gen2lane.py) — differential-tested on SPR; in-chain ratio **1.35×** (pair 6.84ms vs serial-2 9.24ms)
- [x] W3: pairing dispatcher — **KEPT: +27.4% full A/B, champion fa6ffbd, headline 3,461,856, risk p95 90.6ms (best yet)**. Day: 1.95M → 3.46M (+77%)
- [x] W2B: adaptive-LIFO gate — governor v2 (staleness-skip at grant + front-door budgeted shed): grading 1,097,306 @ 4/4 bars, risk p95 243.6ms, errors 0.55% — SHIPPABLE
- [x] W2B: 400-VU overdrive exhibit (k6/overdrive.js): FIFO+deadline DQs at 5.03% errors; governor v2 passes all bars at 0.59% — judged exhibit banked
- [x] W2B: contended calibration + EWMA — idle 12.8 vs contended 15.8ms (+23%) measured; grading 1,154,460 (= keeper level) @ 4/4 bars; boot-jitter gotcha closed
- [ ] W3: pairing dispatcher + boot kernel racing
- [x] W3: x86 bench matrix — superseded by four bracketed full comparisons (baseline/hex/kernel/pairing sets in benchmarks/history.jsonl) + GODEBUG cpu.sha kill-switch measurements: every layer measured on/off on the same testbed, stronger evidence than the planned grid
- [x] W3: hardening bundle — cgroup-sized riskSlots (+RISK_SLOTS knob), boot fingerprint, cpu.stat receipts, HTTP polish, env-gated pprof; flame-graph-equivalent = committed posthex pprof profile
- [~] W3: kernel v2 fused iterations — BUILT + verified (999a87f pair: 143→114ns/pair-iter, in-chain ratio 1.62×; 09a1879 single: 92→77ns/iter for ramp chains); **pair-fused full A/B in flight (fused-x86-01)**, then single+stride bracket; yield-stride sweep queued (cheap path now ~60% of score, med 7ms = scheduler wait knob)
- [x] W3: persistence go/no-go — **user GO (session 5)**: fsync'd WAL + compose volume built; hard-kill durability verified locally (2 POSTs → docker kill → replayed); graded path bit-identical (smoke 35/35, WAL inactive without env). Bars-under-compose check on testbed still owed
- [ ] W4: freeze + final verification (local + x86 + fresh-clone build + compose-bars run)
- [ ] W4: write-up drafted from EXPERIMENTS.md
- [ ] W4: video script + A/B headline numbers
- [ ] W4: locked-script diff (when published) + final push + checklist
