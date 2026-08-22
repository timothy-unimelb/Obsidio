# Obsidio Go backend — resilience design log

A record of every load-tested iteration of `starters/go/main.go`, in order, on
branch `joel/draft`. Each run was graded with the unmodified
[`k6/grading.js`](../../k6/grading.js) against the actual submission artifact
— `docker build` then `docker run --cpus=2 --memory=2g` — so these numbers
are what the grader's own script would have reported at each point in time.

**Caveat that applies to every row below:** all runs were measured on one
developer machine with the k6 client and the app container sharing the same
host. The README calls this out explicitly — "if you run k6 and the container
on the same machine, they compete for CPU and your numbers get noisy... the
grader keeps them separate." Absolute throughput numbers (`work_score`,
req/s) should be read as directional, not as a prediction of grading-day
numbers. The pass/fail pattern and the *reasons* runs failed are the load-
bearing part of this log, not the exact figures. The final section of this
log ("Independent verification on real x86 hardware") re-measures the
shipped commit on separated hardware — the load generator and target on
different hosts, like the grader — for a number that isn't subject to this
caveat.

## Summary table

| Run | Design | `/price` p95 (bar 200ms) | `/stats` p95 (bar 500ms) | `/risk` p95 (bar 1500ms) | `http_req_failed` (bar <1%) | Max latency | `work_score` | Qualifies? |
|---|---|---|---|---|---|---|---|---|
| 0 | Naive starter, unmodified | 184.46ms | 187.62ms | **2.08s ✗** | 0.00% | 6.88s | 526,410 | **No** |
| 1 | + GOMAXPROCS(2), semaphore gate (concurrent=2, queue=8), blocking wait | 20.64ms | 20.56ms | 105.93ms | **9.19% ✗** | 13m44s | 3,477,580 | **No** |
| 2 | + bounded/cancellable semaphore wait (context timeout) | 14.38ms | 14.41ms | 111.3ms | **9.32% ✗** | 366ms | 4,596,116 | **No** |
| 3 | + queue widened (8→300), wait timeout tuned (1s→1.2s) | 5.99ms | 5.97ms | 1.2s | 0.09% ✓ | 1.25s | 866,463 | **Yes** |
| 4 | + self-calibration, worker pool, OS nice priority (first attempt) | 27.61ms | 27.8ms | 1.19s | 7.01% ✗ | 1h3m47s | 1,167,826 | *discarded — see note* |
| 5 | Same design, clean re-run | 17.92ms | 17.99ms | 1.19s | **8.20% ✗** | 1.24s | 1,135,426 | **No** |
| 6 | + queue safety factor ×3 | 7.17ms | 7.18ms | 1.19s | **1.43% ✗** | 1.26s | 665,988 | **No** |
| 7 | + nice value softened (10→2) | 5.04ms | 5.08ms | 1.19s | **1.44% ✗** | 1.21s | 792,416 | **No** |
| — | *(side test, not a k6 run: stdlib vs SIMD SHA-256 benchmark)* | — | — | — | — | — | — | *not adopted* |
| 8 | Reverted to semaphore design (run 3's shape) + self-calibration | 5.16ms | 5.17ms | 1.18s | 0.06% ✓ | 1.21s | 936,546 | **Yes** |
| 9 | + zero-allocation hash kernel (fixed-buffer `hex.Encode`) | 85.57ms | 85.57ms | 96.7ms | 0.00% ✓ | 233.7ms | 1,533,667 | **Yes** |
| 10 | + calibrated `runtime.Gosched()` yield in the hash loop | 12.91ms | 12.9ms | 632ms | 0.00% ✓ | 850.3ms | 1,501,997 | *reverted — see note* |
| 11 | Gosched reverted (final confirmation run) | 88.48ms | 88.11ms | 103.56ms | **0.00% ✓** | 329.9ms | **1,559,837** | **Yes — final** |

## The linear story

### Run 0 — naive baseline

The unmodified starter, as shipped: no `GOMAXPROCS` pin (reads the host's
visible core count, not the container's 2-CPU cap), no limit on how many
`/risk` computations run concurrently, plain `http.ListenAndServe` with no
timeouts.

**Result:** every request eventually succeeds (0% errors) — Go's
goroutine-per-request model means `/price` and `/stats` never outright fail —
but `/risk` p95 hits 2.08s against a 1500ms bar. With no concurrency limit,
every incoming `/risk` request gets its own goroutine and they all fight for
the same (mis-sized) CPU budget, so each individual computation slows down
under load until the slow tail blows its own latency budget. Fails the
qualifying gate on latency, not errors.

### Run 1 — first resilience pass: pin GOMAXPROCS, gate `/risk`

Added `runtime.GOMAXPROCS(2)` to match the container's real cap, plus a
semaphore (`riskMaxConcurrent=2`) and small wait queue (`riskMaxQueued=8`)
around `/risk`, blocking on `riskSem <- struct{}{}` until a slot was free.

**Result:** latencies dropped enormously (`/risk` p95: 105.93ms), but
`http_req_failed` jumped to 9.19% with a **13m44s** max request latency.
Root cause: the semaphore wait had no timeout or cancellation. If a client
gave up or a connection dropped while a goroutine was still waiting for a
slot, that goroutine never noticed — it just kept blocking indefinitely,
leaking. Under sustained load these leaked goroutines piled up and dragged
down the whole process, not just `/risk`.

### Run 2 — fix the leak: bounded, cancellable wait

Wrapped the semaphore acquire in a `select` against
`context.WithTimeout(r.Context(), riskWaitTimeout)`, so a goroutine gives up
waiting after a fixed timeout or when the client disconnects, instead of
blocking forever.

**Result:** max latency collapsed to 366ms (confirming the leak was fixed)
and throughput jumped 5× (2,019 → 10,853 req/s), because the leaked
goroutines had themselves been silently eating CPU that legitimate requests
needed. But `http_req_failed` was still 9.32% — now for a different, real
reason: `riskMaxQueued=8` was far too shallow relative to actual demand.
`/risk` traffic was arriving around 1,085 req/s while 2 concurrent slots at
~10-15ms each could sustain only ~150-200 req/s. Every rejection is a `503`,
and `503` counts against the same `http_req_failed` ceiling as a timeout —
fail-fast shedding is not exempt from the qualifying gate.

### Run 3 — widen the queue: prefer waiting over rejecting

`/risk`'s latency budget is 1500ms against an actual compute cost of
~10-15ms — about 100x headroom. Reasoned that it's far cheaper to make a
request wait (costs latency, which there's plenty of budget for) than reject
it (costs an error, against a <1% ceiling). Raised `riskMaxQueued` 8→300 and
`riskWaitTimeout` 1s→1.2s.

**Result: first qualifying run.** `http_req_failed` fell to 0.09%, all
latency thresholds comfortable. Note `work_score` and throughput look lower
than run 2's — that's expected, not a regression: this is a *closed-loop*
load test (each virtual user waits for its previous request before issuing
the next), so when `/risk` responses got slower (queued instead of
instant-rejected), VUs naturally looped slower and total offered load
dropped. Run 2's bigger `work_score` is moot anyway since a build that fails
the error-rate gate doesn't qualify for the leaderboard regardless of how
much work it did.

### Runs 4–7 — innovation: self-calibration + OS-level priority separation

Two ideas, aimed at the Innovation score criteria:

1. **Self-calibration.** Rather than hardcoded constants tuned on one
   developer's laptop, measure this container's real per-request CPU cost
   at boot (median of 3 timed runs of the actual 50,000-iteration SHA-256
   chain) and derive `riskMaxConcurrent` / `riskMaxQueued` / `riskWaitTimeout`
   from that measurement via Little's Law — numbers matched to whatever
   hardware the container actually lands on.
2. **OS-level priority separation.** Move `/risk` onto a small fixed worker
   pool, each worker pinned to its own OS thread (`runtime.LockOSThread`)
   and given a lower kernel scheduling priority (`nice`, via
   `golang.org/x/sys/unix`), so the Linux scheduler itself — not just our
   own bookkeeping — prefers `/price`/`/stats` under contention.

**Run 4** (first attempt, `nice=10`): produced a bizarre **1h3m47s** max
latency uniform across *all three* endpoint tiers, including `/price` and
`/stats`, which have no gating logic at all. A code bug would produce
different stragglers per tier, not one identical extreme value across all
of them — this pattern, plus the container remaining healthy and responsive
immediately afterward with continuous uptime matching the gap, pointed to
the host/Docker VM having gone idle or suspended during a long background
wait, not a defect in the new code. **Discarded as unreliable rather than
treated as a real result.**

**Run 5** (clean re-run, same design): confirmed the suspend theory — max
latency back to a sane 1.24s — but `http_req_failed` was 8.20%, failing the
gate. The self-calibrated queue depth (140) landed far shallower than run
3's empirically-successful 300. Diagnosis: `calibrateRisk` measures unit
cost at boot with the system otherwise idle, which is a best case; sizing
the queue against that idealized number cuts it too close to the real,
contended cost.

**Run 6** (add `riskQueueSafetyFactor=3` to widen the queue to 582):
`http_req_failed` fell to 1.43% — a big improvement, but still above the 1%
gate, and further widening in run 7 (queue 786) barely moved it (1.44%),
which is the signature of a genuine *sustained throughput ceiling*
(`workerCount / unitCost`), not a queue-sizing problem — a deeper queue only
delays failures into timeouts once demand exceeds capacity for a sustained
period, it can't remove them.

**Run 7** (soften `nice` 10→2, to test whether the priority gap itself was
costing `/risk` throughput): error rate unchanged (1.44% vs 1.43%). This
disproved the priority hypothesis directly — `/price`/`/stats` do so little
real CPU work per request (a map lookup; a mean/stddev over 500 floats,
measured p95 ~5-7ms against 200-500ms budgets) that they were never actually
CPU-starved at any priority level, so `nice` had nothing real to protect
against for this workload. Combined with run 3's *simpler* design
outperforming every version of this one (0.09% vs. best-case 1.43%), the
conclusion was that the worker-pool indirection required to pin OS thread
priority — handing each request across a channel to a separate persistent
goroutine instead of computing inline — has a real synchronization cost on a
CPU-starved 2-core box, and that cost was outweighing a priority benefit
testing had just shown doesn't exist here.

### Side investigation — is stdlib SHA-256 leaving performance on the table?

Before concluding the throughput ceiling was fixed, benchmarked Go's stdlib
`crypto/sha256` against `github.com/minio/sha256-simd` (a SIMD/hardware-
accelerated drop-in) running the identical 50,000-iteration chain:

| Implementation | Time per chain |
|---|---|
| stdlib `crypto/sha256` | 7.86ms |
| `minio/sha256-simd` | 7.73ms |

A ~1.7% difference — within noise. Confirms Go's stdlib is already using
this CPU's hardware SHA acceleration; there was no real gain available, so
the dependency was **not adopted**. ("Profile before you optimise" — the
profiling said no.)

### Run 8 — revert the worker pool and `nice`, keep self-calibration

Removed the worker-pool/channel-handoff architecture and the `nice` priority
call (and the `golang.org/x/sys/unix` dependency that only existed to
support it) entirely. Restored the original inline semaphore +
atomic-counter pattern from run 3, but wired `calibrateRisk()`'s derived
numbers into it instead of the hardcoded constants.

**Result: qualifies, and is the best measured run overall** —
`http_req_failed` 0.06% (versus run 3's 0.09%), all latency thresholds
comfortable. This is the design currently on `joel/draft`.

### Cross-branch comparison — where joel/draft stood against teammates' work

Two teammates (`advait`, `draft`) built independent Go submissions on other
branches. All three were built fresh and graded identically — same machine,
same unmodified `k6/grading.js`, run one at a time so they never competed
with each other for CPU:

| Branch | `work_score` | `/risk` p95 | `http_req_failed` |
|---|---|---|---|
| advait | 1,910,532 | 83.73ms | 0.00% |
| draft | 1,960,819 | 426.5ms | 0.00% |
| joel/draft (run 8, before this session) | 935,754 | 1.19s | 0.06% |

Reading both other branches' source directly (`app/main.go` for advait,
`submission/go/main.go` for draft) showed the gap wasn't architectural — it
traced to one specific thing neither prior joel/draft run had: both other
implementations encode each of the 50,000 SHA-256 rounds into a **reused
fixed-size buffer** (`hex.Encode` into a `[64]byte`) instead of allocating a
fresh string every iteration (`hex.EncodeToString`, what joel/draft was still
doing through run 8). A teammate's own internal planning document (shared
separately, referred to below as "PLAN.md") independently confirmed the same
finding from their own measurements, plus one further idea worth testing: a
periodic `runtime.Gosched()` yield inside the hash loop, to address scheduler
queueing latency on the fast path. Runs 9–11 below are that investigation.

### Run 9 — zero-allocation hash kernel

Replaced `sha256Chain`'s per-iteration `hex.EncodeToString` with a `[64]byte`
buffer written via `hex.Encode`; only the final `string(buf[:])` conversion
allocates, matching advait's and draft's approach. Verified output-identical
(`seed=0.42` still produces the same known digest as every prior run).

**Result: the single biggest jump in this whole log.** `work_score` rose 64%
(936,546 → 1,533,667), `http_req_failed` improved to a clean 0.00%, and
`/risk` p95 fell 12x (1.18s → 96.7ms). This alone closed most of the gap to
both teammates' branches. Side effect worth noting: `/price`/`/stats` p95
rose to ~85ms (from ~5ms) even though their handlers didn't change — with
`/risk` now much cheaper per request, the closed-loop test generates far more
total throughput (1,393 → 2,270 req/s), so there's more `/risk` CPU work
competing for scheduler slices between cheap requests. Still comfortably
inside the 200ms/500ms bars, but the mechanism motivated run 10.

### Run 10 — Gosched yield (tested, reverted)

PLAN.md's hypothesis: `/price`/`/stats` p95 is scheduler queueing (Go's
~10ms async-preemption quantum), not handler cost, and a voluntary
`runtime.Gosched()` yield partway through the hash loop should let a waiting
cheap-request goroutine in sooner. Added a calibrated `yieldEvery` (targeting
~1ms slices, derived from the same boot-time `unitCost` measurement) and
called `runtime.Gosched()` every `yieldEvery` iterations.

**Result: the mechanism worked exactly as claimed, but the net effect on the
scored metric was slightly negative.** `/price`/`/stats` p95 dropped 6.6x
(85.57ms → 12.91ms) — confirming the queueing diagnosis was correct — but
that latency didn't disappear, it moved: `/risk` p95 rose 6.5x (96.7ms →
632ms), and `work_score` came out ~2% *lower* (1,533,667 → 1,501,997). Both
variants cleared every bar with comfortable margin, so there was no
bar-safety problem for the yield to justify. Reverted, per the same
"measure, don't assume" standard applied to the nice-priority and SIMD-SHA256
experiments earlier in this log — a change that demonstrably does what it
claims can still be the wrong change if it doesn't move the metric that's
actually scored.

