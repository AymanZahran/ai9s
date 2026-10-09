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

GitHub Actions runs the required checks on Ubuntu. One job uses Go 1.25 and one uses current stable Go. Each runs `gofmt`, `go test -count=1 -timeout 180s ./...`, and the Python tests under `scripts/`. A separate job runs `govulncheck` with Go 1.26.9, another scans tracked files for token-shaped strings and a machine-specific home path, and another runs `golangci-lint`. macOS and Windows run `go test` as well. Those extra jobs are not required status checks, so a rename does not change branch protection. `cmd/integration_test.go` builds the real binary and runs `index`, `search`, `show`, `resume --print`, and `delete` against temporary fixtures. The fixtures override every agent home, so the test does not read your sessions, and Jules is not contacted. That test skips Windows because the command stubs are POSIX shell scripts. The example plugin scripts skip Windows for the same reason.

`make install` copies the binary to `~/.local/bin`. Override that with `make install PREFIX=/usr/local`.

## Layout

| Path | Role |
| --- | --- |
| `main.go` and `cmd/` | CLI, prompts, and the integration test |
| `internal/discover` | One scanner per agent |
| `internal/store` | SQLite index |
| `internal/query` | Filter language |
| `internal/index` | Rebuild |
| `internal/act` | Resume and delete |
| `internal/tui` | List, views, manual, icons |
| `site` | Documentation website, published by GitHub Pages |
| `examples/plugins` | Opt-in plugins. Copy a script and its yaml into the config `plugins/` directory |

## Adding a harness

1. Add a scanner that returns `discover.Batch`, and register it in `Scanners()`.
2. Point test homes at temporary directories in `internal/discover/scan_test.go`, `internal/index/index_test.go`, and `cmd/integration_test.go`. A missing override must return no sessions and must not scan a developer's real store or call the network.
3. Teach `act.Plan` the resume command. Add a yolo flag only when that CLI documents one. Do not guess a flag.
4. Enable delete only when the removal is one transcript file, one directory that stays inside that agent's session root, a careful rewrite of a single history file, or one CLI command. Otherwise set `CanDelete` false and give the reason.
5. Add a unique two-letter mark in `tui.Icon`. Letters only, two columns wide. An unknown agent stays `??`.
6. Update the harnesses table in `README.md` and `site/agents.html`.

A session's `SourcePath` has to be a path the scanner also returns in its file list. The index drops sessions whose source path was not part of that scan.

## Documentation

Behavior changes belong in `README.md` and on the matching page under `site/`. The site is static HTML. GitHub Pages publishes the `site` directory. There is no site build step. Interface pictures live in `site/assets/shots/`. They are drawn from fixture sessions by `go test -tags shots -run TestWriteScreenshots ./internal/tui`. They are not recordings of a developer's sessions.

## Pull requests

Every feature change lands through a pull request. Merge it only after the checks on that pull request have passed. Do not push feature commits straight to `main`. A release tag is pushed after its pull requests merge.

Keep the change focused. Run `gofmt` and `go test ./...` before you push. Commit subjects in this repository are sentence case and end with a period.

Report vulnerabilities through [GitHub Security Advisories](https://github.com/AymanZahran/ai9s/security/advisories/new). See [SECURITY.md](SECURITY.md).

## Releases

Write the notes under `## Unreleased` in `CHANGELOG.md` and merge that commit first. From a clean `main` that matches origin:

```sh
make release
```

`make release` reads the latest `vX.Y.Z` tag. When `## Unreleased` has notes, it opens a pull request that moves those notes under the new version, waits for the checks, and squash-merges it. It then tags that main commit and pushes the tag. GoReleaser, in [`.github/workflows/release.yml`](.github/workflows/release.yml), builds the binaries, the checksum file, and the GitHub Release. From that tag onward, Cosign signs the checksum file with the GitHub Actions identity. Nothing in the source file stores the release number. `make build` and the formula pass it with `-ldflags -X`.

The formula update is a second pull request, after the tag archive exists, so the checksum matches the bytes GitHub serves. `Formula/ai9s.rb` downloads the public tag archive and passes `-X github.com/AymanZahran/ai9s/cmd.version`. A token header is added only when `HOMEBREW_GITHUB_API_TOKEN` is set.

If a check fails, or GitHub reports none, the command stops and does not create the tag.

`make release VERSION=1.2.3` chooses that version. `make release PART=minor` or `PART=major` bumps that component. The default bump is the patch number. `make release DRY=1` prints the version and notes and changes nothing. `make release INSTALL=1` fast-forwards the tapped checkout and reinstalls ai9s after the formula pull request merges.

Verify a checksum from a signed release:

```sh
cosign verify-blob \
  --certificate-identity "https://github.com/AymanZahran/ai9s/.github/workflows/release.yml@refs/tags/vX.Y.Z" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  --bundle checksums.sha256.sigstore.json \
  checksums.sha256
```

## Repository settings

The repository is public. Branch protection, secret scanning, push protection, private vulnerability reporting, and GitHub Pages are already on. An administrator can merge a pull request without waiting for the required checks. Force pushes stay off. `scripts/public_github.py` reapplies those settings. It does not change visibility.

```sh
python3 scripts/public_github.py
```
