#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")" && pwd)"
PREVIEW=0
for argument in "$@"; do
  case "$argument" in --dry-run|--help|-h) PREVIEW=1 ;; esac
done
if [ "$(id -u)" != 0 ] && [ "$PREVIEW" != 1 ]; then
  exec pkexec bash "$ROOT/uninstall-desktop.sh" "$@"
fi
# Keep the existing state prompts, network restoration, and failure behavior.
bash "$ROOT/uninstall.sh" "$@"
if [ "$PREVIEW" = 1 ]; then
  printf 'Desktop cleanup would remove /opt/caspian and /usr/share/applications/caspian.desktop.\n'
  exit 0
fi
# CASPIAN_SYSROOT is the existing backend uninstaller's test filesystem hook.
# That uninstaller rejects it outside dry-run; mock tests replace only the
# backend boundary so cleanup can be checked without touching installed files.
rm -f -- "${CASPIAN_SYSROOT:-}/usr/share/applications/caspian.desktop"
rm -rf -- "${CASPIAN_SYSROOT:-}/opt/caspian"
printf 'The Caspian desktop application was removed.\n'
