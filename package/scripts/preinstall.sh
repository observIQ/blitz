#!/usr/bin/env sh
set -eu

NOLOGIN="/usr/sbin/nologin"
[ -x "$NOLOGIN" ] || NOLOGIN="/sbin/nologin"

for cmd in getent id groupadd useradd usermod; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Required command '$cmd' not found" >&2; exit 1; }
done

if ! getent group bploader >/dev/null 2>&1; then
  groupadd -r bploader
fi

if ! id -u bploader >/dev/null 2>&1; then
  useradd -r -g bploader -s "$NOLOGIN" -d /nonexistent -M bploader
fi

systemctl daemon-reload