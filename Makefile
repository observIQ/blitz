SHELL := /bin/bash

.PHONY: tools test lint security all

TOOLS := \
    github.com/securego/gosec/v2/cmd/gosec@latest \
    github.com/mgechev/revive@latest

all: install-tools test lint security

install-tools:
	go get -tool github.com/securego/gosec/v2/cmd/gosec
	go get -tool github.com/mgechev/revive
	go tool -n gosec >/dev/null
	go tool -n revive >/dev/null

test:
	go test ./...

lint:
	go tool revive -config .revive.toml -formatter friendly ./...

security:
	go tool gosec ./...

build:
	go build -o bindplane-loader ./cmd/loader/main.go

tidy:
	go mod tidy

bench:
	go test -bench=. ./...