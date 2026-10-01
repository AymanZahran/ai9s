PREFIX ?= $(HOME)/.local
BIN    := $(PREFIX)/bin/air9s

.PHONY: build install test fmt help

help:
	@echo "build    compile ./air9s"
	@echo "install  install to PREFIX/bin (default ~/.local/bin)"
	@echo "test     go test ./..."
	@echo "fmt      gofmt -w ."

build:
	go build -o air9s ./cmd/air9s

install: build
	mkdir -p $(PREFIX)/bin
	install -m 755 air9s $(BIN)

test:
	go test ./...

fmt:
	gofmt -w .
