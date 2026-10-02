#!/usr/bin/env python3
"""Tag an ai9s release. GoReleaser publishes the GitHub Release.

Version, commit, and date are link-time values (`-ldflags -X`), not a constant
in the source. Feature changes still land through a pull request. Cutting a
release does not. From a clean main that matches origin, after checks on that
commit have passed:

    make release

That reads the latest vX.Y.Z tag, bumps it, and pushes an annotated tag. The
tag push runs GoReleaser. The Homebrew formula is then committed straight to
the tap's main. The formula keeps its private-archive download strategy and
passes `-X` so `ai9s version` prints the tag.

    make release VERSION=1.2.3
    make release PART=minor
    make release INSTALL=1
    make release DRY=1

Notes under "## Unreleased" are moved into the changelog and pushed to main
before the tag, with no pull request. Empty Unreleased is fine: the GitHub
Release notes come from the commits. The formula is not a pull request either.
"""

from __future__ import annotations

import hashlib
import os
import re
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CHANGELOG = ROOT / "CHANGELOG.md"
ARCHIVE = "https://github.com/AymanZahran/ai9s/archive/refs/tags/v{version}.tar.gz"
CHANGELOG_RE = re.compile(
    r"\A# Changelog\n\n## Unreleased\n(?P<body>.*?)(?=\n## |\Z)",
    re.S,
)
FORMULA_URL_RE = re.compile(
    r'url "https://github.com/AymanZahran/ai9s/archive/refs/tags/v\d+\.\d+\.\d+\.tar\.gz", using: Ai9sDownloadStrategy'
)
FORMULA_SHA_RE = re.compile(r'sha256 "[0-9a-f]{64}"')
FORMULA_BUILD_RE = re.compile(
    r'(?m)^    system "go", "build", \*std_go_args(?:\([^)]*\))?, "[^"]+"$'
)
ORIGIN_RE = re.compile(r"github\.com[:/](?P<owner>[^/]+)/(?P<name>[^/.]+)")
TAG_RE = re.compile(r"^v(\d+\.\d+\.\d+)$")
PASS_BUCKETS = {"pass", "skipping"}
KNOWN_BUCKETS = PASS_BUCKETS | {"fail", "pending", "cancel"}


class ReleaseError(Exception):
    pass


def die(message: str) -> None:
    raise ReleaseError(message)


def bump_version(current: str, spec: str) -> str:
    parts = current.split(".")
    if len(parts) != 3 or not all(part.isdigit() for part in parts):
        die(f"current version {current} is not X.Y.Z")
    major, minor, patch = (int(part) for part in parts)
    spec = spec or "patch"
    if spec == "patch":
        return f"{major}.{minor}.{patch + 1}"
    if spec == "minor":
        return f"{major}.{minor + 1}.0"
    if spec == "major":
        return f"{major + 1}.0.0"
    if not re.fullmatch(r"\d+\.\d+\.\d+", spec):
        die("pass patch, minor, major, or X.Y.Z")
    nxt = tuple(int(part) for part in spec.split("."))
    if nxt <= (major, minor, patch):
        die(f"{spec} is not newer than {current}")
    return spec


def splice_changelog(text: str, version: str) -> str:
    match = CHANGELOG_RE.search(text)
    if match is None:
        die("CHANGELOG.md must start with ## Unreleased")
    notes = match.group("body").strip()
    if not re.search(r"(?m)^- ", notes):
        die("write the release notes under ## Unreleased in CHANGELOG.md")
    if f"## {version}\n" in text:
        die(f"CHANGELOG.md already has ## {version}")
    tail = text[match.end() :]
    return f"# Changelog\n\n## Unreleased\n\n## {version}\n\n{notes}\n{tail}"


def changelog_notes(text: str, version: str) -> str:
    match = re.search(
        rf"(?m)^## {re.escape(version)}\n+(?P<body>.*?)(?=\n## |\Z)",
        text,
        re.S,
    )
    if match is None:
        die(f"CHANGELOG.md has no ## {version} section")
    notes = match.group("body").strip()
    if not notes:
        die(f"CHANGELOG.md section {version} is empty")
    return notes + "\n"


def unreleased_notes(text: str) -> str:
    match = CHANGELOG_RE.search(text)
    if match is None:
        die("CHANGELOG.md must start with ## Unreleased")
    notes = match.group("body").strip()
    if not re.search(r"(?m)^- ", notes):
        return ""
    return notes + "\n"


