# Packed lowercase-hex feedback

## Question

Would batching independent SHA-256 chains improve the current permanent-worker
champion, or was another part of the risk loop consuming more CPU?

## Profile

A build-tagged pprof image ran for 30 seconds at 200 VUs with the normal
60/30/10 request mix and the target capped at 2 CPUs and 2 GiB. The 25-second
CPU sample attributed:

- 62.21% flat CPU to `encoding/hex.Encode`;
- 21.73% flat CPU to the runtime-selected SHA-256 block implementation; and
- 97.51% cumulative CPU to `calculateRisk`.

The allocation profile was dominated by `net/http`; the risk kernel remained
allocation-free. Mutex contention was negligible. This rejected the initial
two-lane SHA hypothesis: it would replace an optimized hardware path while
leaving the larger hex-encoding cost intact.

## Candidate

Replace 32 separate two-character hex expansions with a 256-entry native-endian
pair table and 32 aligned `uint16` stores. The bytes still contain exactly the
required lowercase hexadecimal digest after every iteration. Normal builds
exclude pprof completely; diagnostic builds opt in with the `profile` build
tag.

Independent tests compare all 256 possible input-byte values with
`encoding/hex` and continue to compare complete 50,000-round results with the
reference implementation.

## Microbenchmark

Five Linux arm64 runs in the pinned Go builder image:

| Implementation | Median complete `/risk` kernel |
| --- | ---: |
| Standard `encoding/hex` | 3.892663 ms |
| Packed pair table | 3.611301 ms |

The packed kernel is 7.79% faster in the focused benchmark, with zero
allocations in both implementations.

## Bracketed screening

Sequence: champion → candidate → champion, 90 seconds each.

| Run | Work score | Errors | `/risk` p95 |
| --- | ---: | ---: | ---: |
| Champion A1 | 964,723 | 0.00% | 322.24 ms |
| Packed candidate B1 | 987,334 | 0.00% | 321.68 ms |
| Champion A2 | 943,765 | 0.00% | 330.64 ms |

The candidate is 2.34% above the stronger champion side and 3.47% above the
champion bracket average, so it promoted to the exact workload.

## Exact full comparison

Sequence: champion → candidate → champion, 4m30s each, using the untouched
`k6/grading.js`.

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 2,793,090 | 1,116,412 | 0.00% | 22.74 ms | 22.82 ms | 340.75 ms |
| Packed candidate B1 | 2,852,984 | 1,141,016 | 0.00% | 23.56 ms | 23.62 ms | 320.86 ms |
| Champion A2 | 2,588,129 | 1,034,440 | 0.00% | 22.07 ms | 22.07 ms | 392.31 ms |

The host drifted materially: A2 scored 7.34% below A1. Even so, the candidate
beat the stronger A1 reference by 2.14%, beat the bracket average by 6.03%,
passed every correctness and latency gate, and reproduced its advantage in the
independent screen and microbenchmark.

## Verdict

**Keep as the current local champion.** The result is strong enough for ongoing
local development, but remains provisional until the next interleaved
six-run milestone and separated x86 validation.

Raw summaries are stored in `benchmarks/results/` with the
`hex-packed-*` run identifiers. The append-only run metadata and decision are
in `benchmarks/history.jsonl`.