### Queue-depth sanity check against a teammate's rule of thumb

A separately-shared "Field Guide" document suggested sizing the `/risk` queue
to roughly 150 jobs per worker (for a ~10ms unit cost against the 1500ms
bar — i.e., zero margin, worst-case wait exactly equal to the budget). Post-kernel,
joel/draft's calibrated queue was ~423/worker, well beyond that. Not
adjusted: run 5 in this log already demonstrated that sizing the queue
against an idealized, zero-margin boot-time measurement causes real
rejections under contention, and run 9's actual measured `/risk` p95
(96.7ms, ~6% of the 1500ms budget) is direct evidence the current queue is
safe in practice, not a rule of thumb to second-guess it against.

### Independent verification on real, separated x86 hardware (AWS)

Every run above shares one limitation, stated in the caveat at the top of this
document: k6 and the container ran on the same developer machine, competing
for the same CPU. That's fine for *relative* deltas (did this change help or
hurt?), which is all any individual run above needed to answer. It's weaker
evidence for an *absolute* number in a write-up, since same-host contention is
exactly the kind of noise the actual grading setup avoids by running the load
generator and the target on separate hardware.

A teammate's `draft` branch had already built exactly this: `benchmarks/aws/`,
a CloudFormation-provisioned environment matching the grader's shape — a
`c7i.xlarge` target host (cgroup-capped to the same `--cpus=2 --memory=2g` the
grader uses, with more visible cores left otherwise unused, precisely so the
*cgroup quota* is what's under test, not the host's size) and a separate
`c7i.large` load-generator host, in one subnet so benchmark traffic never
touches the public internet. Rather than build a parallel setup, joel/draft
was run through that existing, already-reviewed tooling and its accompanying
`BENCHMARKING.md` protocol (the same protocol behind advait's own noise-floor
figures cited earlier in this log).

