VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build install install-plugin test clean

build:
	CGO_ENABLED=1 CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go build $(LDFLAGS) -o bin/claudio-codex ./cmd/claudio-codex

install:
	CGO_ENABLED=1 CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go install $(LDFLAGS) ./cmd/claudio-codex

install-plugin: build
	mkdir -p ~/.claudio/plugins
	cp bin/claudio-codex ~/.claudio/plugins/claudio-codex

test:
	CGO_ENABLED=1 CGO_CFLAGS="-DSQLITE_ENABLE_FTS5" go test ./...

clean:
	rm -rf bin/
