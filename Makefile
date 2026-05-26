SHELL := /bin/bash

.PHONY: tools test lint security all man man-check release release-test generate-o11y generate-o11y-check test-embedded

WEAVER_IMAGE ?= otel/weaver:v0.19.0

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
	go build -o blitz ./cmd/blitz

tidy:
	go mod tidy

bench:
	go test -bench=. ./...

# Runs the embedded-library tests under -tags embed_library. The
# canonical data_library/ lives inside the embeddedlibrary package so
# //go:embed picks it up directly — no separate staging step needed.
test-embedded:
	go test -tags embed_library ./generator/filegen/embeddedlibrary/... ./generator/filegen/...

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
	@echo "Verifying generated completion scripts are up to date..."
	@git diff --quiet || (echo "Completion scripts are out of date. Run 'make completion' and commit the changes." && git --no-pager status --porcelain && exit 1)

generate-o11y-check: generate-o11y
	@echo "Verifying generated metric code is up to date..."
	@git diff --quiet || (echo "Generated metric code is out of date. Run 'make generate-o11y' and commit the changes." && git --no-pager status --porcelain && exit 1)

generate-o11y:
	@echo "Discovering metric registries..."
	@for manifest in $$(find . -name "registry_manifest.yaml" -path "*/monitoring/registry_manifest.yaml" | sort); do \
		registry_path=$$(dirname $$(dirname $$manifest) | sed 's|^\./||'); \
		depth=$$(echo "$$registry_path" | tr '/' '\n' | wc -l | tr -d ' '); \
		rel_path=""; \
		for i in $$(seq 1 $$depth); do rel_path="../$$rel_path"; done; \
		echo "Generating metrics for $$registry_path..."; \
		docker run --rm -v ${PWD}:/workspace -w /workspace/$$registry_path $(WEAVER_IMAGE) registry check --registry=./monitoring; \
		docker run --rm -v ${PWD}:/workspace -w /workspace/$$registry_path $(WEAVER_IMAGE) registry generate --registry=./monitoring --templates=$${rel_path}weaver/templates --config=$${rel_path}weaver-go.yaml go .; \
		docker run --rm -v ${PWD}:/workspace -w /workspace/$$registry_path $(WEAVER_IMAGE) registry generate --registry=./monitoring --templates=$${rel_path}weaver/templates --config=$${rel_path}weaver-markdown.yaml markdown .; \
		echo "✓ Generated $$registry_path/monitoring.go and $$registry_path/monitoring.md"; \
	done
	@go fmt ./...

release-test:
	@source ./scripts/set-build-host.sh && goreleaser release --clean --skip=publish --parallelism=2 --skip=sign --snapshot

