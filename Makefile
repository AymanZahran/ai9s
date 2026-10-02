PREFIX ?= $(HOME)/.local
BIN    := $(PREFIX)/bin/air9s
PACKAGE := github.com/AymanZahran/air9s
BUILD_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
ifeq ($(strip $(BUILD_VERSION)),)
BUILD_VERSION := dev
endif
GIT_REV ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo dev)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X $(PACKAGE)/cmd.version=$(BUILD_VERSION) -X $(PACKAGE)/cmd.commit=$(GIT_REV) -X $(PACKAGE)/cmd.date=$(DATE)

.PHONY: build install test fmt help release

help:
	@echo "build    compile ./air9s with version, commit, and date"
	@echo "install  install to PREFIX/bin (default ~/.local/bin)"
	@echo "test     go test ./... and the release, audit, and settings tests"
	@echo "fmt      gofmt -w ."
	@echo "release  tag the next version after checks pass (VERSION=, PART=minor|major, INSTALL=1, DRY=1)"

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o air9s .

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
