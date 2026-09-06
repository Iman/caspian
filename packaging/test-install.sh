#!/usr/bin/env bash
#
# Tests for install.sh and uninstall.sh.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# These run on any machine with bash, and in particular on a developer machine
# that is not Linux and cannot be installed to. They prove the parts that can
# be proved without a Raspberry Pi: the argument and architecture mapping, the
# refusals, the SHA-256 verification, the password generator, the teardown
# journal replay, and that the unit files embedded in install.sh have not
# drifted from the ones in packaging/.
#
# What they cannot prove is listed in docs/INSTALL.md under "What these tests
# do not cover".
#
# Usage: bash packaging/test-install.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SH="${REPO_ROOT}/install.sh"
UNINSTALL_SH="${REPO_ROOT}/uninstall.sh"

PASSED=0
FAILED=0

pass() {
  PASSED=$((PASSED + 1))
  printf 'ok   %s\n' "$1"
}

fail() {
  FAILED=$((FAILED + 1))
  printf 'FAIL %s\n' "$1"
  if [ $# -gt 1 ]; then
    printf '     %s\n' "$2"
  fi
}

check_eq() {
  local name="$1" want="$2" got="$3"
  if [ "$want" = "$got" ]; then
    pass "$name"
  else
    fail "$name" "want [${want}] got [${got}]"
  fi
}

check_contains() {
  local name="$1" haystack="$2" needle="$3"
  case "$haystack" in
    *"$needle"*) pass "$name" ;;
    *) fail "$name" "expected to find [${needle}]" ;;
  esac
}

check_not_contains() {
  local name="$1" haystack="$2" needle="$3"
  case "$haystack" in
    *"$needle"*) fail "$name" "did not expect to find [${needle}]" ;;
    *) pass "$name" ;;
  esac
}

# ---------------------------------------------------------------------------
# A fake machine.
#
# The refusals and the architecture mapping are decided by what "uname" says,
# so the way to test them is to put a "uname" of our own at the front of PATH.
# The rest of the fakes exist so that a dry run can walk the whole flow on a
# machine that has none of these tools.
# ---------------------------------------------------------------------------

FAKE_ROOT=""

