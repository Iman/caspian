#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
ROOT="$(cd -- "$(dirname -- "$0")" && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bundle" "$WORK/bin"
cp "$ROOT/uninstall-desktop.sh" "$WORK/bundle/"
cat > "$WORK/bin/id" <<'SCRIPT'
#!/usr/bin/env bash
printf '0\n'
SCRIPT
cat > "$WORK/bundle/uninstall.sh" <<'SCRIPT'
#!/usr/bin/env bash
printf '%s\n' "$@" > "$CASPIAN_MOCK_UNINSTALL_LOG"
exit "${CASPIAN_MOCK_UNINSTALL_STATUS:-0}"
SCRIPT
chmod +x "$WORK/bin/id"
export PATH="$WORK/bin:$PATH"
export CASPIAN_SYSROOT="$WORK/system"
export CASPIAN_MOCK_UNINSTALL_LOG="$WORK/arguments"
prepare() {
  mkdir -p "$CASPIAN_SYSROOT/opt/caspian" "$CASPIAN_SYSROOT/opt/other-app" "$CASPIAN_SYSROOT/usr/share/applications"
  touch "$CASPIAN_SYSROOT/opt/caspian/caspian_ui" "$CASPIAN_SYSROOT/usr/share/applications/caspian.desktop"
}
prepare
bash "$WORK/bundle/uninstall-desktop.sh" --keep-state --yes > /dev/null
test ! -e "$CASPIAN_SYSROOT/opt/caspian"
test ! -e "$CASPIAN_SYSROOT/usr/share/applications/caspian.desktop"
test -d "$CASPIAN_SYSROOT/opt/other-app"
test "$(cat "$CASPIAN_MOCK_UNINSTALL_LOG")" = $'--keep-state\n--yes'
printf 'PASS: successful backend uninstall removes only owned desktop files and preserves arguments\n'
prepare
status=0
CASPIAN_MOCK_UNINSTALL_STATUS=7 bash "$WORK/bundle/uninstall-desktop.sh" --keep-state > /dev/null || status=$?
test "$status" = 7
test -e "$CASPIAN_SYSROOT/opt/caspian/caspian_ui"
test -e "$CASPIAN_SYSROOT/usr/share/applications/caspian.desktop"
printf 'PASS: backend failure preserves desktop files and exit status\n'
for argument in --dry-run --help; do
  bash "$WORK/bundle/uninstall-desktop.sh" "$argument" > /dev/null
  test -e "$CASPIAN_SYSROOT/opt/caspian/caspian_ui"
  test -e "$CASPIAN_SYSROOT/usr/share/applications/caspian.desktop"
  printf 'PASS: %s preserves desktop files\n' "$argument"
done
printf '4 tests passed; 0 failed\n'
