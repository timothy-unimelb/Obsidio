# Benchmarking Protocol

How to measure an Obsidio backend so the numbers mean something. Follow this
exactly; the whole point is that everyone's runs are comparable.

## Why a protocol

We measured our noise floor on 2026-08-21 by running **identical code twice**
under identical flags on one machine:

| Metric | Run 1 | Run 2 | Swing |
| --- | ---: | ---: | ---: |
| work_score | 1,273,640 | 1,148,230 | ~10% |
| /risk p95 | 321 ms | 836 ms | ~2.6× |

Nothing changed between those runs except machine conditions (thermals,
background load). The lesson: **a single run cannot distinguish a ~10%
improvement from luck, and tail latencies (p95) are far noisier than
throughput.** `/risk` runs near CPU saturation by design, so its p95 is almost
entirely queue wait — and queue wait explodes when a few percent of CPU goes
missing. Small latency swings are weather, not architecture.

Also: **numbers from different machines are never comparable.** The same
submission scored 3.2M on one laptop and 1.0M on another. Only compare runs
from the same machine, same session, same flags.

## Pick the right tier

Match the measurement cost to the question. Most iteration should live in the
cheap tiers; the expensive ones exist to *confirm*, not to explore.

| Tier | Tool | Time | Use for |
| --- | --- | --- | --- |
| 0 — kernel microbench | `go test -bench=RiskChain` (in `app/`) | seconds | /risk kernel changes; ~1–2% noise |
| 1 — dev loop | `k6 run k6/devloop.js` | ~2 min | "did that help?" iteration; directional |
| 2 — full grading run | `k6 run k6/grading.js` | ~6 min | anything you record or claim |
| 3 — interleaved A/B | 3× each, alternating | ~45 min | close calls (10–30% gaps) and final write-up numbers |

**Tier 0** (no Go on the host — run it in the builder image):

```bash
docker run --rm -v "$PWD/app":/src -w /src golang:1.26-bookworm go test -bench=RiskChain -benchtime=10x -count=3
```

`app/main_test.go` also contains `TestRiskChainMatchesReference`, which checks
the optimised kernel against the naive reference algorithm — run it after any
kernel change; a wrong digest scores zero.

**Tier 1** is `k6/devloop.js`: the identical request mix and work_score metric
as grading.js, but ~75 seconds — it jumps straight to the 200-VU hold instead
of the 3-minute ramp, because the steady-state hold is where the signal is.
Measured dev-loop noise on identical code: **~3% work_score run-to-run**
(tighter than the full script's ~10%, since the short all-peak window leaves
less room for thermal drift), so it cleanly resolves ~5%+ deltas in 2 minutes.
Three hard rules:

- Dev-loop numbers are only comparable to other dev-loop numbers, and they are
  **never recorded to bench/history.jsonl** — confirm with a Tier-2 run before
  recording or claiming anything.
- Dev-loop **p95s read ~2× higher** than grading.js p95s (it samples only peak
  load; grading.js blends 3 minutes of light ramp into its percentiles). A
  threshold failure in the dev loop means "confirm with grading.js", not "we
  fail the bar". Iterate on work_score.
- Same machine-prep and one-bench-at-a-time rules as every other tier.

## Ground rules

1. **No performance claim without a measured delta** — and for effects under
   ~30%, without the repeat protocol below.
2. **Correctness before speed.** A wrong `/risk` digest scores zero no matter
   how fast it is. Verify endpoints before benching (see step 3).
3. **One bench per machine at a time.** k6 and the container share the CPU;
   two benches corrupt each other. Before touching Docker, run `docker ps` —
   if an `obsidio*` container is running, someone else may be mid-bench: wait
   or ask, never `docker rm -f` it. Port 8080 is the mutex.
4. **Name containers per owner** (`obsidio-<yourname>`) so `docker ps` shows
   whose run it is.
5. **Record every run**, including bad ones. An invalidated run gets noted as
   invalidated, not deleted.

## Machine prep (before any measured run)

