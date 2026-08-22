# SHA-NI kernel correctness sweep

## Purpose

The multi-lane risk kernel was, until `1360482`, pure Go: correctness came for
free from `crypto/sha256`. The SHA-NI path (`risk_amd64.s`, ~660 lines of
hand-written assembly, routed in from `8075efe`) replaces that with bespoke
`SHA256RNDS2`/`MSG1`/`MSG2` code. The performance case for it is the separate
`shani-x86` bracket. **This note is the orthogonal gate: proof the assembly is
correct before it can ship.** The perf win is irrelevant if a single digest is
wrong — a wrong `/risk` answer fails verification, scores zero for that request,
and enough of them breaches the <1% error gate.

The `8075efe` subject line ("fix stack alignment") is the reason this is a full
sweep and not a spot check: the kernel had at least one frame/ABI bug recently,
and that failure class is state-dependent — it passes a quiet loop and corrupts
under a stack move or preemption landing on the call.

## What is under test, and its gating

`sum256x2(inA, inB *[64]byte, outA, outB *[32]byte)` — two-lane SHA-256 of two
fixed 64-byte blocks. Reached only when both compile- and run-time gates pass:

- compile: `//go:build amd64 && !purego`; every other target compiles
  `risk_other.go`, whose `sum256Portable` is plain `sha256.Sum256`.
- runtime: `useSHANIPair = cpuidSHA() && RISK_SHANI != "0"`, where `cpuidSHA()`
  checks SSSE3 (leaf 1 ECX bit 9), SSE4.1 (bit 19), SHA (leaf 7 EBX bit 29).

So the correctness question has two halves: (1) is the kernel itself correct on
a SHA-NI CPU, and (2) is the portable fallback byte-identical to it, so a grader
that takes the other path scores the same.

## Existing coverage (before this sweep)

- `TestSum256x2MatchesStandardLibrary` — 10,000 random 64-byte inputs, one host,
  one RNG seed, single-threaded.
- `TestPairKernelPathsAgree` — one seed pair through a full 50,000-round chain.
- `TestRiskBatchSizesMatchReference` / `TestMultiLaneRiskMatchesReference` —
  batch routing at the `calculateRisk` level (runs on whichever path is active).

Good baseline; blind to concurrency/GC pressure, input-pattern edges,
position dependence, and kernel-vs-fallback equivalence.

## The sweep (four added files, `amd64 && !purego`, all skip on non-SHA-NI CPUs)

| File | Test | Targets |
| --- | --- | --- |
| `kernel_stress_amd64_test.go` | `TestKernelUnderGCAndPreemption` | frame/ABI bugs under `GOGC=1`, many goroutines, stack growth between calls, chained inputs, per-iteration diff vs stdlib |
| `kernel_fuzz_amd64_test.go` | `FuzzSum256x2` | input-dependent bugs (wrong shuffle mask / schedule constant); corpus seeds boundary patterns + the real ASCII hex alphabet |
| `kernel_position_amd64_test.go` | `TestKernelLaneIndependence`, `TestChainPositionIndependence` | cross-lane leakage; a seed's result must be identical in every pair lane and quad position regardless of neighbors, tied to the single-lane reference |
| `kernel_pathequiv_amd64_test.go` | `TestKernelAndPortablePathsAgree` (+ shared `requireSHANI`/`forceKernel`) | kernel and portable fallback must be byte-identical, and both equal to the reference |

All honor `-short`. `calculateRisk` (single-lane) stays pure Go and is the
oracle the kernel pair/quad paths are checked against.

## Execution matrix

Run on a SHA-NI host (the `c7i.xlarge` bracket target is ideal — Go 1.26 + real
`sha_ni`). Commands:

| # | Command | Purpose |
| --- | --- | --- |
| 1 | `go test ./... -run 'Kernel\|Position\|PathsAgree' -race -count=1` | correctness + data-race detection |
| 2 | `go test ./... -run TestKernelUnderGCAndPreemption -count=5` | repeat the stress path to shake out state-dependent corruption |
| 3 | `go test -fuzz=FuzzSum256x2 -fuzztime=10m` | explore the input space differentially vs stdlib |
| 4 | `RISK_SHANI=0 go test ./... -count=1` | fallback is sound and the CPUID gate degrades correctly |
| 5 | `GOARCH=arm64 go build ./... ` | non-amd64 fallback still compiles |

Environment coverage to record (same asm can pass on one microarch and corrupt
on another):

| Environment | Status |
| --- | --- |
| Intel SHA-NI (grading class, e.g. `c7i` Xeon 8488C) | pending |
| AMD Zen SHA-NI | pending (reach if available) |
| arm64 build compiles, portable path passes | pending |

## Acceptance criteria

The SHA-NI kernel may be accepted as the submission champion only when, on at
least the grading-class Intel SHA-NI CPU:

1. commands 1, 2, 4, 5 pass with zero failures and no race reports;
2. command 3 runs its budget with no new failing corpus entries;
3. `TestKernelAndPortablePathsAgree` confirms kernel ≡ portable ≡ reference.

Until then the kernel is a prototype on top of the accepted lanes champion
(`88855af`), which remains the fallback the submission is safe to ship on.

## Results

None recorded yet. Authored `2026-08-22`; the four test files were added but
not executed here (no local Go toolchain). Fill this section from the run on the
SHA-NI target, then — only on a clean pass — record the accept/reject decision
alongside the `shani-x86` perf bracket.

## Note on the write-up

`submission/go/RESILIENCE.md` still states the portability trade-off as "no
CPU-specific instructions, cgo, or assembly are assumed." That is now false at
HEAD. Leave it until the kernel is accepted (do not describe an unaccepted build
as shipped); revise the trade-off section as part of the accept step if this
sweep and the perf bracket both pass.
