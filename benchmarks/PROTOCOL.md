# Obsidio testing protocol

This is the authoritative procedure for deciding whether an optimization is
correct, faster, and safe to keep. The visual `/testing` page explains the
procedure; this file defines it. If they disagree, follow this file.

## 1. What is known about judging

The grading machine has not been finalized. The organizers have told us to
assume only:

- x86-64 Linux;
- an enforced 2.0 CPU cgroup quota;
- a 2 GiB memory ceiling;
- host CPUs may remain visible inside the container, so `nproc` and `lscpu` do
  not describe the real budget;
- no particular CPU model, cache size, or optional instruction extension such
  as AVX-512, SHA, or AES-NI; and
- a load generator separate from the target during judging.

The published latency thresholds remain placeholders until the organizers lock
the final grader. Re-run the accepted champion and final candidates when that
happens.

The submission must therefore be portable. Optimize bounded concurrency,
allocation behavior, scheduling, queueing, and the specified computation—not a
particular processor.

## 2. Sources of truth

| Artifact | Role |
| --- | --- |
| `k6/grading.js` | Untouched published 4m30s grader |
| `benchmarks/screening.js` | Proportional 90-second screening workload |
| `benchmarks/protocol.json` | Machine-readable settings and decision rules |
| `benchmarks/run-stage.sh` | Local build, capped run, and result recorder |
| `benchmarks/history.jsonl` | Append-only experiment index |
| `benchmarks/decisions/` | Reviewed machine-readable comparison verdicts |
| `benchmarks/experiments/` | Human-readable hypotheses, profiles, and interpretations |
| `benchmarks/results/` | Raw k6 summaries |

Never edit an old raw summary or history entry. Correct a mistake with a new
entry that identifies the superseded run.

## 3. Environments

### Local development environment

The target runs in a Linux arm64 VM on the Apple Silicon host and k6 runs on
the same physical machine. Use this environment for correctness, profiling,
microbenchmarks, large optimizations, and provisional A/B comparisons.

Local results can distinguish changes larger than the measured local noise,
but they do not predict the judge's absolute score or guarantee the same
ordering on x86-64 Linux.

### Separated x86 reference environment

Use this for the reusable optimization pipeline when it is available:

- target host: fixed-performance x86-64 Linux with more than two visible vCPUs;
- target container: `--cpus=2 --memory=2g --memory-swap=2g`, with no cpuset;
- load host: separate fixed-performance x86-64 Linux machine;
- networking: private, low-latency address in the same region or local network;
- no unrelated workloads on either host; and
- no burstable, flexible-baseline, Spot, or emulated instances.

The current AWS reference shape is a standard `c7i.xlarge` target host and a
standard `c7i.large` load host in the same availability zone. Equivalent
fixed-performance x86 instances are acceptable. Keep both hosts unchanged
throughout a comparison set.

Before recording a comparison set, verify the container reports or is
configured for the equivalent of:

```text
cpu.max     200000 100000
memory.max  2147483648
```

## 4. Four test levels

### Level 0 — correctness and microbenchmarks

Run before load testing:

```sh
cd submission/go
go test ./...
go test -bench=. -benchmem ./...
```

The endpoint tests must independently verify `/risk` rather than trusting the
optimized implementation to verify itself. Reject any incorrect candidate.

Profiling must remain diagnostic-only. The Go submission compiles pprof support
only with `--build-arg GO_BUILD_TAGS=profile`; normal scoring builds must omit
that tag and listener.

### Level 1 — 90-second screening

Use `benchmarks/screening.js`. It preserves the published endpoint mix,
thresholds, and ramp proportions while shortening the stages to 20s, 40s, 20s,
and 10s.

Screen with a bracketed sequence:

```text
champion -> candidate -> champion
```

The surrounding champion runs reveal environmental drift. A screen is a
promotion tool, not a publishable grading result.

### Level 2 — exact full comparison

Use the untouched `k6/grading.js`. A normal promoted comparison is:

```text
champion -> candidate
```

For a small or questionable change, use:

```text
champion -> candidate -> champion
```

