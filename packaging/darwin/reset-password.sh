#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
[[ $(uname -s) == Darwin && $(id -u) == 0 ]] || { echo 'Run on the Mac with administrator privileges.' >&2; exit 1; }
job=org.caspianbyoc.caspian-panel
plist=/Library/LaunchDaemons/org.caspianbyoc.caspian-panel.plist
[[ -f "$plist" && -f '/Library/Application Support/Caspian/state.json' ]] || { echo 'Install Caspian first.' >&2; exit 1; }
panel_password=$(/usr/bin/openssl rand -hex 12)
restore_panel() { /bin/launchctl bootstrap system "$plist"; }
if /bin/launchctl print "system/$job" >/dev/null 2>&1; then
  /bin/launchctl bootout "system/$job"
fi
trap restore_panel EXIT
if /bin/launchctl print "system/$job" >/dev/null 2>&1; then
  echo 'The panel could not be stopped; password was not changed.' >&2
  exit 1
fi
printf '%s\n%s\n' "$panel_password" "$panel_password" | /usr/bin/sudo -n -u _caspian /usr/local/bin/caspian reset-panel-password
restore_panel
trap - EXIT
printf 'New Caspian panel password: %s\nSave this password before opening the panel.\n' "$panel_password"
