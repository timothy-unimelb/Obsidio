---
name: run-capped
description: Build and run the Obsidio backend in Docker exactly as the grader does (2 CPUs, 2 GB, port 8080), with health polling and log capture on failure. Use this whenever the user says "run it", "start the server", "run under caps", or before any smoke/bench run that needs a live container.
---

# run-capped: build + run exactly as the grader does

Never test the app any other way — the caps change its behaviour.

1. **Cleanup.** Remove any previous container (ignore errors) and confirm nothing else
   holds port 8080:
   ```bash
   docker rm -f obsidio 2>/dev/null || true
   lsof -nP -iTCP:8080 -sTCP:LISTEN
   ```
   If lsof shows a non-docker process on 8080, stop and tell the user what it is.

2. **Pick the app dir.** Detect which Dockerfile is the live one: if the repo root has a
   `Dockerfile`, use the root (the app has moved there). Otherwise use the chosen
   starter directory named in CLAUDE.md's "Chosen starter" section. If neither exists,
   ask the user which `starters/<lang>` dir to use — do not guess.

3. **Build.**
   ```bash
   docker build -t obsidio <app-dir>
   ```

4. **Run under the grader's caps.**
   ```bash
   docker run --rm -d --name obsidio --cpus=2 --memory=2g -p 8080:8080 obsidio
   ```

5. **Poll health for up to ~30s:**
   ```bash
   for i in $(seq 1 30); do curl -fsS http://127.0.0.1:8080/health && break; sleep 1; done
   ```
   Expect `{"status":"ok"}`.

6. **On failure**, print the logs and stop — do not retry blindly:
   ```bash
   docker logs obsidio
   ```

**To stop the container:** `docker stop obsidio` (it was started with `--rm`, so it
removes itself).
