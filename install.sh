#!/bin/bash
#
# Caspian-BYOC installer.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# The one command a person runs, and the only one:
#
#   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"
#
# After this finishes, everything else happens in the panel. Nothing here needs
# to be run again except to upgrade, and running it again is exactly how you
# upgrade.
#
# Options:
#
#   --dry-run   print every action without taking any of them
#   --yes, -y   do not ask anything, assume yes
#   --help, -h  usage
#
# See docs/INSTALL.md for the environment variables and for how to exercise
# this script without a published release.

set -euo pipefail

# ---------------------------------------------------------------------------
# Why the whole body of this script is one function, called on the last line
# ---------------------------------------------------------------------------
#
# This script is meant to be piped into bash from a network fetch. A pipe can
# end early: the connection drops, a proxy truncates the body, the CDN returns
# a short read. bash does not read the whole file before it starts; it reads,
# executes, reads more. So a script written as a flat list of commands and cut
# in half executes the first half and stops, and the first half of an installer
# is a box with a user account, some directories, no binary and no services.
#
# Wrapping every statement in a function definition and calling it on the very
# last line makes truncation harmless: a partial download is a partial function
# definition, bash reaches end of input before it ever reaches the call, and
# nothing runs at all. That is the whole reason well-known installers are
# shaped this way, and it is the reason this one is.
#
# The rule that keeps it true: nothing below may execute at the top level.
# Constants and function definitions only, then one call.
# ---------------------------------------------------------------------------

# --- names and paths, all fixed by docs/LAYOUT.md --------------------------

readonly CASPIAN_BIN_PATH="/usr/local/bin/caspian"
readonly CASPIAN_STATE_DIR="/var/lib/caspian"
readonly CASPIAN_RUN_DIR="/run/caspian"
readonly CASPIAN_DNSMASQ_RUN_DIR="/run/caspian/dnsmasq"
readonly CASPIAN_USER="caspian"
readonly CASPIAN_GROUP="caspian"
readonly CASPIAN_UNIT_PRIV="caspian.service"
readonly CASPIAN_UNIT_PANEL="caspian-panel.service"
readonly CASPIAN_UNIT_DIR="/etc/systemd/system"
readonly CASPIAN_TMPFILES_PATH="/etc/tmpfiles.d/caspian.conf"
readonly CASPIAN_MODULES_PATH="/etc/modules-load.d/caspian.conf"

# The plaintext of the generated first-run password. The panel reads it on its
# first start, sets the password through internal/state (Store.SetPanelPassword
# hashes it with argon2id), and deletes this file. It is 0600 and owned by the
# service user inside a 0700 directory. See docs/INSTALL.md, "First run".
readonly CASPIAN_PASSWORD_SEED="/var/lib/caspian/first-run-password"

# A local copy of the uninstaller, so that removing this software does not
# require a working network. That is not a hypothetical: the moment somebody
# wants to uninstall is very often the moment the box's networking is in a
# state they do not like. Same reasoning the panel uses for embedding its own
# assets (design section 5.7).
readonly CASPIAN_UNINSTALL_PATH="/usr/local/bin/caspian-uninstall"

# The panel's listener.
#
# Not a guess and not remembered: docs/LAYOUT.md, "Ports", fixes 53, 5354, 8088
# and 10808, and internal/netcfg/plan.go's DefaultOptions agrees on 8088. This
# is the only port the installer has any business with, because it is the only
# one it prints. It sets none of them. The one that breaks quietly is 5354, the
# pairing between dnsmasq's only permitted upstream and the engine's local DNS
# listener; if those two drift, DNS stops resolving for every joined device
# while the hotspot and the tunnel both still look healthy. Neither end of that
# pairing is set here, and adding a check for it belongs in internal/xcfg,
# where the cross-check test already lives.
readonly CASPIAN_PANEL_PORT="8088"

# The dependencies, named as docs/LAYOUT.md names them.
readonly CASPIAN_DEPS="hostapd dnsmasq nftables iw iproute2"

# systemd 240 introduced Type=exec, which both units use. See the comment in
# packaging/caspian.service.
readonly CASPIAN_MIN_SYSTEMD="240"

# --- settings, overridable from the environment ----------------------------
#
# REPLACE BEFORE THE FIRST RELEASE. docs/LAYOUT.md writes the repository owner
# as a placeholder, so this is a placeholder too rather than an invented name.
# While it is unchanged the installer refuses to download and says how to point
# it somewhere real, which is what makes the script testable today.
CASPIAN_ORG="${CASPIAN_ORG:-Iman}"
CASPIAN_REPO="${CASPIAN_REPO:-caspian}"
CASPIAN_VERSION="${CASPIAN_VERSION:-latest}"
CASPIAN_BASE_URL="${CASPIAN_BASE_URL:-}"
CASPIAN_CHECKSUMS_NAME="${CASPIAN_CHECKSUMS_NAME:-SHA256SUMS}"
CASPIAN_LOCAL_BINARY="${CASPIAN_LOCAL_BINARY:-}"
CASPIAN_LOCAL_CHECKSUMS="${CASPIAN_LOCAL_CHECKSUMS:-}"
CASPIAN_SCRIPT_BASE_URL="${CASPIAN_SCRIPT_BASE_URL:-}"
CASPIAN_UNINSTALL_SRC="${CASPIAN_UNINSTALL_SRC:-}"
CASPIAN_ALLOW_INSECURE_URL="${CASPIAN_ALLOW_INSECURE_URL:-0}"
CASPIAN_SYSROOT="${CASPIAN_SYSROOT:-}"
CASPIAN_ASSUME_YES="${CASPIAN_ASSUME_YES:-0}"

# Runtime state. Set by the argument parser and by detection.
DRY_RUN="0"
ASSUME_YES="0"
ARTEFACT=""
PKG_MANAGER=""
IS_UPGRADE="0"
WORK_DIR=""
GENERATED_PASSWORD=""

# Destination paths, which are the fixed paths above with the test sysroot
# prefixed. On a real run the prefix is empty and these are the fixed paths.
DEST_BIN=""
DEST_STATE_DIR=""
DEST_RUN_DIR=""
DEST_DNSMASQ_RUN_DIR=""
DEST_UNIT_DIR=""
DEST_TMPFILES=""
DEST_MODULES=""
DEST_PASSWORD_SEED=""
DEST_UNINSTALL=""

# --- output ----------------------------------------------------------------
#
# Plain text only. No colour, no escape codes, no emoji: this output is read
# over serial consoles, in journalctl, and pasted into bug reports, and every
# one of those turns an escape sequence into noise.

say() { printf '%s\n' "$*"; }
step() { printf '%s\n' "$*"; }
warn() { printf '%s\n' "warning: $*" >&2; }

die() {
  printf '%s\n' "error: $*" >&2
  exit 1
}

# refuse prints a refusal that names what was found and what is supported, then
# exits non-zero without having changed anything. Every unsupported-platform
# path goes through here so that no refusal can happen halfway through an
# install.
refuse() {
  printf '%s\n' "Caspian-BYOC cannot be installed on this machine." >&2
  printf '%s\n' "$1" >&2
  printf '%s\n' "$2" >&2
  exit 1
}

