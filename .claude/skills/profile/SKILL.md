---
name: profile
description: Find the real bottleneck under load — container-level CPU/memory sampling plus a language-appropriate CPU profile while the k6 load runs. Use this whenever the user says "profile it", "where's the bottleneck", "why is price slow", or before believing any theory about performance.
---

# profile: find the real bottleneck under load

Profile before optimising. A theory without a flame graph is a guess.

1. **Start the capped container** (run-capped steps). For py-spy add
   `--cap-add SYS_PTRACE` to the `docker run`.

2. **Launch a moderate background load** while you profile — either the grading script
   (`k6 run -e TARGET=http://127.0.0.1:8080 k6/grading.js`) or a quick one-liner:
   ```bash
   k6 run -e TARGET=http://127.0.0.1:8080 --vus 50 --duration 60s k6/grading.js
   ```

3. **Sample container vitals 5× at ~10s intervals** while the load runs:
   ```bash
   for i in 1 2 3 4 5; do docker stats --no-stream obsidio; sleep 10; done
   ```
   Read two things: is CPU pinned at ~200% (the 2-core cap — fully CPU-bound) or below
   it (blocked on something else)? Is memory growing run-over-run (leak/queue growth)?

4. **Capture a CPU profile** with the tool for the chosen language:
   - **Go**: import `net/http/pprof`, expose it on a side port, then
     `go tool pprof -top http://127.0.0.1:6060/debug/pprof/profile?seconds=30`
   - **Node**: `npx clinic flame -- node server.js` (outside docker for the flame UI), or
     start with `node --cpu-prof server.js` and inspect the generated .cpuprofile in
     Chrome DevTools
   - **Python**: `pip install py-spy`, find the container pid
     (`docker top obsidio` or `pgrep -f uvicorn`), then
     `py-spy record --pid <pid> -o profile.svg` (container must run with
     `--cap-add SYS_PTRACE`)
   - **Java**: async-profiler —
     `java -agentpath:libasyncProfiler.so=start,event=cpu,file=profile.html -jar app.jar`
     or attach with `asprof -d 30 -f profile.html <pid>`

5. **Summarise for the user**: top hotspots by CPU; whether the box is CPU-saturated;
   and — the key question — when /price is slow, where does its time go? If the hotspot
   is sha256 and /price barely appears, /price latency is QUEUEING behind /risk
   (architecture problem: isolation/backpressure), not compute.

6. **Offer to log the findings via /experiment** so the evidence lands in EXPERIMENTS.md.
