SHELL := /bin/bash

.PHONY: tools test lint security all

TOOLS := \
    github.com/securego/gosec/v2/cmd/gosec@latest \
    github.com/mgechev/revive@latest

all: tools test lint security

install-tools:
	@echo "Installing tools..."
	@for tool in $(TOOLS); do \
		echo "Installing $$tool"; \
		go install $$tool; \
	done

test:
	go test ./...

lint:
	@command -v revive >/dev/null 2>&1 || { echo >&2 "revive not found, run 'make install-tools'"; exit 1; }
	revive -config .revive.toml -formatter friendly ./...

security:
	@command -v gosec >/dev/null 2>&1 || { echo >&2 "gosec not found, run 'make install-tools'"; exit 1; }
	gosec ./...

build:
	go build -o bindplane-loader ./cmd/loader/main.go