usage() {
  cat <<'USAGE_EOF'
Caspian-BYOC installer.

Usage:
  install.sh [--dry-run] [--yes] [--help]

  --dry-run   Print every action that would be taken, take none of them.
  --yes, -y   Do not ask anything. Assume yes.
  --help, -h  This text.

Environment variables are documented in docs/INSTALL.md.
USAGE_EOF
}

# show_cmd renders an argument vector for a human to read. It is for display
# only and is never fed back to a shell: nothing in this script builds a
# command out of a string.
show_cmd() {
  local out="" a
  for a in "$@"; do
    case "$a" in
      "" | *[!A-Za-z0-9_./=:@,+-]*) out="${out} '${a}'" ;;
      *) out="${out} ${a}" ;;
    esac
  done
  printf '%s' "${out# }"
}

# run is the single gate between deciding to do something and doing it. Every
# action that changes the machine goes through run or through write_file, which
# is what makes --dry-run trustworthy rather than approximate.
run() {
  if [ "$DRY_RUN" = "1" ]; then
    printf 'would run: %s\n' "$(show_cmd "$@")"
    return 0
  fi
  "$@"
}

# write_file writes stdin to a path with an exact mode and owner. It writes to a
# temporary file in the same directory and renames, so an interrupted install
# never leaves a half-written unit file that systemd would then try to parse.
write_file() {
  local path="$1" mode="$2" owner="$3" group="$4"
  local content tmp lines
  content="$(cat)"
  lines="$(printf '%s\n' "$content" | wc -l | tr -d ' ')"
  if [ "$DRY_RUN" = "1" ]; then
    printf 'would write: %s (mode %s, owner %s:%s, %s lines)\n' \
      "$path" "$mode" "$owner" "$group" "$lines"
    return 0
  fi
  tmp="$(mktemp "${path}.XXXXXX")"
  printf '%s\n' "$content" >"$tmp"
  chmod "$mode" "$tmp"
  chown "${owner}:${group}" "$tmp"
  mv -f "$tmp" "$path"
}

# confirm asks a yes or no question, and is only ever called when the answer
# can safely be no.
#
# It reads from /dev/tty rather than stdin on purpose. Under
# "bash -c "$(curl ...)"" stdin is still the terminal, but under "curl | bash"
# stdin is the script itself, and a read from it would swallow the rest of the
# installer. /dev/tty is the terminal in both cases, and its absence is the
# definition of non-interactive here.
confirm() {
  local prompt="$1" reply=""
  if [ "$ASSUME_YES" = "1" ]; then
    return 0
  fi
  if [ ! -r /dev/tty ] || [ ! -t 1 ]; then
    return 0
  fi
  printf '%s' "$prompt"
  read -r reply </dev/tty || reply=""
  case "$reply" in
    [Yy] | [Yy][Ee][Ss]) return 0 ;;
    *) return 1 ;;
  esac
}

is_interactive() {
  [ -r /dev/tty ] && [ -t 1 ]
}

# --- refusals: platform, architecture, init system -------------------------

check_platform() {
  local os
  os="$(uname -s)"
  if [ "$os" != "Linux" ]; then
    refuse "Found: $os." "Supported: Linux."
  fi
}

# detect_arch maps the kernel's name for the machine onto the release artefact
# names, which follow Go's convention instead (docs/LAYOUT.md, "Architecture
# naming"):
#
#   x86_64  -> caspian-linux-amd64
#   aarch64 -> caspian-linux-arm64
#   armv7l  -> caspian-linux-arm
#   armv6l  -> caspian-linux-arm
#
# The armv6l row is the one that has been got wrong before. A previous project
# in this workspace mapped armv6 onto an armv7 artefact; armv7 code uses
# instructions the ARM1176 in a Pi 1, a Pi Zero or a Pi Zero W does not have,
# so the binary installed cleanly and then died with an illegal instruction the
# first time it ran. Both 32-bit values map to the single "arm" artefact here,
# which is only correct while that artefact is built with GOARM=6. Building it
# GOARM=7 puts exactly the same bug back, one layer up, in the release pipeline
# instead of in this function. That requirement is recorded in docs/INSTALL.md.
#
# armv8l is deliberately not mapped. It is a 32-bit userland on a 64-bit
# kernel, docs/LAYOUT.md does not say which artefact it takes, and guessing is
# how the armv6 bug happened. It refuses and says so.
detect_arch() {
  local machine
  machine="$(uname -m)"
  case "$machine" in
    x86_64) ARTEFACT="caspian-linux-amd64" ;;
    aarch64) ARTEFACT="caspian-linux-arm64" ;;
    armv7l) ARTEFACT="caspian-linux-arm" ;;
    armv6l) ARTEFACT="caspian-linux-arm" ;;
    *)
      refuse "Found: $machine." \
        "Supported: x86_64, aarch64, armv7l, armv6l."
      ;;
  esac
}

# check_init requires systemd, because the units this installer places are
# systemd units and there is nothing else here that would start the services.
check_init() {
  local pid1="unknown" version=""
  if [ ! -d "${CASPIAN_SYSROOT}/run/systemd/system" ]; then
    if [ -r "${CASPIAN_SYSROOT}/proc/1/comm" ]; then
      pid1="$(tr -d '\n' <"${CASPIAN_SYSROOT}/proc/1/comm")"
    fi
    refuse "Found: init system $pid1, with no /run/systemd/system." \
      "Supported: systemd ${CASPIAN_MIN_SYSTEMD} or newer."
  fi
  if ! command -v systemctl >/dev/null 2>&1; then
    refuse "Found: /run/systemd/system exists but systemctl is not on PATH." \
      "Supported: systemd ${CASPIAN_MIN_SYSTEMD} or newer."
  fi
  version="$(systemctl --version 2>/dev/null | awk 'NR==1{print $2}')"
  version="${version%%[!0-9]*}"
  if [ -z "$version" ]; then
    warn "could not read the systemd version; continuing"
    return 0
  fi
  if [ "$version" -lt "$CASPIAN_MIN_SYSTEMD" ]; then
    refuse "Found: systemd $version." \
      "Supported: systemd ${CASPIAN_MIN_SYSTEMD} or newer, which is where Type=exec arrived."
  fi
}

require_root() {
  if [ "$DRY_RUN" = "1" ]; then
    return 0
  fi
  if [ "$(id -u)" != "0" ]; then
    die "this installer must run as root. Try: sudo /bin/bash -c \"\$(curl -fsSL <url>)\""
  fi
}

# --- dependencies ----------------------------------------------------------

# detect_package_manager finds the manager rather than assuming apt. The order
# is most specific first; a box with both apt and something else is a Debian
# derivative and apt is the right answer.
detect_package_manager() {
  local m
  for m in apt-get dnf yum pacman zypper apk; do
    if command -v "$m" >/dev/null 2>&1; then
      case "$m" in
        apt-get) PKG_MANAGER="apt" ;;
        *) PKG_MANAGER="$m" ;;
      esac
      return 0
    fi
  done
  PKG_MANAGER=""
}

# dep_command maps a package name from docs/LAYOUT.md to the command it
# provides. Presence is tested by command and never by package database,
# because "is nft installed" is the question that matters and it has the same
# answer on every distribution.
dep_command() {
  case "$1" in
    hostapd) printf 'hostapd' ;;
    dnsmasq) printf 'dnsmasq' ;;
    nftables) printf 'nft' ;;
    iw) printf 'iw' ;;
    iproute2) printf 'ip' ;;
    *) printf '%s' "$1" ;;
  esac
}

