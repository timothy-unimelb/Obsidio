---
name: smoke
description: Correctness gate for the four Obsidio endpoints — verifies exact JSON shapes, the 500-point stats math (population stddev), and the full 50k SHA-256 /risk chain against independent references. Use this whenever the user says "smoke test", "verify correctness", "check the endpoints", or after ANY change to endpoint logic (a wrong /risk digest scores zero).
---

# smoke: correctness gate

A fast build with a wrong answer scores ZERO. Run this after every endpoint-logic change.

1. **Ensure the container is up.** If `curl -fsS http://127.0.0.1:8080/health` fails,
   start it via the run-capped skill's steps first.

2. **Run the checker** (stdlib-only Python, prints PASS/FAIL per item, exit 1 on any
   failure):
   ```bash
   python3 .claude/skills/smoke/scripts/smoke.py
   ```
   Optionally pass a base URL: `python3 .claude/skills/smoke/scripts/smoke.py http://127.0.0.1:8080`

3. **What it checks** (all against independent in-script references):
   - `/health` → 200 `{"status":"ok"}`
   - `/price?symbol=AAPL` → exact `{"symbol":"AAPL","price":187.42}`;
     `/price?symbol=FAKE` → 404 exact `{"error":"unknown symbol"}`
   - `/stats` for 2 symbols vs a regenerated reference series
     (`base*(1+sin(i)/50)`, i in 0..499): mean/min/max/POPULATION stddev within 1e-6
     relative tolerance
   - `/risk` for seeds `0.48`, `abc`, and the no-param default (`"none"`) vs the full
     50,000-iteration hex-chain reference — a digest mismatch is a SCORE-ZERO bug and is
     reported loudly
   - `Content-Type: application/json` on every response

4. **Framework-default traps.** If the 404 body arrives as `{"detail": ...}` (FastAPI's
   HTTPException default) or as a framework error page (Spring), that is a REAL contract
   violation even though the app "works" — the graded shape is `{"error":"unknown symbol"}`.
   The script demands the graded shape and flags this class of mismatch explicitly. When
   a check fails, verify by hand whether it's an app bug or a script bug before "fixing"
   either.

5. **Report** the PASS/FAIL table to the user. Any FAIL blocks benching — correctness
   first, speed second.
