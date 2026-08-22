# Obsidio Go submission

A single statically linked Go 1.26.6 binary in a `scratch` image that serves the
complete scored API while keeping the cheap path independent of the heavy one.

- `GOMAXPROCS=2` is pinned to the grading container's two-CPU quota instead of
  the host's visible core count.
- `/risk` work runs only on two permanent worker goroutines. Each takes up to
  four parked jobs (`RISK_LANES`) and hashes them as interleaved independent
  chains; every chain still performs all 50,000 rounds.
- Three kernel tiers. On x86-64 with SHA extensions the chains run in one
  assembly routine (`risk_amd64.s`): Go's own SHA-NI schedule with the lanes
  interleaved, a precomputed schedule for the constant padding block, and the
  next round's hex input produced in-register; selected by CPUID at startup.
  On x86-64 with AVX-512 but no SHA extensions, a worker takes up to sixteen
  parked jobs and advances them in lockstep through a vendored sixteen-lane
  AVX-512 routine (`sha256x16_amd64.s`, minio/sha256-simd, Apache-2.0),
  enabled only after a boot self-test against `crypto/sha256` and a timed
  race it must win by 30%; `RISK_X16=off` disables it. Any other processor,
  or `RISK_SHANI=0`, uses Go's standard library unchanged.
- The scored path is served by a raw-TCP HTTP/1.1 server (`rawserver.go`)
  that parses the four fixed request shapes into a reused per-connection
  request and writes each response with one `write`; the handlers and the
  gate's accounting are the same code as under `net/http`, which
  `RISK_HTTP=std` restores. Worth +1.6% on medians, three runs against three.
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

For the persistence bonus, `docker-compose.yml` runs the same single service
with a named volume; there is no second service, so the whole budget stays with
the app. `./killtest.sh` scripts the check: two `POST /price` updates,
`docker kill`, restart, both values read back.

The image build runs `go vet` and the test suite before compiling: a kernel
that is wrong on the grading CPU fails the build instead of shipping a binary
that returns plausible but incorrect digests.

## Tests and microbenchmarks

```sh
go test ./...
go test -bench=. -benchmem ./...
```

Every `/risk` digest is checked against an independent reference
implementation rather than the optimized kernel. The assembly kernels are
compared with `crypto/sha256` on random inputs (including equal lanes),
fuzzed, checked for lane independence and batch-position independence, and
hammered under GC and preemption pressure; the sixteen-lane tier's tests run
wherever AVX-512 is present. The raw server's wire output is validated with
`http.ReadResponse` across keep-alive, split and pipelined reads, POST
framing, and malformed input. The gate's ordering, skipping, refusal, and
budget accounting have their own tests, and the tests run in both CPU modes.

Diagnostic builds only: `--build-arg GO_BUILD_TAGS=profile` compiles in a pprof
listener on `:6060`; `RISK_TIMING=1` adds a `Server-Timing` header separating
queue wait from hash time. Both are absent from the scoring image.

## Measured result

Separated x86-64 Linux pair (target `c7i.xlarge`, load generator `c7i.large`,
same availability zone, private network), container capped at `--cpus=2
--memory=2g`, untouched published `k6/grading.js`, k6 v2.2.0, 22 August 2026.
Six interleaved full runs (`A B B A A B`) of the submitted build against the
same build with the governor switched off; medians with ranges.

| Metric | Governor off | **Submitted** | Bar |
| --- | ---: | ---: | ---: |
| `work_score` | 4,187,402 (4,177,395–4,204,856) | **4,397,038** (4,380,394–4,402,197) | — |
| completed requests | 1,674,274 | 1,822,054 | — |
| error rate | 0.00% | 0.85% | <1% |
| `/price` p95 | 10.25 ms | 10.20 ms | <200 ms |
| `/stats` p95 | 10.25 ms | 10.20 ms | <500 ms |
| `/risk` p95 | 233 ms | 86 ms | <1,500 ms |

The grading CPU model is unspecified and absolute scores move by 10–15%
between AWS instances of the same type (the previous instance ran the
governor at 4,854,704 against 4,143,222 off, +17.2%; this one +5.0%), so
these are reference numbers, not a prediction; the deltas within each bracket
are the claim. On a simulated grader without SHA extensions the build scores
3,311,947 with the sixteen-lane tier and 769,042 without it, passing every bar
in both cases. Every figure is reproducible from the raw summaries,
append-only history, and decision records under `benchmarks/`;
`benchmarks/PROTOCOL.md` describes how comparisons are run.

`RISK_WORKERS` (1–2), `RISK_LANES` (1–4), `RISK_YIELD_ROUNDS`, `RISK_SHANI`,
`RISK_X16`, `RISK_HTTP`, `RISK_SHED`, and `RISK_SHED_BUDGET_BP` can all be set
on the grading host. The submitted defaults are 2, 4, 256, enabled, automatic
(off where SHA extensions exist), raw, enabled, and 88.

See [RESILIENCE.md](RESILIENCE.md) for the bottleneck analysis, the design,
the measured progression, and what was tried and rejected.
