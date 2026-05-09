# ── Kraken TUI Makefile ───────────────────────────────────────────────────────
BINARY  := kraken
MODULE  := github.com/faran17/kraken-tui
LDFLAGS := -ldflags="-s -w"

# Detect current OS/arch for the default 'build' target
GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: all build run tidy clean cross help

## all: tidy dependencies then build for current platform
all: tidy build

## build: compile for current platform
build:
	go build $(LDFLAGS) -o $(BINARY) .

## run: build and immediately run
run: build
	./$(BINARY)

## tidy: download and tidy dependencies
tidy:
	go mod tidy

## cross: build for all platforms (macOS arm64/amd64, Linux amd64, Windows amd64)
cross: tidy
	@echo "Building macOS arm64…"
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  .
	@echo "Building macOS amd64…"
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64  .
	@echo "Building Linux amd64…"
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   .
	@echo "Building Linux arm64…"
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64   .
	@echo "Building Windows amd64…"
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe .
	@echo "Done! Binaries in ./dist/"

## clean: remove compiled binaries
clean:
	rm -f $(BINARY)
	rm -rf dist/
	rm -rf Kraken.app

## app: package as a clickable macOS launcher
app: build
	@echo "Creating Kraken.app bundle..."
	@mkdir -p Kraken.app/Contents/MacOS
	@mkdir -p Kraken.app/Contents/Resources
	@cp $(BINARY) Kraken.app/Contents/MacOS/kraken-binary
	@echo '#!/bin/bash\n# Get the directory where the script is located\nDIR="$$(cd "$$(dirname "$$0")" && pwd)"\n# Open a new terminal window and run the binary\nosascript -e "tell application \\"Terminal\\" to do script \\"$$DIR/kraken-binary; exit\\""' > Kraken.app/Contents/MacOS/Kraken
	@chmod +x Kraken.app/Contents/MacOS/Kraken
	@cp KrakenTUIv2.0.png Kraken.app/Contents/Resources/icon.png
	@echo "Kraken.app created successfully! Double-click it in Finder to run."

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
