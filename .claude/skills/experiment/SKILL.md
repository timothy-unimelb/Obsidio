---
name: experiment
description: Append a structured entry (hypothesis, change, measured before/after, verdict) to EXPERIMENTS.md for every optimisation attempt — this file is the evidence base for the judged resilience write-up. Use this whenever the user says "log this experiment", "record that attempt", "note what we tried", or after any bench run that tested a change.
---

# experiment: log an optimisation attempt

EXPERIMENTS.md is the write-up's evidence. Numbers, not adjectives. Log failures too —
"tried X, made it worse, reverted" is exactly what the judges want to see.

1. **Gather the facts:**
   - `git rev-parse --short HEAD` for the SHA (note if the tree is dirty).
   - The last TWO lines of `bench/history.jsonl` — before and after. If the change
     hasn't been benched yet, the Result is "not yet measured", never a guess.

2. **Append to EXPERIMENTS.md** (create from the template below; newest entry at the
   bottom):

   ```markdown
   ## <YYYY-MM-DD> <short title>

   - **SHA:** <short sha><, dirty tree if applicable>
   - **Hypothesis:** <what we believed and why — one or two sentences>
   - **Change:** <files touched + a one-liner of what changed>
   - **Result:** work_score <before> → <after>; p95 price <b>→<a>ms,
     stats <b>→<a>ms, risk <b>→<a>ms; errors <b>→<a>.  (or "not yet measured")
   - **Verdict:** kept | reverted | pending
   ```

3. **Pull the before/after numbers from bench/history.jsonl** — never from memory.
   The "before" line is the last bench BEFORE the change, "after" is the bench of the
   change itself. If those two lines aren't adjacent, say which lines were used.

4. **Update the Verdict** of any earlier "pending" entry this run resolves.