# dep_package maps a package name from docs/LAYOUT.md to this distribution's
# name for it. Only one differs, and the failure mode if any of these is wrong
# is a clear message from verify_dependencies naming the command that is still
# missing, not a silent half-install.
dep_package() {
  case "$1:$2" in
    dnf:iproute2 | yum:iproute2) printf 'iproute' ;;
    *) printf '%s' "$2" ;;
  esac
}

missing_dependencies() {
  local dep cmd out=""
  for dep in $CASPIAN_DEPS; do
    cmd="$(dep_command "$dep")"
    if ! command -v "$cmd" >/dev/null 2>&1; then
      out="${out} ${dep}"
    fi
  done
  printf '%s' "${out# }"
}

install_packages() {
  local pkgs="$1"
  # shellcheck disable=SC2086
  # Word splitting is wanted: pkgs is a space-separated list this script built
  # itself from a fixed table, never from anything a user typed.
  set -- $pkgs
  case "$PKG_MANAGER" in
    apt)
      # A fresh Raspberry Pi OS image often has no package lists at all, and
      # apt-get install then fails with "unable to locate package" for software
      # that is in the archive.
      run env DEBIAN_FRONTEND=noninteractive apt-get update
      run env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "$@"
      ;;
    dnf) run dnf install -y "$@" ;;
    yum) run yum install -y "$@" ;;
    pacman) run pacman -S --needed --noconfirm "$@" ;;
    zypper) run zypper --non-interactive install "$@" ;;
    apk) run apk add --no-cache "$@" ;;
    *) die "no supported package manager found (looked for apt-get, dnf, yum, pacman, zypper, apk)" ;;
  esac
}

ensure_dependencies() {
  local missing pkgs dep still
  missing="$(missing_dependencies)"
  if [ -z "$missing" ]; then
    step "Dependencies: all present."
    return 0
  fi
  if [ -z "$PKG_MANAGER" ]; then
    die "these are missing and no supported package manager was found: $missing"
  fi
  pkgs=""
  for dep in $missing; do
    pkgs="${pkgs} $(dep_package "$PKG_MANAGER" "$dep")"
  done
  pkgs="${pkgs# }"

  step "Dependencies to install with ${PKG_MANAGER}: ${pkgs}"
  if is_interactive && [ "$ASSUME_YES" != "1" ]; then
    if ! confirm "Install them now? [y/N] "; then
      die "declined. Nothing has been changed."
    fi
  fi
  install_packages "$pkgs"

  if [ "$DRY_RUN" = "1" ]; then
    return 0
  fi
  still="$(missing_dependencies)"
  if [ -n "$still" ]; then
    for dep in $still; do
      warn "still missing after install: command $(dep_command "$dep"), tried package $(dep_package "$PKG_MANAGER" "$dep")"
    done
    die "dependencies could not be installed. Nothing further has been changed."
  fi
}

# --- fetching and verifying the binary -------------------------------------

resolve_release() {
  # Resolve once so the binary and checksum cannot come from different
  # releases if GitHub's latest pointer changes during this installation.
  [ "$CASPIAN_VERSION" = latest ] || return 0
  [ -z "$CASPIAN_BASE_URL" ] || return 0
  [ "$DRY_RUN" = 0 ] || return 0
  local metadata tag
  metadata="${WORK_DIR}/release.json"
  fetch_to "https://api.github.com/repos/${CASPIAN_ORG}/${CASPIAN_REPO}/releases/latest" "$metadata"
  tag="$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$metadata")"
  case "$tag" in
    ''|*[!a-zA-Z0-9._-]*) die "GitHub did not return a valid latest release tag. Nothing has been changed." ;;
  esac
  CASPIAN_VERSION="$tag"
  step "Latest release: ${CASPIAN_VERSION}"
}

resolve_base_url() {
  if [ -n "$CASPIAN_BASE_URL" ]; then
    printf '%s' "${CASPIAN_BASE_URL%/}"
    return 0
  fi
  if [ -z "$CASPIAN_ORG" ]; then
    die "no download location. CASPIAN_ORG is empty.
Set CASPIAN_BASE_URL to a release directory, or CASPIAN_LOCAL_BINARY to a file on
this machine. See docs/INSTALL.md."
  fi
  if [ "$CASPIAN_VERSION" = "latest" ]; then
    printf 'https://github.com/%s/%s/releases/latest/download' "$CASPIAN_ORG" "$CASPIAN_REPO"
  else
    printf 'https://github.com/%s/%s/releases/download/%s' "$CASPIAN_ORG" "$CASPIAN_REPO" "$CASPIAN_VERSION"
  fi
}

check_url_scheme() {
  local url="$1"
  case "$url" in
    https://*) return 0 ;;
    *) ;;
  esac
  if [ "$CASPIAN_ALLOW_INSECURE_URL" = "1" ]; then
    warn "downloading over a plaintext URL because CASPIAN_ALLOW_INSECURE_URL=1: $url"
    return 0
  fi
  die "refusing to download over a plaintext URL: $url
The SHA-256 check below proves the artefact matches the checksums file, but both
come from the same place, so it cannot detect somebody who controls both. HTTPS is
what defends against that. Set CASPIAN_ALLOW_INSECURE_URL=1 only for local testing."
}

fetch_to() {
  local url="$1" dest="$2"
  check_url_scheme "$url"
  if command -v curl >/dev/null 2>&1; then
    run curl -fsSL --retry 3 --connect-timeout 20 -o "$dest" "$url"
  elif command -v wget >/dev/null 2>&1; then
    run wget -q -O "$dest" "$url"
  else
    die "neither curl nor wget is available to download $url"
  fi
}

sha256_of_file() {
  local f="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$f" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$f" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$f" | awk '{print $NF}'
  else
    die "no SHA-256 tool found (looked for sha256sum, shasum, openssl)"
  fi
}

# expected_sha256 pulls one artefact's hash out of a sha256sum-format file.
# Both the plain and the binary-mode ("*name") spellings are accepted because
# both are produced by the same tool depending on how it was invoked.
expected_sha256() {
  local checksums="$1" name="$2" hash
  hash="$(awk -v n="$name" '$2 == n || $2 == "*" n { print $1; exit }' "$checksums")"
  printf '%s' "$hash"
}

# verify_sha256 refuses on anything it cannot prove. A missing entry, a
# malformed hash and a mismatch are three different messages, because they are
# three different problems: the release is incomplete, the checksums file is
# damaged, or the artefact is not the one that was published.
verify_sha256() {
  local file="$1" checksums="$2" name="$3" expected actual
  expected="$(expected_sha256 "$checksums" "$name")"
  if [ -z "$expected" ]; then
    die "no entry for $name in the checksums file. Refusing to install an unverified binary."
  fi
  case "$expected" in
    [0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F]) ;;
    *) die "the checksums entry for $name is not a SHA-256 hash. Refusing to install an unverified binary." ;;
  esac
  actual="$(sha256_of_file "$file")"
  if [ "$(printf '%s' "$expected" | tr 'A-F' 'a-f')" != "$(printf '%s' "$actual" | tr 'A-F' 'a-f')" ]; then
    die "SHA-256 mismatch for $name.
  expected $expected
  got      $actual
Refusing to install an unverified binary. Nothing has been changed."
  fi
  step "Verified SHA-256 of ${name}."
}

