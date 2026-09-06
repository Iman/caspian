#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail

stop_job() {
  local label="$1" description pid attempt
  if ! description=$(launchctl print "system/$label" 2>/dev/null); then return 0; fi
  pid=$(printf '%s\n' "$description" | awk '$1 == "pid" && $2 == "=" {print $3; exit}')
  launchctl bootout "system/$label" || return 1
  # bootout can return before the process has finished its network cleanup.
  # Do not bootstrap another owner of the socket, tunnel, or hotspot yet.
  for attempt in {1..150}; do
    if ! launchctl print "system/$label" >/dev/null 2>&1; then
      if [[ -z "$pid" ]] || ! kill -0 "$pid" 2>/dev/null; then return 0; fi
    fi
    sleep 0.2
  done
  echo "Timed out stopping $label. Services were not restarted." >&2
  return 1
}

start_job() {
  local label="$1" plist="$2" attempt loaded=0
  launchctl enable "system/$label" || return 1
  if ! launchctl print "system/$label" >/dev/null 2>&1; then
    for attempt in {1..5}; do
      if launchctl bootstrap system "$plist"; then loaded=1; break; fi
      sleep 1
    done
    [[ "$loaded" == 1 ]] || return 1
  fi
  launchctl kickstart "system/$label"
}

main() {
  [[ $(uname -s) == Darwin && $(id -u) == 0 ]] || return 1
  case "${1:-}" in start|stop|restart) ;; *) return 2 ;; esac
  local label
  if [[ "$1" != stop ]]; then
  for label in org.caspianbyoc.caspian org.caspianbyoc.caspian-panel; do
    [[ -f "/Library/LaunchDaemons/$label.plist" ]] || { echo 'Install Caspian first.'; return 1; }
  done
  fi
  if [[ "$1" != start ]]; then
    for label in org.caspianbyoc.caspian-panel org.caspianbyoc.caspian; do
      stop_job "$label" || return 1
    done
  fi
  if [[ "$1" != stop ]]; then
    for label in org.caspianbyoc.caspian org.caspianbyoc.caspian-panel; do
      start_job "$label" "/Library/LaunchDaemons/$label.plist" || return 1
    done
    echo 'Services started. Open the panel to configure and switch on the hotspot.'
  else
    echo 'Services stopped. The hotspot and web panel are unavailable. Use Start services here to restore access.'
  fi
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
