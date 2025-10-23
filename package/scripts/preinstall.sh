#!/usr/bin/env sh
set -eu

for cmd in getent id groupadd useradd usermod; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Required command '$cmd' not found" >&2; exit 1; }
done

if ! getent group blitz >/dev/null 2>&1; then
  groupadd -r blitz
fi

if ! id -u blitz >/dev/null 2>&1; then
  useradd -r -g blitz -s "/sbin/nologin" -d /nonexistent -M blitz
fi

systemctl daemon-reload