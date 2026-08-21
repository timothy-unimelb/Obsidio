#!/usr/bin/env python3
"""Parse a k6 --summary-export JSON and append one run line to bench/history.jsonl.

Usage:
  python3 record.py --note "what this run measured" \
      [--summary bench/last-summary.json] [--history bench/history.jsonl]

Prints: this run vs the qualifying bars, plus the delta vs the previous history line.
Exit 1 if the summary is missing or unparseable (never appends a junk line).
"""
import argparse
import json
import os
import subprocess
import sys
import time

BARS = {  # placeholder thresholds from k6/grading.js
    "p95_price": 200.0,
    "p95_stats": 500.0,
    "p95_risk": 1500.0,
    "error_rate": 0.01,
}


def git(*args):
    try:
        return subprocess.run(["git", *args], capture_output=True, text=True,
                              timeout=10).stdout.strip()
    except Exception:
        return ""


def get(metrics, name, key):
    m = metrics.get(name)
    if not isinstance(m, dict):
        return None
    v = m.get(key)
    return float(v) if isinstance(v, (int, float)) else None


def fmt(v, unit="ms"):
    if v is None:
        return "  n/a"
    if unit == "ms":
        return f"{v:8.1f}ms"
    if unit == "pct":
        return f"{v * 100:7.3f}%"
    return f"{v:10.0f}"


def delta(cur, prev, lower_is_better=True):
    if cur is None or prev is None:
        return ""
    d = cur - prev
    if abs(d) < 1e-9:
        return "  (no change)"
    better = (d < 0) == lower_is_better
    arrow = "improved" if better else "REGRESSED"
    return f"  ({d:+.1f} vs prev, {arrow})"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--note", required=True, help="one-line description of the run")
    ap.add_argument("--summary", default="bench/last-summary.json")
    ap.add_argument("--history", default="bench/history.jsonl")
    args = ap.parse_args()

    try:
        with open(args.summary) as f:
            data = json.load(f)
    except Exception as e:
        print(f"ERROR: cannot read summary export {args.summary!r}: {e}", file=sys.stderr)
        sys.exit(1)

    metrics = data.get("metrics", {})
    row = {
        "ts": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
        "git_sha": git("rev-parse", "--short", "HEAD") or None,
        "dirty": bool(git("status", "--porcelain")),
        "note": args.note,
        "p95_price": get(metrics, "http_req_duration{tier:price}", "p(95)"),
        "p95_stats": get(metrics, "http_req_duration{tier:stats}", "p(95)"),
        "p95_risk": get(metrics, "http_req_duration{tier:risk}", "p(95)"),
        "error_rate": get(metrics, "http_req_failed", "value"),
        "work_score": get(metrics, "work_score", "count"),
        "reqs_total": get(metrics, "http_reqs", "count"),
    }

    core = ["p95_price", "p95_stats", "p95_risk", "error_rate", "work_score"]
    missing = [k for k in core if row[k] is None]
    if missing:
        print(f"ERROR: summary export lacks expected metrics: {missing}."
              " Was the run made with k6/grading.js and --summary-export?",
              file=sys.stderr)
        sys.exit(1)

    passed = {
        "price": row["p95_price"] < BARS["p95_price"],
        "stats": row["p95_stats"] < BARS["p95_stats"],
        "risk": row["p95_risk"] < BARS["p95_risk"],
        "errors": row["error_rate"] < BARS["error_rate"],
    }
    row["bars_passed"] = sum(passed.values())

    prev = None
    if os.path.exists(args.history):
        with open(args.history) as f:
            lines = [ln for ln in f.read().splitlines() if ln.strip()]
        if lines:
            try:
                prev = json.loads(lines[-1])
            except Exception:
                prev = None

    os.makedirs(os.path.dirname(args.history) or ".", exist_ok=True)
    with open(args.history, "a") as f:
        f.write(json.dumps(row) + "\n")

    def pv(k):
        return prev.get(k) if prev else None

    mark = lambda ok: "PASS" if ok else "FAIL"
    print(f"\nBench run recorded  ({row['ts']}, sha {row['git_sha']}"
          f"{', DIRTY tree' if row['dirty'] else ''})")
    print(f"note: {row['note']}\n")
    print(f"  /price p95   {fmt(row['p95_price'])}   bar <{BARS['p95_price']:.0f}ms"
          f"   [{mark(passed['price'])}]{delta(row['p95_price'], pv('p95_price'))}")
    print(f"  /stats p95   {fmt(row['p95_stats'])}   bar <{BARS['p95_stats']:.0f}ms"
          f"   [{mark(passed['stats'])}]{delta(row['p95_stats'], pv('p95_stats'))}")
    print(f"  /risk  p95   {fmt(row['p95_risk'])}   bar <{BARS['p95_risk']:.0f}ms"
          f"   [{mark(passed['risk'])}]{delta(row['p95_risk'], pv('p95_risk'))}")
    print(f"  error rate  {fmt(row['error_rate'], 'pct')}   bar <1%      "
          f"   [{mark(passed['errors'])}]")
    print(f"  work_score  {fmt(row['work_score'], 'n')}"
          f"{delta(row['work_score'], pv('work_score'), lower_is_better=False)}")
    print(f"  requests    {fmt(row['reqs_total'], 'n')}")
    print(f"  bars passed  {row['bars_passed']}/4"
          + (f"   (prev {prev.get('bars_passed')}/4)" if prev and 'bars_passed' in prev else ""))
    if prev and row["work_score"] < (pv("work_score") or 0):
        print("\n  *** REGRESSION: work_score fell vs the previous run. ***")

    # Keep the human-readable comparison table in sync (best-effort: a render
    # failure must never lose the history line we just appended).
    try:
        import render_results
        sys.argv = ["render_results.py", "--history", args.history]
        render_results.main()
    except Exception as e:
        print(f"WARNING: could not render RESULTS.md: {e}", file=sys.stderr)


if __name__ == "__main__":
    main()
