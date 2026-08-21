---
name: bench
description: The measurement loop — rebuild the capped container with pinned cores, run the real k6 grading script, and append a parsed result line (p95s, error rate, work_score) to bench/history.jsonl with a delta vs the previous run. Use this whenever the user says "bench", "run the load test", "did that help", "measure it", or "get a baseline". Never claim a perf change helped without this.
---

# bench: the measurement loop

One full run ≈ **4.5 minutes** (1m→50 VUs, 2m→100, 1m@200, 30s→0). Say so before
starting. No bench delta = no performance claim.

1. **Get a note for the run.** Every history line carries a one-line description of what
   is being measured. Ask the user, or infer it from the recent diff (e.g. "baseline
   node starter", "worker_threads pool of 2"). Also note if the working tree is dirty.

2. **Rebuild + restart the capped container, cores pinned** so k6 doesn't steal the
   app's CPUs. Same as run-capped but with `--cpuset-cpus`:
   ```bash
   docker rm -f obsidio 2>/dev/null || true
   docker build -t obsidio <app-dir>
   docker run --rm -d --name obsidio --cpus=2 --cpuset-cpus=0,1 --memory=2g -p 8080:8080 obsidio
   ```
   Wait for `/health`. Where the OS supports it, pin k6 to OTHER cores (Linux:
   `taskset -c 2,3 k6 run ...`). On macOS there is no taskset and Docker runs in a VM —
   just note that same-machine numbers are noisy/directional and rely on relative deltas.

3. **Run the grading script** (this IS the grader — no hidden version):
   ```bash
   k6 run -e TARGET=http://127.0.0.1:8080 --summary-export=bench/last-summary.json k6/grading.js
   ```
   If k6 is missing: `brew install k6` (macOS) or see
   https://k6.io/docs/get-started/installation. Note: k6 exits non-zero when thresholds
   fail — that is a result, not an error; still record it.

4. **Record the run** — parses last-summary.json, appends ONE line to
   bench/history.jsonl, and prints the bars/delta table:
   ```bash
   python3 .claude/skills/bench/scripts/record.py --note "<the note from step 1>"
   ```
   The line contains: ts, git_sha, dirty, note, p95_price, p95_stats, p95_risk,
   error_rate, work_score, reqs_total, bars_passed. Tier p95s come from the
   `http_req_duration{tier:...}` submetrics; work_score from the Counter.

5. **Report** the table record.py printed: this run vs the bars, and the delta vs the
   previous history line (work_score and each p95). Call out any regression explicitly —
   a higher work_score with a busted p95 bar is NOT a win.
