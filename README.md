# ai9s

ai9s is a keyboard-first finder for local AI coding sessions. This release is the module, the license, CI, the release workflow, the site, and the Homebrew formula. `ai9s version` and `ai9s info` are the commands in this build.

## Install

### Homebrew

```sh
brew tap AymanZahran/ai9s https://github.com/AymanZahran/ai9s
brew trust aymanzahran/ai9s
brew install ai9s
```

The formula is `Formula/ai9s.rb` in this repository. `brew install --HEAD ai9s` builds the latest `main` once this repository can be cloned over HTTPS.

While this repository is private, tap over SSH, trust the tap, and export a GitHub token so Homebrew can download the release archive. Install that tagged formula. `--HEAD` cannot clone the private source repository over HTTPS.

```sh
brew tap AymanZahran/ai9s git@github.com:AymanZahran/ai9s.git
brew trust aymanzahran/ai9s
export HOMEBREW_GITHUB_API_TOKEN="$(gh auth token)"
brew install ai9s
```

### From source

Requires Go 1.25 or newer.

```sh
git clone https://github.com/AymanZahran/ai9s.git
cd ai9s
make install
```

`make install` puts the binary in `~/.local/bin`. Override that with `make install PREFIX=/usr/local`.

```sh
go install github.com/AymanZahran/ai9s@latest
```

