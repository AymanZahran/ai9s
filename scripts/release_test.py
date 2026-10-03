#!/usr/bin/env python3
import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "ai9s_release", Path(__file__).with_name("release.py")
)
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


CHANGELOG = """# Changelog

## Unreleased

- Publish the GitHub release from this command.

## 0.2.8

- Antigravity sessions use the name antigravity.
"""

FORMULA = """  url "https://github.com/AymanZahran/ai9s/archive/refs/tags/v0.2.8.tar.gz", using: Ai9sDownloadStrategy
  sha256 "a37e16606e06b281045380f95f0ab103d27e1840d626099323f945b474ca0f53"
    system "go", "build", *std_go_args, "./cmd/ai9s"
"""


class ReleaseTests(unittest.TestCase):
    def test_first_release_is_0_0_1(self):
        self.assertEqual(release.next_version(None, "patch"), "0.0.1")
        self.assertEqual(
            release.change_log_text("0.0.1", "- First release.\n"),
            "# Release v0.0.1\n\n- First release.\n",
        )

    def test_bump(self):
        self.assertEqual(release.bump_version("0.2.8", "patch"), "0.2.9")
        self.assertEqual(release.bump_version("0.2.8", ""), "0.2.9")
        self.assertEqual(release.bump_version("0.2.8", "minor"), "0.3.0")
        self.assertEqual(release.bump_version("0.2.8", "major"), "1.0.0")
        self.assertEqual(release.bump_version("0.2.8", "1.4.0"), "1.4.0")

    def test_bump_rejects_older_and_junk(self):
        with self.assertRaises(release.ReleaseError):
            release.bump_version("0.2.8", "0.2.8")
        with self.assertRaises(release.ReleaseError):
            release.bump_version("0.2.8", "0.2.7")
        with self.assertRaises(release.ReleaseError):
            release.bump_version("0.2.8", "v0.2.9")

    def test_splice_moves_unreleased_notes(self):
        text = release.splice_changelog(CHANGELOG, "0.2.9")
        self.assertIn("## Unreleased\n\n## 0.2.9\n\n- Publish the GitHub release from this command.\n", text)
        self.assertNotIn("## Unreleased\n\n- Publish", text)
        self.assertIn("## 0.2.8\n", text)
        notes = release.changelog_notes(text, "0.2.9")
        self.assertEqual(notes, "- Publish the GitHub release from this command.\n")

    def test_empty_unreleased_is_refused(self):
        empty = "# Changelog\n\n## Unreleased\n\n## 0.2.8\n\n- old\n"
        with self.assertRaises(release.ReleaseError):
            release.splice_changelog(empty, "0.2.9")

    def test_empty_unreleased_notes_are_blank(self):
        empty = "# Changelog\n\n## Unreleased\n\n## 0.2.8\n\n- old\n"
        self.assertEqual(release.unreleased_notes(empty), "")

    def test_formula_pins_the_new_archive(self):
        sha = "b" * 64
        updated = release.update_formula(FORMULA, "0.2.9", sha)
        self.assertIn("refs/tags/v0.2.9.tar.gz", updated)
        self.assertIn(f'sha256 "{sha}"', updated)
        self.assertNotIn("v0.2.8", updated)
        self.assertIn("cmd.version=#{version}", updated)
        self.assertIn('"."', updated)
        self.assertNotIn("./cmd/ai9s", updated)

    def test_formula_rejects_a_short_hash(self):
        with self.assertRaises(release.ReleaseError):
            release.update_formula(FORMULA, "0.2.9", "abc")

    def test_args(self):
        self.assertEqual(release.parse_args([]), ("patch", False, False, False))
        self.assertEqual(
            release.parse_args(["0.3.0", "--install", "--dry-run"]),
            ("0.3.0", True, True, False),
        )
        with self.assertRaises(release.ReleaseError):
            release.parse_args(["minor", "0.3.0"])

    def test_origin_slug(self):
        self.assertEqual(
            release.origin_slug("git@github.com:AymanZahran/ai9s.git"),
            "AymanZahran/ai9s",
        )
        self.assertEqual(
            release.origin_slug("https://github.com/AymanZahran/ai9s.git"),
            "AymanZahran/ai9s",
        )
        with self.assertRaises(release.ReleaseError):
            release.origin_slug("ssh://example.com/ai9s.git")

    def test_checks_merge_only_when_one_passed_and_none_failed(self):
        self.assertEqual(release.checks_decision([]), "wait")
        self.assertEqual(
            release.checks_decision([{"bucket": "skipping"}]),
            "wait",
        )
        self.assertEqual(
            release.checks_decision(
                [{"bucket": "pass"}, {"bucket": "skipping"}]
            ),
            "pass",
        )
        self.assertEqual(
            release.checks_decision(
                [{"bucket": "fail", "state": "startup_failure"}, {"bucket": "pending"}]
            ),
            "fail",
        )
        self.assertEqual(
            release.checks_decision([{"bucket": "cancel"}]),
            "fail",
        )
        self.assertEqual(
            release.checks_decision([{"bucket": "pending"}]),
            "wait",
        )
        self.assertEqual(
            release.checks_decision([{"bucket": "mystery"}]),
            "fail",
        )

    def test_wait_stops_on_failure_without_sleeping(self):
        clock = Clock()
        with self.assertRaises(release.ReleaseError) as caught:
            release.wait_for_checks(
                lambda: [{"name": "go 1.25", "bucket": "fail", "state": "startup_failure"}],
                clock.sleep,
                clock.now,
            )
        self.assertIn("startup_failure", str(caught.exception))
        self.assertEqual(clock.t, 0)

    def test_wait_stops_when_no_checks_appear(self):
        clock = Clock()
        with self.assertRaises(release.ReleaseError) as caught:
            release.wait_for_checks(
                lambda: [],
                clock.sleep,
                clock.now,
                empty_grace_s=30,
                timeout_s=1200,
                interval_s=15,
            )
        self.assertIn("no checks", str(caught.exception))
        self.assertLess(clock.t, 1200)

    def test_wait_passes_after_a_pending_check(self):
        clock = Clock()
        calls = {"n": 0}

        def fetch():
            calls["n"] += 1
            if calls["n"] == 1:
                return [{"name": "audit", "bucket": "pending", "state": "PENDING"}]
            return [{"name": "audit", "bucket": "pass", "state": "SUCCESS"}]

        release.wait_for_checks(fetch, clock.sleep, clock.now, interval_s=15)
        self.assertEqual(calls["n"], 2)
        self.assertEqual(clock.t, 15)


