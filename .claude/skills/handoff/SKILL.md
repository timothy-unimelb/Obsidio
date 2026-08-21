---
name: handoff
description: End-of-session state dump — sync PLAN.md progress checkboxes to reality and overwrite HANDOFF.md with best score, session changes, in-flight work, and the single next step, so /clear or tomorrow costs nothing. Use this whenever the user says "handoff", "wrap up the session", or "I'm stopping for the day".
---

# handoff: end-of-session state dump

Goal: a fresh session (or a /clear) can resume with zero archaeology.

1. **Sync PLAN.md to reality.** If PLAN.md exists, update its `## Progress` checkboxes
   to match what actually got done — no aspirational ticks. If the plan is finished,
   move it to `archived-plans/` with a dated name.

2. **Write/overwrite HANDOFF.md** (repo root, keep it under ~30 lines):
   - **Best result so far:** best work_score and which bars pass, pulled from
     `bench/history.jsonl` (say "no bench runs yet" if empty).
   - **What changed this session:** 2-5 bullets, referencing experiment entries where
     they exist.
   - **In flight / uncommitted:** `git status --short` summary — anything half-done and
     where it stands.
   - **Next step:** the SINGLE most valuable next action, concrete enough to start cold
     (e.g. "bench the worker-pool change at SHA abc123 — hypothesis logged in
     EXPERIMENTS.md").

3. **Remind the user** of anything that must not be forgotten overnight (e.g. "the
   container is still running: docker stop obsidio", "tree is dirty and unpushed").
