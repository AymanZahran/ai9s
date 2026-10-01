#!/usr/bin/env python3
"""Cut an air9s release through pull requests.

Write the notes under "## Unreleased" in CHANGELOG.md and merge that commit
first. Then, from a clean main that matches origin:

    make release

That opens a pull request for the version bump, waits until the checks pass,
and merges it. Only then does it push an annotated tag and create the GitHub
Release. The Homebrew formula gets its own pull request, and that merges the
same way. A failed or missing check leaves the pull request open.

    make release VERSION=1.2.3
    make release PART=minor
    make release INSTALL=1
    make release DRY=1

The version string in the source is what `air9s version` prints. The formula
builds that source and does not pass -X.
"""

from __future__ import annotations

import hashlib
import json
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
VERSION_FILE = ROOT / "cmd" / "air9s" / "main.go"
CHANGELOG = ROOT / "CHANGELOG.md"
ARCHIVE = "https://github.com/AymanZahran/air9s/archive/refs/tags/v{version}.tar.gz"
VERSION_RE = re.compile(r'(?m)^const version = "(\d+\.\d+\.\d+)"$')
CHANGELOG_RE = re.compile(
    r"\A# Changelog\n\n## Unreleased\n(?P<body>.*?)(?=\n## |\Z)",
    re.S,
)
FORMULA_URL_RE = re.compile(
    r'url "https://github.com/AymanZahran/air9s/archive/refs/tags/v\d+\.\d+\.\d+\.tar\.gz", using: Air9sDownloadStrategy'
)
FORMULA_SHA_RE = re.compile(r'sha256 "[0-9a-f]{64}"')
ORIGIN_RE = re.compile(r"github\.com[:/](?P<owner>[^/]+)/(?P<name>[^/.]+)")
PASS_BUCKETS = {"pass", "skipping"}
KNOWN_BUCKETS = PASS_BUCKETS | {"fail", "pending", "cancel"}


class ReleaseError(Exception):
    pass


def die(message: str) -> None:
    raise ReleaseError(message)


def parse_version(text: str) -> str:
    found = VERSION_RE.findall(text)
    if len(found) != 1:
        die("cmd/air9s/main.go must contain one version const")
    return found[0]


def write_version(text: str, version: str) -> str:
    replacement = f'const version = "{version}"'
    updated, count = VERSION_RE.subn(replacement, text, count=1)
    if count != 1:
        die("cmd/air9s/main.go must contain one version const")
    return updated


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


def update_formula(text: str, version: str, sha256: str) -> str:
    if not re.fullmatch(r"[0-9a-f]{64}", sha256):
        die("sha256 must be 64 hex characters")
    url = (
        f'url "{ARCHIVE.format(version=version)}", using: Air9sDownloadStrategy'
    )
    updated, count = FORMULA_URL_RE.subn(url, text, count=1)
    if count != 1:
        die("formula is missing the air9s tag archive url")
    updated, count = FORMULA_SHA_RE.subn(f'sha256 "{sha256}"', updated, count=1)
    if count != 1:
        die("formula is missing its sha256")
    if updated == text:
        die("formula is already at this archive")
    return updated


def release_branch(version: str) -> str:
    return f"release/{version}"


def formula_branch(version: str) -> str:
    return f"formula/air9s-{version}"


def release_pr_body(notes: str) -> str:
    return (
        notes.rstrip()
        + "\n\nThis pull request merges only after the checks pass. "
        + "The tag and the GitHub Release are created after that merge.\n"
    )


def formula_pr_body(version: str, sha256: str) -> str:
    return (
        f"Point the Homebrew formula at air9s {version}.\n\n"
        f"Archive sha256 `{sha256}`.\n\n"
        "This pull request merges only after the formula check passes.\n"
    )


def origin_slug(url: str) -> str:
    match = ORIGIN_RE.search(url.strip())
    if match is None:
        die(f"origin is not a GitHub repository: {url}")
    return f"{match.group('owner')}/{match.group('name')}"