# acquire_binary leaves a verified executable at ${WORK_DIR}/caspian.
acquire_binary() {
  local base checksums_url artefact_url target
  target="${WORK_DIR}/caspian"

  if [ -n "$CASPIAN_LOCAL_BINARY" ]; then
    [ -f "$CASPIAN_LOCAL_BINARY" ] || die "CASPIAN_LOCAL_BINARY is not a file: $CASPIAN_LOCAL_BINARY"
    step "Using local binary: ${CASPIAN_LOCAL_BINARY}"
    # This copy and the verification below happen even under --dry-run. They
    # touch nothing outside a private temporary directory, and running them for
    # real is the only way a dry run can prove that the checksum path works
    # before a release exists to test it against.
    cp "$CASPIAN_LOCAL_BINARY" "$target"
    if [ -n "$CASPIAN_LOCAL_CHECKSUMS" ]; then
      [ -f "$CASPIAN_LOCAL_CHECKSUMS" ] || die "CASPIAN_LOCAL_CHECKSUMS is not a file: $CASPIAN_LOCAL_CHECKSUMS"
      verify_sha256 "$target" "$CASPIAN_LOCAL_CHECKSUMS" "$ARTEFACT"
    else
      # Stated rather than silent. A local file the operator chose is a
      # different trust decision from a download, and they should know which
      # one they made.
      warn "no CASPIAN_LOCAL_CHECKSUMS given, so the local binary is being installed unverified"
    fi
    return 0
  fi

  resolve_release
  base="$(resolve_base_url)"
  artefact_url="${base}/${ARTEFACT}"
  checksums_url="${base}/${CASPIAN_CHECKSUMS_NAME}"

  step "Downloading ${ARTEFACT}"
  fetch_to "$artefact_url" "$target"
  fetch_to "$checksums_url" "${WORK_DIR}/${CASPIAN_CHECKSUMS_NAME}"

  if [ "$DRY_RUN" = "1" ]; then
    printf 'would verify: SHA-256 of %s against %s\n' "$ARTEFACT" "$CASPIAN_CHECKSUMS_NAME"
    return 0
  fi
  verify_sha256 "$target" "${WORK_DIR}/${CASPIAN_CHECKSUMS_NAME}" "$ARTEFACT"
}

# --- users, directories, units ---------------------------------------------

ensure_group() {
  if getent group "$CASPIAN_GROUP" >/dev/null 2>&1; then
    return 0
  fi
  if command -v groupadd >/dev/null 2>&1; then
    run groupadd --system "$CASPIAN_GROUP"
  elif command -v addgroup >/dev/null 2>&1; then
    run addgroup -S "$CASPIAN_GROUP"
  else
    die "no groupadd or addgroup found; cannot create the ${CASPIAN_GROUP} group"
  fi
}

nologin_shell() {
  local s
  for s in /usr/sbin/nologin /sbin/nologin /bin/false; do
    if [ -x "${CASPIAN_SYSROOT}${s}" ]; then
      printf '%s' "$s"
      return 0
    fi
  done
  printf '%s' "/bin/false"
}

# ensure_user creates the system account the panel runs as. A system account,
# with no login shell and no home directory of its own, because it exists to
# own one directory and one socket and to be unable to do anything else.
ensure_user() {
  local shell
  if getent passwd "$CASPIAN_USER" >/dev/null 2>&1; then
    return 0
  fi
  shell="$(nologin_shell)"
  if command -v useradd >/dev/null 2>&1; then
    run useradd --system --gid "$CASPIAN_GROUP" --home-dir "$CASPIAN_STATE_DIR" \
      --no-create-home --shell "$shell" "$CASPIAN_USER"
  elif command -v adduser >/dev/null 2>&1; then
    run adduser -S -D -H -G "$CASPIAN_GROUP" -h "$CASPIAN_STATE_DIR" -s "$shell" "$CASPIAN_USER"
  else
    die "no useradd or adduser found; cannot create the ${CASPIAN_USER} account"
  fi
}

# ensure_directories creates exactly the directories in the docs/LAYOUT.md path
# table, with exactly the modes in it. It never touches the contents of the
# state directory: on an upgrade that directory already holds the user's config
# and the panel password, and destroying it is the one thing an upgrade must
# never do.
ensure_directories() {
  # 0700 caspian:caspian. It holds a credential, so the mode is the access
  # control: no other account on the box can read it, whatever the file modes
  # inside happen to be.
  run install -d -m 0700 -o "$CASPIAN_USER" -g "$CASPIAN_GROUP" "$DEST_STATE_DIR"

  # 0750 root:caspian. Root owns it and the panel's group can traverse it to
  # reach the socket. Nothing else can. It is on a tmpfs, so it is gone after a
  # reboot; /etc/tmpfiles.d/caspian.conf is what brings it back.
  run install -d -m 0750 -o root -g "$CASPIAN_GROUP" "$DEST_RUN_DIR"

  # 0700 caspian:caspian, and a directory of its own rather than a file in the
  # one above.
  #
  # dnsmasq drops to the caspian account and then writes its pid file.
  # /run/caspian is 0750 root:caspian, so the group can list it and cannot
  # write in it, and whether dnsmasq writes the pid before or after it drops
  # privileges is a property of dnsmasq that nobody here has measured. Giving
  # dnsmasq a directory it owns means the answer stops mattering, which is
  # better than measuring it once and then depending on it staying true across
  # a dnsmasq upgrade.
  #
  # THE TRAP, and if you are reading this it is probably because a pid file
  # will not write: do NOT fix that by making /run/caspian group-writable.
  # Permission to create and delete inside a directory comes from the
  # directory, not from the file, so a group-writable /run/caspian would let
  # the unprivileged panel account delete hostapd.conf and write its own, which
  # the privileged side then hands to hostapd running as root. That turns a
  # pid-file inconvenience into local privilege escalation. See
  # docs/LAYOUT.md, "Why dnsmasq gets its own directory".
  #
  # This one is also recreated at every boot by the tmpfiles fragment.
  run install -d -m 0700 -o "$CASPIAN_USER" -g "$CASPIAN_GROUP" "$DEST_DNSMASQ_RUN_DIR"
}

install_binary() {
  # install(1) writes to a temporary name and renames, so the binary is never
  # observed half written, and replacing a running executable this way is safe
  # because the old inode stays alive until the old process exits.
  if [ "$DRY_RUN" = "1" ]; then
    printf 'would run: %s\n' "$(show_cmd install -m 0755 -o root -g root "${WORK_DIR}/caspian" "$DEST_BIN")"
    return 0
  fi
  install -m 0755 -o root -g root "${WORK_DIR}/caspian" "$DEST_BIN"
}

# --- the files that are placed on the box ----------------------------------
#
# These four are byte-identical copies of the files in packaging/, which is the
# source of truth for them. They are embedded because a script piped from curl
# cannot read a repository, and because downloading them would add unverified
# artefacts beside the one this installer checksums. packaging/test-install.sh fails
# if a copy here drifts from packaging/.

