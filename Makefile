BINARY   := jnl
MODULE   := $(shell head -1 go.mod | awk '{print $$2}')
LDFLAGS  := -ldflags="-s -w"
DIST     := dist

.PHONY: all linux linux-arm64 darwin-amd64 darwin-arm64 install clean fmt vet deps

all: linux linux-arm64 darwin-amd64 darwin-arm64

linux:
	GOOS=linux   GOARCH=amd64   go build $(LDFLAGS) -o $(DIST)/jnl-linux-amd64  .

linux-arm64:
	GOOS=linux   GOARCH=arm64   go build $(LDFLAGS) -o $(DIST)/jnl-linux-arm64  .

darwin-amd64:
	GOOS=darwin  GOARCH=amd64   go build $(LDFLAGS) -o $(DIST)/jnl-macos-amd64  .

darwin-arm64:
	GOOS=darwin  GOARCH=arm64   go build $(LDFLAGS) -o $(DIST)/jnl-macos-arm64  .

install:
	go build $(LDFLAGS) -o ~/.local/bin/jnl .
	@echo "Installed → ~/.local/bin/jnl"

clean:
	rm -rf $(DIST)

fmt:
	gofmt -w .

vet:
	go vet ./...

deps:
	go mod download
	go mod tidy
