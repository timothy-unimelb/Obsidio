# Resilience write-up

## Bottleneck

The expensive endpoint was both CPU-heavy and allocation-heavy in the starter.
Each of its 50,000 rounds converted a string to bytes, produced a digest, and
allocated another hex string. Our local microbenchmark measured approximately
5.00 ms, 9.6 MB, and 150,000 allocations for one request. Allowing an arbitrary
number of those requests to run at once would also give the Go scheduler a
large CPU queue and increase cheap-request tail latency.

## Design

The server sets `GOMAXPROCS=2` explicitly to match the grading quota. `/risk`
uses a two-slot semaphore, so at most two hashing loops can consume CPU at a
time. Excess risk handlers wait on that semaphore, while `/price` and `/stats`
bypass it entirely and remain schedulable.

The risk implementation still performs all 50,000 specified operations. After
hashing the seed once, it alternates SHA-256 with lowercase hex encoding in a
fixed 64-byte buffer. This reduced the local kernel result to approximately
3.68 ms with no loop allocations. We deliberately chose two risk slots rather
than reserving one entire core: the published load left substantial fast-path
latency headroom, while the second slot nearly doubles the score-limiting risk
throughput.

`/stats` performs both aggregation passes over all 500 points on every request.
Static price responses are serialized once at startup, and dynamic responses
use a small buffer pool to reduce garbage-collector pressure. The server uses
the standard Go HTTP stack with bounded headers and idle/read-header timeouts.

## Measured result

We ran the complete published k6 script locally for 4 minutes 30 seconds using
Go 1.26.6 and `GOMAXPROCS=2`. k6 ran on the same Apple M4 host, not on the final
Linux grading hardware.

| Metric | Result |
| --- | ---: |
| `work_score` | 3,245,586 |
| successful requests | 1,298,614 / 1,298,614 |
| HTTP error rate | 0.00% |
| `/price` p95 | 41.90 ms |
| `/stats` p95 | 41.86 ms |
| `/risk` p95 | 62.24 ms |

All published thresholds passed. Unit tests independently recompute risk hashes
using the straightforward starter algorithm and compare the endpoint results;
they also verify the health, price, stats, and error response contracts.

## Trade-offs

We did not attempt the persistence bonus. Its score is unspecified, while an
additional database would consume part of the fixed CPU and memory budget and
could reduce the core score. We also avoid overload rejection in the published
200-VU test: the VU cap itself bounds the number of waiting handlers, preserving
the required zero-error behavior without allowing unlimited active CPU work.
