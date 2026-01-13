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

completion:
	@echo "Generating bash and zsh completion scripts..."
	@mkdir -p package/completions
	@go run ./cmd/blitz completion bash > package/completions/blitz.bash || (echo "Failed to generate bash completion script. Ensure blitz binary can be built." && exit 1)
	@go run ./cmd/blitz completion zsh > package/completions/blitz.zsh || (echo "Failed to generate zsh completion script. Ensure blitz binary can be built." && exit 1)
	@echo "✓ Bash and zsh completions generated successfully"

# CI helper: regenerate completions and fail if the repo becomes dirty
completion-check: completion
	@echo "Verifying generated completion script is up to date..."
	@git diff --quiet || (echo "Completion script is out of date. Run 'make completion' and commit the changes." && git --no-pager status --porcelain && exit 1)

release-test:
	@source ./scripts/set-build-host.sh && goreleaser release --clean --skip=publish --parallelism=2 --skip=sign --snapshot