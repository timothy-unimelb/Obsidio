#!/usr/bin/env python3
"""Secret scan over the STAGED diff (git diff --cached). Exit 1 on any finding.

Checks:
  - staged files named .env / *.pem / *.key
  - added lines matching private-key headers, provider key prefixes
    (sk-, ghp_, AIza, ntn_, sb_secret_, AKIA), and SECRET=/TOKEN=-style assignments
"""
import re
import subprocess
import sys

PATTERNS = [
    ("private key header", re.compile(r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----")),
    ("OpenAI/Stripe-style key (sk-)", re.compile(r"\bsk-[A-Za-z0-9_-]{16,}")),
    ("GitHub token (ghp_)", re.compile(r"\bghp_[A-Za-z0-9]{20,}")),
    ("Google API key (AIza)", re.compile(r"\bAIza[A-Za-z0-9_-]{16,}")),
    ("Notion token (ntn_)", re.compile(r"\bntn_[A-Za-z0-9]{16,}")),
    ("Supabase secret (sb_secret_)", re.compile(r"\bsb_secret_[A-Za-z0-9_-]{8,}")),
    ("AWS access key (AKIA)", re.compile(r"\bAKIA[A-Z0-9]{16}\b")),
    ("secret-looking assignment", re.compile(
        r"(?i)\b[A-Z0-9_]*(SECRET|TOKEN|PASSWD|PASSWORD|API_KEY|ACCESS_KEY)[A-Z0-9_]*"
        r"\s*=\s*['\"]?[A-Za-z0-9+/_.\-]{8,}")),
]
BAD_FILES = re.compile(r"(^|/)\.env(\.|$)|(^|/)\.env$|\.pem$|\.key$")


def run(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True).stdout


def main():
    findings = []

    for f in run("diff", "--cached", "--name-only").splitlines():
        if BAD_FILES.search(f):
            findings.append((f, "(filename)", "sensitive file type staged"))

    current_file = "?"
    for line in run("diff", "--cached", "-U0").splitlines():
        if line.startswith("+++ b/"):
            current_file = line[6:]
            continue
        if not line.startswith("+") or line.startswith("+++"):
            continue
        added = line[1:]
        for label, pat in PATTERNS:
            m = pat.search(added)
            if m:
                shown = added.strip()
                if len(shown) > 120:
                    shown = shown[:117] + "..."
                findings.append((current_file, shown, label))

    if findings:
        print("SECRET SCAN: POSSIBLE SECRETS IN STAGED CHANGES — DO NOT COMMIT\n")
        for path, line, label in findings:
            print(f"  [{label}] {path}\n      {line}")
        print(f"\n{len(findings)} finding(s). Unstage and remove them, or explicitly "
              "confirm each is a false positive before committing.")
        sys.exit(1)

    print("secret scan: clean")
    sys.exit(0)


if __name__ == "__main__":
    main()
