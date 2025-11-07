SHELL := /bin/bash

.PHONY: tools test lint security all man man-check release release-test

TOOLS := \
    github.com/securego/gosec/v2/cmd/gosec@latest \
    github.com/mgechev/revive@latest \
    github.com/cpuguy83/go-md2man/v2@latest

all: install-tools test lint security

install-tools:
	go get -tool github.com/securego/gosec/v2/cmd/gosec
	go get -tool github.com/mgechev/revive
	go get -tool github.com/cpuguy83/go-md2man/v2
	go tool -n gosec >/dev/null
	go tool -n revive >/dev/null
	go tool -n go-md2man >/dev/null

test:
	go test ./...

lint:
	go tool revive -config .revive.toml -formatter friendly ./...

security:
	go tool gosec ./...

build:
	go build -o blitz ./cmd/blitz/main.go

tidy:
	go mod tidy

bench:
	go test -bench=. ./...

man:
	bash ./scripts/gen-man.sh

# CI helper: regenerate man pages and fail if the repo becomes dirty
man-check: man
	@echo "Verifying generated man pages are up to date..."
	@git diff --quiet || (echo "Man pages are out of date. Run 'make man' and commit the changes." && git --no-pager status --porcelain && exit 1)

release-test:
	@eval $$(./scripts/set-build-host.sh) && goreleaser release --clean --skip=publish --parallelism=2 --skip=sign --snapshot