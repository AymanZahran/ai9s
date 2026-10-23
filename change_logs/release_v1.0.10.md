# Release v1.0.10

- The sessions list nests a subagent under the session that launched it. The row starts collapsed, with a count. Space expands and collapses it. A search that matches a subagent keeps the parent on screen. Claude Code, Cursor, Grok Build, Codex, OpenCode, Hermes, OpenClaw, Kilo Code, MiniMax, Mistral Vibe, and MastraCode record that parent. Enter on a Claude Code or Cursor subagent resumes the parent session. Describe says so. The Herdr sidebar is this same list, so the tree is there after you quit and reopen it. `ai9s search` still prints one row per session.
- Release binaries are built with Go 1.26.10, which distrusts the compromised certificate intermediates fixed in that release.