unit_caspian_service() {
  cat <<'CASPIAN_UNIT_EOF'
# Caspian-BYOC, privileged network service.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Installed by install.sh to /etc/systemd/system/caspian.service. This copy in
# packaging/ is the source of truth. install.sh carries a byte-identical copy
# inline, because a script piped from curl has no repository to read from, and
# packaging/test-install.sh proves the two are still identical.
#
# This is the half that holds root. It owns routes, the firewall, the access
# point and the engine (LAYOUT.md, "Two processes, one binary"). The hardening
# below is written to keep that job possible: several obvious directives are
# switched off on purpose and each one says what it would have broken, because
# a directive that silently disables the product is worse than no directive.

[Unit]
Description=Caspian-BYOC privileged network service

# network.target only, deliberately not network-online.target. Waiting for the
# network to be up would add a boot delay of up to 90 seconds on a box with no
# cable plugged in, and it would buy nothing: the uplink can change under the
# service at any time (design section 9, "Uplink change"), so the address and
# gateway have to be re-derived at runtime regardless of what was true at boot.
After=network.target

# /run is a tmpfs, so /run/caspian does not survive a reboot. It is recreated
# on every boot by /etc/tmpfiles.d/caspian.conf, which runs as part of
# sysinit.target. Ordering after that is already implied by the default
# dependencies, and is stated here so that removing DefaultDependencies later
# does not silently break the socket directory.
After=systemd-tmpfiles-setup.service

# A crash loop must not hammer the radio. Five starts in five minutes, then
# stop and wait for a human or a reboot.
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
# Type=exec, not simple: systemd then treats the unit as started only once the
# execve has actually succeeded, so a missing or non-executable binary is a
# failed start rather than a start that reports success and dies. Requires
# systemd 240 or newer, which install.sh checks for before writing this file.
Type=exec
ExecStart=/usr/local/bin/caspian serve --privileged

Restart=on-failure
RestartSec=2s

# The teardown journal on disk (/var/lib/caspian/netcfg.journal) exists
# because this stop can be a SIGKILL, and a process that has been killed cannot
# put the routes back (design section 5.5). The generous stop timeout gives the
# ordinary path a chance to run first.
TimeoutStopSec=20s

# Default KillMode is control-group, which is what is wanted here: hostapd and
# dnsmasq are started with -B and daemonize away from this process, but they
# stay in the unit's cgroup, so stopping this unit takes the hotspot down with
# it instead of leaving an access point beaconing with no tunnel behind it.

# --- hardening: what is granted, and why each grant is needed --------------

# The full root capability set is not needed. This list is the whole of it.
#
#   CAP_NET_ADMIN        routes, ip rules, the nftables ruleset, interface
#                        state, creating the tunnel device, and unblocking the
#                        radio through rfkill. This is the capability the unit
#                        exists for.
#   CAP_NET_RAW          dnsmasq asks for it when it drops privileges; without
#                        it in the bounding set the drop fails and dnsmasq
#                        exits, which shows up as "the hotspot has no DHCP".
#   CAP_NET_BIND_SERVICE dnsmasq binds port 53 for client DNS. Design section 6
#                        requires client DNS to be answered on the box.
#   CAP_SETUID/SETGID    dnsmasq drops from root to its own unprivileged user
#                        after binding. Removing these does not make it safer,
#                        it makes it stay root.
#   CAP_KILL             the supervisor kills stray hostapd and dnsmasq
#                        processes left by a previous run
#                        (internal/hotspot/supervisor.go, stopStrays). A
#                        dnsmasq that has already dropped to another uid is a
#                        different-uid signal target, which needs CAP_KILL.
#   CAP_DAC_OVERRIDE     two directories in docs/LAYOUT.md are 0700 and
#                        owned by caspian while this service runs as root:
#                        /var/lib/caspian, where it writes netcfg.journal and
#                        reads the dnsmasq lease file, and /run/caspian/dnsmasq,
#                        where it reads the pid file dnsmasq wrote after
#                        dropping privileges. Without this, root cannot even
#                        traverse either one.
# CAP_CHOWN is here because the service gives /run/caspian/priv.sock to
# root:caspian, and that ownership is what makes mode 0660 a boundary instead of
# a decoration: it is what lets the unprivileged panel account reach the socket
# while nobody else can. Without this capability the chown fails with EPERM even
# though the service runs as root, and the service exits at startup rather than
# serving a socket anyone could open. Measured on the target on 2026-08-30: that
# is exactly what happened, and it is why this line names eight capabilities and
# not seven.
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_NET_BIND_SERVICE CAP_SETUID CAP_SETGID CAP_KILL CAP_DAC_OVERRIDE CAP_CHOWN

# Nothing this service or its children execute should ever gain privilege from
# a setuid bit or a file capability. Protects against a compromised or
# substituted hostapd/dnsmasq binary escalating beyond the set above.
NoNewPrivileges=yes

# The filesystem is read-only except for the four places that are written.
# Protects against a compromised engine or a hostile generated config file
# writing to /usr, /boot or /etc.
ProtectSystem=strict
ReadWritePaths=/var/lib/caspian /run/caspian
# hostapd puts its control socket here. Optional with the leading dash so that
# a box where the directory does not exist yet still starts.
ReadWritePaths=-/run/hostapd

# No home directory is used by anything here. Protects the user's own files
# from a fault in a network-facing process running as root.
ProtectHome=yes

# A private /tmp and /var/tmp. Protects against a symlink attack on a
# predictable temporary path, which is a classic way to turn a root process
# into an arbitrary-file-write.
PrivateTmp=yes

# --- hardening: what is deliberately NOT set, and what it would have broken -

# PrivateDevices=yes is NOT set. It would give this service a private /dev
# containing only a minimal set of pseudo devices, and /dev/net/tun would not
# be in it. The engine's TUN inbound opens /dev/net/tun to create the tunnel
# (design section 4.2), so this would stop the product working entirely.
#
# Instead of the blunt switch, the device list is closed and reopened for
# exactly the two nodes that are needed. DevicePolicy=closed still permits the
# standard pseudo devices (null, zero, full, random, urandom, tty).
DevicePolicy=closed
DeviceAllow=/dev/net/tun rw
# rfkill: the built-in radio on a Pi is frequently soft blocked, and hostapd's
# own failure in that state is unreadable (internal/hotspot/supervisor.go,
# ensureRadioUnblocked).
DeviceAllow=/dev/rfkill rw

# ProtectKernelTunables=yes is NOT set. It mounts /proc/sys read-only, and this
# service has to write net.ipv4.conf.*.rp_filter. Design section 4.2 is
# explicit that getting rp_filter wrong produces a tunnel that connects and
# carries nothing, which is the hardest failure in this product to diagnose.
ProtectKernelTunables=no

# ProtectKernelModules=yes is NOT set. The kernel on the target carries
# NF_TABLES, NFT_NAT, NFT_MASQ, NFT_REDIR and CONFIG_TUN as modules (design
# section 4.6), and they are autoloaded on demand when the ruleset is applied
# and when the tunnel device is created. Denying module loading would fail at
# ruleset-load time, after forwarding has been enabled.
# /etc/modules-load.d/caspian.conf pre-loads the two that are needed earliest,
# so the common path does not depend on autoload succeeding.
ProtectKernelModules=no

# ProtectProc=invisible is NOT set. The supervisor runs pgrep to find stray
# hostapd and dnsmasq processes from a previous run that are holding the radio
# or port 53. Hiding other processes would make that search always come back
# empty, and the symptom would be an access point that will not start with no
# explanation.
ProtectProc=default

# --- hardening: the rest, none of which restricts the job -------------------

# The cgroup hierarchy is read-only. Protects against a compromised process
# editing its own resource limits or escaping its cgroup.
ProtectControlGroups=yes

# The kernel ring buffer is not readable. Protects against reading kernel
# addresses and other hosts' traffic metadata out of dmesg.
ProtectKernelLogs=yes

# No new namespaces. Protects against a compromised process building a user
# namespace and using it to get capabilities it was not granted here.
RestrictNamespaces=yes

# Cannot set the setuid or setgid bit on any file it creates. Protects against
# leaving a permanent root backdoor on disk.
RestrictSUIDSGID=yes

# No realtime scheduling. Protects against a busy loop at realtime priority
# making the box unresponsive, on a machine with four cores and no console.
RestrictRealtime=yes

# The personality(2) syscall is locked. Protects against switching to an
# emulated or legacy execution domain to dodge the syscall filter below.
LockPersonality=yes

# System V IPC objects belonging to this service are removed when it stops.
RemoveIPC=yes

# Only the address families that are actually used:
#   AF_UNIX    the privileged socket at /run/caspian/priv.sock
#   AF_INET    IPv4, the tunnel and everything the engine dials
#   AF_INET6   IPv6, which is blocked for clients but still used by the box
#   AF_NETLINK ip, nft, iw and the engine's own netlink work
#   AF_PACKET  hostapd speaks 802.11 management frames over a packet socket
# Protects against a compromised process reaching a kernel subsystem it has no
# business in, for example AF_BLUETOOTH or AF_VSOCK.
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK AF_PACKET

# Only this machine's own syscall ABI. Protects against a 32-bit compatibility
# entry point being used to reach a syscall the filter below does not cover.
SystemCallArchitectures=native

# The ordinary system-service syscall set. Protects against a compromised
# process reaching @swap, @reboot, @mount, @raw-io and the other groups a
# network daemon never needs.
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM

[Install]
WantedBy=multi-user.target
CASPIAN_UNIT_EOF
}

