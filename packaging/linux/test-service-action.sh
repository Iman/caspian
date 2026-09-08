#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later
set -euo pipefail
SCRIPT="$(cd -- "$(dirname -- "$0")" && pwd)/service-action.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir "$WORK/bin"
cat > "$WORK/bin/id" <<'MOCK'
#!/bin/sh
printf '%s\n' "${TEST_UID:-0}"
MOCK
cat > "$WORK/bin/systemctl" <<'MOCK'
#!/bin/sh
printf '%s\n' "$*" >> "$TEST_CALLS"
[ "$*" != "${TEST_FAIL:-}" ]
MOCK
chmod 0755 "$WORK/bin/id" "$WORK/bin/systemctl"
export PATH="$WORK/bin:$PATH"
export TEST_CALLS="$WORK/calls"
touch "$TEST_CALLS"
bash "$SCRIPT" restart
cat > "$WORK/expected" <<'EXPECTED'
disable --now caspian-panel.service
disable --now caspian.service
enable --now caspian.service
enable --now caspian-panel.service
EXPECTED
diff -u "$WORK/expected" "$TEST_CALLS"
printf 'PASS: restart stops the panel first and starts the engine first.\n'

: > "$TEST_CALLS"
if TEST_FAIL='disable --now caspian-panel.service' bash "$SCRIPT" restart; then exit 1; fi
printf 'disable --now caspian-panel.service\n' > "$WORK/expected"
diff -u "$WORK/expected" "$TEST_CALLS"
printf 'PASS: failed stop prevents restart.\n'

: > "$TEST_CALLS"
if TEST_FAIL='enable --now caspian.service' bash "$SCRIPT" start; then exit 1; fi
printf 'enable --now caspian.service\n' > "$WORK/expected"
diff -u "$WORK/expected" "$TEST_CALLS"
printf 'PASS: failed engine start prevents panel start.\n'

: > "$TEST_CALLS"
if bash "$SCRIPT" 'start; arbitrary-command'; then exit 1; fi
if TEST_UID=1000 bash "$SCRIPT" start; then exit 1; fi
[[ ! -s "$TEST_CALLS" ]]
printf 'PASS: invalid actions and unelevated callers cannot change services.\n'