def formula_build_line() -> str:
    ldflags = "-s -w -X github.com/AymanZahran/ai9s/cmd.version=#{version}"
    return f'    system "go", "build", *std_go_args(ldflags: "{ldflags}"), "."'


def update_formula(text: str, version: str, sha256: str) -> str:
    if not re.fullmatch(r"[0-9a-f]{64}", sha256):
        die("sha256 must be 64 hex characters")
    url = f'url "{ARCHIVE.format(version=version)}", using: Ai9sDownloadStrategy'
    updated, count = FORMULA_URL_RE.subn(url, text, count=1)
    if count != 1:
        die("formula is missing the ai9s tag archive url")
    updated, count = FORMULA_SHA_RE.subn(f'sha256 "{sha256}"', updated, count=1)
    if count != 1:
        die("formula is missing its sha256")
    updated, count = FORMULA_BUILD_RE.subn(formula_build_line(), updated, count=1)
    if count != 1:
        die("formula is missing its go build line")
    if updated == text:
        die("formula is already at this archive")
    return updated


def origin_slug(url: str) -> str:
    match = ORIGIN_RE.search(url.strip())
    if match is None:
        die(f"origin is not a GitHub repository: {url}")
    return f"{match.group('owner')}/{match.group('name')}"


def checks_decision(checks: list[dict]) -> str:
    """Return pass, fail, or wait.

    A tag is pushed only when at least one check passed and none failed or
    are still running. An empty report waits, so a missing Actions run cannot
    publish.
    """
    if not checks:
        return "wait"
    buckets = []
    for item in checks:
        bucket = item.get("bucket")
        if bucket not in KNOWN_BUCKETS:
            return "fail"
        buckets.append(bucket)
    if "fail" in buckets or "cancel" in buckets:
        return "fail"
    if "pending" in buckets or "pass" not in buckets:
        return "wait"
    return "pass"


def format_checks(checks: list[dict]) -> str:
    parts = []
    for item in checks:
        if item.get("bucket") == "pass":
            continue
        name = str(item.get("name") or "check")
        state = str(item.get("state") or item.get("bucket") or "unknown")
        parts.append(f"{name} {state}")
    return ", ".join(parts) or "checks failed"


def wait_for_checks(fetch, sleep, now, empty_grace_s: int = 180, timeout_s: int = 1200, interval_s: int = 15) -> None:
    started = now()
    deadline = started + timeout_s
    seen = False
    while True:
        checks = fetch()
        if checks:
            seen = True
        decision = checks_decision(checks)
        if decision == "pass":
            return
        if decision == "fail":
            die(f"checks failed: {format_checks(checks)}")
        if not seen and now() - started >= empty_grace_s:
            die("GitHub reported no checks")
        if now() >= deadline:
            die("checks did not finish")
        sleep(interval_s)