unit_caspian_panel_service() {
  cat <<'CASPIAN_UNIT_EOF'
# Caspian-BYOC, web panel.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Installed by install.sh to /etc/systemd/system/caspian-panel.service. This
# copy in packaging/ is the source of truth. install.sh carries a
# byte-identical copy inline, and packaging/test-install.sh proves they match.
#
# This is the half that holds no privilege. It parses everything a user typed
# and serves HTTP; the split exists so that a fault here is not a fault in the
# part that holds root (LAYOUT.md, "Two processes, one binary"). Because it
# needs nothing privileged, its hardening can be as tight as systemd allows,
# and where the privileged unit has to switch a directive off, this one does
# not.

[Unit]
Description=Caspian-BYOC web panel

# Ordered after the privileged service but only Wants=, never Requires=.
# Design section 5.6 records the hazard that a user who cannot reach the panel
# cannot fix anything, so the panel must come up and be able to say what is
# wrong even when the privileged side has failed to start. Requires= would take
# the panel down with it and leave the user with a box and no way in.
After=caspian.service
Wants=caspian.service

StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
# See the note in caspian.service. Requires systemd 240 or newer.
Type=exec
User=caspian
Group=caspian
ExecStart=/usr/local/bin/caspian serve --panel

Restart=on-failure
RestartSec=2s

# --- hardening -------------------------------------------------------------

# No capabilities at all, in either set. The panel listens on port 8088
# (internal/netcfg/plan.go, DefaultOptions), which is above 1024, so it does
# not even need CAP_NET_BIND_SERVICE. Anything privileged goes over the socket
# to caspian.service, which is the whole point of the split.
CapabilityBoundingSet=
AmbientCapabilities=

# Protects against a setuid binary or a file capability turning a bug in the
# HTTP surface into a privilege escalation. This is the directive that makes
# the empty capability set above stick across an exec.
NoNewPrivileges=yes

# The filesystem is read-only except for the two places the panel writes.
# Protects against a path-traversal or template bug in the panel writing
# anywhere outside its own state.
ProtectSystem=strict
# state.json, written atomically by internal/state (LAYOUT.md, "Paths").
ReadWritePaths=/var/lib/caspian
# The socket to the privileged service. Connecting to a unix socket needs write
# access to the socket inode, so a read-only /run would break the split.
ReadWritePaths=/run/caspian

# No home directories. Protects the user's own files from a bug in a process
# that parses untrusted input (design section 6).
ProtectHome=yes

# A private /tmp. Protects against symlink attacks on predictable temporary
# paths, which matters here because the panel accepts an uploaded QR image
# (design section 9, "QR": untrusted image parsing inside the panel process).
PrivateTmp=yes

# A private /dev with only the standard pseudo devices. The panel opens no
# device node; unlike the privileged unit it has no reason to see /dev/net/tun
# or /dev/rfkill, so the blunt switch is correct here.
PrivateDevices=yes

# /proc/sys, /sys, the module loader and the kernel log are all closed.
# Protects against a compromised panel reaching kernel state that only the
# privileged half is allowed to touch.
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes

# The panel cannot see any process but its own. Protects against reading the
# engine's command line or environment, which is where credentials would be if
# anyone ever put them there.
ProtectProc=invisible
ProcSubset=pid

# No new namespaces, no setuid files, no realtime, no personality switch, no
# leftover IPC. Same reasons as in caspian.service.
RestrictNamespaces=yes
RestrictSUIDSGID=yes
RestrictRealtime=yes
LockPersonality=yes
RemoveIPC=yes

# No writable-and-executable memory. Go does not generate code at runtime, so
# this costs nothing here and removes the easiest route from a memory bug to
# arbitrary code.
MemoryDenyWriteExecute=yes

# The panel speaks HTTP over TCP and the privileged socket over AF_UNIX, and
# nothing else. No AF_NETLINK: the panel has no business enumerating or
# changing interfaces, and if it ever appears to need it, that is a sign the
# work belongs on the other side of the socket.
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6

SystemCallArchitectures=native
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM

[Install]
WantedBy=multi-user.target
CASPIAN_UNIT_EOF
}

unit_tmpfiles_conf() {
  cat <<'CASPIAN_UNIT_EOF'
# Caspian-BYOC runtime directory.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Installed by install.sh to /etc/tmpfiles.d/caspian.conf.
#
# /run is a tmpfs, so /run/caspian does not survive a reboot and cannot simply
# be created once by the installer. systemd-tmpfiles-setup.service recreates it
# on every boot with exactly the mode and ownership LAYOUT.md fixes: 0750,
# root:caspian, so that the panel (group caspian) can traverse the directory
# and reach the socket while nothing else on the box can.
#
# Type Path             Mode Owner   Group   Age Argument
d /run/caspian          0750 root    caspian -

# dnsmasq's own directory, for its pid file.
#
# It is a directory rather than a file in the one above because dnsmasq drops
# to the caspian account and then writes its pid, /run/caspian is 0750
# root:caspian so the group can list it and cannot write in it, and whether
# dnsmasq writes the pid before or after dropping privileges is a property of
# dnsmasq nobody here has measured. A directory dnsmasq owns makes the answer
# stop mattering.
#
# THE TRAP: do not "fix" a pid file that will not write by making /run/caspian
# group-writable. Permission to create and delete inside a directory comes from
# the directory, not the file, so that would let the unprivileged panel account
# delete hostapd.conf and write its own, which the privileged side then hands
# to hostapd running as root. See docs/LAYOUT.md, "Why dnsmasq gets its own
# directory".
d /run/caspian/dnsmasq  0700 caspian caspian -

# hostapd's control socket directory, which the supervisor talks to through
# hostapd_cli to ask whether the access point is actually beaconing.
d /run/hostapd          0750 root    root    -
CASPIAN_UNIT_EOF
}

