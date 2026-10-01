#!/usr/bin/env python3
import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "air9s_release", Path(__file__).with_name("release.py")
)
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


CHANGELOG = """# Changelog

## Unreleased

- Publish the GitHub release from this command.

## 0.2.8

- Antigravity sessions use the name antigravity.
"""

FORMULA = """  url "https://github.com/AymanZahran/air9s/archive/refs/tags/v0.2.8.tar.gz", using: Air9sDownloadStrategy
  sha256 "a37e16606e06b281045380f95f0ab103d27e1840d626099323f945b474ca0f53"
"""

SOURCE = 'package main\n\nconst version = "0.2.8"\n'


class ReleaseTests(unittest.TestCase):
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

    def test_version_const_round_trip(self):
        self.assertEqual(release.parse_version(SOURCE), "0.2.8")
        updated = release.write_version(SOURCE, "0.2.9")
        self.assertEqual(release.parse_version(updated), "0.2.9")

    def test_formula_pins_the_new_archive(self):
        sha = "b" * 64
        updated = release.update_formula(FORMULA, "0.2.9", sha)
        self.assertIn("refs/tags/v0.2.9.tar.gz", updated)
        self.assertIn(f'sha256 "{sha}"', updated)
        self.assertNotIn("v0.2.8", updated)

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


if __name__ == "__main__":
    unittest.main()
