# Gap-bridging plan — Obsidio `draft` branch

**Audience:** a fresh Claude Code session that will implement the worthwhile gaps.
**Author context:** produced 2026-08-22 after reading a shared explainer artifact
("Obsidio Internals", built from another team/track's `EXPERIMENTS.md` + `PLAN.md`)
and diffing its design against our current `draft` branch at HEAD `5c3d681`.

You do **not** need the artifact or the originating conversation to act on this
file. Everything needed is below. Read the whole thing before editing code.

---

## 0. Ground rules (read first)

- **The spec is authoritative.** The graded contract lives in
  [`OBSIDIO-DETAIL-PAGE.md`](OBSIDIO-DETAIL-PAGE.md) and the load script in
  [`k6/grading.js`](k6/grading.js). Do not change endpoint shapes. The four
  endpoints are `GET /health`, `GET /price?symbol=`, `GET /stats?symbol=`,
  `GET /risk?seed=` (the persistence bonus adds `POST /price`).
- **Box:** Docker, listens on `:8080`, **2 CPUs / 2 GB RAM total**. Reproducible
  pinned build. Container is CPU-throttled but sees all host cores.
- **Scoring:** qualify by clearing per-tier p95 bars (`/price`<200ms,
  `/stats`<500ms, `/risk`<1500ms) and error rate <1%; then
  `score = 1×price + 3×stats + 10×risk` for requests served correctly **and**
  within budget. Thresholds are placeholders "pending final calibration"; the
  locked numbers ship before the event and **may be harsher / higher-load than
  today's**. That uncertainty is the whole reason Gap 1 below matters.
- **Load mix:** 60% `/price`, 30% `/stats`, 10% `/risk`, closed-loop
  (each virtual user waits for its reply before firing again), ramping to a
  sustained peak. Today's script peaks at 200 VUs — treat that as a placeholder.
- **Measure, never guess.** Every change is accepted or rejected on a bracketed
  benchmark (champion → candidate → champion) per
  [`benchmarks/PROTOCOL.md`](benchmarks/PROTOCOL.md). Record results in
  `benchmarks/`. Same-host arm64 is directional only; the authoritative
  environment is the separated x86-64 AWS reference
  ([`benchmarks/aws/`](benchmarks/aws/)), which spins up billable instances —
  **the user must trigger AWS runs; do not launch them yourself.**
- **No local Go toolchain on the author's machine.** You may be on a host that
  has one; if not, gate compile/test/bench steps on the user or the AWS box.

---

## 1. Current state of `draft` (what already exists — do NOT rebuild these)

All in [`submission/go/main.go`](submission/go/main.go) unless noted. HEAD `5c3d681`.

| Mechanism | Status | Where |
| --- | --- | --- |
| `GOMAXPROCS(2)` pinned (core-count gotcha) | ✅ done | `main()` + Dockerfile env |
| 2 permanent risk workers + bounded FIFO channel (`riskQueue`, cap 32) | ✅ done | `startRiskWorkers`, `riskWorker` |
| `/price`, `/stats` bypass the risk queue entirely | ✅ done | `handlePrice`, `handleStats` |
| Allocation-free hex kernel (packed 256-entry pair table, unrolled) | ✅ done | `encodeDigest`, `buildLowercaseHexPairs` |
| SHA-NI two/four-lane assembly kernel + runtime `cpuidSHA()` gate + `RISK_SHANI=0` off-switch + pure-Go fallback | ✅ done | `risk_amd64.s`, `risk_amd64.go`, `risk_other.go` |
| **Yield** (chunk the kernel every `RISK_YIELD_ROUNDS`, default 256, so cheap handlers run) | ✅ done at HEAD | `yieldChunk`, `yieldAfterChunk` in `calculateRiskPair`/`Quad` |
| Interleaved "paired hashing" (2/4 independent chains per core) | ✅ done | `calculateRiskPair`, `calculateRiskQuad`, `riskChain2x/4x` |
| SHA-NI kernel correctness sweep (stress/fuzz/position/path-equivalence) | ✅ committed, **not yet executed** | `submission/go/kernel_*_test.go`; gate: [`benchmarks/experiments/2026-08-22-shani-correctness-sweep.md`](benchmarks/experiments/2026-08-22-shani-correctness-sweep.md) |

Relevant existing knobs (env-overridable, see the `const`/`var` block near the
top of `main.go`): `RISK_WORKERS` (1–2), `RISK_QUEUE` (1–200), `RISK_LANES`
(1–4), `RISK_YIELD_ROUNDS` (0–50000).

**Overflow handling today (the thing Gap 1 replaces):** a `/risk` handler does
`select { case riskQueue <- job: case <-r.Context().Done(): }` — it *blocks*
trying to enqueue until a slot frees or the client cancels, and workers pull
**FIFO** (`for job := range riskQueue`). There is **no** active shedding, **no**
LIFO, **no** patience/deadline discard, **no** proactive 503.

---

## 2. What the artifact confirmed (no action — just confidence)

These de-risk work already in flight; don't redo them, just know they're validated
by an independent write-up of the same problem:

- **Yield is correct and high-value.** Artifact reports `/price` p95 140ms→12ms
  from yielding ~every 1ms. Our own AWS yield screen showed 43ms→10ms — same
  direction. **But note their yield cadence is ~1ms; ours is a fixed 256-round
  chunk.** Consider whether 256 rounds ≈ 1ms on the grading CPU (see §5, tuning).
- **Hex-encoding was the surprise cost** (their profile: hex 62% vs SHA 22% of
  CPU). We found the same. The packed-table fix is already in.
- **"Paired hashing" (interleaving independent chains) is the single biggest
  throughput lever.** Already in our kernel.
- **SHA-NI must be runtime-detected with a fallback** (rules forbid hard-coding
  a CPU feature). Already how ours works.

---

## 3. THE GAPS TO BRIDGE (this is the work)

Two mechanisms the artifact's design has and our branch lacks. Both were
explicitly deferred/declined in our own `submission/go/RESILIENCE.md`, and the
artifact shows they're worth revisiting. **Do them in the order below.**

### GAP 1 — Overload admission gate (HIGH priority, pure insurance)

**Why it matters:** our current blocking-FIFO overflow path is fine at today's
placeholder 200-VU load (0 errors). But the locked grader "may carry harsher
numbers." If sustained `/risk` arrivals exceed our 2-slot capacity, a
blocking-FIFO design is exposed to a closed-loop **retry storm**: the artifact
states its predecessor FIFO "wait-then-reject-after-deadline" design
**self-disqualified at 5% errors** (5× the 1% ceiling), and the gate below
passed the *same* overload at **0.6%**. This gate can only help the tail; under
the current load it should be a no-op (nothing overflows), so it carries
negligible downside.

**Design to implement (all four pieces work together):**

1. **Concurrency stays capped at 2** (already true via 2 workers). The gate sits
   in front of the workers / queue.

2. **LIFO parking, not FIFO.** When both slots are busy, park waiting `/risk`
   jobs on a **stack (last-in-first-out)**. When a slot frees, pop the
   **newest** waiter. Rationale: under sustained overload, the oldest waiter has
   been parked longest and will blow its latency bar even if you serve it now,
   while new arrivals pile up behind it — a queue that never catches up. Newest-
   first always serves the request most likely to still finish on time.
   - Implementation: replace the single buffered channel dispatch with a mutex-
     guarded slice used as a stack (`park = append(park, job)`; pop from the
     end), plus a signal (condition variable or a small channel) to wake a free
     worker. Keep the existing 2-worker model; only the *waiting structure*
     changes from FIFO channel to LIFO stack.

3. **Patience window (deadline discard).** Calibrate at boot:
   `patience = 1500ms − (measured cost of one /risk chain) − safety_margin`.
   Measure one chain's wall-time once at startup (run `calculateRisk` on a dummy
   seed; you already benchmark this). When popping a waiter, if it has been
   parked longer than `patience`, **discard it immediately** (return an error /
   drop) instead of running it — running it now would still miss the bar and
   would only drag p95 down. Store `queuedAt` per job (the `riskJob` struct
   already has a `queuedAt time.Time` field — currently only used for timing
   diagnostics; reuse it).

