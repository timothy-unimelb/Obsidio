# In-kernel hex encoding, then yielding between kernel chunks

Two candidates on top of the accepted SHA-NI kernel (`8075efe`, carried at
`771cb43`). They are reported together because the second was found by
explaining the first's flat result.

## Candidate 1: hex encoding and the chain loop inside the kernel (`b848e3c`)

Advait's plan listed "kernel v2: in-register hex via PSHUFB" as the follow-up.
`riskChain2x`/`riskChain4x` now run the whole feedback chain in assembly: after
each round the digest is split into nibbles (`PSRLW`, `PAND`), mapped through a
16-byte `PSHUFB` table, interleaved with `PUNPCKLBW/HBW`, and written back in
place as the next 64-byte input. The Go `encodeDigest` calls and the per-round
call boundary disappear. The earlier `sum256x2` tests (fuzz, lane
independence, GC/preemption stress, path equivalence) are kept through a
one-round shim.

### Level 0 (same c7i.xlarge host, back-to-back, five-run medians)

| Form | v1 `8075efe` | v2 `b848e3c` | Per-chain change |
| --- | ---: | ---: | ---: |
| Pair | 7.227 ms | 6.694 ms | −7.4% |
| Quad | 13.297 ms | 11.937 ms | −10.2% |

All tests pass, including a 20-second fuzz run and `RISK_SHANI=0`.

### Level 1 x86 screen: 779,120 → 785,885 → 788,597

−0.34% versus the stronger control with +1.2% drift: **unresolved**. A 10%
kernel gain produced no score change.

### Why

Per-tier averages from the raw summaries:

| Champion | Price avg / median | Risk avg | Request rate |
| --- | ---: | ---: | ---: |
| Go lanes (`88855af`) | 16.0 / 19.3 ms | 135.8 ms | 2,958/s |
| SHA-NI v1 (`771cb43`) | 19.4 / 17.6 ms | 64.3 ms | 3,465/s |
| SHA-NI v2 (`b848e3c`) | 19.9 / 18.0 ms | 58.0 ms | 3,488/s |

A `/price` lookup takes microseconds; its 17 ms median is time waiting for a
core. The assembly chain loop cannot be asynchronously preempted, so each
worker holds its core for a whole ~12 ms chain and cheap handlers queue behind
it. With 200 closed-loop virtual users, the request rate—and therefore the
score, 2.5 points per request on average—is bounded by fast-path latency, not
by risk CPU. Making the kernel faster could not show up until the cheap
requests stopped waiting for it.

## Candidate 2: yield between chunked kernel calls (`5c3d681`)

The kernel is called in `RISK_YIELD_ROUNDS` chunks (default 256 rounds, about
60 µs for a quad) with `runtime.Gosched()` between them, so a waiting handler
runs within tens of microseconds instead of milliseconds. The cost is about
200 yields per chain. Kernel throughput is unchanged.

### Level 1 x86 screen

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 783,602 | 313,210 | 0.00% | 42.6 ms | 42.6 ms | 186.7 ms |
| Candidate B1 | 1,202,215 | 479,586 | 0.00% | 10.3 ms | 10.3 ms | 259.3 ms |
| Champion A2 | 775,453 | 308,643 | 0.00% | 43.2 ms | 43.2 ms | 187.0 ms |

+53.4% versus the stronger control with −1.0% drift. `/price` p95 fell 4×;
`/risk` p95 rose because more risk requests now arrive per second, and remains
6× inside its bar.

### Level 2 x86 exact full bracket

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 2,338,684 | 935,737 | 0.00% | 43.28 ms | 43.16 ms | 180.18 ms |
| Candidate B1 | 3,598,675 | 1,440,622 | 0.00% | 10.43 ms | 10.44 ms | 257.99 ms |
| Champion A2 | 2,339,747 | 935,322 | 0.00% | 43.13 ms | 43.09 ms | 180.88 ms |

+53.8% versus the stronger control with +0.05% control drift.

### Yield cadence

A 2,048-round chunk screened at 1,085,636 against 256-round controls of
1,197,651 and 1,203,656 (−9.8%, price p95 20 ms versus 10 ms), so 256 is not
over-yielding. Finer cadences were not tested.

## Gap-bridging follow-up: overload gate and durable POST /price

An admission gate (LIFO parking, boot-calibrated patience, front-door 503 on
an error budget) and an fsynced write-ahead log for `POST /price` were added
on top of the champion. Published-load screens of the gate variants:

| Variant | Score vs controls | Errors |
| --- | ---: | ---: |
| LIFO, park 64 | +1.9% | 0.63% (front-door rejections at the 200-VU peak) |
| LIFO, park 256 | +0.2% | 0.22% (starvation discards past patience) |
| Adaptive FIFO→LIFO | −0.9% | 0.00% |

At 800 VUs (`benchmarks/stress.js`) the plain champion scored 1,332,296 with
zero errors and risk p95 1,266 ms, passing every bar; the gate with a stale
sweep scored 1,705,624 at 3.53% errors, failing the error gate; the first
gate revision also left a job unserved for 29 s because stale jobs were only
discarded when popped. Shedding therefore ships **off by default**
(`RISK_SHED=1`). The final exact bracket of the shipped build, gate off and
WAL on, was 3,607,469 → 3,601,743 → 3,611,091 with zero errors: inert.
`killtest.sh` confirmed two updates survive `docker kill` and restart.

## Verdict

**Keep `5c3d681` (in-kernel hex + yield) as the champion, carried forward in
`4d1d2ea` with durable `POST /price` and the opt-in gate.** The yield is the
largest single gain in the project and the bracket was the most stable
recorded. Outstanding: six-run milestone, `RISK_SHANI=0` full set, and the
rerun after the grader locks.
