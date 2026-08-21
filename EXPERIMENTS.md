# Experiments log

One entry per optimisation attempt. This file IS the resilience write-up's evidence —
numbers, not adjectives. Log failures and reverts too; before/after numbers come from
`bench/history.jsonl`, never from memory. Newest entry at the bottom.

Entry format (see the /experiment skill):

```markdown
## YYYY-MM-DD Short title
- **SHA:** abc1234
- **Hypothesis:** ...
- **Change:** files + one-liner
- **Result:** work_score X → Y; p95 price a→b ms, stats c→d ms, risk e→f ms; errors g→h
- **Verdict:** kept | reverted | pending
```

## 2026-08-21 Engineered Go v1 vs naive Go baseline (capped)

- **SHA:** 4519449, dirty tree (scaffolding + app/ not yet committed)
- **Hypothesis:** The /risk hash chain gates the closed loop. A Go app with
  GOMAXPROCS=2, /risk bounded to 2 concurrent chains (semaphore; waiters park),
  a zero-allocation sha256/hex kernel, and reflection-free JSON should raise
  work_score and cut all p95s vs the naive Go starter.
- **Change:** new `app/` (main.go, go.mod, Dockerfile). Baseline = untouched
  `starters/go` in the same capped container (2 CPU / 2 GB, cpuset 0,1),
  same-machine k6 (directional).
- **Result:** work_score 745,484 → 1,273,640 (+71%); p95 price 163.4→119.0ms,
  stats 162.7→118.5ms, risk 1011.4→320.7ms; errors 0%→0%; requests 297k→509k.
  (history.jsonl lines 2 and 3; line 1 is an uncapped Node run, not comparable.)
- **Verdict:** kept
