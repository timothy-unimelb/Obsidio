# Interleaved multi-lane risk batching

## Hypothesis

The packed-hex champion profile attributed 58% of flat CPU to `encodeDigest`,
a 32-entry table copy. That is implausible as real work; it is more likely
where samples land while the core waits for the hardware SHA-256 result that
the encoder consumes. If the kernel is latency-bound on the SHA unit rather
than throughput-bound, one core can overlap two or more *independent* chains.

Each `/risk` request is a strictly serial 50,000-round chain, but under load
the queue always holds many independent requests. A worker can therefore take
several jobs and interleave their rounds in one loop: round *i* of chain A,
round *i* of chain B, then both encodes. Every chain still performs all of its
rounds in order; only the instruction stream is interleaved. The output is
unchanged and verified against the independent reference.

## Candidate

- `calculateRiskPair` and `calculateRiskQuad` in `submission/go/main.go`, with
  explicit local buffers so the kernel stays allocation-free.
- `riskWorker` takes one job, then non-blockingly drains up to `RISK_LANES-1`
  more (default 4, bounded by `maxRiskLanes`). Three jobs run as pair + single.
- An empty queue still runs a single job immediately, so light load is
  unaffected. Under load a job waits at most one batch longer, which is small
  against the observed 300 ms queue waits.
- Portability: no new instructions or assembly; Go's runtime still chooses the
  SHA implementation at startup.

## Level 0 (Linux arm64, pinned Go 1.26.6 builder, `--cpus=2`)

Five-run medians of the complete kernel, expressed per chain:

| Form | Default CPU | Per chain | Change | `GODEBUG=cpu.all=off` | Per chain | Change |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Single | 3.482 ms | 3.482 ms | — | 15.501 ms | 15.501 ms | — |
| Pair | 5.938 ms | 2.969 ms | −14.7% | 31.334 ms | 15.667 ms | +1.1% |
| Quad | 10.503 ms | 2.626 ms | −24.6% | 61.818 ms | 15.454 ms | −0.3% |

Interleaving helps only where hardware SHA exists, and is neutral on the
scalar fallback, so it cannot regress a CPU without optional instructions.
The endpoint, reference-vector, batch-size (1–4), and escaping tests pass in
both CPU modes. Kernel allocations remain zero.

## Level 1: bracketed 90-second screen (local arm64, same host)

Sequence: champion `45ce2c7` → candidate `b29480e` → champion.

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 1,022,022 | 409,413 | 0.00% | 24.4 ms | 24.3 ms | 318.2 ms |
| Candidate B1 | 1,100,233 | 440,062 | 0.00% | 29.8 ms | 29.8 ms | 179.4 ms |
| Champion A2 | 1,022,792 | 408,217 | 0.00% | 24.3 ms | 24.3 ms | 236.5 ms |

Candidate +7.57% versus the stronger control and +7.61% versus the bracket
average, with +0.08% control drift. Promoted to the exact grader.

Note the fast-path cost: `/price` p95 rose about 5 ms. Longer uninterrupted
batches on both cores leave the cheap handlers slightly less scheduling
opportunity. The bar is 200 ms, so this is headroom spent, not a risk, but it
is the number to watch if `RISK_LANES` were raised further.

## Level 2: exact full bracket (local arm64, same host)

Sequence: champion `45ce2c7` → candidate `b29480e` → champion, untouched
4m30s `k6/grading.js`.

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 3,038,794 | 1,215,007 | 0.00% | 24.33 ms | 24.33 ms | 261.02 ms |
| Candidate B1 | 3,316,347 | 1,327,206 | 0.00% | 29.76 ms | 29.78 ms | 173.08 ms |
| Champion A2 | 3,039,407 | 1,214,926 | 0.00% | 25.31 ms | 25.33 ms | 265.21 ms |

Candidate +9.11% versus the stronger control and +9.12% versus the bracket
average. Control drift was +0.02%, so essentially all of the difference is the
change. Every gate passed with zero errors; `/risk` p95 fell by 88 ms and
`/price` p95 rose by about 4.5 ms, consistent with the screen.

## Separated x86-64 confirmation

AWS reference shape: `c7i.xlarge` target (Xeon Platinum 8488C with `sha_ni`),
container capped to 2 CPUs and 2 GiB; separate `c7i.large` load host; same AZ,
private network; pinned k6 2.2.0. Stack destroyed after the comparison.

Kernel on the x86 target (five-run medians, per chain): single 6.302 ms, pair
6.003 ms (−4.7%), quad 6.034 ms (−4.3%). SHA-NI leaves far less latency to
hide than Apple's SHA2 unit, so the gain is real but much smaller than on
arm64. Pair and quad are equivalent here, and quad is clearly better on
arm64, so `RISK_LANES=4` stays the default.

Bracketed 90-second screen: 667,278 → 682,365 → 671,805. Candidate +1.57% vs
the stronger control, +0.68% control drift.

Exact full bracket:

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 2,018,311 | 809,159 | 0.00% | 23.72 ms | 23.46 ms | 423.70 ms |
| Candidate B1 | 2,062,911 | 826,875 | 0.00% | 22.25 ms | 22.22 ms | 422.60 ms |
| Champion A2 | 2,003,123 | 801,792 | 0.00% | 22.83 ms | 22.77 ms | 437.77 ms |

Candidate +2.21% vs the stronger control and +2.60% vs the bracket average,
with −0.75% control drift. The gain exceeds the observed noise by about three
times, agrees with the x86 screen and kernel benchmark, and the fast path did
not pay for it on x86.

## Verdict

**Accept interleaved multi-lane risk batching (`b29480e`) as the submission
champion.** Local arm64: +9.11%. Separated x86: +2.21%. Both brackets were
stable, every gate passed with zero errors, and the scalar-path kernel is
neutral so no CPU without SHA instructions is penalised. The judge-day
magnitude depends on the unspecified CPU; the direction does not.

Outstanding: interleaved six-run milestone and the optional-instruction
portability full-run set for the finalist.
