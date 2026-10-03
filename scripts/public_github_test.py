#!/usr/bin/env python3
import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "ai9s_public_github", Path(__file__).with_name("public_github.py")
)
settings = importlib.util.module_from_spec(spec)
spec.loader.exec_module(settings)

ROOT = Path(__file__).resolve().parents[1]


class PublicGitHubTests(unittest.TestCase):
    def test_private_repository_is_not_configured(self):
        self.assertEqual(
            settings.repos_to_configure({settings.AI9S: "private"}),
            [],
        )

    def test_public_repository_is_configured(self):
        self.assertEqual(
            settings.repos_to_configure({settings.AI9S: "public"}),
            [settings.AI9S],
        )

    def test_protection_requires_the_ci_contexts(self):
        body = settings.protection_body(settings.CHECKS[settings.AI9S])
        contexts = body["required_status_checks"]["contexts"]
        self.assertEqual(
            contexts,
            ["audit", "go 1.25", "go stable", "govulncheck", "formula"],
        )
        self.assertNotIn("checks", body["required_status_checks"])
        self.assertTrue(body["required_status_checks"]["strict"])
        self.assertTrue(body["enforce_admins"])
        self.assertIsNone(body["required_pull_request_reviews"])
        self.assertFalse(body["allow_force_pushes"])

    def test_ci_job_names_match_the_protected_contexts(self):
        workflow = (ROOT / ".github" / "workflows" / "ci.yml").read_text()
        self.assertIn('go: ["1.25", "stable"]', workflow)
        self.assertIn("name: go ${{ matrix.go }}", workflow)
        self.assertIn("name: govulncheck", workflow)
        self.assertIn("name: audit", workflow)
        self.assertIn("name: formula", workflow)
        self.assertIn("Formula/ai9s.rb", workflow)
        self.assertIn("golang.org/x/vuln/cmd/govulncheck@v1.8.0", workflow)


if __name__ == "__main__":
    unittest.main()
