# Resilience Write-up

Team taj_mahal, Obsidio track.

A visual walkthrough of this report, with diagrams, is here: **[claude.ai/code/artifact/3a662b77-6248-4805-82d6-9fb29b9e2d8c](https://claude.ai/code/artifact/3a662b77-6248-4805-82d6-9fb29b9e2d8c)**

Obsidio grades one thing: how much useful work a submission serves, correctly and inside its latency bars, from a fixed 2 CPU / 2 GB box, under the published load. Every change below started as a measurement against that scoring rule, not a guess, and every kept change has a before/after number behind it in
[EXPERIMENTS.md](EXPERIMENTS.md) and
[benchmarks/history.jsonl](benchmarks/history.jsonl).

## 1. Keep the cheap path cheap

`/price` and `/stats` are simple lookups. `/risk` is 50,000 rounds of
SHA-256. If the cheap endpoints are slow under load, it isn't because they
do real work, it's because they're stuck in line behind the endpoint that
does.

That's exactly what we saw. For nine runs in a row, `/price` p95 tracked
`/stats` p95 almost exactly, even though `/stats` does about 1,500x more
work per request. Both were waiting on the same CPU, held by a hashing
goroutine that wouldn't give it up: Go's scheduler preempts roughly every
10 ms, long enough for a `/risk` worker to block a `/price` request behind
it the whole time.

Two fixes:

- A fixed pool of `/risk` workers, sized from the container's real CPU
  budget (the cgroup's `cpu.max`), not the number of cores the host
  happens to expose. This one change, bounding concurrency to what the box
  can actually run and stripping allocations from the hot path, took the
  naive starter from 745,484 to 1,273,640 work_score (+71%).
- A voluntary yield inside the hash loop, calibrated at boot to the
  container's own measured chain cost, so a `/risk` worker gives up the
  CPU before it can starve the fast path.

Result: `/price` p95 139.9 ms to 12.0 ms, about 12x, with work_score
unchanged (a faster cheap response doesn't add score, it only adds margin
under the latency bar). A later tuning pass on x86 hardware tightened the
yield further and added +2.0% work_score on a bracketed run.
(EXPERIMENTS.md: "Engineered Go v1 vs naive baseline," "Gosched yield in
the hash loop," "Sprint-3 Item 1.")

## 2. Fail fast beats hanging

This was the largest single finding on the project.

An early version of our admission gate used a FIFO queue with a wait
deadline: reasonable on paper, but it collapsed under sustained overload.
We proved it by pushing the same build to 2x the grading peak (400 VUs)
with two different gates, back to back, same machine:

- FIFO + deadline: 5.03% error rate. Disqualified (5x the 1% gate).
- LIFO, budget-governed shedding at arrival: 0.59% errors, all four
  latency bars held.

The difference is order and cost. FIFO serves the oldest waiter first,
the one most likely already abandoned by a client stuck in a ~50
millisecond retry loop. We switched to a LIFO stack instead: adaptive
LIFO, the pattern Ben Maurer describes in "Fail at Scale" (Facebook
Engineering, also in Communications of the ACM, 2015). Serve the
freshest waiter, and skip stale ones cheaply at grant time instead of
letting them occupy a slot until their deadline fires. Cost matters too:
k6's virtual users retry immediately after any failure, and the FIFO
queue took up to 1.2 seconds to report one, so retries kept arriving
faster than the queue could drain. The LIFO gate rejects in about 1
millisecond, so the same retry lands on a system that has already
recovered.

Getting there took three LIFO variants, logged in EXPERIMENTS.md's
"400-VU overdrive exhibit." Plain LIFO with no deadline starved old
waiters until the pool of active clients shrank (score fell 22%). LIFO
with a fixed deadline looked like a huge win, +139% work_score, before a
closer look showed 8.4% errors: shedding a stale request still recycles
that client into fast, cheap traffic, so score and errors climbed
together. Shedding at arrival instead of after a wait, the version
shipped above, closed that gap.

One correctness fix belongs here too: the budget check was
check-then-charge, so a burst of simultaneous requests could overshoot it
before any single one registered as spent. Fixed with an atomic
reservation charged in the same step as the decision, proven with a
race-hammered test. Measured score-neutral. (EXPERIMENTS.md, "Sprint-3
Item 2.")

## 3. The hash loop was the real ceiling

`/risk` is worth 10x a `/price` request, so once the scheduling and
shedding problems were fixed, its per-chain cost became the single biggest
lever on score. We profiled it under load on x86 grading-class hardware
and found CPU going to the wrong places, three times over:

- **Hex encoding, 62% of loop CPU.** Go's `hex.Encode` handles
  arbitrary-length input a byte at a time: two table lookups per byte, one
  per nibble. Our input is always exactly 32 bytes, so we built a
  256-entry table that maps each input byte straight to its 2-character
  hex pair in one lookup instead of two: +7.8% work_score (1.95M to
  2.10M).
- **A digest wrapper we didn't need, ~16%.** Go's `sha256.Digest` is
  built to stream arbitrary-length input: it buffers writes, tracks how
  much padding to add, and copies state in and out on every call. Our
  input after the first round is always exactly two 64-byte blocks, one
  of them a constant padding block that never changes, so none of that
  bookkeeping does anything useful. We call the same compression
  instructions the standard library uses, just without the wrapper around
  them: +28.6% (2.10M to 2.71M).
- **The hashing instruction's own latency.** `SHA256RNDS2`, the CPU
  instruction that does the actual round of hashing, takes a fixed number
  of cycles to complete, and a single chain running alone has nothing
  else to do while it finishes. Interleaving a second, independent chain
  on the same core fills those idle cycles with useful work: +27.4%
  (2.71M to 3.46M). A four-lane version fills more of them for another
  +9.4% (4.40M to 4.81M).

Every step was a bracketed A-B-A run (champion, candidate, champion again,
same machine, same load), with testbed noise measured under 1%.

## 4. Don't hardcode what you can measure

We don't know the grading hardware's exact speed, so none of the timing
constants above are hardcoded. At startup the container reads its own CPU
budget from the cgroup (`cpu.max`) to size the worker pool, then times
real 50,000-round chains on itself, under contention, to derive the shed
gate's patience window and the hash loop's yield cadence. A live moving
average keeps refining that estimate as real requests arrive, so the
constants track what the box is actually delivering, not a one-time
guess at boot.

Early boots showed why that mattered: identical builds measured 11.9 to
15.8 ms per chain across four boots on the same machine, about ±15%
noise from VM cold-start variance, enough to throw off a boot-derived
deadline. Running 8 untimed chains before the real measurement fixed it:
boot-to-boot spread dropped to under 1%.

The test that actually matters: we forced the same build to run without
its fast SHA-256 path, simulating a slower, no-SHA-NI grading box. It
re-derived its own constants for the ~2.5x slower chain cost with no code
changes and no retuning, and all four latency bars still passed. Whatever
the grading hardware turns out to be, the gate finds out for itself in
the first second of boot.
(EXPERIMENTS.md: "Contended calibration... EWMA," "Boot warmup," "Slow-grader
simulation.")

## Also shipped

- **Generic HTTP overhead removed.** `net/http`'s header-map allocations
  and routing machinery cost CPU that a fixed four-endpoint contract
  doesn't need. A hand-rolled HTTP/1.1 parser: +2.5% work_score, +10.4%
  when the load generator competes for the same cores as the container.
- **Hardware we haven't seen.** Every SHA-256 speedup in idea 3 only runs
  on SHA-NI silicon, gated by the same feature check Go's standard library
  uses. Forcing the software fallback drops work_score to 812,074 (about
  5x worse), though every bar still passes. A vendored AVX-512 kernel
  recovers 4.0x of that gap on hardware with AVX-512 but no SHA-NI
  (812,074 to 3,255,335, re-checked on the current build), and stays
  measurably silent on hardware that doesn't need it.
- **Persistence bonus.** A fsync'd write-ahead log records `POST /price`
  writes outside the graded GET path. A value written before a hard
  `docker kill` survived and read back correctly; the run with persistence
  active still cleared all four bars (4,089,803 work_score).

We logged what didn't work with the same rigor: an in-asm loop fusion 0.7%
slower, a 16-lane batch flat because the queue rarely gets big enough to
use it, PGO flat because the hot path is hand-written assembly, and a
wider error budget that wasn't worth the noise it let through. Full trail:
EXPERIMENTS.md.

## Results

Freeze-verified: 3 independent cold container boots on x86 grading-class
hardware (c7i.xlarge, 2 CPU / 2 GB cgroup-capped container, full
`k6/grading.js`), build `ee09171`. Numbers below are the mean; spread
across the 3 boots was 0.92%.

| Metric | Bar | Measured | Margin |
| --- | --- | --- | --- |
| `/price` p95 | < 200 ms | 9.73 ms | 21x |
| `/stats` p95 | < 500 ms | 9.72 ms | 51x |
| `/risk` p95 | < 1500 ms | 81.7 ms | 18x |
| Error rate | < 1% | 0.85-0.87% | gate stays just under its budget |
| **work_score** | n/a | **4,789,220** | 6.4x the naive starter |

The bracket that identified this champion peaked at 4,814,186
(EXPERIMENTS.md, "Sprint-3 Item 3"); the number above is the more
conservative 3-boot freeze mean, which is what we're standing behind.

The naive, unmodified starter scored 745,484 in the same capped
container (measured locally; the freeze above is from the separate x86
testbed, so treat the 6.4x as directional, not exact). On x86 hardware
specifically, where both ends of the comparison are on the same machine,
profiling and the SHA-256 kernel work were worth 2.45x on their own
(1,953,437 to 4,789,220).

All of this ran in VMs, not bare metal: local development on an arm64 VM
(Apple Silicon host), verification on a separate x86 cloud VM pair. Full
protocol in `benchmarks/PROTOCOL.md`.
