#!/usr/bin/env sh
set -eu

# Detect action: install/upgrade vs remove
# Note: Currently both install and remove perform the same operations
# (user/group creation, config dir setup, systemd reload)

# Create user and group if missing; ensure no login shell
NOLOGIN="/usr/sbin/nologin"
[ -x "$NOLOGIN" ] || NOLOGIN="/sbin/nologin"

if ! getent group bploader >/dev/null 2>&1; then
  addgroup -S bploader 2>/dev/null || groupadd -r bploader || true
fi
if ! id -u bploader >/dev/null 2>&1; then
  adduser -S -G bploader -H -D -s "$NOLOGIN" bploader 2>/dev/null || \
  useradd -r -g bploader -s "$NOLOGIN" -d /nonexistent bploader || true
else
  # Ensure shell is set to nologin for existing user
  if command -v usermod >/dev/null 2>&1; then
    usermod -s "$NOLOGIN" bploader || true
  fi
fi

# Ensure config dir and ownership
mkdir -p /etc/bindplane-loader
chown -R bploader:bploader /etc/bindplane-loader || true

# Reload systemd if present (install/upgrade/remove)
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

exit 0