unit_modules_load_conf() {
  cat <<'CASPIAN_UNIT_EOF'
# Caspian-BYOC kernel modules.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Installed by install.sh to /etc/modules-load.d/caspian.conf.
#
# On the target these are modules rather than built in (design section 4.6).
# Loading them at boot means the two earliest and least recoverable failures do
# not depend on on-demand autoload succeeding inside a hardened unit: the
# engine opening /dev/net/tun, and the first nftables ruleset being applied
# before any client traffic is forwarded.
tun
nf_tables
CASPIAN_UNIT_EOF
}

# install_units places the two units and the two boot-time fragments. The unit
# text is embedded above rather than downloaded, for two reasons: a script
# piped from curl has no repository to read from, and a downloaded unit file
# would be a second unverified artefact next to the one this installer goes to
# the trouble of checksumming. packaging/ holds the source of truth and
# packaging/test-install.sh proves the embedded copies still match it byte for byte.
install_units() {
  unit_caspian_service | write_file "${DEST_UNIT_DIR}/${CASPIAN_UNIT_PRIV}" 0644 root root
  unit_caspian_panel_service | write_file "${DEST_UNIT_DIR}/${CASPIAN_UNIT_PANEL}" 0644 root root
  unit_tmpfiles_conf | write_file "$DEST_TMPFILES" 0644 root root
  unit_modules_load_conf | write_file "$DEST_MODULES" 0644 root root
}

reload_systemd() {
  run systemctl daemon-reload
}

# apply_tmpfiles creates /run/caspian now, rather than waiting for the next
# boot. Without this the first start after a fresh install has nowhere to put
# the privileged socket.
apply_tmpfiles() {
  if ! command -v systemd-tmpfiles >/dev/null 2>&1; then
    warn "systemd-tmpfiles not found; ${CASPIAN_RUN_DIR} will only appear at the next boot"
    return 0
  fi
  if ! run systemd-tmpfiles --create "$CASPIAN_TMPFILES_PATH"; then
    warn "systemd-tmpfiles could not create ${CASPIAN_RUN_DIR}"
  fi
}

# load_modules is best effort. If it fails the service will still try, and a
# failure here is much easier to read than the same failure later from inside a
# hardened unit.
load_modules() {
  local m
  if ! command -v modprobe >/dev/null 2>&1; then
    return 0
  fi
  for m in tun nf_tables; do
    if ! run modprobe "$m"; then
      warn "could not load the ${m} kernel module now; it will be loaded at the next boot"
    fi
  done
}

# unit_installed answers from the filesystem rather than from systemctl,
# because it has to give the same answer under --dry-run with a test sysroot,
# where there is no systemd to ask.
unit_installed() {
  [ -f "${DEST_UNIT_DIR}/$1" ]
}

# stop_services runs before the binary is replaced. Replacing the executable
# under a running process is safe on Linux, but the running process would keep
# the old code until something restarted it, and an upgrade that appears to
# have happened and has not is worse than one that visibly stops for a moment.
#
# Panel first, then the privileged service, which is the reverse of the order
# they start in.
stop_services() {
  local u
  for u in "$CASPIAN_UNIT_PANEL" "$CASPIAN_UNIT_PRIV"; do
    if unit_installed "$u"; then
      run systemctl stop "$u"
    fi
  done
}

enable_services() {
  run systemctl enable "$CASPIAN_UNIT_PRIV"
  run systemctl enable "$CASPIAN_UNIT_PANEL"
  # restart rather than start, so this is the same call on a fresh install and
  # on an upgrade.
  run systemctl restart "$CASPIAN_UNIT_PRIV"
  run systemctl restart "$CASPIAN_UNIT_PANEL"
}

# --- the first-run password ------------------------------------------------

# random_chars gathers characters from /dev/urandom.
#
# The obvious one-liner, "tr -dc set </dev/urandom | head -c n", is not used:
# head closing the pipe kills tr with SIGPIPE, and under "set -o pipefail" that
# makes the whole command fail. Reading a fixed block and filtering it has no
# such race.
random_chars() {
  local want="$1" alphabet="$2" out="" chunk="" guard=0
  while [ "${#out}" -lt "$want" ]; do
    guard=$((guard + 1))
    if [ "$guard" -gt 20 ]; then
      die "could not read enough randomness from /dev/urandom"
    fi
    chunk="$(dd if=/dev/urandom bs=256 count=1 2>/dev/null | LC_ALL=C tr -dc "$alphabet" || true)"
    out="${out}${chunk}"
  done
  printf '%s' "${out:0:$want}"
}

# generate_password produces the password printed at the end.
#
# Twenty characters from a thirty-two character alphabet is one hundred bits,
# which is far past anything that matters here; the alphabet is the part that
# was chosen with care. It has no 0, O, 1, l or I in it, because this password
# is read off a terminal and typed into a phone by somebody who did not choose
# it, and a character they cannot tell apart from another one is a support
# problem, not a security one. The hyphens are for reading, and are part of the
# password.
generate_password() {
  local raw
  raw="$(random_chars 20 'abcdefghijkmnpqrstuvwxyz23456789')"
  printf '%s-%s-%s-%s' "${raw:0:5}" "${raw:5:5}" "${raw:10:5}" "${raw:15:5}"
}

state_file_exists() {
  [ -f "${DEST_STATE_DIR}/state.json" ]
}

# seed_first_run_password writes the plaintext password where the panel will
# find it on its first start.
#
# It is written only when there is no state file, which is what makes a second
# run an upgrade rather than a lockout: an upgrade must not change a password
# the user has since chosen for themselves.
#
# The handoff is a file rather than a value passed on a command line or in the
# environment, because both of those are readable from /proc by anything on the
# box. The file is 0600, owned by the service user, inside a 0700 directory,
# and the panel deletes it once the hash is stored.
seed_first_run_password() {
  if state_file_exists; then
    step "Existing state found; keeping the current panel password."
    return 0
  fi
  if [ "$DRY_RUN" = "1" ]; then
    GENERATED_PASSWORD="xxxxx-xxxxx-xxxxx-xxxxx"
  else
    GENERATED_PASSWORD="$(generate_password)"
  fi
  printf '%s' "$GENERATED_PASSWORD" |
    write_file "$DEST_PASSWORD_SEED" 0600 "$CASPIAN_USER" "$CASPIAN_GROUP"
}

# --- the uninstaller -------------------------------------------------------

