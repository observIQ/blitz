#!/usr/bin/env bash
# Fail if any non-test Go file uses the global logger (zap.L() / zap.S()).
# Blitz components must log through an injected *zap.Logger so an embedding
# host controls the log stream (PIPE-1067).
set -euo pipefail

matches="$(grep -rnE 'zap\.(L|S)\(\)' --include='*.go' . | grep -v '_test\.go' || true)"
if [ -n "$matches" ]; then
  echo "Global-logger usage is banned; use an injected *zap.Logger instead:" >&2
  echo "$matches" >&2
  exit 1
fi
echo "OK: no zap.L() / zap.S() global-logger usage in non-test Go files."
