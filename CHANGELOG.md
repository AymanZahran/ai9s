# Changelog

## 0.2.0

- The session list uses a k9s-style menu, crumbs bar, frame, and row cursor. Colors, the mouse, icons, and the starting view come from a skin.
- Config lives in `$AIR9S_CONFIG_DIR`, `$XDG_CONFIG_HOME/air9s`, or `~/.config/air9s`. `air9s info` prints that path, the index path, the skin, and the plugin count.
- Plugins in `plugins.yaml` run a local program for a shortcut. Core keys stay reserved.
- Esc clears the filter from the list and from the filter line. Preview, manual, and command-mode Esc stay as they were.
- A filter word matches a substring of the title, summary, directory, branch, model, agent, or excerpt. `agent:`, `dir:`, `branch:`, and `model:` do the same, and a leading `~` expands.
- The logo is a bone of two octagons and propellers on a transparent background.
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