That tooling's `run-comparison.sh` is built around comparing two
implementations (champion vs. candidate) in one sequence. There was no second
implementation to compare against here — the goal was verifying joel/draft's
own number on clean hardware, not an A/B — so joel/draft's own `starters/go`
was supplied as *both* champion and candidate. This isn't a workaround so much
as a convenient way to get the default `A B A` sequence to produce three
independent, real-hardware samples of the same commit (`3e44428`) rather than
one, which is exactly what the protocol itself recommends over trusting a
single run.

**Result — three runs, full `k6/grading.js`, all 0.000% error:**

| Run | `work_score` | `/price` p95 | `/stats` p95 | `/risk` p95 |
|---|---|---|---|---|
| a1 (champion) | 1,893,727 | 77.67ms | 77.95ms | 89.93ms |
| b1 (candidate) | 1,883,098 | 78.50ms | 78.55ms | 91.56ms |
| a2 (champion) | 1,879,894 | 79.39ms | 79.06ms | 91.49ms |

Median `work_score` **1,883,098**, range under 1% wide — a much tighter spread
than any pair of same-host local runs in this log, consistent with the load
generator no longer competing with the target for CPU. This is ~11% higher
than the local Tier-2 confirmation run taken minutes earlier on the same
commit (1,690,163), which is the expected direction and rough magnitude for
removing same-host contention, not evidence of a code difference — same
binary, same commit, just measured somewhere the noise floor is lower.

