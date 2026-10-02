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
            release.origin_slug("https://github.com/AymanZahran/homebrew-ai9s.git"),
            "AymanZahran/homebrew-ai9s",
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


class Clock:
    def __init__(self):
        self.t = 0

    def now(self):
        return self.t

    def sleep(self, seconds):
        self.t += seconds


if __name__ == "__main__":
    unittest.main()
