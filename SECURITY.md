# Security

## Reporting a vulnerability

Report security issues privately through [GitHub Security Advisories](https://github.com/AymanZahran/air9s/security/advisories/new).

Do not open a public issue for a vulnerability. Do not include session transcripts, tokens, credentials, or the contents of an index database.

## What air9s stores

air9s reads session files that are already on the machine. The index keeps titles, metadata, and short excerpts. It does not upload them. Treat `index.db` as sensitive and do not commit it. Database files are gitignored.

Jules is contacted only when `AIR9S_JULES_REMOTE=1`. Other agents are read from local files. Resume runs the agent CLI you already have installed. air9s does not add an auto-approve flag unless that CLI documents one and you pass `--yolo`.

Delete removes or rewrites session data for the agents where that is supported. `delete` without `--yes` refuses to run when stdin is not a terminal. Read the agents page before using `--yes`.

Delete resolves the real path. It refuses a symlink, and it refuses a path that resolves outside that agent's session directory. Resume, and the delete commands that call an agent CLI, refuse a session id, profile, or path that starts with `-`, so a crafted id is not passed as a flag. The index file is mode `0600`. Scans open agent databases read-only. Kiro delete writes only the rows for that conversation and does not change the shell history table. A Jules cloud delete runs only when `JULES_API_KEY` is set, for that session id, and air9s does not read the Jules keyring.

`config.yaml` is created mode `0600`. Plugins run the program named in the file, with arguments, and do not pass the line to a shell. A plugin shortcut that collides with a core key is ignored. Do not put tokens or credentials in the config.

CI rejects token-shaped strings and a machine-specific home path in tracked files, and it runs `govulncheck` on Go dependencies.