make_fake_machine() {
  local machine="$1" os="${2:-Linux}" systemd_version="${3:-252}"
  FAKE_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/caspian-fake.XXXXXX")"
  mkdir -p "${FAKE_ROOT}/bin" "${FAKE_ROOT}/sysroot/run/systemd/system" \
    "${FAKE_ROOT}/sysroot/etc/systemd/system" "${FAKE_ROOT}/sysroot/usr/local/bin" \
    "${FAKE_ROOT}/sysroot/etc/tmpfiles.d" "${FAKE_ROOT}/sysroot/etc/modules-load.d" \
    "${FAKE_ROOT}/sysroot/proc/1"

  printf 'systemd\n' >"${FAKE_ROOT}/sysroot/proc/1/comm"

  # shellcheck disable=SC2016
  # Single quotes are the point: these lines are the text of a script being
  # written to a file, not expressions to expand here.
  printf '%s\n' \
    '#!/bin/sh' \
    'case "${1:-}" in' \
    "  -s) echo ${os} ;;" \
    "  -m) echo ${machine} ;;" \
    "  *) echo ${os} ;;" \
    'esac' >"${FAKE_ROOT}/bin/uname"

  # shellcheck disable=SC2016
  printf '%s\n' \
    '#!/bin/sh' \
    'if [ "${1:-}" = "--version" ]; then' \
    "  echo \"systemd ${systemd_version} (${systemd_version}.1)\"" \
    '  exit 0' \
    'fi' \
    'exit 0' >"${FAKE_ROOT}/bin/systemctl"

  # An address, so the closing message has something to print.
  printf '%s\n' \
    '#!/bin/sh' \
    'echo "2: eth0    inet 192.168.4.31/24 brd 192.168.4.255 scope global dynamic eth0"' \
    >"${FAKE_ROOT}/bin/ip"

  # Present so that detection finds them, never actually invoked: every one is
  # reached through run(), which only prints under --dry-run.
  local tool
  for tool in apt-get groupadd useradd userdel groupdel systemd-tmpfiles modprobe install; do
    printf '%s\n' '#!/bin/sh' 'exit 0' >"${FAKE_ROOT}/bin/${tool}"
  done
  # getent answers "no such user or group", which is exit status 2.
  # shellcheck disable=SC2016
  printf '%s\n' '#!/bin/sh' 'exit ${CASPIAN_FAKE_GETENT_STATUS:-2}' >"${FAKE_ROOT}/bin/getent"

  chmod 0755 "${FAKE_ROOT}"/bin/*
}

drop_fake_machine() {
  if [ -n "$FAKE_ROOT" ] && [ -d "$FAKE_ROOT" ]; then
    rm -rf "$FAKE_ROOT"
  fi
  FAKE_ROOT=""
}

# run_installer runs install.sh with the fake machine in front of PATH.
# env, not a bare assignment prefix: a VAR=VALUE that arrives through "$@" is
# not recognised as an assignment by the shell, it is treated as the command to
# run. That mistake made twenty-three of these tests fail against an installer
# that was working correctly.
run_installer() {
  env PATH="${FAKE_ROOT}/bin:${PATH}" \
    CASPIAN_SYSROOT="${FAKE_ROOT}/sysroot" \
    "$@" \
    bash "$INSTALL_SH" --dry-run --yes 2>&1
}

# Takes uninstall.sh's own flags, not environment assignments.
run_uninstaller() {
  env PATH="${FAKE_ROOT}/bin:${PATH}" \
    CASPIAN_SYSROOT="${FAKE_ROOT}/sysroot" \
    bash "$UNINSTALL_SH" --dry-run "$@" 2>&1
}

# ---------------------------------------------------------------------------
# 1. Syntax
# ---------------------------------------------------------------------------

section() { printf '\n== %s ==\n' "$1"; }

section "syntax"
for script in "$INSTALL_SH" "$UNINSTALL_SH" "${REPO_ROOT}/packaging/test-install.sh"; do
  if bash -n "$script" 2>/dev/null; then
    pass "bash -n $(basename "$script")"
  else
    fail "bash -n $(basename "$script")"
  fi
done

if command -v shellcheck >/dev/null 2>&1; then
  for script in "$INSTALL_SH" "$UNINSTALL_SH"; do
    if shellcheck --shell=bash --severity=style "$script" >/dev/null 2>&1; then
      pass "shellcheck $(basename "$script")"
    else
      fail "shellcheck $(basename "$script")" "$(shellcheck --shell=bash --severity=style "$script" 2>&1 | head -20)"
    fi
  done
else
  printf 'skip shellcheck (not installed)\n'
fi

# ---------------------------------------------------------------------------
# 2. House rules: no escape codes, no emoji, no em dashes, anywhere.
# ---------------------------------------------------------------------------

section "house rules"
for f in "$INSTALL_SH" "$UNINSTALL_SH" "${REPO_ROOT}"/packaging/*; do
  base="$(basename "$f")"
  if LC_ALL=C grep -q "$(printf '\033')" "$f" 2>/dev/null; then
    fail "no escape codes in ${base}"
  else
    pass "no escape codes in ${base}"
  fi
  # U+2014 EM DASH is E2 80 94 in UTF-8. Matching the bytes rather than the
  # character keeps this file itself pure ASCII.
  if LC_ALL=C grep -q "$(printf '\xe2\x80\x94')" "$f" 2>/dev/null; then
    fail "no em dash in ${base}"
  else
    pass "no em dash in ${base}"
  fi
  # Every emoji in the supplementary planes starts F0 9F in UTF-8.
  if LC_ALL=C grep -q "$(printf '\xf0\x9f')" "$f" 2>/dev/null; then
    fail "no emoji in ${base}"
  else
    pass "no emoji in ${base}"
  fi
done

# ---------------------------------------------------------------------------
# 3. The embedded unit files still match packaging/
#
# install.sh carries its own copy because a script piped from curl has no
# repository to read. This is the check that keeps the two the same.
# ---------------------------------------------------------------------------

section "embedded units match packaging"
check_embedded() {
  local fn="$1" file="$2" embedded on_disk
  embedded="$(CASPIAN_SOURCE_ONLY=1 bash -c "source '${INSTALL_SH}'; ${fn}")"
  on_disk="$(< "${REPO_ROOT}/${file}")"
  if [ "$embedded" = "$on_disk" ]; then
    pass "${fn} matches ${file}"
  else
    fail "${fn} matches ${file}" "the embedded copy has drifted"
  fi
}
check_embedded unit_caspian_service packaging/caspian.service
check_embedded unit_caspian_panel_service packaging/caspian-panel.service
check_embedded unit_tmpfiles_conf packaging/caspian.tmpfiles.conf
check_embedded unit_modules_load_conf packaging/caspian.modules-load.conf

# /run is a tmpfs, so both runtime directories have to be recreated at every
# boot with the exact modes docs/LAYOUT.md fixes, not just at install time.
tmpfiles="$(< "${REPO_ROOT}/packaging/caspian.tmpfiles.conf")"
check_contains "tmpfiles recreates /run/caspian as 0750 root:caspian" "$tmpfiles" \
  "d /run/caspian          0750 root    caspian -"
check_contains "tmpfiles recreates /run/caspian/dnsmasq as 0700 caspian:caspian" "$tmpfiles" \
  "d /run/caspian/dnsmasq  0700 caspian caspian -"
check_contains "the reason dnsmasq gets its own directory is written down" "$tmpfiles" \
  "not the file"

# ---------------------------------------------------------------------------
# 4. Architecture mapping
#
# The armv6l row is the one this project has seen got wrong before: an armv6
# machine given an armv7 build installs cleanly and dies with an illegal
# instruction. Both 32-bit values must land on the same "arm" artefact.
# ---------------------------------------------------------------------------

section "architecture mapping"
arch_for() {
  local machine="$1"
  make_fake_machine "$machine"
  local out
  out="$(PATH="${FAKE_ROOT}/bin:${PATH}" CASPIAN_SOURCE_ONLY=1 bash -c \
    "source '${INSTALL_SH}'; detect_arch; printf '%s' \"\$ARTEFACT\"" 2>&1)"
  drop_fake_machine
  printf '%s' "$out"
}
check_eq "x86_64 maps to amd64" "caspian-linux-amd64" "$(arch_for x86_64)"
check_eq "aarch64 maps to arm64" "caspian-linux-arm64" "$(arch_for aarch64)"
check_eq "armv7l maps to arm" "caspian-linux-arm" "$(arch_for armv7l)"
check_eq "armv6l maps to arm, not to an armv7 artefact" "caspian-linux-arm" "$(arch_for armv6l)"

# ---------------------------------------------------------------------------
# 5. Refusals. Each names what was found and what is supported, and exits
#    non-zero without having touched anything.
# ---------------------------------------------------------------------------

section "refusals"

# 5a. Not Linux.
make_fake_machine x86_64 Darwin
out="$(run_installer)"
status=$?
check_contains "refuses a non-Linux kernel" "$out" "Found: Darwin."
check_contains "names the supported kernel" "$out" "Supported: Linux."
if [ "$status" -ne 0 ]; then pass "non-Linux exits non-zero"; else fail "non-Linux exits non-zero"; fi
check_not_contains "non-Linux changes nothing" "$out" "would run:"
drop_fake_machine

# 5b. An architecture with no artefact. armv8l is a 32-bit userland on a
#     64-bit kernel; docs/LAYOUT.md does not say which artefact it takes, and
#     guessing is exactly how the armv6 bug happened.
make_fake_machine armv8l
out="$(run_installer)"
status=$?
check_contains "refuses an unmapped architecture" "$out" "Found: armv8l."
check_contains "names the supported architectures" "$out" "Supported: x86_64, aarch64, armv7l, armv6l."
if [ "$status" -ne 0 ]; then pass "unmapped architecture exits non-zero"; else fail "unmapped architecture exits non-zero"; fi
check_not_contains "unmapped architecture changes nothing" "$out" "would run:"
drop_fake_machine

# 5c. No systemd.
make_fake_machine aarch64
rm -rf "${FAKE_ROOT}/sysroot/run/systemd/system"
printf 'openrc-init\n' >"${FAKE_ROOT}/sysroot/proc/1/comm"
out="$(run_installer)"
status=$?
check_contains "refuses a box with no systemd" "$out" "Found: init system openrc-init"
check_contains "names the supported init system" "$out" "Supported: systemd 240 or newer."
if [ "$status" -ne 0 ]; then pass "no systemd exits non-zero"; else fail "no systemd exits non-zero"; fi
drop_fake_machine

# 5d. systemd too old for Type=exec.
make_fake_machine aarch64 Linux 239
out="$(run_installer)"
status=$?
check_contains "refuses systemd older than 240" "$out" "Found: systemd 239."
if [ "$status" -ne 0 ]; then pass "old systemd exits non-zero"; else fail "old systemd exits non-zero"; fi
drop_fake_machine

# 5e. The default installation selects GitHub's latest release.
make_fake_machine aarch64
out="$(run_installer)"
status=$?
check_contains "default download selects latest" "$out" "releases/latest/download/caspian-linux-arm64"
check_eq "default download dry run succeeds" "0" "$status"
drop_fake_machine

# 5f. A plaintext base URL.
make_fake_machine aarch64
out="$(run_installer CASPIAN_BASE_URL=http://example.invalid/rel)"
status=$?
check_contains "refuses a plaintext download URL" "$out" "refusing to download over a plaintext URL"
if [ "$status" -ne 0 ]; then pass "plaintext URL exits non-zero"; else fail "plaintext URL exits non-zero"; fi
drop_fake_machine

# 5g. The test sysroot is refused outside a dry run.
out="$(CASPIAN_SOURCE_ONLY=1 CASPIAN_SYSROOT=/tmp/nope bash -c "source '${INSTALL_SH}'; DRY_RUN=0; setup_dest_paths" 2>&1)"
check_contains "sysroot is refused outside a dry run" "$out" "only allowed together with --dry-run"

# ---------------------------------------------------------------------------
# 6. SHA-256 verification
# ---------------------------------------------------------------------------

section "checksum verification"
sha_dir="$(mktemp -d "${TMPDIR:-/tmp}/caspian-sha.XXXXXX")"
printf 'this is not really a binary\n' >"${sha_dir}/caspian-linux-arm64"
real_hash="$(shasum -a 256 "${sha_dir}/caspian-linux-arm64" 2>/dev/null | awk '{print $1}')"
if [ -z "$real_hash" ]; then
  real_hash="$(sha256sum "${sha_dir}/caspian-linux-arm64" | awk '{print $1}')"
fi

printf '%s  caspian-linux-arm64\n' "$real_hash" >"${sha_dir}/GOOD"
printf '%s  caspian-linux-arm64\n' "0000000000000000000000000000000000000000000000000000000000000000" >"${sha_dir}/BAD"
printf '%s  caspian-linux-amd64\n' "$real_hash" >"${sha_dir}/OTHER"
printf 'not-a-hash  caspian-linux-arm64\n' >"${sha_dir}/MALFORMED"

verify_with() {
  CASPIAN_SOURCE_ONLY=1 bash -c \
    "source '${INSTALL_SH}'; verify_sha256 '${sha_dir}/caspian-linux-arm64' '${sha_dir}/$1' caspian-linux-arm64" 2>&1
}

out="$(verify_with GOOD)"
status=$?
check_contains "a matching checksum is accepted" "$out" "Verified SHA-256 of caspian-linux-arm64."
check_eq "a matching checksum exits zero" "0" "$status"

out="$(verify_with BAD)"
status=$?
check_contains "a mismatched checksum is refused" "$out" "SHA-256 mismatch"
check_contains "the refusal says nothing was changed" "$out" "Nothing has been changed."
if [ "$status" -ne 0 ]; then pass "a mismatch exits non-zero"; else fail "a mismatch exits non-zero"; fi

out="$(verify_with OTHER)"
check_contains "a missing entry is refused" "$out" "no entry for caspian-linux-arm64"

out="$(verify_with MALFORMED)"
check_contains "a malformed hash is refused" "$out" "is not a SHA-256 hash"

# The whole point, stated as a test: no path installs an unverified binary.
out="$(verify_with BAD)"
check_contains "the refusal says why it matters" "$out" "Refusing to install an unverified binary."

rm -rf "$sha_dir"

# ---------------------------------------------------------------------------
# 7. The generated first-run password
# ---------------------------------------------------------------------------

section "first-run password"
pw="$(CASPIAN_SOURCE_ONLY=1 bash -c "source '${INSTALL_SH}'; generate_password")"
check_eq "password is 23 characters (20 plus 3 separators)" "23" "${#pw}"
case "$pw" in
  [a-z2-9][a-z2-9][a-z2-9][a-z2-9][a-z2-9]-[a-z2-9][a-z2-9][a-z2-9][a-z2-9][a-z2-9]-[a-z2-9][a-z2-9][a-z2-9][a-z2-9][a-z2-9]-[a-z2-9][a-z2-9][a-z2-9][a-z2-9][a-z2-9])
    pass "password has the expected shape" ;;
  *) fail "password has the expected shape" "got [${pw}]" ;;
esac
case "$pw" in
  *[01lIO]*) fail "password avoids characters that are read wrongly" "got [${pw}]" ;;
  *) pass "password avoids characters that are read wrongly" ;;
esac
pw2="$(CASPIAN_SOURCE_ONLY=1 bash -c "source '${INSTALL_SH}'; generate_password")"
if [ "$pw" != "$pw2" ]; then
  pass "two passwords differ"
else
  fail "two passwords differ" "the generator returned the same value twice"
fi

# ---------------------------------------------------------------------------
# 8. The teardown journal replay
# ---------------------------------------------------------------------------

section "teardown journal replay"
if command -v python3 >/dev/null 2>&1; then
  replay_dir="$(mktemp -d "${TMPDIR:-/tmp}/caspian-replay.XXXXXX")"
  CASPIAN_SOURCE_ONLY=1 bash -c "source '${UNINSTALL_SH}'; replay_program" >"${replay_dir}/replay.py"

  if python3 -m py_compile "${replay_dir}/replay.py" 2>/dev/null; then
    pass "the replay program compiles"
  else
    fail "the replay program compiles"
  fi

  # The record shape is the one internal/netcfg/journal.go writes: several
  # lines per step, keyed by seq, with the inverse in "undo". Entry 3 is
  # already undone and must not be replayed again.
  {
    printf '%s\n' '{"seq":1,"phase":"begin","t":"2026-08-30T00:00:00Z","op":"nft","why":"fail-closed ruleset","do":{"path":"nft","args":["-f","-"],"stdin":"table inet caspian {}"},"undo":{"path":"nft","args":["delete","table","inet","caspian"]}}'
    printf '%s\n' '{"seq":1,"phase":"done","t":"2026-08-30T00:00:01Z"}'
    printf '%s\n' '{"seq":2,"phase":"begin","t":"2026-08-30T00:00:02Z","op":"route","why":"pinned host route to the server","do":{"path":"ip","args":["route","add","203.0.113.7","via","192.168.4.1"]},"undo":{"path":"ip","args":["route","del","203.0.113.7","via","192.168.4.1"]}}'
    printf '%s\n' '{"seq":2,"phase":"done","t":"2026-08-30T00:00:03Z"}'
    printf '%s\n' '{"seq":3,"phase":"begin","t":"2026-08-30T00:00:04Z","op":"rule","do":{"path":"ip","args":["rule","add","fwmark","0x20da","table","8410"]},"undo":{"path":"ip","args":["rule","del","priority","8410"]}}'
    printf '%s\n' '{"seq":3,"phase":"undone","t":"2026-08-30T00:00:05Z"}'
  } >"${replay_dir}/good.journal"

  out="$(python3 "${replay_dir}/replay.py" "${replay_dir}/good.journal" --dry-run 2>&1)"
  status=$?
  check_eq "a good journal replays cleanly" "0" "$status"
  check_contains "the nft inverse is replayed" "$out" "would replay entry 1 (nft)"
  check_contains "the route inverse is replayed" "$out" "would replay entry 2 (route)"
  # Entry.NeedsUndo in internal/netcfg/journal.go: phase "undone" is finished.
  check_not_contains "an entry already undone is not replayed again" "$out" "entry 3"
  # Applier.Teardown replays newest first.
  first_line="$(printf '%s\n' "$out" | head -1)"
  check_contains "the newest inverse goes first" "$first_line" "entry 2"

  # docs/LAYOUT.md: never print the user's proxy config. The pinned host route
  # carries the address of their server.
  check_not_contains "the server address is not printed by default" "$out" "203.0.113.7"
  out="$(python3 "${replay_dir}/replay.py" "${replay_dir}/good.journal" --dry-run --show-commands 2>&1)"
  check_contains "--show-commands does print it" "$out" "203.0.113.7"

  # An undo naming a binary outside internal/netcfg/command.go's allowlist.
  printf '%s\n' '{"seq":1,"phase":"begin","op":"route","do":{"path":"ip","args":["route","add","x"]},"undo":{"path":"/bin/sh","args":["-c","touch /tmp/caspian-should-not-exist"]}}' \
    >"${replay_dir}/evil.journal"
  out="$(python3 "${replay_dir}/replay.py" "${replay_dir}/evil.journal" 2>&1)"
  status=$?
  check_eq "an undo naming a binary off the allowlist is refused" "2" "$status"
  check_contains "the refusal names the binary" "$out" "which is not one of"
  if [ -e /tmp/caspian-should-not-exist ]; then
    fail "nothing off the allowlist is executed"
    rm -f /tmp/caspian-should-not-exist
  else
    pass "nothing off the allowlist is executed"
  fi

  # A malformed command inside an otherwise valid record.
  printf '%s\n' '{"seq":1,"phase":"begin","op":"route","undo":{"path":"ip","args":[7]}}' \
    >"${replay_dir}/malformed.journal"
  out="$(python3 "${replay_dir}/replay.py" "${replay_dir}/malformed.journal" 2>&1)"
  status=$?
  check_eq "a malformed command is refused" "2" "$status"
  check_contains "the refusal says which entry" "$out" "entry 1 has a malformed command"

  # Unreadable lines are skipped, the way LoadJournal skips them, but they are
  # counted and they make the replay partial rather than clean.
  {
    printf '%s\n' '{"seq":1,"phase":"begin","op":"nft","undo":{"path":"nft","args":["delete","table","inet","caspian"]}}'
    printf '%s\n' 'this line is not json'
  } >"${replay_dir}/torn.journal"
  out="$(python3 "${replay_dir}/replay.py" "${replay_dir}/torn.journal" --dry-run 2>&1)"
  status=$?
  check_eq "a torn journal is a partial replay, not a refusal" "1" "$status"
  check_contains "the skipped line is counted" "$out" "1 unreadable line(s)"
  check_contains "the readable entries are still replayed" "$out" "would replay entry 1 (nft)"

  # Everything already undone.
  {
    printf '%s\n' '{"seq":1,"phase":"begin","op":"nft","undo":{"path":"nft","args":["delete","table","inet","caspian"]}}'
    printf '%s\n' '{"seq":1,"phase":"undone"}'
  } >"${replay_dir}/done.journal"
  out="$(python3 "${replay_dir}/replay.py" "${replay_dir}/done.journal" --dry-run 2>&1)"
  status=$?
  check_eq "a fully undone journal is not an error" "0" "$status"
  check_contains "it says there is nothing left" "$out" "nothing left to undo"

  # An empty or absent journal means nothing was ever changed.
  : >"${replay_dir}/empty.journal"
  python3 "${replay_dir}/replay.py" "${replay_dir}/empty.journal" >/dev/null 2>&1
  check_eq "an empty journal is not an error" "0" "$?"
  python3 "${replay_dir}/replay.py" "${replay_dir}/absent.journal" >/dev/null 2>&1
  check_eq "an absent journal is not an error" "0" "$?"

  rm -rf "$replay_dir"
else
  printf 'skip replay tests (python3 not installed)\n'
fi

# ---------------------------------------------------------------------------
# 9. The dry run walks the whole flow, twice: fresh, then upgrade.
# ---------------------------------------------------------------------------

section "dry run"
make_fake_machine aarch64
out="$(run_installer CASPIAN_BASE_URL=https://example.invalid/rel/v1)"
check_contains "fresh run says it is fresh" "$out" "fresh install"
check_contains "fresh run creates the group" "$out" "groupadd --system caspian"
check_contains "fresh run creates the user" "$out" "useradd --system"
check_contains "fresh run creates the state directory at 0700" "$out" "install -d -m 0700 -o caspian -g caspian"
check_contains "fresh run creates the run directory at 0750" "$out" "install -d -m 0750 -o root -g caspian"
check_contains "fresh run creates the dnsmasq run directory at 0700 caspian" "$out" \
  "install -d -m 0700 -o caspian -g caspian ${FAKE_ROOT}/sysroot/run/caspian/dnsmasq"
# docs/LAYOUT.md dropped /etc/caspian on 2026-08-30. Creating it again would
# recreate the directory the layout says should not exist.
check_not_contains "fresh run does not create /etc/caspian" "$out" "/etc/caspian"
check_contains "fresh run writes the privileged unit" "$out" "caspian.service (mode 0644"
check_contains "fresh run writes the panel unit" "$out" "caspian-panel.service (mode 0644"
check_contains "fresh run writes the tmpfiles fragment" "$out" "tmpfiles.d/caspian.conf (mode 0644"
check_contains "fresh run seeds a password at 0600" "$out" "first-run-password (mode 0600, owner caspian:caspian"
check_contains "fresh run enables both units" "$out" "systemctl enable caspian-panel.service"
check_contains "fresh run prints the panel address" "$out" "http://192.168.4.31:8088/"
check_contains "fresh run only installs what is missing" "$out" "hostapd dnsmasq nftables iw"
check_not_contains "fresh run does not reinstall iproute2" "$out" "iw iproute2"
check_contains "fresh run prints a password" "$out" "Password: xxxxx-xxxxx-xxxxx-xxxxx"
check_not_contains "fresh run does not claim an unchanged password" "$out" "Password: unchanged"
drop_fake_machine

# The same machine, but with a previous install already on it.
make_fake_machine aarch64
mkdir -p "${FAKE_ROOT}/sysroot/var/lib/caspian"
printf 'not a real binary\n' >"${FAKE_ROOT}/sysroot/usr/local/bin/caspian"
printf '{"version":1}\n' >"${FAKE_ROOT}/sysroot/var/lib/caspian/state.json"
printf 'placeholder\n' >"${FAKE_ROOT}/sysroot/etc/systemd/system/caspian.service"
printf 'placeholder\n' >"${FAKE_ROOT}/sysroot/etc/systemd/system/caspian-panel.service"
out="$(run_installer CASPIAN_BASE_URL=https://example.invalid/rel/v1)"
check_contains "second run says it is an upgrade" "$out" "This run is an upgrade."
check_contains "upgrade stops the panel first" "$out" "systemctl stop caspian-panel.service"
check_contains "upgrade stops the privileged service" "$out" "systemctl stop caspian.service"
check_contains "upgrade keeps the existing password" "$out" "keeping the current panel password"
check_contains "upgrade reports the password as unchanged" "$out" "Password: unchanged"
check_not_contains "upgrade never seeds a new password" "$out" "first-run-password"
check_not_contains "upgrade never removes the state directory" "$out" "rm -rf"
check_not_contains "upgrade never recreates the state file" "$out" "state.json"
drop_fake_machine

# ---------------------------------------------------------------------------
# 10. The uninstaller's dry run
# ---------------------------------------------------------------------------

section "uninstall dry run"
make_fake_machine aarch64
mkdir -p "${FAKE_ROOT}/sysroot/var/lib/caspian" \
  "${FAKE_ROOT}/sysroot/run/caspian/dnsmasq"
printf 'not a real binary\n' >"${FAKE_ROOT}/sysroot/usr/local/bin/caspian"
printf '{"version":1}\n' >"${FAKE_ROOT}/sysroot/var/lib/caspian/state.json"
printf 'placeholder\n' >"${FAKE_ROOT}/sysroot/etc/systemd/system/caspian.service"
printf 'placeholder\n' >"${FAKE_ROOT}/sysroot/etc/systemd/system/caspian-panel.service"
printf '%s\n' '{"seq":1,"phase":"begin","op":"route","do":{"path":"ip","args":["route","add","203.0.113.7","via","192.168.4.1"]},"undo":{"path":"ip","args":["route","del","203.0.113.7","via","192.168.4.1"]}}' \
  >"${FAKE_ROOT}/sysroot/var/lib/caspian/netcfg.journal"

out="$(run_uninstaller --keep-state)"
check_contains "uninstall disables the panel first" "$out" "systemctl disable --now caspian-panel.service"
check_contains "uninstall replays the journal" "$out" "would replay entry 1 (route)"
check_not_contains "uninstall does not print the server address" "$out" "203.0.113.7"
check_contains "uninstall removes the binary" "$out" "rm -f ${FAKE_ROOT}/sysroot/usr/local/bin/caspian"
# One rm -rf of /run/caspian takes the dnsmasq directory inside it as well.
check_contains "uninstall removes the run directory" "$out" "rm -rf ${FAKE_ROOT}/sysroot/run/caspian"
check_not_contains "uninstall does not look for /etc/caspian" "$out" "/etc/caspian"
check_contains "uninstall keeps state when told to" "$out" "Keeping /var/lib/caspian."
check_contains "uninstall keeps the account with the state" "$out" "Keeping the caspian account"
check_not_contains "uninstall does not delete state when told to keep it" "$out" "rm -rf ${FAKE_ROOT}/sysroot/var/lib/caspian"

out="$(env CASPIAN_FAKE_GETENT_STATUS=0 PATH="${FAKE_ROOT}/bin:${PATH}" \
  CASPIAN_SYSROOT="${FAKE_ROOT}/sysroot" bash "$UNINSTALL_SH" --dry-run --purge 2>&1)"
check_contains "purge deletes the state directory" "$out" "rm -rf ${FAKE_ROOT}/sysroot/var/lib/caspian"
check_contains "purge removes the account" "$out" "userdel caspian"

# Without a flag and with no terminal, the answer to silence is keep.
out="$(env PATH="${FAKE_ROOT}/bin:${PATH}" CASPIAN_SYSROOT="${FAKE_ROOT}/sysroot" \
  bash "$UNINSTALL_SH" --dry-run </dev/null 2>&1)"
check_contains "with no answer available, state is kept" "$out" "Keeping /var/lib/caspian."

# The journal name is the one internal/netcfg/journal.go writes. It was
# teardown.journal in docs/LAYOUT.md until 2026-08-30, and an uninstaller that
# looks for the wrong name silently leaves the routes, rules and firewall in
# place while telling the user the network was restored.
journal_const="$(grep -o 'CASPIAN_NETCFG_JOURNAL="[^"]*"' "$UNINSTALL_SH" | head -1)"
check_eq "the uninstaller looks for the name the code writes" \
  'CASPIAN_NETCFG_JOURNAL="/var/lib/caspian/netcfg.journal"' "$journal_const"
rm -f "${FAKE_ROOT}/sysroot/var/lib/caspian/netcfg.journal"
out="$(run_uninstaller --keep-state)"
check_contains "no journal is not an error" "$out" "No journal"

drop_fake_machine

# ---------------------------------------------------------------------------

section "latest release selection"
out="$(CASPIAN_SOURCE_ONLY=1 bash -c '
  source "$1"
  WORK_DIR=$(mktemp -d)
  trap "rm -rf \"$WORK_DIR\"" EXIT
  fetch_to() { printf "{\"tag_name\":\"v9.8.7\"}\n" >"$2"; }
  resolve_release
  resolve_base_url
' _ "$INSTALL_SH")"
check_contains "latest resolves to one immutable tag" "$out" "/releases/download/v9.8.7"
out="$(CASPIAN_SOURCE_ONLY=1 CASPIAN_VERSION=v1.2.3 bash -c '
  source "$1"
  fetch_to() { exit 99; }
  resolve_release
  resolve_base_url
' _ "$INSTALL_SH")"
check_contains "explicit version does not query latest" "$out" "/releases/download/v1.2.3"

out="$(CASPIAN_SOURCE_ONLY=1 bash -c '
  source "$1"
  CASPIAN_ORG=""
  resolve_base_url
' _ "$INSTALL_SH" 2>&1)"
status=$?
check_contains "an explicitly missing location is refused" "$out" "no download location"
if [ "$status" -ne 0 ]; then pass "missing location exits non-zero"; else fail "missing location exits non-zero"; fi

printf '\n%s passed, %s failed\n' "$PASSED" "$FAILED"
if [ "$FAILED" -ne 0 ]; then
  exit 1
fi
