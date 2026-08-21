# Obsidio: Resilient Backend — project guide

## What this is

A 3-day hackathon entry for the Obsidio track: a small price-analytics API that must
stay fast and correct while the graders flood it with k6 load inside a Docker container
capped at **2 CPUs / 2 GB RAM**. Every team does identical work per endpoint; the whole
game is serving that work more efficiently and keeping the cheap path fast while the
expensive path burns CPU. The grade is measured, not argued.

**Official sources:** the live directions page is
https://cissa-unimelb.notion.site/Obsidio-Directions-Resources-3c199473577c80968edcc87d91e386c7
(verified 2026-08-21 to match `OBSIDIO-DETAIL-PAGE.md`, the fuller version bundled with
the skeleton — no rule drift; thresholds still placeholders). Skeleton repo:
https://github.com/solpercival/Obsidio. Re-check the Notion page before submission for
the locked threshold numbers.

**Submission checklist:**
- [ ] A working backend as a `Dockerfile` (grader builds and runs it, port 8080)
- [ ] `docker-compose.yml` ONLY if attempting the persistence bonus
- [ ] Resilience write-up with measured k6 numbers (see `/writeup`, backed by EXPERIMENTS.md)
- [ ] Video pitch: architecture + trade-offs, one number per claim

## The contract (verbatim-accurate — do not drift from this)

Grader: `docker build` the Dockerfile, `docker run` capped at 2 CPUs / 2 GB, app listens
on **port 8080**. Traffic mix: **60% /price, 30% /stats, 10% /risk**.

| Endpoint | Weight | Returns (200) | Errors |
| --- | --- | --- | --- |
| `GET /health` | not scored | `{"status":"ok"}` | — |
| `GET /price?symbol=SYM` | 1 | `{"symbol":"AAPL","price":187.42}` | unknown → `404 {"error":"unknown symbol"}` |
| `GET /stats?symbol=SYM` | 3 | `{"symbol":...,"mean":...,"min":...,"max":...,"stddev":...}` over the symbol's 500-point series, computed EVERY request | unknown → `404 {"error":"unknown symbol"}` |
| `GET /risk?seed=VALUE` | 10 | `{"seed":<seed>,"risk_hash":<final digest>}` | — |

**/risk algorithm (exact):** `h = seed`; repeat **50,000** times: `h = hex(sha256(utf8(h)))`;
return the final `h`. Deterministic — the grader can verify the digest against the seed; a
wrong or constant digest scores **ZERO**. Uncacheable (seed differs per request). Missing
seed defaults to `"none"` in the starters.

**/stats:** mean, min, max, and **POPULATION stddev** (variance divided by n, not n−1 —
verified in all four starters).

**Data (generated in code, identical across starters):** 8 symbols:
AAPL 187.42, GOOG 141.80, MSFT 412.30, AMZN 178.10, NVDA 120.15, META 502.60,
TSLA 244.70, JPM 198.35. Series: `series[i] = base * (1 + sin(i)/50)` for i in 0..499.

**Load schedule** (`k6/grading.js` — the REAL grading script, no hidden version):
ramp 1m→50 VUs, 2m→100, 1m hold at 200, 30s→0 (4.5 minutes total).

**Thresholds** (placeholders, finalised on grading hardware — so optimise `work_score`,
not just bar-clearance):
- `/price` p95 < 200 ms
- `/stats` p95 < 500 ms
- `/risk` p95 < 1500 ms
- `http_req_failed` rate < 1%

**Leaderboard:** `work_score = (1 × price) + (3 × stats) + (10 × risk)`, counting only
200-status responses. Late/errored/timed-out requests score zero.

**Optional bonus:** `POST /price {"symbol","price"}` with storage that survives a
container restart, via docker-compose sharing the SAME 2 CPU / 2 GB budget. Only worth it
if all latency bars still pass afterward.

**Bonus clarifications (per organizer, 2026-08-21):**
- Bonus POSTs are **0% of the graded load mix** — evaluated separately from the
  throughput siege with a small fixed set of writes, not a proportion of traffic.
- Scoring is **pass/fail durability**, not volume: grader writes a fixed set of values,
  restarts the container, reads them back. All survive + all latency thresholds still
  pass → bonus. Number of writes handled doesn't factor in.
