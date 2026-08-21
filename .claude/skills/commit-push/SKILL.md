---
name: commit-push
description: Stage, secret-scan (hard gate), commit, and push to the current branch. Use this whenever the user says "commit and push", "ship it", or "push my changes". Never bypasses the secret scan, never force-pushes.
---

# commit-push: gated commit + push

1. **Stage everything:**
   ```bash
   git add -A
   git status --short
   ```

2. **HARD GATE — secret scan the staged diff:**
   ```bash
   python3 .claude/skills/commit-push/scripts/secret-scan.py
   ```
   It scans `git diff --cached` added lines for private-key headers, provider key
   prefixes (sk-, ghp_, AIza, ntn_, sb_secret_, AKIA), `SECRET=`/`TOKEN=`-style
   assignments, and any staged `.env`/`.pem`/`.key` file.

   **Exit 1 → STOP.** Show the findings to the user, unstage
   (`git reset <flagged files>`), and never commit or push over an unacknowledged hit.
   Only proceed past a finding if the user explicitly confirms it is a false positive.

3. **Commit** with a concise message matching the recent style (`git log --oneline -5`).

4. **Push to the current branch:**
   ```bash
   git push
   ```
   If no upstream is set: `git push -u origin <branch>`.
   On a non-fast-forward rejection: **stop and report** — never force-push; let the user
   decide how to reconcile.
