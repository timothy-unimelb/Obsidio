# Handoff — 2026-08-22 (sprint-2 session)

**Best result / current submission build: `5c42e72`** — freeze2-x86-01 verified
3× cold boots on the testbed: **4,299,037 / 4,289,957 / 4,305,181** (spread
0.355%), errors 0.85%, 4/4 bars every run (risk p95 ~64ms, price/stats 11.2ms).
No-SHA-NI insurance regime: **3,530,813** (was 812k — 4.3×). Docs-only commits
sit on top; `app/` is byte-identical to 5c42e72.

**What changed this session** (full narrative: EXPERIMENTS.md sprint-2 entries;
verdicts: benchmarks/decisions/):
- Item A kernel-v3 in-asm loop: Tier-0 NEGATIVE (114.9 vs 114.1ns) — default
  off behind RISK_KERNEL_V3, walls kept in the test suite.
- Item B raw-TCP HTTP/1.1 server: KEPT +2.5% (bracket rawhttp-x86-01); regime
  revalidation all green (co-located +10.4%, overdrive holds, slow-box 842k).
  Kill switch RISK_HTTP=std.
- Item C vendored AVX-512 16-lane kernel (minio, Apache-2.0 attributed): KEPT
  for no-SHA-NI graders — forced-sim grading 3.53M vs 812k baseline; gate
  provably silent on SHA-NI boxes. Force-env RISK_KERNEL=avx512.
- Item D x16-on-SHA-NI: bracket FLAT (−0.26%) despite +24% boot race — queue
  equilibrium sits below the ~15-waiter batch floor. Reverted to RISK_X16=on
  opt-in.
- Fresh-clone docker build + capped /smoke green on arm64 (fallback path).

**In flight / uncommitted:** tree clean after this handoff commit; all pushed
to `advait`. No containers left on target/dev/local. EC2 shutdown timers armed
(boxes self-poweroff ~12:00 UTC; restart from EC2 console, IPs change).

**Next step:** draft the judged deliverables — `/writeup` from EXPERIMENTS.md +
benchmarks/decisions (PLAN.md W4). Submission-morning launch-blocker unchanged:
when final thresholds publish, set RISK_ERR_GATE_BP in app/Dockerfile and
re-run one grading verification.
