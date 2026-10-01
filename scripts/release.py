#!/usr/bin/env python3
"""Cut an air9s release and publish it on GitHub.

Write the notes under "## Unreleased" in CHANGELOG.md, then from a clean main:

    make release

That bumps the version in cmd/air9s/main.go, moves those notes under the new
version, runs the tests, pushes an annotated tag, and creates the GitHub
Release. It then points the Homebrew formula at that tag's archive.

    make release VERSION=1.2.3
    make release PART=minor
    make release INSTALL=1
    make release DRY=1

The version string in the source is what `air9s version` prints. The formula
builds that source and does not pass -X.
"""

from __future__ import annotations

import hashlib
import os
import re
import subprocess
import sys
import tarfile
import tempfile
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


def require_clean_main() -> None:
    branch = git("rev-parse", "--abbrev-ref", "HEAD").strip()
    if branch != "main":
        die(f"releases are cut from main (this checkout is {branch})")
    dirty = git("status", "--porcelain").strip()
    if dirty:
        die("commit or stash the working tree before releasing")
    git("fetch", "origin", "main")
    counts = git("rev-list", "--left-right", "--count", "origin/main...HEAD").split()
    if len(counts) != 2:
        die("could not compare main with origin")
    behind = int(counts[0])
    if behind:
        die("main is behind origin; pull before releasing")


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


def restore_sources() -> None:
    subprocess.run(
        ["git", "checkout", "--", "CHANGELOG.md", "cmd/air9s/main.go"],
        cwd=ROOT,
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


def publish_formula(version: str, sha256: str) -> None:
    tap = tap_dir()
    dirty = git("status", "--porcelain", cwd=tap).strip()
    if dirty:
        die(f"{tap} has uncommitted changes")
    formula = tap / "Formula" / "air9s.rb"
    formula.write_text(update_formula(formula.read_text(), version, sha256))
    git("add", "Formula/air9s.rb", cwd=tap)
    git("commit", "-m", f"Point the formula at air9s {version}.", cwd=tap)
    git("push", "origin", "main", cwd=tap)
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
        require_clean_main()
        current = parse_version(VERSION_FILE.read_text())
        version = bump_version(current, spec)
        require_current_tag(current)
        require_new_tag(version)
        changelog = splice_changelog(CHANGELOG.read_text(), version)
        notes = changelog_notes(changelog, version)
        source = write_version(VERSION_FILE.read_text(), version)
        print(f"release {version}")
        print(notes, end="" if notes.endswith("\n") else "\n")
        if dry:
            print("dry run")
            return 0
        CHANGELOG.write_text(changelog)
        VERSION_FILE.write_text(source)
        try:
            run(["go", "test", "-count=1", "-timeout", "180s", "./..."], cwd=ROOT)
        except ReleaseError:
            restore_sources()
            raise
        git("add", "CHANGELOG.md", "cmd/air9s/main.go")
        staged = set(git("diff", "--cached", "--name-only").split())
        if staged != {"CHANGELOG.md", "cmd/air9s/main.go"}:
            restore_sources()
            die("release commit would include unexpected files: " + " ".join(sorted(staged)))
        git("commit", "-m", f"Release {version}.")
        git("tag", "-a", f"v{version}", "-m", f"air9s {version}.")
        git("push", "origin", "main", f"v{version}")
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
        if skip_formula:
            return 0
        sha = archive_sha(version)
        publish_formula(version, sha)
        if install:
            reinstall(run(["gh", "auth", "token"]).strip())
            print(run(["air9s", "version"]).strip())
        return 0
    except ReleaseError as err:
        print(f"release: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
