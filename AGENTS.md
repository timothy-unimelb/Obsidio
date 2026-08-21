# Obsidio repository guidance

Before changing or evaluating performance-sensitive code, read:

1. `OBSIDIO-DETAIL-PAGE.md` for the endpoint and scoring contract.
2. `benchmarks/PROTOCOL.md` for the authoritative testing procedure.
3. `benchmarks/history.jsonl`, when present, for the accepted experiment trail.
4. `benchmarks/ROADMAP.md` for the current champion, active experiment, and next
   decision gate. Update it as work advances so another session can resume
   without relying on conversation context.
4. The latest report referenced by a history decision for its interpretation,
   limitations, and raw-evidence paths.

## Performance work

- Correctness comes before throughput. Run endpoint tests and independent risk
  vectors before any load comparison.
- Never edit `k6/grading.js`; it is the published grading workload. Use
  `benchmarks/screening.js` for abbreviated experiments.
- Compare a candidate with the current champion in the same environment and
  comparison set. Do not compare absolute scores across different machines.
- Use the screening, promotion, full-run, and repetition rules in
  `benchmarks/PROTOCOL.md`. Small changes require paired or bracketed runs.
- Record failed and reverted experiments as well as successful ones. Preserve
  raw summaries and use a unique `RUN_ID`; do not silently overwrite evidence.
- Do not describe a performance improvement as accepted until its correctness,
  error rate, latency headroom, score delta, and observed noise all support it.
- Treat local Apple Silicon results as local evidence, not a prediction of the
  judge's unspecified x86-64 Linux CPU.

## Working tree safety

This repository may contain in-progress experiments and uncommitted raw
results. Preserve unrelated changes, inspect the worktree before editing, and
never discard or rewrite another experiment's evidence.
