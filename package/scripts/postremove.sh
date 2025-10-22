#!/usr/bin/env sh
set -eu

# Reload systemd only; do not stop/disable service here
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

exit 0

