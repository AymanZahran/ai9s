#!/usr/bin/env python3
"""Turn on the GitHub settings that a private repository cannot store.

This does not change visibility. Branch protection, secret scanning, push
protection, and GitHub Pages return an error until the repository is public
(or the account plan includes them). Run this after the repository is public
and a pull request's checks have passed once:

    python3 scripts/public_github.py

The status-check names have to match the job names in CI.
"""

from __future__ import annotations

import json
import subprocess
import sys

AI9S = "AymanZahran/ai9s"
HOMEPAGE = "https://ai9scli.io/"

CHECKS = {
    AI9S: ["audit", "go 1.25", "go stable", "govulncheck", "formula"],
}


class SettingsError(Exception):
    pass


def die(message: str) -> None:
    raise SettingsError(message)


def protection_body(contexts: list[str]) -> dict:
    # GitHub rejects a body that sets both contexts and checks.
    return {
        "required_status_checks": {
            "strict": True,
            "contexts": list(contexts),
        },
        # An administrator can merge without waiting for the required checks.
        # Force pushes stay off.
        "enforce_admins": False,
        "required_pull_request_reviews": None,
        "restrictions": None,
        "allow_force_pushes": False,
        "allow_deletions": False,
        "required_conversation_resolution": True,
    }


def repos_to_configure(visibilities: dict[str, str]) -> list[str]:
    if visibilities.get(AI9S) == "public":
        return [AI9S]
    return []


def api(method: str, path: str, body: dict | None = None) -> tuple[int, str]:
    cmd = [
        "gh",
        "api",
        "--method",
        method,
        "-H",
        "Accept: application/vnd.github+json",
        path,
    ]
    payload = None
    if body is not None:
        cmd.extend(["--input", "-"])
        payload = json.dumps(body)
    proc = subprocess.run(
        cmd,
        input=payload,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    detail = "\n".join(part.strip() for part in (proc.stdout, proc.stderr) if part.strip())
    return proc.returncode, detail


def require_ok(code: int, detail: str, action: str) -> dict | list | None:
    if code != 0:
        die(f"{action} failed:\n{detail[-2000:]}")
    if not detail:
        return None
    try:
        return json.loads(detail)
    except json.JSONDecodeError:
        return None


def configure(repo: str) -> None:
    require_ok(
        *api(
            "PATCH",
            f"repos/{repo}",
            {
                "security_and_analysis": {
                    "secret_scanning": {"status": "enabled"},
                    "secret_scanning_push_protection": {"status": "enabled"},
                },
            },
        ),
        f"secret scanning for {repo}",
    )
    if repo == AI9S:
        code, _detail = api("GET", f"repos/{repo}/pages")
        if code != 0:
            require_ok(
                *api("POST", f"repos/{repo}/pages", {"build_type": "workflow"}),
                f"GitHub Pages for {repo}",
            )
        require_ok(
            *api("PATCH", f"repos/{repo}", {"homepage": HOMEPAGE}),
            f"homepage for {repo}",
        )
    require_ok(
        *api(
            "PUT",
            f"repos/{repo}/branches/main/protection",
            protection_body(CHECKS[repo]),
        ),
        f"branch protection for {repo}",
    )
    require_ok(
        *api(
            "PATCH",
            f"repos/{repo}",
            {"delete_branch_on_merge": True, "allow_auto_merge": True},
        ),
        f"merge settings for {repo}",
    )
    # Reporting may 404 on a plan that does not offer it. Advisories stay
    # available either way, so this call does not fail the command.
    api("PUT", f"repos/{repo}/private-vulnerability-reporting")
    print(f"configured {repo}")


def main() -> int:
    try:
        payload = require_ok(*api("GET", f"repos/{AI9S}"), f"read {AI9S}")
        if not isinstance(payload, dict):
            die(f"could not read {AI9S}")
        visibilities = {AI9S: payload.get("visibility", "")}
        ready = repos_to_configure(visibilities)
        skipped = [AI9S] if AI9S not in ready else []
        for repo in ready:
            configure(repo)
        if skipped:
            die(
                "still private: "
                + ", ".join(skipped)
                + ". Making a repository public is a separate step. Run this again after that."
            )
        return 0
    except SettingsError as err:
        print(f"public settings: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
