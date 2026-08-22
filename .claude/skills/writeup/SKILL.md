---
name: writeup
description: Draft the two judged deliverables — WRITEUP.md (resilience write-up: bottleneck → fix → measured before/after) and a video-pitch outline — strictly from the evidence in EXPERIMENTS.md and bench/history.jsonl. Use this whenever the user says "draft the write-up", "prep the submission", or "video outline".
---

# writeup: draft the judged deliverables from evidence

The judges reward deliberate engineering shown with numbers. Never pad with adjectives.

1. **Read the evidence:** EXPERIMENTS.md, all of `bench/history.jsonl`, and CLAUDE.md
   (contract + chosen-starter rationale).

2. **Write WRITEUP.md** (repo root), structured as:
   - **Architecture in one paragraph** — what the system is and the one core decision
     (fast-path isolation, worker sizing for the 2-core cap, backpressure).
   - **Bottleneck → what we did → measured before/after**, one subsection per real
     bottleneck found, each backed by a specific experiment entry and its history.jsonl
     delta (work_score and p95s). Chronological is fine; causal is better.
   - **Final results table:** the BEST qualifying run vs every bar
     (price p95 <200ms, stats <500ms, risk <1500ms, errors <1%) plus its work_score.
   - **Honesty section:** anything unmeasured, noisy (same-machine k6), or unresolved.

3. **Write the video-pitch outline** (append to WRITEUP.md or a separate section):
   bullets covering architecture (30s), the 2-3 trade-offs chosen and what was given up,
   and the results — **one number per claim**, each traceable to a history.jsonl line.

4. **If the evidence is thin**, do NOT pad. List exactly which bench runs or experiment
   entries are missing (e.g. "no before/after for the worker-pool change; re-run /bench
   at SHA X") so the user can fill the gaps.
