#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Remove Caspian-BYOC from macOS. Stops both jobs first, which makes the
# privileged service replay its teardown journal, so the Mac's network is
# left as it was found. Run with sudo. Keeps the role account unless
# CASPIAN_REMOVE_ACCOUNT=1.
set -euo pipefail
[ "$(id -u)" -eq 0 ] || { echo "run with sudo" >&2; exit 1; }
for label in org.caspianbyoc.caspian-panel org.caspianbyoc.caspian; do
  launchctl bootout "system/$label" >/dev/null 2>&1 || true
  rm -f "/Library/LaunchDaemons/$label.plist"
done
rm -f /usr/local/bin/caspian
rm -rf /var/run/caspian "/Library/Application Support/Caspian" /Library/Logs/Caspian
if [ "${CASPIAN_REMOVE_ACCOUNT:-0}" = 1 ] && id _caspian >/dev/null 2>&1; then
  sysadminctl -deleteUser _caspian >/dev/null 2>&1 || true
fi
echo "removed"
