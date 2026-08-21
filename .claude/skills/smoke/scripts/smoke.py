#!/usr/bin/env python3
"""Obsidio smoke test: correctness gate for the graded endpoint contract.

stdlib only. Usage:  python3 smoke.py [BASE_URL]     (default http://127.0.0.1:8080)
Exit 0 = all PASS, exit 1 = any FAIL.

Every check compares against an INDEPENDENT reference computed in this script,
not against the app's own code.
"""
import hashlib
import json
import math
import sys
import urllib.error
import urllib.request

BASE = sys.argv[1].rstrip("/") if len(sys.argv) > 1 else "http://127.0.0.1:8080"

PRICES = {
    "AAPL": 187.42, "GOOG": 141.80, "MSFT": 412.30, "AMZN": 178.10,
    "NVDA": 120.15, "META": 502.60, "TSLA": 244.70, "JPM": 198.35,
}

failures = []


def report(name, ok, detail=""):
    tag = "PASS" if ok else "FAIL"
    line = f"[{tag}] {name}"
    if detail and not ok:
        line += f"\n       {detail}"
    print(line)
    if not ok:
        failures.append(name)


def fetch(path):
    """Return (status, content_type, parsed_json_or_None, raw_bytes)."""
    url = BASE + path
    try:
        with urllib.request.urlopen(url, timeout=30) as r:
            status, ctype, body = r.status, r.headers.get("Content-Type", ""), r.read()
    except urllib.error.HTTPError as e:
        status, ctype, body = e.code, e.headers.get("Content-Type", ""), e.read()
    except Exception as e:
        return None, "", None, str(e).encode()
    try:
        parsed = json.loads(body)
    except Exception:
        parsed = None
    return status, ctype, parsed, body


def check_content_type(name, ctype):
    report(f"{name}: Content-Type is application/json", "application/json" in (ctype or ""),
           f"got Content-Type: {ctype!r}")


def close(a, b):
    return math.isclose(a, b, rel_tol=1e-6, abs_tol=1e-9)


def risk_chain(seed):
    h = seed
    for _ in range(50000):
        h = hashlib.sha256(h.encode("utf-8")).hexdigest()
    return h


def stats_reference(base):
    series = [base * (1 + math.sin(i) / 50) for i in range(500)]
    n = len(series)
    mean = sum(series) / n
    variance = sum((x - mean) ** 2 for x in series) / n  # POPULATION variance (÷ n)
    return {"mean": mean, "min": min(series), "max": max(series),
            "stddev": math.sqrt(variance)}


def main():
    print(f"Obsidio smoke test against {BASE}\n")

    # --- /health ---
    status, ctype, parsed, raw = fetch("/health")
    report("/health: status 200", status == 200, f"got {status} body={raw[:200]!r}")
    report('/health: body {"status":"ok"}', parsed == {"status": "ok"},
           f"got {parsed!r}")
    check_content_type("/health", ctype)

    # --- /price known symbol ---
    status, ctype, parsed, raw = fetch("/price?symbol=AAPL")
    report("/price AAPL: status 200", status == 200, f"got {status}")
    report('/price AAPL: exact {"symbol":"AAPL","price":187.42}',
           parsed == {"symbol": "AAPL", "price": 187.42}, f"got {parsed!r}")
    check_content_type("/price AAPL", ctype)

    # --- /price unknown symbol ---
    status, ctype, parsed, raw = fetch("/price?symbol=FAKE")
    report("/price FAKE: status 404", status == 404, f"got {status}")
    shape_ok = parsed == {"error": "unknown symbol"}
    detail = f"got {parsed!r}"
    if isinstance(parsed, dict) and "detail" in parsed and not shape_ok:
        detail += ("  <-- framework-default shape (e.g. FastAPI HTTPException renders"
                   ' {"detail": ...}); the graded contract demands {"error":"unknown symbol"}')
    report('/price FAKE: exact {"error":"unknown symbol"}', shape_ok, detail)
    check_content_type("/price FAKE", ctype)

    # --- /stats for two symbols vs regenerated reference ---
    for sym in ("AAPL", "TSLA"):
        ref = stats_reference(PRICES[sym])
        status, ctype, parsed, raw = fetch(f"/stats?symbol={sym}")
        report(f"/stats {sym}: status 200", status == 200, f"got {status}")
        if isinstance(parsed, dict):
            report(f"/stats {sym}: symbol field", parsed.get("symbol") == sym,
                   f"got {parsed.get('symbol')!r}")
            for field in ("mean", "min", "max", "stddev"):
                got = parsed.get(field)
                ok = isinstance(got, (int, float)) and close(got, ref[field])
                extra = ""
                if field == "stddev" and isinstance(got, (int, float)) and not ok:
                    # diagnose the classic ÷(n-1) mistake
                    sample = ref["stddev"] * math.sqrt(500 / 499)
                    if close(got, sample):
                        extra = "  <-- this is SAMPLE stddev (÷(n-1)); contract wants POPULATION (÷n)"
                report(f"/stats {sym}: {field} within 1e-6 rel tol", ok,
                       f"got {got!r}, want {ref[field]!r}{extra}")
        else:
            report(f"/stats {sym}: JSON body", False, f"got {raw[:200]!r}")
        check_content_type(f"/stats {sym}", ctype)

    # --- /risk: full 50k chain verification ---
    risk_cases = [("0.48", "/risk?seed=0.48"), ("abc", "/risk?seed=abc"),
                  ("none", "/risk")]  # no-param default must be "none"
    for expect_seed, path in risk_cases:
        ref = risk_chain(expect_seed)
        status, ctype, parsed, raw = fetch(path)
        label = f"/risk {path.split('?')[-1] if '?' in path else '(no seed param)'}"
        report(f"{label}: status 200", status == 200, f"got {status}")
        if isinstance(parsed, dict):
            report(f"{label}: seed echoed as {expect_seed!r}",
                   parsed.get("seed") == expect_seed, f"got {parsed.get('seed')!r}")
            digest_ok = parsed.get("risk_hash") == ref
            report(f"{label}: 50,000-iteration digest matches reference", digest_ok,
                   f"got {parsed.get('risk_hash')!r}\n       want {ref!r}\n"
                   "       *** SCORE-ZERO BUG: the grader verifies this digest; "
                   "a mismatch means every /risk response is worth nothing. ***")
        else:
            report(f"{label}: JSON body", False, f"got {raw[:200]!r}")
        check_content_type(label, ctype)

    print()
    if failures:
        print(f"RESULT: {len(failures)} FAILURE(S)")
        for f in failures:
            print(f"  - {f}")
        sys.exit(1)
    print("RESULT: all checks passed")
    sys.exit(0)


if __name__ == "__main__":
    main()
