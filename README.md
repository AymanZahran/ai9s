# air9s

air9s is a keyboard-first finder for local AI coding sessions. It indexes the session files already on your machine, then lets you search, preview, filter, resume, and — where it is safe — delete them.

The interface is a terminal list: a header with the hotkeys on top, a filter line, a session table, a preview pane, and a footer of key hints. Each agent has its own icon in the header, the table, and the preview. `1` through `5` switch the table between sessions, providers, directories, branches, and models. `/` edits the filter and `:` opens a command line for those views and for filter tokens. `?` opens a scrollable manual. Tab moves into the preview so the excerpt can be scrolled; Tab or Esc returns to the list. The CTX column, the preview, and `air9s show` include context size and token counts when the agent recorded them.

## Install

Requires Go 1.23 or newer.

```sh
git clone https://github.com/AymanZahran/air9s.git
cd air9s
make install
```

`make install` puts the binary in `~/.local/bin`. Override that with `make install PREFIX=/usr/local`.

```sh
go install github.com/AymanZahran/air9s/cmd/air9s@latest
```

## Usage

```sh
air9s                 # open the list
air9s index           # refresh the local index
air9s stats
air9s search 'agent:claude dir:my-repo date:<7d auth'
air9s show claude:<session-id>
air9s resume claude:<session-id>
air9s resume codex:<session-id> --print   # show the command, do not run it
air9s delete claude:<session-id>          # asks you to type the id
```

`--json` works on `index`, `stats`, `search`, and `show`. `resume --yolo` adds an auto-approve flag only for agents that document one. `delete --yes` skips the prompt.

Resume runs that agent's own CLI, in the session's directory when that directory still exists.

## Keys

| Key | Action |
| --- | --- |
| `1`–`5` | Sessions, providers, directories, branches, models. The active view is bold in the top hotkey bar. |
| Enter | Resume the selected session. On a group view, apply that group as a filter and return to sessions. |
| Tab | Focus the preview. `j`/`k` or the arrows scroll a line, Page Up/Down or Ctrl-B/Ctrl-F scroll a page, `g`/`G` jump to the top or the end. Tab or Esc returns to the list. The mouse wheel scrolls the preview when the pointer is over it. |
| `d`, Ctrl-D | Delete, after confirmation. Sessions view only. |
| `/` | Edit the filter |
| `:` | Command mode. Type a view name or a filter token. Enter applies it. Enter on an empty command cycles the view. Another `:` cycles the view name in the field. Esc closes it. |
| `a` | Cycle the `agent:` filter |
| `p` | Add a `dir:` filter |
| `o` | Cycle sort: recent, oldest, messages, title |
| `y` | Toggle yolo for the next resume |
| `r` | Reindex |
| `s` | Stats |
| `?` | Scrollable manual. `j`/`k` scroll, `g`/`G` jump, Esc or `q` returns to the list. |
| `q` | Quit |
| `j` / `k` | Move down / up |

## Filters

Free text is matched against the title and a capped excerpt of the transcript. These tokens are filters:

| Token | Meaning |
| --- | --- |
| `agent:claude` | Agent id. Also `a:`. |
| `dir:repo` | Working directory contains the text. Also `cwd:` and `directory:`. |
| `branch:main` | Git branch contains the text. |
| `model:sonnet` | Model name contains the text. |
| `date:<7d` | Updated in the last 7 days. Units: `m`, `h`, `d`, `w`. |
| `date:>30d` | Updated more than 30 days ago. |
| `date:2026-03-02` | That calendar day. `date:2026-03` is the whole month. |
| `sort:messages` | `recent` (default), `oldest`, `messages`, or `title`. |

Quote a phrase to keep it together: `"auth bug"`.

## Metadata

The preview and `air9s show` print a context line and a token line when the session file has them: latest prompt size, context window, input, output, cache read, cache write, reasoning tokens, cost, premium requests, and reasoning effort. The CTX column is the latest context size, or `used/window` when both are known.

Claude records per-turn usage and cost. Codex records token totals and, when present, the context window. Copilot CLI records the latest prompt size, cache, reasoning effort, and premium requests. OpenCode records session token totals and the latest prompt size. Grok records reasoning effort. Hermes records token totals and cost. OpenClaw records the context window separately from the estimated prompt size. Goose and Cline record token totals, and Cline records cost. Gemini, Cursor, Antigravity, Junie, Jules, Aider, and Kiro leave the token lines empty when their files do not carry totals.

## Agents

Paths below are the defaults. Each one can be pointed somewhere else with the environment variable in the last column. air9s only reads session stores that exist; a missing directory is skipped.