def checks_decision(checks: list[dict]) -> str:
    """Return pass, fail, or wait.

    A pull request merges only when at least one check passed and none failed
    or are still running. An empty report waits, so a missing Actions run
    cannot merge.
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
            die(f"checks failed: {format_checks(checks)}. The pull request is still open.")
        if not seen and now() - started >= empty_grace_s:
            die("GitHub reported no checks. The pull request is still open.")
        if now() >= deadline:
            die("checks did not finish. The pull request is still open.")
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
    raw = os.environ.get("AIR9S_TAP", "").strip()
    path = Path(raw) if raw else ROOT.parent / "homebrew-air9s"
    formula = path / "Formula" / "air9s.rb"
    if not formula.is_file():
        die(f"Homebrew formula not found at {formula}. Set AIR9S_TAP to the tap checkout.")
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


def ensure_new_branch(cwd: Path, branch: str) -> None:
    local = subprocess.run(
        ["git", "show-ref", "--verify", "--quiet", f"refs/heads/{branch}"],
        cwd=cwd,
    )
    if local.returncode == 0:
        die(f"{branch} already exists")
    remote = git("ls-remote", "--heads", "origin", branch, cwd=cwd).strip()
    if remote:
        die(f"{branch} already exists on origin")


def restore_sources() -> None:
    subprocess.run(
        ["git", "checkout", "--", "CHANGELOG.md", "cmd/air9s/main.go"],
        cwd=ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def child_env() -> dict[str, str]:
    env = os.environ.copy()
    env["PYTHONDONTWRITEBYTECODE"] = "1"
    return env


def commit_on_new_branch(cwd: Path, branch: str, paths: list[str], message: str) -> None:
    if git("rev-parse", "--abbrev-ref", "HEAD", cwd=cwd).strip() != "main":
        die(f"{cwd.name} is not on main")
    ensure_new_branch(cwd, branch)
    git("checkout", "-b", branch, cwd=cwd)
    try:
        git("add", *paths, cwd=cwd)
        staged = set(git("diff", "--cached", "--name-only", cwd=cwd).split())
        if staged != set(paths):
            die("commit would include unexpected files: " + " ".join(sorted(staged)))
        git("commit", "-m", message, cwd=cwd)
        git("push", "-u", "origin", branch, cwd=cwd)
    except ReleaseError:
        head = git("rev-parse", "HEAD", cwd=cwd).strip()
        main_sha = git("rev-parse", "main", cwd=cwd).strip()
        if head == main_sha:
            subprocess.run(
                ["git", "restore", "--source=HEAD", "--staged", "--worktree", "--", *paths],
                cwd=cwd,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            subprocess.run(
                ["git", "checkout", "main"],
                cwd=cwd,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            subprocess.run(
                ["git", "branch", "-D", branch],
                cwd=cwd,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        raise


def open_pull_request(repo: str, branch: str, title: str, body: str) -> int:
    out = run(
        [
            "gh",
            "pr",
            "create",
            "--repo",
            repo,
            "--base",
            "main",
            "--head",
            branch,
            "--title",
            title,
            "--body",
            body,
        ]
    )
    match = re.search(r"/pull/(\d+)", out)
    if match is None:
        die("could not read the pull request number")
    print(out.strip())
    return int(match.group(1))


def fetch_checks(repo: str, number: int) -> list[dict]:
    proc = subprocess.run(
        [
            "gh",
            "pr",
            "checks",
            str(number),
            "--repo",
            repo,
            "--json",
            "name,bucket,state",
        ],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    text = proc.stdout.strip()
    if not text:
        return []
    try:
        data = json.loads(text)
    except json.JSONDecodeError:
        die("could not read check status")
    if not isinstance(data, list):
        die("could not read check status")
    return data


def merge_pull_request(repo: str, number: int, title: str, head: str, body: str) -> None:
    run(
        [
            "gh",
            "pr",
            "merge",
            str(number),
            "--repo",
            repo,
            "--squash",
            "--delete-branch",
            "--match-head-commit",
            head,
            "--subject",
            title,
            "--body",
            body,
        ]
    )


def land_pull_request(cwd: Path, repo: str, branch: str, number: int, title: str, body: str) -> None:
    head = git("rev-parse", branch, cwd=cwd).strip()
    git("checkout", "main", cwd=cwd)
    try:
        wait_for_checks(lambda: fetch_checks(repo, number), time.sleep, time.time)
        merge_pull_request(repo, number, title, head, body)
    except ReleaseError:
        print(f"pull request {repo}#{number} is still open", file=sys.stderr)
        raise
    git("pull", "--ff-only", "origin", "main", cwd=cwd)
    subprocess.run(
        ["git", "branch", "-D", branch],
        cwd=cwd,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def archive_sha(version: str) -> str:
    token = run(["gh", "auth", "token"]).strip()
    if not token:
        die("gh auth token is empty")
    url = ARCHIVE.format(version=version)
    request = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    fd, name = tempfile.mkstemp(prefix=f"air9s-v{version}-", suffix=".tar.gz")
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
            member = f"air9s-{version}/cmd/air9s/main.go"
            extracted = bundle.extractfile(member)
            if extracted is None:
                die("tag archive is missing cmd/air9s/main.go")
            source = extracted.read().decode()
        if f'const version = "{version}"' not in source:
            die("tag archive version does not match the release")
        return hashlib.sha256(data).hexdigest()
    finally:
        path.unlink(missing_ok=True)


def publish_formula(version: str, sha256: str, tap: Path) -> None:
    branch = formula_branch(version)
    ensure_new_branch(tap, branch)
    formula = tap / "Formula" / "air9s.rb"
    formula.write_text(update_formula(formula.read_text(), version, sha256))
    message = f"Point the formula at air9s {version}."
    body = formula_pr_body(version, sha256)
    commit_on_new_branch(tap, branch, ["Formula/air9s.rb"], message)
    repo = origin_slug(git("remote", "get-url", "origin", cwd=tap).strip())
    number = open_pull_request(repo, branch, message, body)
    land_pull_request(tap, repo, branch, number, message, body)
    print(f"formula {version} {sha256}")


def reinstall(token: str) -> None:
    clone = run(["brew", "--repository", "aymanzahran/air9s"]).strip()
    git("pull", "--ff-only", cwd=Path(clone))
    env = os.environ.copy()
    env["HOMEBREW_NO_AUTO_UPDATE"] = "1"
    env["HOMEBREW_GITHUB_API_TOKEN"] = token
    run(["brew", "reinstall", "air9s"], env=env)


def main(argv: list[str] | None = None) -> int:
    spec, dry, install, skip_formula = parse_args(sys.argv[1:] if argv is None else argv)
    try:
        require_ready_checkout(ROOT)
        tap = None
        if not skip_formula:
            tap = tap_dir()
            require_ready_checkout(tap)
        current = parse_version(VERSION_FILE.read_text())
        version = bump_version(current, spec)
        require_current_tag(current)
        require_new_tag(version)
        changelog = splice_changelog(CHANGELOG.read_text(), version)
        notes = changelog_notes(changelog, version)
        source = write_version(VERSION_FILE.read_text(), version)
        print(f"release {version}")
        print(notes, end="" if notes.endswith("\n") else "\n")
        print(f"pull request {release_branch(version)} -> main")
        if tap is not None:
            print(f"formula pull request {formula_branch(version)}")
        if dry:
            print("dry run")
            return 0
        ensure_new_branch(ROOT, release_branch(version))
        CHANGELOG.write_text(changelog)
        VERSION_FILE.write_text(source)
        try:
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
        except ReleaseError:
            restore_sources()
            raise
        branch = release_branch(version)
        title = f"Release {version}."
        body = release_pr_body(notes)
        commit_on_new_branch(ROOT, branch, ["CHANGELOG.md", "cmd/air9s/main.go"], title)
        repo = origin_slug(git("remote", "get-url", "origin").strip())
        number = open_pull_request(repo, branch, title, body)
        land_pull_request(ROOT, repo, branch, number, title, body)
        landed = parse_version(VERSION_FILE.read_text())
        if landed != version:
            die(f"main is {landed} after the merge, not {version}")
        git("tag", "-a", f"v{version}", "-m", f"air9s {version}.")
        git("push", "origin", f"v{version}")
        fd, name = tempfile.mkstemp(prefix="air9s-notes-", suffix=".md")
        os.close(fd)
        notes_path = Path(name)
        try:
            notes_path.write_text(notes)
            created = run(
                [
                    "gh",
                    "release",
                    "create",
                    f"v{version}",
                    "--verify-tag",
                    "--latest",
                    "--title",
                    f"air9s {version}",
                    "--notes-file",
                    str(notes_path),
                ],
                cwd=ROOT,
            )
            print(created.strip())
        finally:
            notes_path.unlink(missing_ok=True)
        print(f"github release v{version}")
        if skip_formula or tap is None:
            return 0
        sha = archive_sha(version)
        publish_formula(version, sha, tap)
        if install:
            reinstall(run(["gh", "auth", "token"]).strip())
            print(run(["air9s", "version"]).strip())
        return 0
    except ReleaseError as err:
        print(f"release: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
