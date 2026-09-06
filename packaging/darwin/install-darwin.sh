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
#   the _caspian role account and group (no shell, hidden)
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

# The role account. Current macOS releases require a role account UID in the
# system range, so choose a free one explicitly. A matching group is still
# required by the launchd job and by the privileged socket. Use free system UID
# and GID values and repair an account left by an interrupted older installer.
if ! id "$ACCOUNT" >/dev/null 2>&1; then
  uid=450
  while dscl . -search /Users UniqueID "$uid" | awk 'NF { found=1 } END { exit !found }'; do
    uid=$((uid + 1))
  done
  [ "$uid" -lt 500 ] || refuse "no free system UID between 450 and 499"
  sysadminctl -addUser "$ACCOUNT" -UID "$uid" -roleAccount -shell /usr/bin/false -home /var/empty >/dev/null 2>&1 ||
    refuse "creating the $ACCOUNT role account failed"
fi
if ! dscl . -read "/Groups/$ACCOUNT" >/dev/null 2>&1; then
  gid=450
  while dscl . -search /Groups PrimaryGroupID "$gid" | awk 'NF { found=1 } END { exit !found }'; do
    gid=$((gid + 1))
  done
  [ "$gid" -lt 500 ] || refuse "no free service-group GID between 450 and 499"
  dseditgroup -o create -i "$gid" "$ACCOUNT" >/dev/null ||
    refuse "creating the $ACCOUNT group failed"
fi
dseditgroup -o edit -a "$ACCOUNT" -t user "$ACCOUNT" >/dev/null ||
  refuse "adding $ACCOUNT to its service group failed"
group_id="$(dscl . -read "/Groups/$ACCOUNT" PrimaryGroupID | awk '{print $2}')"
[ -n "$group_id" ] || refuse "reading the $ACCOUNT group ID failed"
dscl . -create "/Users/$ACCOUNT" PrimaryGroupID "$group_id" ||
  refuse "making $ACCOUNT the account's primary group failed"

install -d -m 0755 -o root -g wheel "$LOGS"
# launchd opens each StandardOutPath as the job's user. Pre-create the files so
# the unprivileged panel does not need write permission on the log directory.
touch "$LOGS/caspian.log" "$LOGS/caspian-panel.log"
chown root:wheel "$LOGS/caspian.log"
chown "$ACCOUNT:$ACCOUNT" "$LOGS/caspian-panel.log"
chmod 0640 "$LOGS/caspian.log" "$LOGS/caspian-panel.log"
install -d -m 0700 -o "$ACCOUNT" -g "$ACCOUNT" "$STATE"
install -d -m 0750 -o root -g "$ACCOUNT" "$RUN"
source "$HERE/service-action.sh"
stop_job org.caspianbyoc.caspian-panel
stop_job org.caspianbyoc.caspian
install -m 0755 -o root -g wheel "$BIN_SRC" "$BIN_DST"

fresh=0
if [ ! -f "$STATE/state.json" ]; then
  fresh=1
  if [ -f "$STATE/first-run-password" ]; then
    # An interrupted install may have already generated the credential.
    # Display that same credential instead of silently hiding it on retry.
    password="$(<"$STATE/first-run-password")"
    [ "${#password}" -ge 8 ] || refuse "invalid first-run password file; use Reset Password after installation"
  else
    password="$(/usr/bin/openssl rand -hex 12)"
    [ "${#password}" -eq 24 ] || refuse "generating the first-run password failed"
    umask 077
    printf '%s' "$password" >"$STATE/first-run-password"
    chown "$ACCOUNT:$ACCOUNT" "$STATE/first-run-password"
    chmod 0600 "$STATE/first-run-password"
  fi
fi

for label in org.caspianbyoc.caspian org.caspianbyoc.caspian-panel; do
  plist="/Library/LaunchDaemons/$label.plist"
  install -m 0644 -o root -g wheel "$HERE/$label.plist" "$plist"
  start_job "$label" "$plist" || refuse "loading $label with launchd failed"

done

printf 'installed. Panel: http://127.0.0.1:8088 (the hotspot address once it is up)\n'
if [ "$fresh" -eq 1 ]; then
  printf 'first-run panel password: %s\n' "$password"
  printf 'It is consumed and deleted by the panel on its first start.\n'
fi
printf 'logs: %s/caspian.log and %s/caspian-panel.log\n' "$LOGS" "$LOGS"