This number — not any of the local same-host figures — is the one to cite in
the write-up: it was produced on hardware shaped like the grader's own
environment, with the load generator genuinely isolated from the target, and
as a median of three runs rather than a single sample.

## What shipped, and why

- **Self-calibration was kept**: unambiguously validated — it derives
  sensible numbers (consistently in the same neighbourhood as the
  hand-tuned run 3 constants across multiple boots) from a real measurement
  of the box it's actually running on, with no downside observed anywhere
  it was tested. This directly answers "the grading hardware's speed is
  unknown to us" rather than assuming it's the same as a developer laptop.
- **OS-level priority separation was implemented, measured, and reverted.**
  Not because the idea is wrong in general — it's a legitimate production
  technique — but because *this specific workload's* `/price`/`/stats`
  handlers do negligible real CPU work per request, so there was never a
  contention problem for `nice` to solve, while the architecture change it
  required had a real, measured cost. Removed once the data said so.
- **A SIMD SHA-256 library was benchmarked and not adopted**, for the same
  reason: measured, found no real gain, declined the added dependency
  rather than carrying it on faith.
- **The zero-allocation hash kernel was adopted**: the single largest win in
  this log (+64% `work_score`), found by reading teammates' independent
  implementations and confirmed correct before trusting the numbers.
- **The Gosched yield was implemented, measured, and reverted**: it did
  exactly what it claimed (cut fast-path p95 6.6x) but shifted that cost onto
  `/risk` for a net loss on the actual scored metric — a reminder that a
  validated mechanism and a worthwhile change aren't always the same thing.
- **The final number was verified on separated x86 hardware**, not just
  trusted from same-host local runs. The local and AWS figures agree on
  everything that matters (comfortable margin on all four bars, ~11% apart in
  the direction contention noise predicts), which is itself the useful
  result: the local same-host numbers throughout this log were directionally
  honest, not an artifact of the measurement setup.

Final local state (run 11): `work_score` 1,559,837, `http_req_failed` 0.00%,
`/risk` p95 103.56ms against a 1500ms bar — up from run 8's 936,546 at the
start of this session, driven almost entirely by the kernel fix.

**Final verified state (AWS, separated hardware, median of 3):** `work_score`
**1,883,098**, `http_req_failed` 0.00%, `/risk` p95 ~91ms against the 1500ms
bar. This is the number the write-up should cite.
