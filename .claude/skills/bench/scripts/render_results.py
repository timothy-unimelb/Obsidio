#!/usr/bin/env python3
"""Render RESULTS.md from bench/history.jsonl — the at-a-glance run comparison.

One row per bench run: commit (linked to the GitHub repo), the run's note (the
approach being measured), all metrics, and the work_score delta vs the previous
comparable run. Runs whose note contains "uncapped" are shown but excluded from
comparisons and best-run detection (uncapped numbers are fiction — CLAUDE.md).

Usage:
  python3 render_results.py [--history bench/history.jsonl] [--out RESULTS.md]

record.py calls this automatically after appending a run, so RESULTS.md is
always current. Safe to run standalone to regenerate.
"""
import argparse
import json
import re
import subprocess
import sys

BARS_TOTAL = 4


def repo_web_url():
    """https base of origin, or None (renders shas as plain text)."""
    try:
        url = subprocess.run(["git", "remote", "get-url", "origin"],
                             capture_output=True, text=True, timeout=10).stdout.strip()
    except Exception:
        return None
    if not url:
        return None
    url = re.sub(r"\.git$", "", url)
    m = re.match(r"git@([^:]+):(.+)", url)
    if m:
        return f"https://{m.group(1)}/{m.group(2)}"
    if url.startswith(("https://", "http://")):
        return url
    return None


def load_runs(path):
    runs = []
    try:
        with open(path) as f:
            for ln in f:
                ln = ln.strip()
                if not ln:
                    continue
                try:
                    runs.append(json.loads(ln))
                except json.JSONDecodeError:
                    print(f"WARNING: skipping unparseable history line: {ln[:80]}",
                          file=sys.stderr)
    except FileNotFoundError:
        pass
    return runs


def comparable(run):
    return "uncapped" not in (run.get("note") or "").lower()


def fnum(v, dec=1):
    return "—" if v is None else f"{v:,.{dec}f}"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--history", default="bench/history.jsonl")
    ap.add_argument("--out", default="RESULTS.md")
    args = ap.parse_args()

    runs = load_runs(args.history)
    if not runs:
        print(f"no runs in {args.history}; nothing to render", file=sys.stderr)
        sys.exit(0)

    web = repo_web_url()
    scores = [r.get("work_score") for r in runs
              if comparable(r) and r.get("work_score") is not None]
    best = max(scores) if scores else None

    lines = [
        "# Bench results",
        "",
        "_Generated from `bench/history.jsonl` by the `/bench` skill — do not edit by",
        "hand (regenerate: `python3 .claude/skills/bench/scripts/render_results.py`)._",
        "_Full hypothesis → verdict narrative per attempt: [EXPERIMENTS.md](EXPERIMENTS.md)._",
        "",
        "Score = weighted 200s under the 4.5-min k6 grading load"
        " (1×/price + 3×/stats + 10×/risk). All p95 in ms; 4 bars ="
        " price<200 · stats<500 · risk<1500 · errors<1%.",
        "",
        "| # | When | Commit | Approach | work_score | Δ vs prev | p95 price"
        " | p95 stats | p95 risk | Err | Bars |",
        "|---|---|---|---|---|---|---|---|---|---|---|",
    ]

    prev_score = None
    any_dirty = False
    for i, r in enumerate(runs, 1):
        sha = r.get("git_sha") or "—"
        dirty = r.get("dirty")
        any_dirty = any_dirty or bool(dirty)
        commit = f"[`{sha}`]({web}/commit/{sha})" if web and sha != "—" else f"`{sha}`"
        if dirty:
            commit += "\\*"

        ws = r.get("work_score")
        comp = comparable(r)
        if not comp:
            score_cell = f"~~{fnum(ws, 0)}~~"
            delta_cell = "excluded"
        else:
            star = " ★" if best is not None and ws == best else ""
            score_cell = f"**{fnum(ws, 0)}**{star}"
            if prev_score is not None and ws is not None:
                delta_cell = f"{(ws - prev_score) / prev_score:+.0%}"
            else:
                delta_cell = "baseline"
            prev_score = ws

        when = (r.get("ts") or "")[:16].replace("T", " ")
        err = r.get("error_rate")
        err_cell = "—" if err is None else f"{err:.2%}"
        bars = r.get("bars_passed")
        bars_cell = "—" if bars is None else f"{bars}/{BARS_TOTAL}"

        lines.append(
            f"| {i} | {when} | {commit} | {r.get('note', '')} | {score_cell}"
            f" | {delta_cell} | {fnum(r.get('p95_price'))} | {fnum(r.get('p95_stats'))}"
            f" | {fnum(r.get('p95_risk'))} | {err_cell} | {bars_cell} |")

    lines.append("")
    if any_dirty:
        lines.append("\\* dirty tree — run included uncommitted changes on top of"
                     " that commit.")
    lines.append("")
    lines.append("Runs on this machine share CPU between k6 and the container —"
                 " absolute numbers are directional; trust the deltas between"
                 " back-to-back runs.")
    lines.append("")

    with open(args.out, "w") as f:
        f.write("\n".join(lines))
    print(f"rendered {args.out} ({len(runs)} runs)")


if __name__ == "__main__":
    main()
