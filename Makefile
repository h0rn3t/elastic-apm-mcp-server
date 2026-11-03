BINARY := elastic-apm-mcp-server
VERSION := 0.2.0

.PHONY: build install clean test

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/elastic-apm-mcp-server/

install:
	go build -ldflags "-X main.version=$(VERSION)" -o /usr/local/bin/$(BINARY) ./cmd/elastic-apm-mcp-server/

test:
	go test ./...

clean:
	rm -f $(BINARY)