def run(args: list[str], cwd: Path | None = None, env: dict[str, str] | None = None) -> str:
    proc = subprocess.run(
        args,
        cwd=cwd,
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0:
        detail = "\n".join(part.strip() for part in (proc.stdout, proc.stderr) if part.strip())
        die(f"{' '.join(args)} failed:\n{detail[-4000:]}")
    return proc.stdout


def git(*args: str, cwd: Path = ROOT) -> str:
    return run(["git", *args], cwd=cwd)


def parse_args(argv: list[str]) -> tuple[str, bool, bool, bool]:
    spec = ""
    dry = False
    install = False
    skip_formula = False
    for arg in argv:
        if arg == "--dry-run":
            dry = True
        elif arg == "--install":
            install = True
        elif arg == "--skip-formula":
            skip_formula = True
        elif arg.startswith("-"):
            die(f"unknown flag {arg}")
        elif spec:
            die("pass one of patch, minor, major, or X.Y.Z")
        else:
            spec = arg
    return spec or "patch", dry, install, skip_formula


def tap_dir() -> Path:
    raw = os.environ.get("AI9S_TAP", "").strip()
    path = Path(raw) if raw else ROOT.parent / "homebrew-ai9s"
    formula = path / "Formula" / "ai9s.rb"
    if not formula.is_file():
        die(f"Homebrew formula not found at {formula}. Set AI9S_TAP to the tap checkout.")
    return path


def require_ready_checkout(cwd: Path) -> None:
    label = cwd.name
    branch = git("rev-parse", "--abbrev-ref", "HEAD", cwd=cwd).strip()
    if branch != "main":
        die(f"{label} is on {branch}, not main")
    dirty = git("status", "--porcelain", cwd=cwd).strip()
    if dirty:
        die(f"{label} has uncommitted changes")
    git("fetch", "origin", "main", cwd=cwd)
    counts = git("rev-list", "--left-right", "--count", "origin/main...HEAD", cwd=cwd).split()
    if len(counts) != 2:
        die(f"could not compare {label} with origin")
    behind, ahead = int(counts[0]), int(counts[1])
    if behind:
        die(f"{label} is behind origin")
    if ahead:
        die(f"{label} has unpushed commits. Open a pull request for them first.")


def latest_version() -> str | None:
    raw = git("tag", "--list", "v[0-9]*.[0-9]*.[0-9]*")
    found = []
    for line in raw.splitlines():
        match = TAG_RE.fullmatch(line.strip())
        if match:
            found.append(tuple(int(part) for part in match.group(1).split(".")))
    if not found:
        return None
    major, minor, patch = max(found)
    return f"{major}.{minor}.{patch}"


def next_version(current: str | None, spec: str) -> str:
    if not current:
        current = "0.0.0"
    return bump_version(current, spec)


def change_log_file(version: str) -> Path:
    return ROOT / "change_logs" / f"release_v{version}.md"


def change_log_text(version: str, notes: str) -> str:
    return f"# Release v{version}\n\n{notes.strip()}\n"


def require_current_tag(current: str) -> None:
    tag = f"v{current}"
    git("rev-parse", "-q", "--verify", f"refs/tags/{tag}")
    remote = git("ls-remote", "--tags", "origin", f"refs/tags/{tag}")
    if tag not in remote:
        die(f"{tag} is not on origin")


def require_new_tag(version: str) -> None:
    tag = f"v{version}"
    local = subprocess.run(
        ["git", "rev-parse", "-q", "--verify", f"refs/tags/{tag}"],
        cwd=ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )
    if local.returncode == 0:
        die(f"{tag} already exists")
    remote = git("ls-remote", "--tags", "origin", f"refs/tags/{tag}").strip()
    if remote:
        die(f"{tag} already exists on origin")


def child_env() -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONDONTWRITEBYTECODE"] = "1"
    return env


def commit_and_push(cwd: Path, paths: list[str], message: str) -> str:
    if git("rev-parse", "--abbrev-ref", "HEAD", cwd=cwd).strip() != "main":
        die(f"{cwd.name} is not on main")
    git("add", *paths, cwd=cwd)
    staged = set(git("diff", "--cached", "--name-only", cwd=cwd).split())
    if staged != set(paths):
        die("commit would include unexpected files: " + " ".join(sorted(staged)))
    git("commit", "-m", message, cwd=cwd)
    git("push", "origin", "main", cwd=cwd)
    return git("rev-parse", "HEAD", cwd=cwd).strip()


def check_bucket(status: str, conclusion: str) -> tuple[str, str]:
    if status != "completed":
        return "pending", status or "pending"
    if conclusion == "success":
        return "pass", conclusion
    if conclusion in {"neutral", "skipped"}:
        return "skipping", conclusion
    if conclusion == "cancelled":
        return "cancel", conclusion
    return "fail", conclusion or status or "fail"


def fetch_commit_checks(repo: str, sha: str) -> list[dict]:
    proc = subprocess.run(
        [
            "gh",
            "api",
            "--paginate",
            f"repos/{repo}/commits/{sha}/check-runs",
            "--jq",
            ".check_runs[] | [.name, .status, .conclusion] | @tsv",
        ],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if proc.returncode != 0:
        detail = proc.stderr.strip() or proc.stdout.strip()
        die(f"could not read check status: {detail[-1000:]}")
    checks = []
    for line in proc.stdout.splitlines():
        parts = line.split("\t")
        if len(parts) != 3:
            continue
        name, status, conclusion = parts
        bucket, state = check_bucket(status, conclusion)
        checks.append({"name": name, "bucket": bucket, "state": state})
    return checks


def wait_for_commit(repo: str, sha: str) -> None:
    wait_for_checks(lambda: fetch_commit_checks(repo, sha), time.sleep, time.time)


def run_tests() -> None:
    env = child_env()
    run(["go", "test", "-count=1", "-timeout", "180s", "./..."], cwd=ROOT, env=env)
    run(
        [
            "python3",
            "-m",
            "unittest",
            "scripts/release_test.py",
            "scripts/audit_test.py",
            "scripts/public_github_test.py",
        ],
        cwd=ROOT,
        env=env,
    )
    run(["python3", "scripts/audit.py"], cwd=ROOT, env=env)


def archive_sha(version: str) -> str:
    token = run(["gh", "auth", "token"]).strip()
    if not token:
        die("gh auth token is empty")
    url = ARCHIVE.format(version=version)
    request = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    fd, name = tempfile.mkstemp(prefix=f"ai9s-v{version}-", suffix=".tar.gz")
    os.close(fd)
    path = Path(name)
    try:
        with urllib.request.urlopen(request) as response, path.open("wb") as out:
            while True:
                chunk = response.read(1024 * 1024)
                if not chunk:
                    break
                out.write(chunk)
        data = path.read_bytes()
        if not data.startswith(b"\x1f\x8b"):
            die("tag archive is not gzip")
        with tarfile.open(path, "r:gz") as bundle:
            names = bundle.getnames()
        if not any(item.endswith("/go.mod") for item in names):
            die("tag archive is missing go.mod")
        return hashlib.sha256(data).hexdigest()
    finally:
        path.unlink(missing_ok=True)


def publish_formula(version: str, sha256: str, tap: Path) -> None:
    formula = tap / "Formula" / "ai9s.rb"
    formula.write_text(update_formula(formula.read_text(), version, sha256))
    message = f"Point the formula at ai9s {version}."
    commit_and_push(tap, ["Formula/ai9s.rb"], message)
    print(f"formula {version} {sha256}")


def reinstall(token: str) -> None:
    clone = run(["brew", "--repository", "aymanzahran/ai9s"]).strip()
    git("pull", "--ff-only", cwd=Path(clone))
    env = os.environ.copy()
    env["HOMEBREW_NO_AUTO_UPDATE"] = "1"
    env["HOMEBREW_GITHUB_API_TOKEN"] = token
    run(["brew", "reinstall", "ai9s"], env=env)


def main(argv: list[str] | None = None) -> int:
    spec, dry, install, skip_formula = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        require_ready_checkout(ROOT)
        tap = None
        if not skip_formula:
            tap = tap_dir()
            require_ready_checkout(tap)
        current = latest_version()
        if current is not None:
            require_current_tag(current)
        version = next_version(current, spec)
        require_new_tag(version)
        original = CHANGELOG.read_text()
        notes = unreleased_notes(original)
        changelog = splice_changelog(original, version) if notes else original
        print(f"release {version}")
        if notes:
            print(notes, end="" if notes.endswith("\n") else "\n")
        else:
            print("release notes come from the commits")
        print(f"tag v{version}")
        if tap is not None:
            print("formula commit on the tap main")
        if dry:
            print("dry run")
            return 0
        run_tests()
        repo = origin_slug(git("remote", "get-url", "origin").strip())
        if notes:
            CHANGELOG.write_text(changelog)
            log_path = change_log_file(version)
            log_path.parent.mkdir(parents=True, exist_ok=True)
            log_path.write_text(change_log_text(version, notes))
            log_rel = log_path.relative_to(ROOT).as_posix()
            try:
                sha = commit_and_push(ROOT, ["CHANGELOG.md", log_rel], f"Release {version}.")
            except ReleaseError:
                subprocess.run(
                    ["git", "checkout", "--", "CHANGELOG.md", log_rel],
                    cwd=ROOT,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                )
                raise
            wait_for_commit(repo, sha)
        else:
            wait_for_commit(repo, git("rev-parse", "HEAD").strip())
        message = f"ai9s {version}."
        if notes:
            message = f"ai9s {version}.\n\n{notes.rstrip()}"
        git("tag", "-a", f"v{version}", "-m", message)
        git("push", "origin", f"v{version}")
        print(f"tag v{version}")
        if skip_formula or tap is None:
            return 0
        sha256 = archive_sha(version)
        publish_formula(version, sha256, tap)
        if install:
            reinstall(run(["gh", "auth", "token"]).strip())
            print(run(["ai9s", "version"]).strip())
        return 0
    except ReleaseError as err:
        print(f"release: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