| Agent | What is read | Resume | Delete | Override |
| --- | --- | --- | --- | --- |
| `claude` | `$CLAUDE_CONFIG_DIR/projects/**/*.jsonl` | `claude --resume <id>` | the transcript file | `CLAUDE_CONFIG_DIR` |
| `codex` | `$CODEX_HOME/sessions/**/rollout-*.jsonl` | `codex resume <id>` | the rollout file | `CODEX_HOME` |
| `copilot` | `$COPILOT_HOME/session-state/<id>/` | `copilot --resume <id>` | that session directory | `COPILOT_HOME` |
| `grok` | `$GROK_HOME/sessions/**/summary.json` | `grok --resume <id>` | no | `GROK_HOME` |
| `agy` | `$GEMINI_HOME/antigravity-cli/history.jsonl` | `agy --conversation <id>` | that conversation's lines | `GEMINI_HOME` |
| `gemini` | `$GEMINI_HOME/tmp/<project>/chats/session-*.json` | `gemini --session-file <path>` | no | `GEMINI_HOME` |
| `cursor` | `$CURSOR_HOME/projects/**/agent-transcripts/<id>/<id>.jsonl` | `cursor-agent --resume <id>` (or `agent`) | the transcript file | `CURSOR_HOME` |
| `opencode` | `$XDG_DATA_HOME/opencode/opencode.db` | `opencode --session <id>` | `opencode session delete <id>` | `OPENCODE_DB` |
| `hermes` | `$HERMES_HOME/state.db` and `profiles/<name>/state.db` | `hermes --resume <id>` (`-p <profile>` for a named profile) | `hermes sessions delete <id> --yes` | `HERMES_HOME` |
| `openclaw` | `$OPENCLAW_STATE_DIR/agents/<id>/agent/openclaw-agent.sqlite` | `openclaw resume <session-key>` | `openclaw sessions delete <key> --yes` | `OPENCLAW_STATE_DIR`, else `OPENCLAW_HOME` |
| `junie` | `$JUNIE_HOME/sessions/session-*/transcript.md` | `junie --resume --session-id=<id>` | that session directory | `JUNIE_HOME` |
| `jules` | `$JULES_HOME/sessions.json` or `sessions.txt` | `jules teleport <id>` | no | `JULES_HOME` |
| `goose` | `$GOOSE_HOME/sessions/sessions.db` | `goose session --resume --session-id <id>` | `goose session remove --session-id <id>` | `GOOSE_HOME` |
| `cline` | `$CLINE_HOME/data/state/taskHistory.json` | `cline task open <id>` | rewrite the history, then remove the task directory | `CLINE_HOME` |
| `aider` | `.aider.chat.history.md` under the configured roots | `aider --restore-chat-history` | that history file | `AIDER_CHAT_ROOTS`, `AIDER_HOME`, `AIDER_CHAT_HISTORY` |
| `kiro` | `kiro-cli` `data.sqlite3` (`conversations_v2`) | `kiro-cli chat --resume-id <id>` | no | `KIRO_CLI_DB`, else `KIRO_HOME` |

Yolo maps to a documented flag: Claude and Antigravity `--dangerously-skip-permissions`, Grok `--always-approve`, Copilot `--allow-all-tools`, Cursor `--force`, Hermes `--yolo`, Junie `--brave`, Cline `--yolo`, Kiro `--trust-all-tools`. Codex, Gemini, OpenCode, OpenClaw, Jules, Goose, and Aider are resumed without an extra approval flag.

Subagent transcripts are skipped. Kiro's shell `history` table is skipped. Resume for Kiro uses `kiro-cli`, which is separate from the Kiro IDE. Goose resume and delete need the `goose` command on `PATH`. Jules talks to Google only when `AIR9S_JULES_REMOTE=1`; otherwise air9s reads the local snapshot. Aider does not walk the home directory unless `AIDER_SCAN_HOME=1`.

### Not indexed yet

These need a stable file layout and a real resume command before an adapter should claim them: Copilot in VS Code, Kimi, Qwen, Pi, Crush, Vibe, and Prime. Adding one is a scanner that returns `discover.Batch` plus a branch in `act.Plan`.

### Why some deletes are refused

Grok and Gemini keep indexes, active-session records, or project side files next to the transcript. Jules sessions live in Google's cloud. Kiro keeps every conversation in one database. Removing the file air9s can see would leave those tools inconsistent, so delete is disabled and the preview says so.

Antigravity stores every conversation in one `history.jsonl`. Delete rewrites that file without the chosen conversation id. Cline delete rewrites `taskHistory.json` first, then removes that task's directory. OpenCode, Hermes, OpenClaw, and Goose delete through each CLI. Other enabled deletes remove a single transcript or Aider history file, or one Copilot `session-state` or Junie `session-*` directory, and only when the path is still inside that agent's session root.

## Index

The index lives at `$AIR9S_CACHE_DIR/index.db`, or `$XDG_CACHE_HOME/air9s/index.db`, or `~/.cache/air9s/index.db`. It stores titles, metadata, and short excerpts (the first and last part of each transcript), not a second full copy of every log.

`air9s index`, and opening the UI, scan again and skip files whose modification time has not changed. One agent failing to scan does not drop the others. Delete the index directory any time; the next run rebuilds it from the agent files.

## Development

```sh
go test ./...
make build
```

GitHub Actions runs that test suite on Ubuntu for the Go version in `go.mod` and for the current stable Go. `cmd/air9s/integration_test.go` builds the binary and runs `index`, `search`, `show`, `resume --print`, and `delete` against temporary fixtures. The fixtures override every agent home, so the test does not read a developer's real sessions, and Jules is not contacted. A missing `gofmt` diff fails the same workflow.

## License

MIT. See [LICENSE](LICENSE).
