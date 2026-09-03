#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Install Caspian-BYOC on macOS. Run with sudo, with the binary you built:
#
#   go build -trimpath -o caspian ./cmd/caspian
#   sudo CASPIAN_LOCAL_BINARY="$PWD/caspian" bash packaging/darwin/install-darwin.sh
#
# What it creates, and nothing else:
#   /usr/local/bin/caspian                            0755 root:wheel
#   /Library/Application Support/Caspian              0700 _caspian
#   /Library/Application Support/Caspian/first-run-password  0600 _caspian (fresh install only)
#   /var/run/caspian                                  0750 root:_caspian (recreated at every start)
#   /Library/Logs/Caspian                             0755 root, logs 0640
#   /Library/LaunchDaemons/org.caspianbyoc.caspian.plist        root half
#   /Library/LaunchDaemons/org.caspianbyoc.caspian-panel.plist  panel, as _caspian
#   the _caspian role account (UID 450 to 499, no shell, hidden)
#
# It refuses anything that is not macOS, anything not run as root, and a
# missing binary. It is idempotent: running it twice is an upgrade.
set -euo pipefail

BIN_SRC="${CASPIAN_LOCAL_BINARY:-}"
BIN_DST=/usr/local/bin/caspian
STATE="/Library/Application Support/Caspian"
RUN=/var/run/caspian
LOGS=/Library/Logs/Caspian
ACCOUNT=_caspian
HERE="$(cd -- "$(dirname -- "$0")" && pwd)"

refuse() { printf 'caspian: %s\n' "$1" >&2; exit 1; }

[ "$(uname -s)" = Darwin ] || refuse "this installer is for macOS; found $(uname -s)"
[ "$(id -u)" -eq 0 ] || refuse "run with sudo"
[ -n "$BIN_SRC" ] && [ -f "$BIN_SRC" ] || refuse "set CASPIAN_LOCAL_BINARY to the caspian binary you built"

# The role account. sysadminctl wants a name starting with an underscore and
# a UID in 450 to 499; the first free one is taken.
if ! id "$ACCOUNT" >/dev/null 2>&1; then
  uid=450
  while id "$uid" >/dev/null 2>&1 && [ "$uid" -lt 500 ]; do uid=$((uid + 1)); done
  [ "$uid" -lt 500 ] || refuse "no free role-account UID between 450 and 499"
  sysadminctl -addUser "$ACCOUNT" -roleAccount -UID "$uid" -shell /usr/bin/false -home /var/empty >/dev/null 2>&1 ||
    refuse "creating the $ACCOUNT role account failed"
fi

install -d -m 0755 -o root -g wheel "$LOGS"
install -d -m 0700 -o "$ACCOUNT" -g "$ACCOUNT" "$STATE"
install -d -m 0750 -o root -g "$ACCOUNT" "$RUN"
install -m 0755 -o root -g wheel "$BIN_SRC" "$BIN_DST"

fresh=0
if [ ! -f "$STATE/state.json" ] && [ ! -f "$STATE/first-run-password" ]; then
  fresh=1
  password="$(LC_ALL=C tr -dc 'a-z0-9' </dev/urandom | head -c 20)"
  umask 077
  printf '%s' "$password" >"$STATE/first-run-password"
  chown "$ACCOUNT:$ACCOUNT" "$STATE/first-run-password"
  chmod 0600 "$STATE/first-run-password"
fi

for label in org.caspianbyoc.caspian org.caspianbyoc.caspian-panel; do
  plist="/Library/LaunchDaemons/$label.plist"
  launchctl bootout "system/$label" >/dev/null 2>&1 || true
  install -m 0644 -o root -g wheel "$HERE/$label.plist" "$plist"
  launchctl bootstrap system "$plist"
  launchctl enable "system/$label"
done

printf 'installed. Panel: http://127.0.0.1:8088 (the hotspot address once it is up)\n'
if [ "$fresh" -eq 1 ]; then
  printf 'first-run panel password: %s\n' "$password"
  printf 'It is consumed and deleted by the panel on its first start.\n'
fi
printf 'logs: %s/caspian.log and %s/caspian-panel.log\n' "$LOGS" "$LOGS"
