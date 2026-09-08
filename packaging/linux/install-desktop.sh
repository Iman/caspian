#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")" && pwd)"
if [ "$(id -u)" != 0 ]; then exec pkexec bash "$ROOT/install-desktop.sh" "$@"; fi
test -x "$ROOT/caspian_ui"
test -s "$ROOT/backend/caspian"
# Check dynamic libraries before installing services. The desktop archive uses
# the host's GTK and C++ runtime; a headless Pi uses the web application.
LIBRARIES="$(ldd "$ROOT/caspian_ui" "$ROOT/lib/libflutter_linux_gtk.so" 2>&1)" || {
  printf '%s\n' "$LIBRARIES" >&2
  exit 1
}
if [[ "$LIBRARIES" == *'not found'* ]]; then
  printf 'Install the missing desktop libraries before installing Caspian:\n%s\n' "$LIBRARIES" >&2
  exit 1
fi
# Install or upgrade the existing service without changing its state ownership.
CASPIAN_LOCAL_BINARY="$ROOT/backend/caspian" \
CASPIAN_LOCAL_CHECKSUMS="$ROOT/backend/SHA256SUMS" \
CASPIAN_UNINSTALL_SRC="$ROOT/uninstall.sh" bash "$ROOT/install.sh" --yes
if [ "$ROOT" != /opt/caspian ]; then
  mkdir -p /opt/caspian
  cp -R "$ROOT/." /opt/caspian/
  chown -R root:root /opt/caspian
  chmod -R go-w /opt/caspian
fi
install -m 0644 "$ROOT/caspian.desktop" /usr/share/applications/caspian.desktop
printf 'Open Caspian from your applications menu and sign in with the password above.\n'
