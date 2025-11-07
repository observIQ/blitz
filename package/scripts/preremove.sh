#!/usr/bin/env sh
set -eu

# Stop the blitz service before removal
if command -v systemctl >/dev/null 2>&1; then
  systemctl stop blitz.service || true
fi

exit 0

