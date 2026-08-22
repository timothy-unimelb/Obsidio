# Obsidio Go submission

A single statically linked Go 1.26.6 binary in a `scratch` image that serves the
complete scored API while keeping the cheap path independent of the heavy one:

- `GOMAXPROCS=2` is pinned to the grading container's two-CPU quota instead of
  the host's visible core count.
- `/risk` work runs only on two permanent worker goroutines fed by a bounded
  FIFO queue (`RISK_QUEUE`, default 32). Handlers enqueue a job and wait on
  their own result channel; a cancelled request is dropped without being hashed.
- `/price` and `/stats` never touch that queue, so they stay schedulable no
  matter how deep the risk backlog is.
- The risk kernel performs all 50,000 SHA-256 → lowercase-hex rounds in a fixed
  64-byte buffer with zero heap allocations, using a packed 256-entry hex-pair
  table written four bytes at a time.
- `/stats` recomputes mean, min, max, and population standard deviation over all
  500 points on every request, as the contract requires.
- Static `/price` bodies are serialized once at startup; dynamic responses use a
  small `sync.Pool` of buffers.

## Build and run under the grading limits

```sh
docker build -t obsidio-go .
docker run --rm --cpus=2 --memory=2g -p 8080:8080 obsidio-go
curl http://127.0.0.1:8080/health
```

## Tests and microbenchmarks

```sh
go test ./...
go test -bench=. -benchmem ./...
```

The tests recompute every `/risk` digest with an independent straightforward
reference implementation rather than trusting the optimized kernel, check all
256 byte values of the hex table against `encoding/hex`, and verify the health,
price, stats, escaping, concurrency, and error-response contracts.

Diagnostic builds only: `--build-arg GO_BUILD_TAGS=profile` compiles in a pprof
listener on `:6060`, and `RISK_TIMING=1` adds a `Server-Timing` header that
separates queue wait from hash time. Both are absent from the normal scoring
image.

## Measured result

Measured on a separated x86-64 Linux pair (target `c7i.xlarge`, load generator
`c7i.large`, same availability zone, private network) with the container capped
at `--cpus=2 --memory=2g` and the untouched published `k6/grading.js`
(k6 v2.2.0). This is the current accepted champion, recorded on 2026-08-22.

| Metric | Result | Bar |
| --- | ---: | ---: |
| `work_score` | 1,987,151 | — |
| completed requests | 796,456 / 796,456 | — |
| error rate | 0.00% | <1% |
| `/price` p95 | 25.07 ms | <200 ms |
| `/stats` p95 | 24.90 ms | <500 ms |
| `/risk` p95 | 442.78 ms | <1,500 ms |

The grading CPU model is unspecified, so this is reference evidence, not a
prediction of the judge's absolute score. Every number above is reproducible
from the raw summaries, append-only history, and decision records under
`benchmarks/`; see `benchmarks/PROTOCOL.md` for how comparisons are run.

`RISK_WORKERS` accepts `1` or `2` so both configurations can be compared on the
real grading hardware. The submitted default is `2`.

See [RESILIENCE.md](RESILIENCE.md) for the bottleneck analysis, design
rationale, measured progression, and the trade-offs and rejected experiments.
