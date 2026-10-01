# Contributing

Issues and pull requests are welcome. By participating, you agree to the [code of conduct](CODE_OF_CONDUCT.md).

Do not paste session transcripts, titles from private work, tokens, or credentials into an issue, a pull request, or a test fixture. The index stores excerpts of local transcripts. Treat a bug report as something a stranger will read.

## Build and test

Requires Go 1.25 or newer.

```sh
go test ./...
make build
gofmt -w .
```

GitHub Actions runs `gofmt` and `go test -count=1 -timeout 180s ./...` on Ubuntu, for Go 1.25 and for current stable Go. `cmd/air9s/integration_test.go` builds the real binary and runs `index`, `search`, `show`, `resume --print`, and `delete` against temporary fixtures. The fixtures override every agent home, so the test does not read your sessions, and Jules is not contacted. The test skips Windows because the command stubs are POSIX shell scripts.

`make install` copies the binary to `~/.local/bin`. Override that with `make install PREFIX=/usr/local`.

## Layout

| Path | Role |
| --- | --- |
| `cmd/air9s` | CLI, prompts, and the integration test |
| `internal/discover` | One scanner per agent |
| `internal/store` | SQLite index |
| `internal/query` | Filter language |
| `internal/index` | Rebuild |
| `internal/act` | Resume and delete |
| `internal/tui` | List, views, manual, icons |
| `site` | Documentation website, published by GitHub Pages |
| `examples/plugins` | Opt-in plugins. Copy a script and its yaml into the config `plugins/` directory |

## Adding an agent

1. Add a scanner that returns `discover.Batch`, and register it in `Scanners()`.
2. Point test homes at temporary directories in `internal/discover/scan_test.go`, `internal/index/index_test.go`, and `cmd/air9s/integration_test.go`. A missing override must return no sessions and must not scan a developer's real store or call the network.
3. Teach `act.Plan` the resume command. Add a yolo flag only when that CLI documents one. Do not guess a flag.
4. Enable delete only when the removal is one transcript file, one directory that stays inside that agent's session root, a careful rewrite of a single history file, or one CLI command. Otherwise set `CanDelete` false and give the reason.
5. Add an icon in `tui.Icon` that is two columns wide. Use a glyph that product prints. If it prints a name and no glyph, use the first two letters of the name.
6. Update the agents table in `README.md` and `site/agents.html`.

A session's `SourcePath` has to be a path the scanner also returns in its file list. The index drops sessions whose source path was not part of that scan.

## Documentation

Behavior changes belong in `README.md` and on the matching page under `site/`. The site is static HTML. GitHub Pages publishes the `site` directory. There is no site build step.

## Releases

Write the notes under `## Unreleased` in `CHANGELOG.md` and commit them. From a clean `main` that is not behind origin:

```sh
make release
```

`make release` bumps `version` in `cmd/air9s/main.go`, moves those notes under the new version, runs the tests, and pushes an annotated tag. It creates the GitHub Release from those notes, then sets the Homebrew formula in [AymanZahran/homebrew-air9s](https://github.com/AymanZahran/homebrew-air9s) to that tag's archive and sha256. The formula builds the tagged source. `air9s version` prints the version constant. Do not override it with `-X`.

`make release VERSION=1.2.3` chooses that version. `make release PART=minor` or `PART=major` bumps that component. The default bump is the patch number. `make release DRY=1` prints the version and notes and changes nothing. `make release INSTALL=1` fast-forwards the tapped formula and reinstalls it. `AIR9S_TAP` selects the tap checkout when it is not the sibling `homebrew-air9s` directory.

GitHub Actions on this private repository stop at startup, so this command publishes the release with `gh` from the maintainer's machine.

## Pull requests

Keep the change focused. Run `gofmt` and `go test ./...` before you push. Commit subjects in this repository are sentence case and end with a period.

Report vulnerabilities through [GitHub Security Advisories](https://github.com/AymanZahran/air9s/security/advisories/new). See [SECURITY.md](SECURITY.md).
