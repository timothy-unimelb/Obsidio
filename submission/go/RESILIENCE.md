# Resilience write-up

## Where the bottleneck was

The Go starter fails the siege for two compounding reasons.

**Unbounded heavy work.** Every `/risk` request got its own goroutine running
50,000 hash rounds, so at 200 virtual users the scheduler had dozens of runnable
CPU-bound goroutines competing with the cheap handlers for two cores. The fast
path was slow not because it did work, but because it queued behind work.

**An allocation-heavy kernel.** Each round converted a string to bytes, hashed,
and allocated a new hex string: about 150,000 allocations and 9.6 MB per request,
measured at roughly 5.0 ms per request on our development host. That garbage
both cost CPU directly and pulled the GC into the critical path.

A build-tagged pprof image under the published 60/30/10 mix at 200 VUs confirmed
the shape after the first fixes: the risk kernel consumed 97% of CPU, and within
it hex encoding — not SHA-256 — was the largest single cost.

## Design

**Bounded concurrency, cheap path outside it.** `GOMAXPROCS` is pinned to 2.
Two permanent worker goroutines are the only code that runs the hash loop; they
pull from a bounded FIFO channel. A `/risk` handler enqueues a job and blocks on
its own result channel, so at most two hash loops are ever runnable and the
number of goroutines waiting is bounded by the virtual-user count. `/price` and
`/stats` never enter the queue. The separate `Server-Timing` diagnostic shows
the split under peak load: median hash time 4.0 ms, median queue wait 321 ms —
the backlog lives in the queue, not on the cores, which is exactly why `/price`
p95 stays near 25 ms while `/risk` p95 sits around 450 ms.

**An allocation-free kernel that still does all the work.** After hashing the
seed once, the loop alternates `sha256.Sum256` with in-place lowercase-hex
encoding into a fixed 64-byte buffer. Nothing is skipped, cached, or
precomputed across requests; the tests verify every output against an
independent reference implementation. Hex encoding uses a 256-entry
native-endian pair table with aligned 16-bit stores, unrolled four bytes at a
time — small enough for the compiler to inline into the 50,000-round loop.

**Low-noise HTTP path.** Static `/price` bodies are pre-serialized; `/stats`
and `/risk` bodies are built with `strconv.Append*` into pooled buffers. Header
size and read-header/idle timeouts are bounded. There is no reflection-based
JSON anywhere on the request path.

**Why two workers, not one reserved core.** The published load leaves roughly
8× headroom on the `/price` bar, while `/risk` carries 10× the weight. The second
worker nearly doubles score-limiting risk throughput while `/price` p95 stays
near 25 ms in every recorded run. `RISK_WORKERS=1` remains available if
the locked grader narrows the fast-path budget.

## Measured progression

Every step was measured with the untouched published `k6/grading.js` against a
container capped at 2 CPUs and 2 GB, following the bracketed and repeated
procedure in `benchmarks/PROTOCOL.md`. Raw summaries and verdicts are under
`benchmarks/`.

Local development host (Linux arm64 VM, k6 on the same Apple Silicon machine;
medians of three full runs, directional only):

| Checkpoint | `work_score` | Errors |
| --- | ---: | ---: |
| Naive Go starter | 1,425,795 | 0.00% |
| Bounded risk concurrency + allocation-free kernel | 2,790,902 | 0.00% |
| Permanent workers + bounded queue | 2,874,253 | 0.00% |
| + packed hex-pair table (bracketed, +2.1% vs stronger control) | 2,852,984\* | 0.00% |

\* Different session from the rows above; compared only against its own
champion bracket (2,793,090 / 2,588,129).

Separated x86-64 reference (AWS `c7i.xlarge` target, `c7i.large` load
generator, private network, container-only cap), champion → candidate →
champion:

| Run | `work_score` | Requests | Errors | `/price` p95 | `/stats` p95 | `/risk` p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Packed hex (control A1) | 1,953,954 | 779,720 | 0.00% | 23.73 ms | 23.70 ms | 478.81 ms |
| **+ compact hex unrolling (submitted)** | **1,987,151** | 796,456 | 0.00% | 25.07 ms | 24.90 ms | 442.78 ms |
| Packed hex (control A2) | 1,938,551 | 774,455 | 0.00% | 21.95 ms | 21.97 ms | 462.88 ms |

The submitted build beat the stronger control by 1.70% with only −0.79% control
drift, passing every published gate with zero errors. Absolute scores differ
between hosts by design; only within-environment deltas are claimed.

## What we tried and rejected

Deliberately measured dead ends, kept in `benchmarks/decisions/`:

- **Go profile-guided optimization** (representative 95 s profile): the risk
  kernel became 19.7% slower and regained 50,000 allocations per request.
  Reverted at Level 0.
- **Fixed-shape scalar SHA-256** (precomputed padding and schedule for the
  always-64-byte input): correct across 10,000 random vectors but 5.9× slower
  than Go's runtime-selected hardware SHA path, and still 23% slower with
  optional CPU features disabled. Reverted at Level 0.
- **Fully unrolled 32-store hex encoder:** faster in isolation but slower in the
  complete kernel because it stopped inlining. The compact four-at-a-time form
  was kept instead.

## Trade-offs

- **No persistence bonus.** Its value is unspecified, and a database would share
  the fixed 2 CPU / 2 GB budget with the kernel that earns the score.
- **No overload rejection under the published load.** The 200-VU cap bounds the
  waiting handlers and the queue keeps them off the cores, so we serve every
  request correctly with zero errors rather than shedding. Cancelled requests
  are discarded before hashing. If the locked grader raises the peak, the queue
  depth and a fail-fast 503 path are the first knobs.
- **Portability over peak.** No CPU-specific instructions, cgo, or assembly are
  assumed; Go's runtime picks the SHA implementation at startup, so the build
  behaves the same on whichever x86-64 CPU grades it.
