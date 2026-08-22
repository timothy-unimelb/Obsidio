# Handover — 2026-08-22 ~22:00 AEST

Read `AGENTS.md`, then `benchmarks/PROTOCOL.md`, then `benchmarks/ROADMAP.md`.
This file is the session-to-session state; delete it when it stops being true.

## State of the submission

- **Branch `draft` on `fork` (timothy-unimelb/Obsidio) is the submission.**
  Champion source `8cc47de`: the governor build `6808e7d` plus Advait's raw
  HTTP/1.1 server (Item B) and sixteen-lane AVX-512 insurance kernel
  (Item C), both ported, tested, and bracketed. Docs, evidence, and the
  results artifact are current at the last commit on `draft`.
- **Headline (separated c7i, exact grader, six-run milestone):** final
  4,397,038 (4,380,394–4,402,197) vs the same build with the governor off
  4,187,402 (4,177,395–4,204,856): +5.0%, 0.85% errors, all bars. This
  instance runs the unchanged `6808e7d` build at 4.31–4.34M against 4.85M on
  the previous instance; the governor-off build is flat across instances, the
  governor build moves with the hardware; the load generator was 23–40% busy
  at the peak. Only within-bracket deltas are claimed.
- **Raw server:** +1.90% bracket plus a repeat pair; pooled medians +1.6%,
  ranges disjoint, errors unchanged. **Sixteen-lane kernel:** off on SHA-NI
  boxes; simulated no-SHA-NI grader 769k → 3.31M (4.3×), bars pass both ways.
  Report: `benchmarks/experiments/2026-08-22-raw-http-x16.md`; decisions
  `rawhttp-*`, `x16-sim-*`, `milestone-*`, `loadhost-cpu-*`.
- **Organizers have not updated the grader** (`k6/grading.js` SHA
  `d7b259eb…`). Deadline 10:00 AEST 23 Aug.
- Teammate branches `fork/advait`, `fork/joel/draft`: independent, not merged.
- Another session rewrote `submission/go/RESILIENCE.md` (numbers-first
  layout); this session filled its pending markers. `explainer/obsidio-line.html`
  belongs to a third session.

## If work resumes

1. Re-run the milestone when the grader locks; if the error bar tightens,
   ship `RISK_SHED=0` (zero errors, −5 to −17% depending on the instance).
2. Not re-measured on the final build: the 800-VU stress (overload policy
   unchanged; the raw server never cancels a parked job when a client gives
   up, which only makes the budget accounting more conservative).
3. The cheap-path latency shift under the raw server (`/price` median 3.4 →
   4.1 ms while `/risk` mean 92 → 83 ms) is recorded, not explained.

## Operating notes

- AWS: `AWS_CONFIRM=create-paid-resources AWS_BUDGET_EMAIL=timothymanojmathews@gmail.com AWS_REGION=us-east-1 ./benchmarks/aws/provision.sh`; always destroy after (`./benchmarks/aws/destroy.sh`).
- Comparisons: `AWS_REGION=us-east-1 BENCH_COMPARISON_SET=<set> RUN_PREFIX=<prefix> ./benchmarks/aws/run-comparison.sh <champion-context> <candidate-context> screen|full` — contexts must be committed git checkouts (`git worktree add --detach`); environment variants (`RISK_SHED=0`, `RISK_SHANI=0 GODEBUG=cpu.sha=off`) are committed Dockerfile `ENV` lines on throwaway branches (`governor-off-final`, `x16-sim-off`, `x16-sim-auto`). Wrap runs over 10 minutes in background tasks.
- Local Level 0: no Go toolchain on the Mac; use the pinned builder image via Docker. `GOARCH=amd64 go vet ./...` cross-vets the asm; kernel and x16 tests only run on x86 (the AVX-512 routine needs a real AVX-512 host).
- Stress (`benchmarks/stress.js`, 800 VUs) only when the overload policy changes.
