# Changelog

## Unreleased

- Grok context and window come from signals.json. Token totals, cache, reasoning, and cost come from usage.json. Cost is costUsdTicks divided by 10^10. The percent field is not a token count.
- Hermes context is the latest prompt size in model_config, under _usage_anchor.prompt_tokens. The database has no context window. Token totals and cost were already recorded.
- Cursor context and window come from composerData when that row exists: contextTokensUsed, or the prompt breakdown total, over the limit. Transcripts still have no session token total.
- Antigravity context and window come from each conversation database. Input, output, cache read, and reasoning are summed across generations. A conversation with no database stays a dash.
- Kiro's database does not store context or token counts, so those columns stay a dash.
- The hotkey list is only the top menu, including g and G. The bottom hotkey line is gone. A wide line still has its horizontal scrollbar.
- The manual uses the same true-black screen as the list. j and k, the wheel, page keys, and Command-Up and Command-Down scroll it.
- Icons were checked again against the projects that print one: Claude Code's ✳, Gemini CLI's ✦, Hermes ☤, OpenClaw's lobster, and Goose's goose. A font without ✦ may draw it as a plus. Codex, Copilot, Grok, Antigravity, Cursor, OpenCode, Junie, Jules, Cline, Aider, and Kiro publish a picture and no emoji, so those cells stay the first two letters.

## 0.2.11

- Quitting a resumed agent returns to the list. air9s takes the terminal back, so the shell does not stop it with "suspended (tty output)".
- The mouse wheel scrolls. Mouse tracking is buttons and drags, so a terminal that keeps the wheel when all-motion tracking is on still delivers it. Shift with the wheel pans sideways.
- Command-Left and Command-Right move a page of columns, the same way Command-Up and Command-Down move a page of rows. Control or Alt with those arrows do the same, including while the filter or command field is open.
- AGE stays a relative age. DATE is the local date and time. Sort still follows AGE.
- The hotkey bar shows `j`/`k` with the up and down arrows, and `h`/`l` with the left and right arrows.
- Icons use a character that product prints: Claude Code's ✳, Gemini CLI's ✦, Hermes ☤, OpenClaw's lobster, and Goose's goose. A font without ✦ may draw it as a plus. Agents that publish a picture and no emoji use the first two letters of the name.

## 0.2.10

- Left and right pan by the same column width the table paints, so a cut-off title moves and the bottom scrollbar appears when a line is wider than the window.
- The built-in skin is true black with k9s accent colors: blue text, an orange logo, a blue border, and an aqua selection bar. The crumbs line stays black.
- OpenClaw, Goose, and Hermes use the mark published in that project's README. Gemini uses a sparkle, because its prompt glyph draws as a plus. The other agents publish a picture logo and no emoji, so those cells are a brand-colored mark.
- Describe always shows context and token usage, with a dash when the file has none. It no longer prints whether delete is available. The session list has a TOKENS column beside CTX.
- Command-Up and Command-Down move a page on the list and in describe. Control or Alt with those arrows do the same. Page Up and Page Down still work.
- The screen no longer has a yolo toggle. `air9s resume --yolo` is unchanged.
- Escape returns to the group you opened, and on that group it clears the filter and stays there. The line under the crumbs no longer lists every provider.

## 0.2.9

- Delete stays inside air9s. Confirming Ctrl-D no longer takes over the terminal, so a delete cannot leave a blank screen that ignores Ctrl-C.
- `make release` opens a pull request for the next version, merges it after checks pass, publishes the GitHub Release, and opens a pull request that points the Homebrew formula at that tag. Write the notes under Unreleased first.

## 0.2.8

- Antigravity sessions use the name antigravity. Resume still runs the `agy` command.

## 0.2.7

- View 4 lists each git branch together with the worktree that contains the session. Enter filters by that branch and that checkout.

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
