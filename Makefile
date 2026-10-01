PREFIX ?= $(HOME)/.local
BIN    := $(PREFIX)/bin/air9s

.PHONY: build install test fmt help release

help:
	@echo "build    compile ./air9s"
	@echo "install  install to PREFIX/bin (default ~/.local/bin)"
	@echo "test     go test ./... and the release script tests"
	@echo "fmt      gofmt -w ."
	@echo "release  publish the next GitHub release (VERSION=, PART=minor|major, INSTALL=1, DRY=1)"

build:
	go build -o air9s ./cmd/air9s

install: build
	mkdir -p $(PREFIX)/bin
	install -m 755 air9s $(BIN)

test:
	go test ./...
	python3 -m unittest scripts/release_test.py

release:
	python3 scripts/release.py $(VERSION) $(PART) $(if $(filter 1,$(INSTALL)),--install,) $(if $(filter 1,$(DRY)),--dry-run,)

fmt:
	gofmt -w .
