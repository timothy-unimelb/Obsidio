# Obsidio Go submission

A single statically linked Go 1.26.6 binary in a `scratch` image that serves the
complete scored API while keeping the cheap path independent of the heavy one.

- `GOMAXPROCS=2` is pinned to the grading container's two-CPU quota instead of
  the host's visible core count.
- `/risk` work runs only on two permanent worker goroutines. Each takes up to
  four parked jobs (`RISK_LANES`) and hashes them as interleaved independent
  chains; every chain still performs all 50,000 rounds.
- On x86-64 with SHA extensions the chains run in one assembly routine
  (`risk_amd64.s`): Go's own SHA-NI schedule with the lanes interleaved, a
  precomputed schedule for the constant padding block, and the next round's
  hex input produced in-register. Selected by CPUID at startup; `RISK_SHANI=0`
  or any other processor uses Go's standard library unchanged.
- The kernel is called in chunks of `RISK_YIELD_ROUNDS` (default 256, about
  60 µs) with a scheduler yield between them, so a waiting `/price` request
  never sits behind an uninterruptible loop.
- `/risk` admission is governed: when no worker is idle and a request is
  already parked, a new arrival is refused with 503 in about a millisecond,
  inside an error budget of `RISK_SHED_BUDGET_BP` (88 basis points of all
  responses, against the published 100bp gate). Parked requests are served
  newest-first; requests that have aged past the patience window are skipped.
  `RISK_SHED=0` disables all of this: first-in-first-out, never refused.
- `/price` and `/stats` never touch the gate.
- `POST /price` appends to a write-ahead log and fsyncs it before answering;
  `GET /price` serves the latest value, and the log is replayed at startup from
  the `/data` volume, so updates survive a hard kill.
- `/stats` recomputes mean, min, max, and population standard deviation over
  all 500 points on every request, as the contract requires.

## Build and run under the grading limits

```sh
docker build -t obsidio-go .
docker run --rm --cpus=2 --memory=2g -v obsidio-data:/data -p 8080:8080 obsidio-go
curl http://127.0.0.1:8080/health
```

`./killtest.sh` scripts the persistence check: two `POST /price` updates,
`docker kill`, restart, both values read back.

## Tests and microbenchmarks

```sh
go test ./...
go test -bench=. -benchmem ./...
```

Every `/risk` digest is checked against an independent reference
implementation rather than the optimized kernel. The assembly kernel is
compared with `crypto/sha256` on random inputs, fuzzed, checked for lane
independence and batch-position independence, and hammered under GC and
preemption pressure. The gate's ordering, skipping, refusal, and budget
accounting have their own tests, and the tests run in both CPU modes.

Diagnostic builds only: `--build-arg GO_BUILD_TAGS=profile` compiles in a pprof
listener on `:6060`; `RISK_TIMING=1` adds a `Server-Timing` header separating
queue wait from hash time. Both are absent from the scoring image.

## Measured result

Separated x86-64 Linux pair (target `c7i.xlarge`, load generator `c7i.large`,
same availability zone, private network), container capped at `--cpus=2
--memory=2g`, untouched published `k6/grading.js`, k6 v2.2.0, 22 August 2026.
Bracketed champion → candidate → champion against the same build with the
governor switched off.

| Metric | Governor off | **Submitted** | Bar |
| --- | ---: | ---: | ---: |
| `work_score` | 4,143,222 / 4,141,819 | **4,854,704** | — |
| completed requests | 1,659,471 | 2,009,080 | — |
| error rate | 0.00% | 0.85% | <1% |
| `/price` p95 | 10.91 ms | 9.35 ms | <200 ms |
| `/stats` p95 | 10.93 ms | 9.34 ms | <500 ms |
| `/risk` p95 | 215.4 ms | 79.5 ms | <1,500 ms |

The grading CPU model is unspecified and absolute scores move by about 15%
between AWS instances of the same type, so these are reference numbers, not a
prediction; the deltas within each bracket are the claim. Every figure is
reproducible from the raw summaries, append-only history, and decision records
under `benchmarks/`; `benchmarks/PROTOCOL.md` describes how comparisons are run.

`RISK_WORKERS` (1–2), `RISK_LANES` (1–4), `RISK_YIELD_ROUNDS`, `RISK_SHANI`,
`RISK_SHED`, and `RISK_SHED_BUDGET_BP` can all be set on the grading host. The
submitted defaults are 2, 4, 256, enabled, enabled, and 88.

See [RESILIENCE.md](RESILIENCE.md) for the bottleneck analysis, the design,
the measured progression, and what was tried and rejected.
