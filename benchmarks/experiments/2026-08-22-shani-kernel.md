# Two-lane SHA-NI risk kernel

## Origin

Advait's `PLAN.md` (branch `fork/advait`) proposed, but never started, a
two-lane interleaved SHA-NI kernel modelled on the Linux 6.18 `finup2x`
routine, with the constant second (padding) block's message schedule
precomputed. Our Go-level lane interleaving (`b29480e`) captured only ~5% per
chain on SHA-NI, far below the +38% the plan cited for Sapphire Rapids, which
suggested the remaining gain needed instruction-level interleaving inside one
routine.

## Candidate

`submission/go/risk_amd64.s` (generated; generator kept in the session
scratchpad, the committed `.s` is the source of truth) implements
`sum256x2(inA, inB *[64]byte, outA, outB *[32]byte)`:

- block 1 follows the Go standard library's SHA-NI schedule exactly, with the
  two lanes' quads interleaved so the core overlaps their `SHA256RNDS2`
  latency;
- block 2 is the constant padding block for a 64-byte message, so its W+K
  values are a precomputed 256-byte table and no `SHA256MSG1/2` or `PALIGNR`
  work is done for it;
- only SSE encodings are used; selection requires CPUID SHA, SSSE3, and
  SSE4.1, and `RISK_SHANI=0` forces the portable path;
- every other architecture, and x86 without SHA, uses the unchanged Go path.

`calculateRiskPair` and `calculateRiskQuad` call the kernel once or twice per
round; hex encoding stays in Go. Tests compare 10,000 random input pairs with
`crypto/sha256` and full 50,000-round chains with the reference.

## Level 0 (separated x86, c7i.xlarge, Xeon Platinum 8488C, `--cpus=2`)

Five-run medians:

| Benchmark | Result | Per chain |
| --- | ---: | ---: |
| `Sum256Standard2x` (two stdlib hashes) | 249.4 ns | — |
| `Sum256x2` (kernel, two hashes) | 117.6 ns | — |
| `Risk` (single chain, stdlib) | 6.213 ms | 6.213 ms |
| `RiskPair`, Go interleaving (`RISK_SHANI=0`) | 12.003 ms | 6.001 ms |
| `RiskPair`, kernel | 6.473 ms | 3.236 ms |
| `RiskQuad`, two kernel calls | 11.980 ms | 2.995 ms |

The kernel primitive is 2.12× the standard library on this CPU and the
complete four-lane risk batch is 51.8% cheaper per chain than the stdlib
single chain. Kernel allocations remain zero. All tests pass normally and with
`GODEBUG=cpu.all=off`. The first assembly revision faulted on an unaligned
SP-relative `PADDD`; the saved states are now loaded through a register.

## Level 1: x86 bracketed screen

Champion `88855af` (Go-interleaved lanes) → candidate `8075efe` → champion.

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 698,788 | 279,602 | 0.00% | 22.6 ms | 22.6 ms | 409.6 ms |
| Candidate B1 | 861,582 | 344,900 | 0.00% | 39.1 ms | 39.1 ms | 164.2 ms |
| Champion A2 | 697,274 | 278,929 | 0.00% | 23.9 ms | 23.7 ms | 398.1 ms |

Candidate +23.30% versus the stronger control with −0.22% control drift.
`/price` p95 rose by about 16 ms: the workers now spend longer uninterrupted
stretches in assembly, so cheap handlers wait slightly longer for a core. The
bar is 200 ms.

## Level 2: x86 exact full bracket

Same sequence with the untouched 4m30s grader.

| Run | Work score | Requests | Errors | Price p95 | Stats p95 | Risk p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Champion A1 | 2,090,598 | 835,451 | 0.00% | 23.52 ms | 23.65 ms | 400.09 ms |
| Candidate B1 | 2,545,521 | 1,018,708 | 0.00% | 39.98 ms | 40.06 ms | 169.21 ms |
| Champion A2 | 2,071,268 | 829,772 | 0.00% | 24.88 ms | 24.95 ms | 406.98 ms |

Candidate +21.76% versus the stronger control and +22.33% versus the bracket
average, with −0.92% control drift. `/risk` p95 fell by 231 ms; `/price` p95
rose by 16 ms to about 40 ms.

## Portability

The kernel is selected only where CPUID reports SHA-NI; elsewhere the server
behaves exactly as the accepted champion. `GODEBUG=cpu.all=off` does not
disable our own CPUID check, so the explicit portability stress for this
candidate is `RISK_SHANI=0`, which is the path exercised by every non-SHA
processor. The judge's CPU remains unspecified: on a processor without SHA
extensions this change is neutral; on one with them it is the largest single
gain in the project.

## Verdict

**Accept the two-lane SHA-NI kernel (`8075efe`) as the submission champion.**
It is the largest single gain in the project (+21.8% on separated x86), the
bracket was stable, every gate passed with zero errors, and the change is
inert on processors without SHA extensions.

Follow-ups, in order: (1) the fast-path cost is now visible—if the locked
grader tightens the `/price` bar, `RISK_LANES=2` or a periodic yield in the
batch loop are the first knobs; (2) the plan's v2 idea, in-register hex via
`PSHUFB`, would remove the remaining Go encode between kernel calls;
(3) six-run milestone and `RISK_SHANI=0` full-run set for the finalist.
