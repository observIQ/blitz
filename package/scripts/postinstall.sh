#!/usr/bin/env sh
set -eu

# Detect action
# Note: Currently both install and remove perform the same operations
# (config dir setup, systemd reload)

# Ensure config dir ownership remains correct
mkdir -p /etc/bindplane-loader
chown -R bploader:bploader /etc/bindplane-loader || true

# Reload systemd only; do not start/enable/restart service
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

exit 0

