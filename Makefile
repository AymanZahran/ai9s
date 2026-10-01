PREFIX ?= $(HOME)/.local
BIN    := $(PREFIX)/bin/air9s

.PHONY: build install test fmt help release

help:
	@echo "build    compile ./air9s"
	@echo "install  install to PREFIX/bin (default ~/.local/bin)"
	@echo "test     go test ./... and the release, audit, and settings tests"
	@echo "fmt      gofmt -w ."
	@echo "release  open a release pull request and merge it after checks pass (VERSION=, PART=minor|major, INSTALL=1, DRY=1)"

build:
	go build -o air9s ./cmd/air9s

install: build
	mkdir -p $(PREFIX)/bin
	install -m 755 air9s $(BIN)

test:
	go test ./...
	PYTHONDONTWRITEBYTECODE=1 python3 -m unittest scripts/release_test.py scripts/audit_test.py scripts/public_github_test.py
	PYTHONDONTWRITEBYTECODE=1 python3 scripts/audit.py

release:
	python3 scripts/release.py $(VERSION) $(PART) $(if $(filter 1,$(INSTALL)),--install,) $(if $(filter 1,$(DRY)),--dry-run,)

fmt:
	gofmt -w .
