# Changelog

## Unreleased

- Delete refuses symlinks and paths that resolve outside the agent directory.
- Resume refuses session ids, profiles, and paths that look like CLI flags.
- The index database file is mode 0600.
- Go 1.25 is the minimum. `golang.org/x/sys` and `golang.org/x/text` are updated past reported vulnerabilities.

## 0.1.0

- Local index and terminal list for Claude, Codex, Copilot CLI, Grok, Antigravity, Gemini CLI, Cursor, OpenCode, Hermes, OpenClaw, Junie, Jules, Goose, Cline, Aider, and Kiro.
- Search, preview, filter, resume, and delete where a single safe removal exists.
- Views for sessions, providers, directories, branches, and models. `:` opens command mode. `?` opens the manual.
- GitHub Actions runs formatting and the test suite, including a fixture-backed CLI integration test.
- Documentation website published with GitHub Pages.
- Homebrew tap `AymanZahran/air9s`.
