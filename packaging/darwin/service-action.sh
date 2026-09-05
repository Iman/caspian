#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
[[ $(uname -s) == Darwin && $(id -u) == 0 ]] || exit 1
case "${1:-}" in start|stop|restart) ;; *) exit 2 ;; esac
for label in org.caspianbyoc.caspian org.caspianbyoc.caspian-panel; do
  [[ -f "/Library/LaunchDaemons/$label.plist" ]] || { echo 'Install Caspian first.'; exit 1; }
done
if [[ "$1" != start ]]; then
  for label in org.caspianbyoc.caspian-panel org.caspianbyoc.caspian; do
    if launchctl print "system/$label" >/dev/null 2>&1; then launchctl bootout "system/$label"; fi
  done
fi
if [[ "$1" != stop ]]; then
  for label in org.caspianbyoc.caspian org.caspianbyoc.caspian-panel; do
    launchctl enable "system/$label"
    if ! launchctl print "system/$label" >/dev/null 2>&1; then
      launchctl bootstrap system "/Library/LaunchDaemons/$label.plist"
    fi
    launchctl kickstart "system/$label"
  done
  echo 'Services started. Open the panel to configure and switch on the hotspot.'
else
  echo 'Services stopped. The hotspot and web panel are unavailable. Use Start services here to restore access.'
fi
