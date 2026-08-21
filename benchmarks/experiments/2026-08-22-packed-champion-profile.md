# Packed-hex champion profile

## Purpose

Re-profile the accepted packed-lowercase-hex champion before selecting another
optimization. This prevents the pre-optimization profile from directing work
after the bottleneck has moved.

## Method

- Source: commit `64e38e0`.
- Target: Linux arm64 Docker on local Colima, capped at 2 CPUs and 2 GiB.
- Diagnostic build: `GO_BUILD_TAGS=profile`; normal scoring builds omit pprof.
- Load: 200 constant VUs for 30 seconds with the published 60/30/10 endpoint
  mix; CPU sample covered 25 seconds of peak traffic.
- Result: 136,122 requests, 0 errors, approximately 4,495 requests/second.
- Raw profiles: `benchmarks/profiles/packed-hex-20260822/`.

## CPU result

| Function | Flat CPU | Cumulative CPU |
| --- | ---: | ---: |
| `main.encodeDigest` | 57.84% | 57.89% |
| Go SHA-256 block implementation | 24.18% | 24.18% |
| `main.calculateRisk` | 0.65% | 97.79% |

Packed encoding is faster than `encoding/hex`, but it remains the largest
single cost because it runs after every one of the 50,000 hashes. SHA is not yet
the dominant flat cost.

## Other profiles

- Allocation space was dominated by Go's `net/http` request parsing and header
  handling. `handleRisk` accounted for about 4.9% cumulatively, while the hash
  kernel itself remained allocation-free.
- Block time was expected queue waiting in risk handlers under overload.
- Recorded mutex delay was under one millisecond in total and is not a useful
  target.

## Decision

Test a compact, fixed-size unrolling of `encodeDigest` before PGO. This directly
targets the measured hot function without changing the algorithm, wire output,
queueing, or portability. It must improve the complete risk microbenchmark—not
only the encoding primitive—before load screening. PGO remains next if this
candidate is rejected.

## Level 0 candidate result

Five-run Linux arm64 medians from the pinned Go builder:

| Form | Complete risk kernel | Hex primitive |
| --- | ---: | ---: |
| Current one-at-a-time loop | 3.711219 ms | 9.535 ns |
| Compact four-at-a-time loop | 3.618571 ms | 6.765 ns |

The compact form improved the complete kernel by 2.50% and the isolated encoder
by about 29%. Independent endpoint and 50,000-round reference-vector tests pass
with optional CPU acceleration disabled. It therefore advances to bracketed
screening.

## Bracketed screening

Sequence: packed-hex champion -> compact-unroll candidate -> packed-hex
champion, 90 seconds each.

| Run | Work score | Errors | `/risk` p95 |
| --- | ---: | ---: | ---: |
| Champion A1 | 992,877 | 0.00% | 296.83 ms |
| Candidate B1 | 1,007,901 | 0.00% | 263.04 ms |
| Champion A2 | 980,888 | 0.00% | 278.47 ms |

The candidate was 1.51% above the stronger champion side and 2.13% above the
champion bracket average. Champion drift was -1.21%. The candidate passed every
latency gate with zero errors and is supported by the independent 2.50% kernel
gain, so it advances under the protocol's bracketed-evidence path for small
changes.

## Exact full comparison

Sequence: packed-hex champion -> compact-unroll candidate -> packed-hex
champion, using the untouched 4m30s grader.

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 2,892,463 | 1,157,669 | 0.00% | 23.64 ms | 23.73 ms | 348.30 ms |
| Candidate B1 | 2,919,075 | 1,169,136 | 0.00% | 22.87 ms | 22.96 ms | 339.98 ms |
| Champion A2 | 2,783,480 | 1,112,981 | 0.00% | 22.64 ms | 22.67 ms | 349.93 ms |

The candidate beat the stronger champion side by 0.92% and the champion bracket
average by 2.86%. It remained correct and within every latency gate. However,
the champion controls drifted by -3.77%, which is larger than the conservative
candidate advantage.

## Verdict

**Unresolved; do not promote to champion yet.** The microbenchmark, screen, and
both direct control comparisons point in the favorable direction, so reverting
and forgetting the candidate would discard useful evidence. But the exact-run
advantage does not exceed observed environmental noise. Commit `5bb6824`
preserves the candidate for an interleaved finalist comparison or separated
x86-64 validation; commit `64e38e0` remains the accepted champion.
