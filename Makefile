BINARY   := jnl
MODULE   := $(shell head -1 go.mod | awk '{print $$2}')
LDFLAGS  := -ldflags="-s -w"
DIST     := dist

.PHONY: all linux darwin-amd64 darwin-arm64 windows install clean fmt vet

all: linux darwin-amd64 darwin-arm64 windows

linux:
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o $(DIST)/jnl-linux-amd64   .

darwin-amd64:
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o $(DIST)/jnl-macos-amd64   .

darwin-arm64:
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o $(DIST)/jnl-macos-arm64   .

windows:
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o $(DIST)/jnl.exe           .

# Build for the current machine and install to ~/.local/bin/jnl-go
install:
	go build $(LDFLAGS) -o ~/.local/bin/jnl-go .
	@echo "Installed → ~/.local/bin/jnl-go"

clean:
	rm -rf $(DIST)

fmt:
	gofmt -w .

vet:
	go vet ./...

# Download dependencies (run once after clone)
deps:
	go mod download
	go mod tidy
