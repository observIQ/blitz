#!/usr/bin/env sh
set -eu

# Detect action
# Note: Currently both install and remove perform the same operations

# Reload systemd only; do not start/enable/restart service
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

exit 0

