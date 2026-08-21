---
name: status
description: Session-start orientation — read HANDOFF.md, PLAN.md, EXPERIMENTS.md, recent bench history and git state, then report current best score, trend, and the recommended next action. Read-only. Use this whenever the user says "status", "where are we", "catch me up", or as the first action of a fresh session.
---

# status: session-start orientation (read-only — change nothing)

1. **Read the state files** (skip any that don't exist yet, and say so):
   - `HANDOFF.md` — last session's dump and its declared next step
   - `PLAN.md` — the active plan and its `## Progress` checklist position
   - `EXPERIMENTS.md` — the last 2-3 entries (any pending verdicts?)
   - `bench/history.jsonl` — the last ~5 lines
   - `git log --oneline -5` and `git status --short`

2. **Report, concisely:**
   - **Current best:** highest work_score so far and which of the 4 bars pass on the
     best run.
   - **Trend:** latest run vs the one before — improving, flat, or regressed (name the
     number that moved).
   - **Loose ends:** dirty tree, unpushed commits, pending experiment verdicts,
     unchecked PLAN.md step in flight.
   - **Recommended next action:** one concrete step — prefer HANDOFF.md's declared next
     step unless the evidence says it's stale.

3. **Change nothing.** No builds, no benches, no file edits — orientation only.
