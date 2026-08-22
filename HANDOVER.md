# Handover — 2026-08-22 ~19:10 AEST

Read `AGENTS.md`, then `benchmarks/PROTOCOL.md`, then `benchmarks/ROADMAP.md`.
This file is the session-to-session state; delete it when it stops being true.

## State of the submission

- **Branch `draft` on `fork` (timothy-unimelb/Obsidio) is clean and pushed at
  `432b27d`.** It is a complete, verified submission: champion source
  `6808e7d`, build gate (`go vet && go test -short` inside `docker build`),
  single-service `docker-compose.yml` with a named volume, fresh-clone build
  and kill test verified from GitHub, README and RESILIENCE.md rewritten for
  the governor build, visuals site and `explainer/obsidio-line.html` current,
  results artifact current. AWS stack destroyed.
- **Headline (separated c7i, exact grader, bracketed):** governor 4,854,704 vs
  zero-error 4,143,222 / 4,141,819 (+17.2%, 0.85% errors by design, all
  bars). Head-to-head vs Advait's frozen build `c0c8f95`: 4,835,626 vs
  4,321,831 / 4,285,200 (+11.9%, same error budget). Passes all bars at
  800 VUs at 0.878% errors. Absolute scores move ~15% between AWS instances;
  only within-bracket deltas are claimed.
- **Organizers have not updated the grader** (`k6/grading.js` SHA
  `d7b259eb…` unchanged since 19 Aug). Deadline 10:00 AEST 23 Aug.
- Teammate branches: `fork/advait` (4.30M frozen build, last commit 18:27
  AEST 22 Aug), `fork/joel/draft`. Both independent; not merged.

## In progress: porting Advait's Items B and C (branch `port-advait-items-b-c`)

Tim asked for both to be brought into ours. Branch is pushed.

- **Item B, raw-TCP HTTP/1.1 server — ported, compiles, unit suite passes,
  NOT load-tested.** `submission/go/rawserver.go` is Advait's
  `app/rawserver.go` with dispatch replaced by a single `route(resp, req)`
  call; `writeJSON`/`writeJSONBytes` short-circuit into `rawResponse`;
  `main()` boots the raw listener unless `RISK_HTTP=std`. Still to do:
  (1) port his `app/rawserver_test.go` (12 tests: keep-alive, split reads,
  pipelining, oversized head closes, encoded query, POST framing,
  Connection: close, rejects, risk end-to-end) against our server — use a
  `net.Listen(":0")` + `rawServe` fixture; (2) `killtest.sh` through the raw
  server; (3) x86 bracket vs `6808e7d` (expect +2–3%; his was +2.1% with risk
  p95 76→65 ms, errors unchanged). Ship only if the bracket is positive.
- **Item C, AVX-512 16-lane multi-buffer kernel — NOT started.** Vendored
  `sha256x16_amd64.s` (minio/sha256-simd v1.0.1, Apache-2.0) and
  `LICENSE.minio-sha256-simd` are parked at
  `/private/tmp/claude-501/-Users-mac-code-Obsidio/553a9b39-0088-4fb7-bbd2-1d4a090b4d27/scratchpad/parked/`
  (re-fetch with `git show fork/advait:app/sha256x16_amd64.s` if the
  scratchpad is gone). Source of the glue to adapt:
  `fork/advait:app/shakernel_x16_amd64.go` (+ `_test.go`) and the worker
  integration in `fork/advait:app/main.go` around `riskChainX16` /
  `riskX16MinBatch`. Adaptation notes for our structure:
  - the asm is `func sha256X16Avx512(digests, scratch *[512]byte, table *[512]uint64, mask []uint64, inputs [16][]byte)`; it needs a Go declaration (vet failed without one, which is why the `.s` is parked);
  - our hex helper is `encodeDigest(*[32]uint16, *[32]byte)`; write a `hexEncode64(*[64]byte, *[32]byte)` wrapper or reuse `lowercaseHexPairs`;
  - our gate's `take(limit, batch)` takes a limit: when the x16 path is live and `parkedCount() >= minBatch`, take up to 16 and run `x16ChainRun`; `riskJob.result` expects `[64]byte` hashes, not strings;
  - yield every `riskYieldRounds` steps (one step = 16 lanes, ~0.7 µs);
  - gating: AVX-512 F/DQ/BW/VL from `/proc/cpuinfo` flags AND no SHA-NI (`!useSHANIPair`) unless `RISK_X16=on`; boot differential self-test; boot race must win ≥30% vs the displaced path; `RISK_X16=off` kill switch. On SHA-NI boxes Advait measured it flat (−0.26%), so it is insurance only;
  - validation on c7i (SHA-NI + AVX-512): unit tests for `x16Sum64` vs `crypto/sha256` and full chains vs `referenceRisk` run unconditionally when AVX-512 is present; the load validation of the insurance regime is `RISK_SHANI=0 GODEBUG=cpu.sha=off RISK_X16=on` (his sim: 812k → 3.53M). His tests to adapt: `TestX16Sum64MatchesStdlib`, `TestX16ChainMatchesReference`, `TestX16RaceHammer`, `BenchmarkX16Step`.

## Before freezing the final commit (in this order)

1. Merge `port-advait-items-b-c` into `draft` only with bracket evidence.
2. Fresh-clone build + `killtest.sh` again (the build gate runs the tests).
3. One AWS session: six-run milestone `A B B A A B` (final vs governor-off)
   and one `RISK_SHANI=0` full run; write medians into RESILIENCE.md.
4. Update the results artifact
   (https://claude.ai/code/artifact/8fd6e556-4d1d-4fa8-956c-892850c812ac)
   from `history.jsonl` (generator pattern: see the scratchpad `art/` script
   or regenerate; the artifact's share pin must be moved manually).
5. Push; `git status` clean; AWS destroyed (`./benchmarks/aws/destroy.sh`).

## Operating notes

- AWS: `AWS_CONFIRM=create-paid-resources AWS_BUDGET_EMAIL=timothymanojmathews@gmail.com AWS_REGION=us-east-1 ./benchmarks/aws/provision.sh`; session may need `aws login` first; always destroy after.
- Comparisons: `AWS_REGION=us-east-1 BENCH_COMPARISON_SET=<set> RUN_PREFIX=<prefix> ./benchmarks/aws/run-comparison.sh <champion-context> submission/go screen|full` — contexts must be committed git checkouts (use `git worktree add --detach`). Wrap long runs in background tasks; the runner is killed by a 10-minute foreground timeout.
- Local Level 0: no Go toolchain on the Mac; use the pinned builder image (`golang:1.26.6-bookworm@sha256:116d58…`) via Docker; Rosetta has no SHA-NI, so kernel tests skip locally and must run on x86.
- Stress (`benchmarks/stress.js`, 800 VUs) only when the overload policy changes.
- Another session has been editing `explainer/obsidio-line.html`; coordinate before touching it.
