PREFIX ?= $(HOME)/.local
BIN    := $(PREFIX)/bin/air9s

.PHONY: build install test

build:
	go build -o air9s ./cmd/air9s

install: build
	mkdir -p $(PREFIX)/bin
	install -m 755 air9s $(BIN)

test:
	go test ./...
