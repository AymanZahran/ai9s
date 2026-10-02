#!/usr/bin/env python3
import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location(
    "ai9s_audit", Path(__file__).with_name("audit.py")
)
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class AuditTests(unittest.TestCase):
    def test_flags_a_github_token(self):
        text = "token " + "ghp_" + ("a" * 36)
        self.assertEqual(audit.findings(text), [("github token", 1)])

    def test_flags_a_private_key_header(self):
        text = "-----BEGIN " + "PRIVATE KEY-----"
        self.assertEqual(audit.findings(text), [("private key", 1)])

    def test_flags_the_machine_home(self):
        text = "/" + "Users/" + "aymanzahran" + "/src"
        self.assertEqual(audit.findings(text), [("home path", 1)])

    def test_allows_token_env_names_and_docs(self):
        text = "\n".join(
            [
                "export HOMEBREW_GITHUB_API_TOKEN",
                "JULES_API_KEY",
                "echo secret",
                "test-key",
                "user@example.com",
            ]
        )
        self.assertEqual(audit.findings(text), [])

    def test_reports_the_line_number(self):
        text = "ok\n" + "AKIA" + ("A" * 16)
        self.assertEqual(audit.findings(text), [("aws access key", 2)])


if __name__ == "__main__":
    unittest.main()
