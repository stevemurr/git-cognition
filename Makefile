VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/stevemurr/git-cognition/cmd.Version=$(VERSION)

.PHONY: build test install lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o git-cognition .

test:
	go test ./...

install:
	go install -ldflags "$(LDFLAGS)" .

lint:
	go vet ./...

clean:
	rm -f git-cognition
