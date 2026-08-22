# Resilience write-up

Every number below comes from the untouched published `k6/grading.js` run by a
separate load host against a container capped at 2 CPUs and 2 GB, with each
change bracketed between runs of the build it replaced. Raw summaries,
decisions, and per-experiment reports are under `benchmarks/`.

## The bottleneck moved three times

**1. Heavy work with no bound.** The starter runs every `/risk` request on its
own goroutine: 50,000 hash rounds, each allocating a new hex string (about
150,000 allocations and 9.6 MB per request). At 200 virtual users the two cores
carried dozens of runnable hash loops and every cheap request queued behind
them. Fix: two permanent workers are the only code that hashes, `/price` and
`/stats` never enter their queue, and the kernel runs in a fixed 64-byte buffer
with zero allocations. Local score doubled; `/price` p95 fell from 86 ms to
25 ms.

**2. The kernel itself.** Profiling the allocation-free loop attributed most
CPU to the 32-byte hex encoder — implausible as work, and in fact the core
stalling on the hardware SHA result the encoder consumes. Interleaving four
independent chains per worker let the core overlap that latency (+9% on Apple
Silicon, +2% on SHA-NI x86, where `sha256rnds2` leaves less to hide). Moving
the two-lane interleave into one assembly routine, with the constant padding
block's schedule precomputed and hex encoding done in-register, took the
per-chain cost on a Xeon 8488C from 6.21 ms to 2.66 ms: **+22%** on the exact
grader. The routine is selected only when CPUID reports SHA, SSSE3, and SSE4.1;
any other processor runs Go's own SHA-256, so the build cannot be slower than
its portable form.

**3. The fast path again, from the other side.** Making the kernel another 10%
faster produced no score at all. The per-tier averages showed why: `/price`,
which does microseconds of work, had a 17 ms median — time waiting for a core
behind an assembly loop the Go scheduler cannot interrupt. With 200 closed-loop
clients, the request rate, and therefore the score, was bound by cheap-path
latency, not by hashing. Calling the kernel in 256-round chunks with a yield
between them cut `/price` p95 from 43 ms to 10 ms and scored **+54%**. A
2,048-round cadence was tried and lost 10%.

## The design that ships

- Two hash workers, four lanes each, SHA-NI assembly with a Go fallback,
  256-round yields. `GOMAXPROCS=2`. `GOGC=off` with a 512 MiB memory limit on a
  near-zero steady-state heap.
- **An admission governor for `/risk`.** Under the published load the workers
  are saturated: a client parked on `/risk` earns nothing while it waits, and
  serving every arrival means every client waits. When no worker is idle and
  someone is already parked, a new arrival is refused with 503 in about a
  millisecond; the refused client comes straight back and orders cheap work. A
  refusal is charged against a budget of 88 basis points of all responses —
  reserved atomically, so a burst of waiters cannot overshoot it — against the
  published 1% gate. Parked requests are served newest-first, because under
  sustained load the oldest waiter is the one least likely to finish inside its
  bar; a waiter that has aged past the patience window (1500 ms minus the live
  batch-cost estimate minus 300 ms) is skipped and charged rather than hashed
  for nothing. Effect on the exact grader, same box, bracketed: **+17.2%**
  (4,143,222 → 4,854,704), `/risk` p95 215 → 80 ms, errors 0.00% → 0.85%.
- **Durable `POST /price`.** Each update is appended to a write-ahead log and
  fsynced before the 200 is written, then replayed at startup from the `/data`
  volume. The scored siege is read-only, so the bonus costs the scored path
  nothing: the bracket with and without it was flat (3,607,469 → 3,601,743 →
  3,611,091, zero errors). `killtest.sh` reproduces the grader's write, kill,
  restart, read procedure.

## Measured progression on the x86 reference

Exact grader, each row bracketed against the build before it; absolute scores
are comparable only within a bracket (the same build scored 3.60M on one c7i
instance and 4.14M on another).

| Step | Bracket | Δ vs stronger control | Errors |
| --- | ---: | ---: | ---: |
| Compact hex unrolling | 1,953,954 / 1,938,551 → 1,987,151 | +1.7% | 0.00% |
| Interleaved lanes (Go) | 2,018,311 / 2,003,123 → 2,062,911 | +2.2% | 0.00% |
| Two-lane SHA-NI kernel | 2,090,598 / 2,071,268 → 2,545,521 | +21.8% | 0.00% |
| In-kernel hex + 256-round yield | 2,338,684 / 2,339,747 → 3,598,675 | +53.8% | 0.00% |
| Durable `POST /price` | 3,607,469 / 3,611,091 → 3,601,743 | −0.3% (inert) | 0.00% |
| **Shedding governor (submitted)** | 4,143,222 / 4,141,819 → **4,854,704** | **+17.2%** | 0.85% |

Control drift never exceeded 0.9% in any bracket. Head-to-head on one box
against a teammate's independently built governor design (same error budget),
ours served 4,835,626 against 4,321,831 / 4,285,200: +11.9%, with equivalent
risk latency and a `/price` median of 3.4 ms against 6.8 ms.

## Under overload

`benchmarks/stress.js` ramps to 800 virtual users, four times the published
peak. The submitted build scores 1,663,973 at 0.88% errors, `/price` p95
8.2 ms, `/risk` p95 167 ms — every bar passed. With the governor off the same
build also passes (1,550,350, zero errors, `/risk` p95 1,078 ms against the
1,500 ms bar), which is the failure mode the governor exists to prevent:
without it, overload shows up as latency creeping toward the bar rather than as
errors.

## What was tried and rejected

- **Go PGO:** kernel 19.7% slower, 50,000 allocations per request reintroduced.
- **Fixed-shape scalar SHA-256 in Go:** correct, 5.9× slower than the hardware
  path.
- **Pure LIFO shedding at the published load** (two variants): errors for no
  score, because the stack never drained.
- **Serving held stale waiters late:** bounded the worst wait at 2.4 s and was
  flat at the published load, but at 4× the peak late serves exceeded 5% of
  risk samples — errors 1.49%, `/risk` p95 2.4 s. Reverted; held waiters stay
  held.
- **Yield every 2,048 rounds:** −9.8%.

## Trade-offs, stated plainly

- **We spend the error budget.** The submitted default refuses about 0.85% of
  all requests, against a 1% gate, to serve 17% more work. Under 4× overload
  the rate sits at exactly the 88bp budget (0.12pp of margin). `RISK_SHED=0` is
  the zero-error variant at about 17% less score; it is one environment
  variable away if the grader's error bar is tighter than published.
- **A held waiter can wait a long time.** A request that ages past patience
  while the budget is spent is held rather than refused; a few per run wait
  tens of seconds. It does not touch any published bar — the alternative was
  measured to break two of them — and it is documented rather than hidden.
- **Portability by fallback.** No cgo, no hard instruction-set dependency:
  without SHA extensions the binary runs Go's own SHA-256 and the same
  scheduling, yielding, and governor. The score would be lower; the bars
  would still pass.
