# Changelog

## Unreleased

## 1.0.1

- Install instructions use the public tap URL.
- Branch protection sends the status-check names GitHub still accepts.
- The README and the website show the interface, drawn from example sessions.
- A release opens a pull request for the changelog and another for the formula.
- CI tests on macOS and Windows, and runs golangci-lint.
- Windows opens the SQLite index, matches a directory with either slash, replaces an open history file, and restores a dashed drive path.
- Future releases sign the checksum file with keyless cosign.
- The security policy names 1.0.x as the supported release.
- An administrator can merge a pull request without waiting for the required checks.

## 1.0.0

- First release intended for a public repository. Install this version.

## 0.6.1

- Starting ai9s does not print `ai9s: indexing sessions`.

## 0.6.0

- Every column except NAME is cut to a fixed width. A path keeps its ending. NAME stays whole and last, and the row pans. Describe shows the full value.

## 0.5.0

- `n` renames the selected session. The name stays in the index through a reindex. An empty name restores the default.
- `u` shows usage for the selected session. `s` still shows counts for every session.

## 0.4.0

- Started in a project directory, the list shows sessions for that directory and its subdirectories. Started in your home directory, the list shows every session.

## 0.3.0

- The keyboard UI lists, filters, describes, resumes, and deletes sessions.
- The session list has no TITLE column and no two-letter agent column. NAME is the last column. `sort:title` sorts NAME.
- Session text is escaped before it is drawn. Control characters are left out of human output and are not passed to an agent CLI.

## 0.2.0

- ai9s indexes local AI coding sessions and can search, show, resume, and delete them from the command line.

## 0.1.0

- Scaffold of the module, license, CI, release workflow, site, and Homebrew formula.
