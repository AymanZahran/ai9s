#!/usr/bin/env python3
"""Fail when a tracked file contains a token or a machine-specific home path.

The printed finding is the path, line number, and label. The matched text stays
in the file.
"""

from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

# The home path is concatenated so this file does not contain it.
HOME_PATH = "/" + "Users/" + "aymanzahran"

PATTERNS = (
    ("github token", re.compile(r"(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}")),
    ("github fine-grained token", re.compile(r"github_pat_[A-Za-z0-9_]{22,}")),
    ("aws access key", re.compile(r"AKIA[0-9A-Z]{16}")),
    ("private key", re.compile(r"-----BEGIN (?:RSA |OPENSSH |EC |DSA )?PRIVATE KEY-----")),
    ("api key", re.compile(r"(?<![A-Za-z0-9])sk-[A-Za-z0-9]{20,}")),
    ("slack token", re.compile(r"xox[baprs]-[A-Za-z0-9-]{10,}")),
    ("google api key", re.compile(r"AIza[0-9A-Za-z\-_]{35}")),
    ("home path", re.compile(re.escape(HOME_PATH))),
)


def findings(text: str) -> list[tuple[str, int]]:
    found = []
    for lineno, line in enumerate(text.splitlines(), 1):
        for label, pattern in PATTERNS:
            if pattern.search(line):
                found.append((label, lineno))
    return found


def main() -> int:
    listed = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if listed.returncode != 0:
        print(listed.stderr.decode().strip() or "git ls-files failed", file=sys.stderr)
        return 1
    failed = False
    for raw in listed.stdout.split(b"\0"):
        if not raw:
            continue
        rel = raw.decode()
        path = ROOT / rel
        if not path.is_file():
            continue
        data = path.read_bytes()
        if b"\0" in data[:8192]:
            continue
        for label, lineno in findings(data.decode("utf-8", errors="replace")):
            print(f"{rel}:{lineno}: {label}", file=sys.stderr)
            failed = True
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
