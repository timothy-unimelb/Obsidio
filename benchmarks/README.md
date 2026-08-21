# Obsidio performance record

This directory keeps the raw evidence behind the visual performance log. Every
checkpoint is tested with the published `k6/grading.js` workload against a
freshly built target container capped at 2 CPUs and 2 GB of memory.

## Run a checkpoint

```sh
./benchmarks/run-stage.sh <stage-name> <docker-build-context>
```

Examples:

```sh
./benchmarks/run-stage.sh baseline starters/go
./benchmarks/run-stage.sh current submission/go
```

The script builds the image, starts it with the grading resource caps, confirms
`/health`, runs the complete 4 minute 30 second published siege, and writes the
k6 summary to `benchmarks/results/<stage-name>-summary.json`.

## Comparability rules

- Do not alter `k6/grading.js` between checkpoint runs.
- Record the grading-script SHA-256 and k6 version with each comparison set.
- Build every checkpoint from committed source.
- Use the same load-generator and target environment for every checkpoint in a
  comparison set.
- Keep all HTTP responses correct; a 200 status alone is not a correctness
  proof, so endpoint and reference-vector tests remain mandatory.
- Treat local results as grader-shaped evidence, not grading-hardware claims.
  The organizer keeps k6 and the target on separate machines; this local setup
  runs k6 on macOS and the target in a Linux arm64 VM on the same physical host.
- Repeat important checkpoints three times and compare medians before accepting
  a small improvement. The raw files contain three full runs for the starter,
  bounded Go, and permanent-worker checkpoints.

The optional `benchmarks/risk-timing.js` diagnostic records queue wait and hash
execution separately during a 30-second, 200-VU peak slice. Run the target with
`RISK_TIMING=1` for that diagnostic only; normal scoring leaves timing headers
disabled.

## Current comparison set

- Date: 2026-08-21 (Australia/Melbourne)
- Workload: published 4m30s ramp, 200 VUs, 60/30/10 endpoint mix
- Target cap: 2 CPUs, 2 GB memory
- k6: v2.2.0, darwin/arm64
- Target: Docker on Colima, Linux arm64
- Grading script SHA-256:
  `d7b259eb36cd1a13da1366c2d61b3cddcde36354a3604bc78a7a33f56998d20f`
- Repetitions: 3 full runs per measured checkpoint; report the median
