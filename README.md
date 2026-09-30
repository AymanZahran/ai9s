# ai9s

ai9s is a keyboard-first finder for local AI coding sessions. It indexes the session files already on your machine, then lets you search, preview, filter, resume, and — where it is safe — delete them.

The interface is a terminal list: a header with counts, a filter line, a session table, a preview pane, and a footer of key hints.

## Install

Requires Go 1.23 or newer.

```sh
git clone https://github.com/AymanZahran/ai9s.git
cd ai9s
make install
```

`make install` puts the binary in `~/.local/bin`. Override that with `make install PREFIX=/usr/local`.

```sh
go install github.com/AymanZahran/ai9s/cmd/ai9s@latest
```

## Usage

```sh
ai9s                 # open the list
ai9s index           # refresh the local index
ai9s stats
ai9s search 'agent:claude dir:my-repo date:<7d auth'
ai9s show claude:<session-id>
ai9s resume claude:<session-id>
ai9s resume codex:<session-id> --print   # show the command, do not run it
ai9s delete claude:<session-id>          # asks you to type the id
```

`--json` works on `index`, `stats`, `search`, and `show`. `resume --yolo` adds an auto-approve flag only for agents that document one. `delete --yes` skips the prompt.

Resume runs that agent's own CLI, in the session's directory when that directory still exists.

## Keys

| Key | Action |
| --- | --- |
| Enter | Resume the selected session |
| `d`, Ctrl-D | Delete, after confirmation |
| `/` | Edit the filter |
| `a` | Cycle the `agent:` filter |
| `p` | Add a `dir:` filter |
| `o` | Cycle sort: recent, oldest, messages, title |
| `y` | Toggle yolo for the next resume |
| `r` | Reindex |
| `s` | Stats |
| `?` | Help |
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

## Agents

Paths below are the defaults. Each one can be pointed somewhere else with the environment variable in the last column. ai9s only reads session stores that exist; a missing directory is skipped.

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

Yolo maps to a documented flag: Claude and Antigravity `--dangerously-skip-permissions`, Grok `--always-approve`, Copilot `--allow-all-tools`, Cursor `--force`. Codex, Gemini, and OpenCode are resumed without an extra approval flag.

Subagent transcripts are skipped.

### Not indexed yet

These need a stable file layout and a real resume command before an adapter should claim them: Copilot in VS Code, Hermes, Kimi, Qwen, Pi, Crush, Vibe, and Prime. Adding one is a scanner that returns `discover.Batch` plus a branch in `act.Plan`.

### Why some deletes are refused

Grok and Gemini keep indexes, active-session records, or project side files next to the transcript. Removing the file ai9s can see would leave those tools inconsistent, so delete is disabled and the preview says so.

Antigravity stores every conversation in one `history.jsonl`. Delete rewrites that file without the chosen conversation id. OpenCode delete goes through `opencode session delete` rather than editing the database. Other enabled deletes remove a single transcript file, or one Copilot `session-state` directory, and only when the path is still inside that agent's session root.

## Index

The index lives at `$AI9S_CACHE_DIR/index.db`, or `$XDG_CACHE_HOME/ai9s/index.db`, or `~/.cache/ai9s/index.db`. It stores titles, metadata, and short excerpts (the first and last part of each transcript), not a second full copy of every log.

`ai9s index`, and opening the UI, scan again and skip files whose modification time has not changed. One agent failing to scan does not drop the others. Delete the index directory any time; the next run rebuilds it from the agent files.

## Development

```sh
go test ./...
make build
```

## License

MIT. See [LICENSE](LICENSE).