- Plugged into power; lid open.
- Close heavy apps (browsers with many tabs, IDEs indexing, video calls).
- No other containers or load tests running (`docker ps`).
- Note anything unusual alongside the run ("machine was hot", "Slack call in
  background") — context makes weird numbers explainable later.

## Single run procedure (Tier 2)

This is one measurement. It takes ~6 minutes (build + 4.5 min load test).
For a Tier-1 dev-loop run, substitute `k6/devloop.js` in step 4 and skip the
recording step.

```bash
# 1. Build the candidate (from its app directory)
docker build -t obsidio-<yourname> <app-dir>
```

```bash
# 2. Run exactly as the grader does, with cores pinned so k6 can't steal them
docker run --rm -d --name obsidio-<yourname> --cpus=2 --cpuset-cpus=0,1 --memory=2g -p 8080:8080 obsidio-<yourname>
```

```bash
# 3. Correctness gate — all four must look right before you burn 4.5 minutes
curl -fsS http://127.0.0.1:8080/health
curl -fsS 'http://127.0.0.1:8080/price?symbol=AAPL'   # {"symbol":"AAPL","price":187.42}
curl -fsS 'http://127.0.0.1:8080/stats?symbol=MSFT'   # mean/min/max/stddev (population, ÷n)
curl -fsS 'http://127.0.0.1:8080/risk?seed=abc'       # deterministic digest — verify once against a reference impl
curl -s -o /dev/null -w '%{http_code}\n' 'http://127.0.0.1:8080/price?symbol=NOPE'  # 404
```

```bash
# 4. The real grading script (k6 exits non-zero if thresholds fail — that's a result, record it)
k6 run -e TARGET=http://127.0.0.1:8080 --summary-export=bench/last-summary.json k6/grading.js
```

```bash
# 5. Tear down
docker rm -f obsidio-<yourname>
```

On the `advait` branch, `python3 .claude/skills/bench/scripts/record.py --note "<what this run measures>"`
parses the summary, appends to `bench/history.jsonl`, and regenerates
RESULTS.md. On other branches, keep the `--summary-export` JSON and note the
four headline numbers: work_score, the three p95s, error rate.

Linux only: pin k6 away from the container's cores with
`taskset -c 2,3 k6 run ...`. On macOS there is no taskset and Docker runs in a
VM — treat same-machine numbers as directional and rely on relative deltas.

## Comparing two implementations (the A/B protocol)

**This is not the default workflow — most of the time, don't do this.**
work_score is stable enough (~10% run-to-run) that a single 6-minute run
reliably resolves any change bigger than ~15%, and most optimisation attempts
either clear that easily or do nothing. Day to day: one run, compare against
history, move on.

Reserve this protocol for the two cases where a single run genuinely can't
answer the question — expect to use it maybe twice in the whole hackathon:

- **Close rankings:** two implementations within ~10–30% of each other, where
  one run is a coin flip.
- **Final write-up numbers:** the once-at-the-end measurement where citing a
  median and range makes the judged claim defensible.

Budget ~45 minutes (or drop to 2 runs each, ~30 minutes, at some confidence
cost).

1. **Warm-up:** one throwaway run (either variant). Discard the numbers — it
   pays the thermal ramp and cache warming so run 1 isn't privileged.
2. **Interleave:** run A, B, A, B, A, B — three measured runs each,
   **alternating**. Never AAA-then-BBB: machine conditions drift over 45
   minutes, and batching gifts the drift to whichever went first.
3. **Same everything:** same machine, same session, same flags, no breaks long
   enough for the machine to cool unevenly.
4. **Compare medians of work_score**, and look at the ranges:
   - Ranges don't overlap (A's worst beats B's best) → **real difference**.
   - Ranges overlap → **indistinguishable at this noise level**. Say that;
     don't pick the median you like.
5. **Ignore p95 for ranking.** Use p95 only to check threshold margin — and
   check it on the *worst* run, not the best: an implementation whose worst
   /risk p95 is 900 ms against the 1500 ms bar is safe; one that passes only
   on its lucky runs is not.

## Interpreting a delta (single run vs. previous)

| Observed work_score change | Read it as |
| --- | --- |
| < 10% | Noise. Not a claim. Repeat protocol if it matters. |
| 10–30% | Suggestive. Needs the A/B protocol before claiming. |
| > 30% | Probably real. One confirming re-run is still cheap insurance. |
| p95 moved but work_score didn't | Weather. Queue-wait tails swing 2–3× on their own. |

## What goes in the write-up

Judged claims cite **medians and ranges over N interleaved runs**, never a
single lucky number. "work_score 1.21M (range 1.15–1.27M, n=3)" is a stronger
sentence than "1.27M" — and it's the honest one. Absolute numbers don't
transfer to the grading hardware anyway; what transfers is the relative delta
and the explanation for it.
