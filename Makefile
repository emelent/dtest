.PHONY: build run clean install fmt vet test

BINARY  := dtest
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

clean:
	rm -f $(BINARY)

install: build
	sudo mv $(BINARY) /usr/local/bin/

fmt:
	gofmt -l -w .

vet:
	go vet ./...

test:
	go test ./...