class GitFlow:
    def __init__(self):
        self.branch = "main"
        self.staged = set()
        self.local = {"main"}
        self.remote = set()
        self.head = {"main": "base"}
        self.calls = []
        self.ran = []

    def git(self, *args, cwd=None):
        self.calls.append(args)
        if args[:2] == ("rev-parse", "--abbrev-ref"):
            return self.branch + "\n"
        if args[:2] == ("branch", "--list"):
            name = args[2]
            if name in self.local and name != self.branch:
                return name + "\n"
            return ""
        if args[:3] == ("ls-remote", "--heads", "origin"):
            if args[3] in self.remote:
                return "x\trefs/heads/%s\n" % args[3]
            return ""
        if args[:2] == ("checkout", "-b"):
            self.branch = args[2]
            self.local.add(self.branch)
            self.head.setdefault(self.branch, self.head["main"])
            return ""
        if args == ("checkout", "main"):
            self.branch = "main"
            return ""
        if args[0] == "add":
            self.staged = set(args[1:])
            return ""
        if args[:3] == ("diff", "--cached", "--name-only"):
            return "".join(path + "\n" for path in sorted(self.staged))
        if args[0] == "commit":
            self.head[self.branch] = "branchsha"
            self.staged = set()
            return ""
        if args[0] == "push":
            self.remote.add(self.branch)
            return ""
        if args[:2] == ("pull", "--ff-only"):
            self.head["main"] = "squashsha"
            return ""
        if args[:2] == ("rev-parse", "HEAD"):
            return self.head[self.branch] + "\n"
        raise AssertionError(args)

    def run(self, args, cwd=None, env=None):
        self.ran.append(args)
        return ""


class ReleasePullRequestTests(unittest.TestCase):
    def test_release_branch_names(self):
        self.assertEqual(release.release_branch("release", "1.2.3"), "release-v1.2.3")
        self.assertEqual(release.release_branch("formula", "1.2.3"), "formula-v1.2.3")
        with self.assertRaises(release.ReleaseError):
            release.release_branch("docs", "1.2.3")
        with self.assertRaises(release.ReleaseError):
            release.release_branch("release", "v1.2.3")

    def test_land_opens_a_pull_request_and_does_not_push_main(self):
        flow = GitFlow()
        original = (
            release.git,
            release.run,
            release.wait_for_commit,
            release.discard_local_branch,
        )
        release.git = flow.git
        release.run = flow.run
        release.wait_for_commit = lambda repo, sha: None
        release.discard_local_branch = lambda branch: None
        try:
            got = release.land_through_pull_request(
                ["CHANGELOG.md"],
                "Release 1.2.3.",
                "release-v1.2.3",
                "AymanZahran/ai9s",
            )
        finally:
            (
                release.git,
                release.run,
                release.wait_for_commit,
                release.discard_local_branch,
            ) = original
        self.assertEqual(got, "squashsha")
        self.assertNotIn(("push", "origin", "main"), flow.calls)
        self.assertTrue(any(args[:3] == ["gh", "pr", "create"] for args in flow.ran))
        self.assertTrue(any(args[:3] == ["gh", "pr", "merge"] for args in flow.ran))
        self.assertIn("--squash", flow.ran[-1])
        self.assertNotIn("origin", [args[1] if len(args) > 1 else "" for args in flow.calls if args[:1] == ("push",) and "main" in args])

    def test_land_refuses_a_branch_other_than_main(self):
        flow = GitFlow()
        flow.branch = "public-gaps"
        original = release.git
        release.git = flow.git
        try:
            with self.assertRaises(release.ReleaseError):
                release.land_through_pull_request(
                    ["CHANGELOG.md"],
                    "Release 1.2.3.",
                    "release-v1.2.3",
                    "AymanZahran/ai9s",
                )
        finally:
            release.git = original


class Clock:
    def __init__(self):
        self.t = 0

    def now(self):
        return self.t

    def sleep(self, seconds):
        self.t += seconds


if __name__ == "__main__":
    unittest.main()
