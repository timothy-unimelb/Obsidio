# Obsidio performance tooling

Read [`PROTOCOL.md`](PROTOCOL.md) before evaluating a performance change. It is
the authoritative testing and acceptance procedure. This directory keeps the
scripts, machine-readable settings, append-only history, and raw evidence
behind the visual performance log.

## Run a checkpoint

```sh
RUN_ID=<unique-id> ./benchmarks/run-stage.sh <stage-name> <docker-build-context> [screen|full]
```

Examples:

```sh
RUN_ID=baseline-a1 ./benchmarks/run-stage.sh baseline starters/go full
RUN_ID=queue-screen-b1 ./benchmarks/run-stage.sh candidate submission/go screen
```

The runner builds the image, starts it with the grading resource caps, confirms
`/health`, runs the selected workload, stores the raw k6 summary, and appends a
compact record to `history.jsonl`. Existing result paths are not overwritten by
default.

- `screen` uses the separate proportional 90-second screening script.
- `full` uses the untouched published 4 minute 30 second grader.
- `BENCH_CPU_MODE=portable` runs the target with `GODEBUG=cpu.all=off`.
- `BENCH_COMPARISON_SET` and `BENCH_ENVIRONMENT_ID` label related evidence.

## Comparability rules

- Never alter `k6/grading.js`; abbreviated work belongs in `screening.js`.
- Record the grading-script SHA-256 and k6 version with each comparison set.
- Build every checkpoint from committed source.
- Use the same load-generator and target environment for every checkpoint in a
  comparison set.
- Keep all HTTP responses correct; a 200 status alone is not a correctness
  proof, so endpoint and reference-vector tests remain mandatory.
- Treat local results as grader-shaped evidence, not grading-hardware claims.
  The organizer keeps k6 and the target on separate machines; this local setup
  runs k6 on macOS and the target in a Linux arm64 VM on the same physical host.
- Follow the bracketed screening, full comparison, and milestone repetition
  sequences in `PROTOCOL.md`. The existing legacy comparison contains three
  full runs for the starter, bounded Go, and permanent-worker checkpoints; it
  predates the interleaved milestone sequence and is labelled accordingly.

The optional `benchmarks/risk-timing.js` diagnostic records queue wait and hash
execution separately during a 30-second, 200-VU peak slice. Run the target with
`RISK_TIMING=1` for that diagnostic only; normal scoring leaves timing headers
disabled.

Build-tagged Go profiling is also excluded from normal images. To create a
diagnostic image, use `--build-arg GO_BUILD_TAGS=profile` and publish port 6060
separately. The normal Docker build leaves the pprof listener out of the binary.

After interpreting a comparison, create a reviewed JSON decision under
`benchmarks/decisions/` and append it without rewriting prior history:

```sh
node benchmarks/record-decision.mjs benchmarks/decisions/<comparison-set>.json
```

## Recorded local comparison set

- Date: 2026-08-21 (Australia/Melbourne)
- Workload: published 4m30s ramp, 200 VUs, 60/30/10 endpoint mix
- Target cap: 2 CPUs, 2 GB memory
- k6: v2.2.0, darwin/arm64
- Target: Docker on Colima, Linux arm64
- Grading script SHA-256:
  `d7b259eb36cd1a13da1366c2d61b3cddcde36354a3604bc78a7a33f56998d20f`
- Repetitions: 3 full runs per measured checkpoint; report the median

## Latest accepted comparison

- Date: 2026-08-22
- Environment: separated AWS x86-64 (`c7i.xlarge` target capped to 2 CPUs /
  2 GiB, `c7i.large` load host, same AZ, private network)
- Sequence: Go-lanes champion `88855af` → SHA-NI kernel candidate `8075efe` →
  champion
- Exact scores: 2,090,598 → 2,545,521 → 2,071,268
- Candidate improvement: +21.76% over the stronger champion side; −0.92%
  champion drift
- Errors: 0.00%; every published p95 gate passed (`/price` p95 40 ms)
- Verdict: accepted champion, pending six-run milestone and `RISK_SHANI=0` set
- Detailed evidence:
  [`experiments/2026-08-22-shani-kernel.md`](experiments/2026-08-22-shani-kernel.md)
