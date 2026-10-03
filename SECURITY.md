# Security

## Supported versions

| Version | Supported |
| --- | --- |
| 1.0.x | Yes |
| Older tags | No |

1.0.0 is the supported release. Tags before 1.0.0 stay in the history and are not patched.

## Reporting a vulnerability

Report security issues privately through [GitHub Security Advisories](https://github.com/AymanZahran/ai9s/security/advisories/new).

Do not open a public issue for a vulnerability. Do not include session transcripts, tokens, credentials, or the contents of an index database.

## What ai9s stores

ai9s reads session files that are already on the machine. The index keeps titles, metadata, and short excerpts. It does not upload them. Treat `index.db` as sensitive and do not commit it. Database files are gitignored.

Jules is contacted only when `AI9S_JULES_REMOTE=1`. Other agents are read from local files. Resume runs the agent CLI you already have installed. ai9s does not add an auto-approve flag unless that CLI documents one and you pass `--yolo`.

Delete removes or rewrites session data for the agents where that is supported. `delete` without `--yes` refuses to run when stdin is not a terminal. Read the agents page before using `--yes`.

Delete resolves the real path. It refuses a symlink, and it refuses a path that resolves outside that agent's session directory. Resume, and the delete commands that call an agent CLI, refuse a session id, profile, or path that starts with `-` or contains a control character, so a crafted id is not passed as a flag or an escape sequence. Rewriting a session file creates a new temporary file in that directory, so a planted symlink is not followed. The index file is mode `0600`. Scans open agent databases read-only. Kiro delete writes only the rows for that conversation and does not change the shell history table. A Jules cloud delete runs only when `JULES_API_KEY` is set, for that session id, and ai9s does not read the Jules keyring.

Session text is escaped before it is drawn. A title, name, path, or excerpt cannot change colors or insert a terminal hyperlink. The same text is stripped of control characters in human command output. `--json` keeps the original strings. A custom name drops bidi and other format characters when it is saved.

`config.yaml` is created mode `0600`. Plugins run the program named in the file, with arguments, and do not pass the line to a shell. A plugin shortcut that collides with a core key is ignored. Do not put tokens or credentials in the config.

CI rejects token-shaped strings and a machine-specific home path in tracked files, and it runs `govulncheck` on Go dependencies.