Each full run lasts 4m30s. Restart the target container before every run; build
each image once per comparison set.

### Level 3 — milestone validation

Use only when selecting finalists, publishing a checkpoint, changing the test
environment, or preparing the final submission. Run three complete sieges per
implementation and interleave them:

```text
A -> B -> B -> A -> A -> B
```

Report medians and the complete range. Keep all six raw summaries.

### Overload stress (not a level)

`benchmarks/stress.js` ramps to four times the published peak. It is not a
grading workload and is never publishable. Run it only when the overload
policy itself changes (shedding logic, error budget, patience), because the
published load cannot exercise that policy. Kernel, batching, yield, and
cross-build comparisons do not need it.

## 5. Promotion and acceptance rules

These rules are defaults, not substitutes for inspecting the data:

| Observed score change | Action |
| --- | --- |
| Greater than 2% in screening | Promote to an exact full comparison |
| 0.5% to 2% | Require bracketed or repeated paired evidence |
| Less than 0.5% | Treat as unresolved unless profiling explains the gain |
| Any correctness failure | Reject |
| Error rate at or above 1% | Reject under the current published gate |
| Material loss of latency headroom | Investigate before accepting |

For an accepted optimization, the improvement must exceed the observed noise
of its comparison set. Prefer the simpler implementation when results overlap.
Do not combine several unmeasured changes and attribute the result to one of
them.

`GODEBUG=cpu.all=off` is a portability stress test for important finalists. It
does not emulate every possible CPU, but it checks behavior without optional Go
runtime instruction-set acceleration. Record those runs as a separate
comparison set; never mix them into a default-CPU median.

## 6. Valid and invalid runs

A run is comparable only when:

- the source version and dirty status are recorded;
- the workload script and its SHA-256 are recorded;
- target limits, image identity, runtime mode, and environment ID are recorded;
- the same environment is used for every member of the comparison set;
- k6 is not CPU-saturated;
- the host does not sleep, suspend, update, or run another heavy workload; and
- the target passes correctness checks.

Mark a run invalid rather than deleting it when there is host suspension,
network interruption, generator saturation, accidental source drift, missing
metadata, or a resource-limit mismatch. Threshold failures caused by the
candidate are valid negative results and must remain in the history.

## 7. Recording a run

Use a unique run identifier:

```sh
RUN_ID=exp-name-a1 ./benchmarks/run-stage.sh candidate submission/go screen
RUN_ID=exp-name-a1 ./benchmarks/run-stage.sh candidate submission/go full
```

The runner writes a raw summary under `benchmarks/results/` and appends a compact
entry to `benchmarks/history.jsonl`. For a separated environment, set a stable
identifier such as:

```sh
BENCH_ENVIRONMENT_ID=aws-c7i-set-01
BENCH_TARGET_RELATIONSHIP=separate-private-network
```

Every accepted result must retain:

- timestamp, stage, profile, run ID, and comparison-set ID;
- Git SHA and meaningful dirty source files;
- image ID, Go/runtime mode, CPU mode, and environment ID;
- k6 version and workload fingerprint;
- work score, request count, error rate, and all three p95 values;
- raw-summary path; and
- verdict: pending, kept, reverted, invalid, or superseded.

Record the hypothesis and interpretation in the commit, pull request, or
performance write-up. Numbers without the change being tested are not a useful
experiment.

After reviewing a comparison, preserve the verdict without editing older run
entries:

```sh
node benchmarks/record-decision.mjs benchmarks/decisions/<comparison-set>.json
```

## 8. Operating checklist

Before a comparison:

1. Confirm source and current champion.
2. Run correctness tests.
3. Confirm no result path will be overwritten.
4. Confirm target limits and environment identity.
5. Build candidate and champion images.
6. Keep unrelated workloads idle.

After a comparison:

1. Check correctness, errors, p95 latency, score, and environmental anomalies.
2. Compare the delta with the bracketed champion and measured noise.
3. Preserve raw summaries and history, including failures.
4. Mark the verdict explicitly.
5. Update the performance page only from accepted, auditable results.
6. Stop or terminate paid infrastructure when the session is complete.