- Restart is a **hard kill (`docker kill`)**, not a graceful compose restart. Data must
  be durably written (fsync'd) at the moment the POST is accepted — a shutdown-time
  flush does NOT count. Design as if power is lost right after the 200 is sent.

**Grading hardware (per Competitions Director, Discord 2026-08-21 — not finalized):**
- Assume ONLY: 2.0 CPUs (cgroup-enforced), 2 GB RAM, x86-64 Linux. Do NOT assume a
  CPU model, cache size, or instruction-set extension (AVX-512, SHA-NI, AES-NI) —
  any may change; code that hard-depends on them may not carry over. Runtime feature
  detection (e.g. Go's crypto/sha256) is fine; hard-coded ISA paths are not.
- It will be a cloud VM with recent hardware or a powerful home-lab workstation —
  not ancient silicon — but treat that as a floor, not a target.
- `nproc`/`lscpu` inside the container report HOST CPUs, not the cap. Read the real
  budget from the cgroup: `cat /sys/fs/cgroup/cpu.max` (`200000 100000` = 2.0 CPUs)
  and `cat /sys/fs/cgroup/memory.max`. Size pools/threads against the 2-CPU cap.
- Organizer's stated priorities: right-sized concurrency, unblocked fast path,
  bounded queues, no memory leaks — these transfer to any machine.

**Container-privilege rules (per Competitions Director, Discord 2026-08-21):**
- NO `privileged: true` in docker-compose.yml (called a security hole).
- NO added capabilities (`cap_add`) — could exceed the standard Docker resource
  environment.
- NO CPU core binding/pinning (`cpuset`) in the submission — grading runs on
  different machines, so core-specific optimisation is disallowed and pointless.
- Local-only impact: our `/bench` harness uses `--cpuset-cpus=0,1` on the
  `docker run` line purely for same-machine measurement isolation. That is our
  local harness, not the submitted Dockerfile/compose, so it stays — but never
  let cpuset/cap_add/privileged leak into `app/Dockerfile` or any submitted
  docker-compose.yml.

## Commands

```bash
# Build + run exactly as the grader does (or just use /run-capped)
docker build -t obsidio <app-dir>
docker run --rm -d --name obsidio --cpus=2 --memory=2g -p 8080:8080 obsidio
curl -fsS http://127.0.0.1:8080/health

# The grading load test (also what /bench runs; full run ≈ 4.5 min)
k6 run -e TARGET=http://127.0.0.1:8080 --summary-export=bench/last-summary.json k6/grading.js
```

Skills: `/run-capped` (build+run under caps), `/smoke` (correctness gate),
`/bench` (measured load-test run, appended to bench/history.jsonl), `/profile`
(find the bottleneck), `/experiment` (log an attempt), `/writeup` (draft deliverables),
`/commit-push` (secret-scanned commit+push), `/handoff` (end session), `/status`
(start session).

## Chosen starter

**Go — decided 2026-08-21. The app dir is `app/` (main.go + Dockerfile).**
Why Go: the /risk hash chain gates the whole closed loop, and Go's sha256 kernel is
~5× cheaper per request than Node/Python's; goroutines + a semaphore give precise
control over how much CPU the heavy path may occupy. The engineered app pins
GOMAXPROCS=2, bounds /risk to 2 concurrent chains (waiters park cheaply), runs a
zero-allocation hash loop, and serializes without encoding/json reflection.
`starters/go` remains untouched as the naive baseline for comparison.

## Critical gotchas

- **CORE-COUNT TRAP:** the container is CPU-throttled to 2 but SEES all host cores.
  Go reads host cores for GOMAXPROCS; uvicorn/gunicorn workers, Node cluster, and the
  JVM auto-size the same way. Always pin worker/thread counts explicitly to the 2-core
  cap (`GOMAXPROCS=2`, `--workers 2`, `-XX:ActiveProcessorCount=2`, etc.).
- **Never benchmark or debug performance outside the capped container** — behaviour
  under the caps is different. Uncapped numbers are fiction.
- **The fast path is the product:** if `/price` p95 climbs under load it is queueing
  behind `/risk` (fix the architecture: worker isolation, backpressure) — it is not slow
  compute.
- **Late/errored responses score zero** → shed overflow fast; never let requests rot in
  a slow queue. But `http_req_failed` must stay < 1%, so the error budget for shedding
  is tiny — size the system so overflow is rare.
- **Population stddev (÷n), not sample (÷(n−1)).**
- **Pin all image versions; no `latest`** — the grader rebuilds on its own machine and
  it must build identically.
- **Local k6 + container on one machine compete for CPU** → noisy numbers. Isolate with
  `--cpuset-cpus` (see `/bench`) and treat same-machine runs as directional.
- **Known starter deviations from the contract** (verified by code reading; Node starter
  behaviour execution-verified via /smoke):
  - Python/FastAPI: 404 renders `{"detail":"unknown symbol"}`, NOT the contract's
    `{"error":"unknown symbol"}` (FastAPI's HTTPException default shape). Must be fixed
    if the Python starter is chosen. Same class of bug: Java/Spring's
    ResponseStatusException renders Spring's default error body, not the contract shape.
  - Python/FastAPI: `/risk?seed=` (present but empty) returns `seed:""`; Node and Go
    coerce empty to `"none"`. Only the missing-param case is contract-specified.

## Workflow rules

- **Never claim a performance change helped without a `/bench` delta.** No numbers, no
  claim.
- **Run `/smoke` after any change to endpoint logic** — a correctness bug in /risk
  scores zero no matter how fast it is.
- **Log every optimisation attempt via `/experiment`** — EXPERIMENTS.md is the evidence
  base for the judged write-up. Keep it current: it doubles as the record of the
  improvement journey, so failures and reverts get entries too, and stale verdicts
  (e.g. a "kept" that later gets reverted) must be corrected when reality changes.
- **Start each session with `/status`. End each session with `/handoff`.**

## Planning workflow

For multi-step tasks: plan in plan mode first. On approval, write the plan to `PLAN.md`
(repo root). PLAN.md must be self-contained (no reliance on chat context) and end with a
`## Progress` checklist of `- [ ]` steps, each executable with zero prior context. The
user then runs `/clear`; on resume, read PLAN.md and execute the next unchecked step,
checking off steps as they complete. Archive finished plans to `archived-plans/`.