4. **Front-door fast-reject on an error budget.** Track a rolling error rate
   (an **EWMA** — exponentially weighted moving average, leaning on recent
   samples — is what the artifact uses; a simple windowed counter also works).
   When both slots are busy AND the park stack is at capacity AND the error
   budget still has room (keep target **<0.6%** to stay under the 1% bar with
   margin), **reject immediately (~1ms) with 503** rather than parking. Critical
   reason: **k6 counts a request's full duration even when it fails** — a 1ms
   rejection barely moves the latency numbers, but a request parked for 2s
   before rejection poisons the p95. So shed at the *front door*, not after a
   long wait.

**Endpoint behavior:** a shed/timed-out `/risk` should return quickly with a
non-200 (503 is appropriate). It will not count toward score (correct — it
wasn't served), but it also won't wreck the latency percentiles or trigger a
retry storm. `/price` and `/stats` never touch this gate.

**Config knobs to add (env, matching the existing style):** e.g. `RISK_PARK_MAX`
(stack capacity), `RISK_ERROR_BUDGET` (target error fraction, default 0.006),
`RISK_PATIENCE_MS` (override the boot-calibrated window). Keep defaults so the
gate is inert under the published load.

**Acceptance criteria:**
- Under the published `k6/grading.js` (unchanged), score and error rate are
  **no worse** than champion (gate should be a no-op here: 0 shed, 0 errors).
- Under a **stress variant** that pushes VUs well past 2-slot capacity (write a
  throwaway k6 profile ramping to e.g. 800–2000 VUs), the gate keeps error rate
  under the bar and keeps `/price` p95 low, where the current FIFO design would
  spike errors. Show the before/after with numbers.
- All existing correctness tests still pass; `/risk` answers that ARE returned
  are still correct.

**Files:** `submission/go/main.go` (dispatch/handler). Add a focused test file
`submission/go/admission_test.go` covering: LIFO pop order, patience discard,
front-door reject when budget-limited, and `/price` unaffected while `/risk`
overloads.

---

### GAP 2 — Persistence bonus via a write-ahead log (MEDIUM priority, near-free points)

**Why reconsider:** our `RESILIENCE.md` declined the bonus because "a database
would share the fixed 2 CPU / 2 GB budget." **That objection is a strawman — you
don't need a database.** The artifact wins the bonus with a plain **append-only
write-ahead log (WAL) + `fsync`**, and the cost lands **only on `POST /price`**.
The scored siege (`k6/grading.js`) is **60/30/10 GET-only — zero POSTs** — so the
fsync cost never touches the throughput-scored path. `GET /price` stays a pure
in-memory lookup. Net: the bonus is close to free points, contingent only on the
bonus being worth points in the final rules (confirm with the user if unknown).

**How the bonus is graded:** the grader sends some `POST /price` updates,
**`docker kill`s** the container (hard kill, not graceful — so shutdown-flush
designs fail), restarts it, and checks the updated values survived. Bonus is
awarded **only if data survives AND you still clear all core thresholds.**

**Design to implement:**

1. **`POST /price`** with body `{"symbol":"AAPL","price":190.0}`: validate
   symbol, update the in-memory latest-price map, and **before returning 200**,
   append the record to a WAL file and `fsync` it. The `fsync`-before-200 is the
   whole point: on a hard kill a millisecond later, fsync'd data is guaranteed
   on disk. (A design that buffers in RAM and flushes on shutdown FAILS the hard
   kill.)
2. **`GET /price`** returns the most-recently-recorded value (in-memory map,
   unchanged fast path).
3. **On startup**, replay the WAL to reconstruct the in-memory latest-price map
   before serving.
4. **Compaction (optional):** WAL can grow; a simple periodic rewrite/snapshot
   keeps it bounded. Not required for correctness of the bonus test; skip unless
   easy.

**Deliverables per the spec:** if you add persistence, the submission needs a
`docker-compose.yml` shape (see [`compose/`](compose/)) **only if** you add a
separate service — a single-container WAL needs no compose, just a writable
volume/path inside the container. Keep it single-container if possible (simpler,
no shared-budget cost).

**Acceptance criteria:**
- Two `POST /price` writes, a real `docker kill`, restart → both values present
  via `GET /price`. (This is the exact grader procedure; script it.)
- The main GET-only siege score and latencies are **unchanged** vs champion
  (prove the WAL doesn't touch the scored path).
- Reproducible build still passes; resource caps unchanged.

**Files:** `submission/go/main.go` (add `POST /price`, WAL writer, startup
replay). New test `submission/go/persistence_test.go`. A small
`submission/go/killtest.sh` (or extend `benchmarks/`) that does the
write→kill→restart→verify loop.

---

## 4. Smaller techniques worth lifting (LOW priority, optional)

- **cgroup as source of truth for the CPU cap.** We hard-code `GOMAXPROCS(2)`.
  Reading the container's cgroup CPU quota at boot and sizing from that would
  auto-adapt if the locked grader caps at something other than 2. Low effort,
  robustness only. Keep the explicit `2` as fallback.
- **EWMA-adaptive tuning.** Beyond the error-budget EWMA in Gap 1, an EWMA of
  measured chain cost could adapt `RISK_YIELD_ROUNDS` and the patience window to
  the actual grading CPU instead of fixed constants. Only pursue if Gap 1's EWMA
  is already in place.

---

## 5. Tuning note carried over

The artifact yields **~every 1ms**; ours yields every **256 rounds** fixed.
Confirm 256 rounds ≈ 1ms on the grading-class x86 SHA-NI CPU (kernel single-chain
was ~6.3ms / 50,000 rounds ≈ 0.126ms per 1,000 rounds → 256 rounds ≈ ~0.03ms,
i.e. we may be yielding **much more often than 1ms**, which is safe for `/price`
latency but spends throughput). Consider a bracket at `RISK_YIELD_ROUNDS` ≈
8,000 (~1ms) vs 256 to see if we're over-yielding. Measure; don't assume.

---

## 6. Suggested execution order

1. **Gap 1 (admission gate)** — biggest resilience win, pure insurance, no
   downside under current load. Implement, unit-test, then bracket under both the
   published load (expect no-op) and a stress profile (expect it saves errors).
2. **Confirm bonus is worth points** with the user; if yes, **Gap 2 (WAL)**.
3. **Gap 1 tuning** (`RISK_YIELD_ROUNDS`, patience margins) once the AWS box is
   available.
4. Low-priority items only if time allows.

For every code change: bracket per `benchmarks/PROTOCOL.md`, record the summary +
a decision note under `benchmarks/`, and **do not** claim a result you didn't
measure. Keep `submission/go/RESILIENCE.md` in sync as things land (it currently
declines persistence and doesn't describe an admission gate).

---

## 7. Do-not-break checklist

- Endpoint JSON shapes exactly as `OBSIDIO-DETAIL-PAGE.md` specifies.
- `/risk` output must stay correct (verified against `referenceRisk` in tests) —
  never return a constant/fake to shed work; shed by *rejecting*, not by lying.
- `/price` and `/stats` must not regress in p95 (they must never enter the risk
  gate).
- Reproducible pinned Docker build; port `:8080`; 2 CPU / 2 GB honored.
- SHA-NI stays runtime-detected with the portable fallback intact.
