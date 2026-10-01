# Changelog

## Unreleased

## 0.2.6

- Left and right pan the list and the describe view. `h` and `l` do the same. A bottom scrollbar appears when a line is wider than the window, and the horizontal wheel moves it.
- The example git plugin uses `b`. `h` and `l` stay reserved for panning.

## 0.2.5

- The built-in skin is a black screen: white text, a white selection bar, and a black crumbs line.

## 0.2.4

- The session list is the only window. `d` replaces it with a describe view of the selected row. Esc returns to the list.
- Delete moves to Ctrl-D. `d` no longer deletes.

## 0.2.3

- The session list and the preview each show a scrollbar on the right. The mouse wheel scrolls the pane under the pointer. Drag or click a scrollbar to jump.
- Page Up and Page Down move a page in the focused pane. Ctrl-B and Ctrl-F do the same. Arrows and `j`/`k` still move one line.

## 0.2.2

- Quitting a resumed session returns to air9s. Ctrl-C stays with the agent.
- Sessions indexed while delete was turned off are read again. Grok and Gemini then offer delete, like every other indexed agent.

## 0.2.1

- Up and down move the list while `/` or `:` is open. Page Up and Page Down do the same. Letters, including `j` and `k`, stay in the field. Enter applies the command row the arrows landed on. An empty command still cycles the view when the arrows were not used.
- Delete is available for every indexed agent. Grok removes that session directory and its active-session and metadata entries. Gemini removes that chat file. Kiro removes that conversation and leaves the shell history table. Jules drops the local snapshot row, and deletes a cloud session when `JULES_API_KEY` is set.
- `examples/plugins` adds four opt-in plugins: open the directory in an editor, copy a session reference, show git status and recent commits, and open a terminal there. A bare plugin command is taken from `PATH`, then from the config `plugins/` directory. Enabled plugin keys are drawn on the menu.
- The shortcut bar is a grid. View keys are the first row, actions continue under them, and installed plugins occupy their own rows. An empty `ui.skin` uses the built-in skin. `AIR9S_SKIN` or `ui.skin` selects a file.

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