# place_uninstaller keeps a copy of the uninstall script on the box. Best
# effort: failing to fetch it is not a reason to fail an otherwise complete
# install, and the same script can always be run from the network instead.
place_uninstaller() {
  local base tmp
  if [ -n "$CASPIAN_UNINSTALL_SRC" ]; then
    if [ ! -f "$CASPIAN_UNINSTALL_SRC" ]; then
      warn "CASPIAN_UNINSTALL_SRC is not a file: $CASPIAN_UNINSTALL_SRC"
      return 0
    fi
    run install -m 0755 -o root -g root "$CASPIAN_UNINSTALL_SRC" "$DEST_UNINSTALL"
    return 0
  fi
  base="$CASPIAN_SCRIPT_BASE_URL"
  if [ -z "$base" ]; then
    if [ -z "$CASPIAN_ORG" ]; then
      if [ "$DRY_RUN" = "1" ]; then
        printf 'would skip: local uninstaller copy (no source configured)\n'
      fi
      return 0
    fi
    base="https://raw.githubusercontent.com/${CASPIAN_ORG}/${CASPIAN_REPO}/main"
  fi
  # This step is best effort, so it must never be the thing that ends an
  # otherwise complete install. check_url_scheme inside fetch_to exits on a
  # plaintext URL, which is right for the binary and wrong here, so the scheme
  # is checked first and a URL that would be refused is skipped instead.
  case "$base" in
    https://*) ;;
    *)
      if [ "$CASPIAN_ALLOW_INSECURE_URL" != "1" ]; then
        warn "not fetching the uninstaller from a plaintext URL: $base"
        return 0
      fi
      ;;
  esac
  tmp="${WORK_DIR}/uninstall.sh"
  if ! fetch_to "${base%/}/uninstall.sh" "$tmp"; then
    warn "could not fetch the uninstaller; ${CASPIAN_UNINSTALL_PATH} was not created"
    return 0
  fi
  run install -m 0755 -o root -g root "$tmp" "$DEST_UNINSTALL"
}

# --- the closing message ---------------------------------------------------

# panel_address finds an address the panel can be reached on right now.
#
# This is a best effort and the reason is worth stating: design section 5.6
# says the panel listens on the hotspot interface, and the hotspot does not
# exist until the user switches it on, so at the end of an install the only
# address that exists is the one the box already had. See docs/INSTALL.md.
panel_address() {
  local addr=""
  if command -v ip >/dev/null 2>&1; then
    addr="$(ip -4 -o addr show scope global 2>/dev/null | awk 'NR==1{split($4,a,"/"); print a[1]}')"
  fi
  if [ -z "$addr" ] && command -v hostname >/dev/null 2>&1; then
    addr="$(hostname -I 2>/dev/null | awk '{print $1}')"
  fi
  printf '%s' "$addr"
}

# final_message is the last thing on screen, and is kept to the two facts the
# user needs. The proxy config is never read by this installer, never printed
# and never written to any log: the only thing that ever holds it is the panel,
# and the only place it is stored is the state file (docs/LAYOUT.md).
final_message() {
  local addr
  addr="$(panel_address)"
  printf '\n'
  if [ "$DRY_RUN" = "1" ]; then
    say "Dry run finished. Nothing was changed."
    say ""
    say "On a real run the closing message would be:"
  fi
  say "Caspian-BYOC is installed."
  say ""
  if [ -n "$addr" ]; then
    say "Panel:    http://${addr}:${CASPIAN_PANEL_PORT}/"
  else
    say "Panel:    port ${CASPIAN_PANEL_PORT} on this box"
  fi
  if [ -n "$GENERATED_PASSWORD" ]; then
    say "Password: ${GENERATED_PASSWORD}"
  else
    say "Password: unchanged from the previous install"
    say "Forgot it? Run: sudo /usr/local/bin/caspian reset-password"
  fi
  say ""
  say "The panel also answers on the hotspot network once you switch it on."
}

# --- wiring ----------------------------------------------------------------

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --dry-run) DRY_RUN="1" ;;
      -y | --yes) ASSUME_YES="1" ;;
      -h | --help)
        usage
        exit 0
        ;;
      *) die "unknown option: $1 (try --help)" ;;
    esac
    shift
  done
  if [ "$CASPIAN_ASSUME_YES" = "1" ]; then
    ASSUME_YES="1"
  fi
}

# setup_dest_paths applies the test sysroot. On a real run CASPIAN_SYSROOT is
# empty and every destination is the path docs/LAYOUT.md fixes. It is refused
# outside a dry run, so it can never move a real install somewhere unexpected.
setup_dest_paths() {
  if [ -n "$CASPIAN_SYSROOT" ] && [ "$DRY_RUN" != "1" ]; then
    die "CASPIAN_SYSROOT is a testing hook and is only allowed together with --dry-run"
  fi
  DEST_BIN="${CASPIAN_SYSROOT}${CASPIAN_BIN_PATH}"
  DEST_STATE_DIR="${CASPIAN_SYSROOT}${CASPIAN_STATE_DIR}"
  DEST_RUN_DIR="${CASPIAN_SYSROOT}${CASPIAN_RUN_DIR}"
  DEST_DNSMASQ_RUN_DIR="${CASPIAN_SYSROOT}${CASPIAN_DNSMASQ_RUN_DIR}"
  DEST_UNIT_DIR="${CASPIAN_SYSROOT}${CASPIAN_UNIT_DIR}"
  DEST_TMPFILES="${CASPIAN_SYSROOT}${CASPIAN_TMPFILES_PATH}"
  DEST_MODULES="${CASPIAN_SYSROOT}${CASPIAN_MODULES_PATH}"
  DEST_PASSWORD_SEED="${CASPIAN_SYSROOT}${CASPIAN_PASSWORD_SEED}"
  DEST_UNINSTALL="${CASPIAN_SYSROOT}${CASPIAN_UNINSTALL_PATH}"
}

make_work_dir() {
  WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/caspian-install.XXXXXX")"
  trap 'rm -rf "$WORK_DIR"' EXIT
}

caspian_install_main() {
  # Sourcing hook, for the test harness. It makes main do nothing so that the
  # functions above can be called one at a time. It cannot cause a partial
  # install, and it is not a way to skip any check.
  if [ "${CASPIAN_SOURCE_ONLY:-0}" = "1" ]; then
    return 0
  fi

  parse_args "$@"

  # New files are readable and not group writable unless something asks for
  # otherwise. Every mode that matters is set explicitly anyway; this covers
  # the ones that do not.
  umask 022

  # Append rather than replace. useradd, groupadd and systemctl live in sbin
  # directories that are missing from the PATH of some sudo configurations,
  # which is a common way for an installer to fail late. Appending only adds
  # places to look and never overrides what the operator already has.
  PATH="${PATH}:/usr/local/sbin:/usr/sbin:/sbin"

  if [ "$DRY_RUN" = "1" ]; then
    say "Dry run. Nothing will be changed."
  fi

  # Every refusal happens here, before anything has been touched.
  check_platform
  detect_arch
  check_init
  require_root
  setup_dest_paths

  if [ -f "$DEST_BIN" ]; then
    IS_UPGRADE="1"
  fi

  detect_package_manager
  say "Architecture: $(uname -m), artefact ${ARTEFACT}."
  if [ "$IS_UPGRADE" = "1" ]; then
    say "Existing installation found. This run is an upgrade."
  else
    say "No existing installation found. This run is a fresh install."
  fi

  ensure_dependencies

  # The binary is fetched and verified before anything on the box is stopped or
  # replaced. A failed download or a checksum mismatch therefore leaves a
  # working installation exactly as it was.
  make_work_dir
  acquire_binary

  stop_services

  ensure_group
  ensure_user
  ensure_directories
  install_binary
  install_units
  reload_systemd
  apply_tmpfiles
  load_modules
  seed_first_run_password
  enable_services
  place_uninstaller

  final_message
}

# The single call, on the last line. See the note at the top: everything above
# is a definition, so a truncated download of this script does nothing at all.
caspian_install_main "$@"
