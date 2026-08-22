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

## Must verify on real x86 (blocking list)
SHA-NI actually selected (full feature gate) · contended chain cost (decides the 12ms-cliff side + all gate constants) · 2-lane correctness on real silicon (QEMU-TCG passes are necessary, NOT sufficient — and Rosetta is unusable: no AVX ⇒ stdlib silently falls back) · interleave ratio on ≥1 Zen and 1 Intel · GC/Gosched deltas carry over · true /price p95 off Docker-Desktop-VM (ours ~100ms is suspected harness artifact; Joel's 7.3ms proves the floor) · one end-to-end grading run with the final build.

## Verification protocol (every wave)
Kernel change ⇒ differential test + Tier-0 + /smoke (full 50k digests) before any bench. Scheduling/gate change ⇒ devloop A/B (≥5% resolvable), Tier-2 recorded before claiming, /experiment always. One bench at a time; nothing CPU-heavy during a bench. Deltas <10% = noise; 10-30% = A/B protocol; >30% = one confirming re-run. Write-up cites medians+ranges only.

## SESSION-2 HANDOFF (2026-08-22 ~01:15, written for a fresh context after /clear)

**⚠️ TREE STATE: the working tree (committed as WIP on `advait`) is NOT SHIPPABLE.**
The last fully-green submittable build is commit **f1379c4** (Wave 1 keeper:
1,154,625 recorded, 4/4 bars, /price p95 12ms). The tree since then contains the
LIFO+governor gate: smoke-passing, race-clean, +4.9% devloop score at 0.45%
errors, **but devloop risk p95 = 3.4s (bar breach)**. Fix forward or revert the
gate before any submission.

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

**The designed-but-unbuilt fix (next session, ~1-2h): staleness rule at grant
time.** In releaseRiskSlot, skip (leave parked) any waiter whose age already
exceeds ~(1500ms − unitCost − margin) — serving it would emit a bar-breaking
duration sample; its eventual 60s timeout is cheaper (1/60 err/s) than the p95
damage. Add `enqueued time.Time` to riskWaiter. Then re-run the devloop trilogy
comparison + a full grading run; expect ≥324k devloop, risk p95 back under bar,
errors <0.6%. If it fails: `git checkout f1379c4 -- app/main.go` restores Wave 1.

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
block-2 W+K table); overdrive exhibit NOT RUN (build k6/overdrive.js = devloop
at 400 VUs when gate is fixed); contended-calibration/EWMA NOT STARTED.

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
- [ ] W2A: x86 VM verification session (SHA-NI, ratios, grading run)
- [ ] W2A: 2-lane SHA-NI kernel v1 + differential tests
- [~] W2B: adaptive-LIFO gate — trilogy measured, governor built (race-clean, +4.9%), **staleness rule at grant time still needed (p95 breach)** — see SESSION-2 HANDOFF
- [ ] W2B: 400-VU overdrive FIFO-vs-governor exhibit (after staleness rule)
- [ ] W2B: contended calibration + EWMA (also fixes boot-jitter gotcha)
- [ ] W3: pairing dispatcher + boot kernel racing
- [ ] W3: x86 bench matrix, recorded
- [ ] W3: hardening bundle (cgroup, fingerprint, HTTP polish, pprof graph)
- [ ] W3: (if ahead) kernel v2 hex / micro-trims / RISK_SLOTS
- [ ] W3: 18:00 persistence go/no-go (user)
- [ ] W4: freeze + final verification (local + x86 + fresh-clone build)
- [ ] W4: write-up drafted from EXPERIMENTS.md
- [ ] W4: video script + A/B headline numbers
- [ ] W4: locked-script diff (when published) + final push + checklist
