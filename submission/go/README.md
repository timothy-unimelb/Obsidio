# Obsidio Go submission

This submission implements the complete scored API while isolating the heavy
work from the fast path:

- `GOMAXPROCS` and `/risk` concurrency are both fixed at two for the grading
  container's two-CPU budget.
- `/price` and `/stats` never enter the `/risk` admission queue.
- `/risk` performs all 50,000 SHA-256 and hex-feedback iterations, using a
  fixed 64-byte buffer after the first iteration to avoid transient strings.
- `/stats` recomputes the 500-point aggregates on every request.
- Response serialization uses reusable buffers to reduce garbage collection.

Build and run under the grading limits:

```sh
docker build -t obsidio-go .
docker run --rm --cpus=2 --memory=2g -p 8080:8080 obsidio-go
```

Run tests and microbenchmarks:

```sh
go test ./...
go test -bench=. -benchmem
```

`RISK_WORKERS` accepts `1` or `2`, so the two configurations can be compared on
the actual grading hardware. The submitted default is `2` for throughput.

## Current measurements

The complete published 4 minute 30 second k6 siege was run locally with Go
1.26.6 and `GOMAXPROCS=2`. k6 ran on the same Apple M4 host, so these figures
are comparative development results rather than grading-hardware claims.

| Metric | Result | Bar |
| --- | ---: | ---: |
| `work_score` | 3,245,586 | — |
| completed requests | 1,298,614 | — |
| error rate | 0.00% | <1% |
| `/price` p95 | 41.90 ms | <200 ms |
| `/stats` p95 | 41.86 ms | <500 ms |
| `/risk` p95 | 62.24 ms | <1,500 ms |

The allocation-free risk kernel measured 3.68 ms per request on that host. The
starter-style string implementation measured 5.00 ms and allocated about 9.6
MB across 150,000 allocations per request.

See [RESILIENCE.md](RESILIENCE.md) for the design rationale and trade-offs.
