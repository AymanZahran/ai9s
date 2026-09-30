PREFIX ?= $(HOME)/.local
BIN    := $(PREFIX)/bin/ai9s

.PHONY: build install test

build:
	go build -o ai9s ./cmd/ai9s

install: build
	mkdir -p $(PREFIX)/bin
	install -m 755 ai9s $(BIN)

test:
	go test ./...
