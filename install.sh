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
# License payloads are embedded so a streamed installer carries its notices.
# packaging/test-install.sh checks these against the source files.
license_project() {
  cat <<'CASPIAN_LICENSE_EOF'
                    GNU AFFERO GENERAL PUBLIC LICENSE
                       Version 3, 19 November 2007

 Copyright (C) 2007 Free Software Foundation, Inc. <http://fsf.org/>
 Everyone is permitted to copy and distribute verbatim copies
 of this license document, but changing it is not allowed.

                            Preamble

  The GNU Affero General Public License is a free, copyleft license for
software and other kinds of works, specifically designed to ensure
cooperation with the community in the case of network server software.

  The licenses for most software and other practical works are designed
to take away your freedom to share and change the works.  By contrast,
our General Public Licenses are intended to guarantee your freedom to
share and change all versions of a program--to make sure it remains free
software for all its users.

  When we speak of free software, we are referring to freedom, not
price.  Our General Public Licenses are designed to make sure that you
have the freedom to distribute copies of free software (and charge for
them if you wish), that you receive source code or can get it if you
want it, that you can change the software or use pieces of it in new
free programs, and that you know you can do these things.

  Developers that use our General Public Licenses protect your rights
with two steps: (1) assert copyright on the software, and (2) offer
you this License which gives you legal permission to copy, distribute
and/or modify the software.

  A secondary benefit of defending all users' freedom is that
improvements made in alternate versions of the program, if they
receive widespread use, become available for other developers to
incorporate.  Many developers of free software are heartened and
encouraged by the resulting cooperation.  However, in the case of
software used on network servers, this result may fail to come about.
The GNU General Public License permits making a modified version and
letting the public access it on a server without ever releasing its
source code to the public.

  The GNU Affero General Public License is designed specifically to
ensure that, in such cases, the modified source code becomes available
to the community.  It requires the operator of a network server to
provide the source code of the modified version running there to the
users of that server.  Therefore, public use of a modified version, on
a publicly accessible server, gives the public access to the source
code of the modified version.

  An older license, called the Affero General Public License and
published by Affero, was designed to accomplish similar goals.  This is
a different license, not a version of the Affero GPL, but Affero has
released a new version of the Affero GPL which permits relicensing under
this license.

  The precise terms and conditions for copying, distribution and
modification follow.

                       TERMS AND CONDITIONS

  0. Definitions.

  "This License" refers to version 3 of the GNU Affero General Public License.

  "Copyright" also means copyright-like laws that apply to other kinds of
works, such as semiconductor masks.

  "The Program" refers to any copyrightable work licensed under this
License.  Each licensee is addressed as "you".  "Licensees" and
"recipients" may be individuals or organizations.

  To "modify" a work means to copy from or adapt all or part of the work
in a fashion requiring copyright permission, other than the making of an
exact copy.  The resulting work is called a "modified version" of the
earlier work or a work "based on" the earlier work.

  A "covered work" means either the unmodified Program or a work based
on the Program.

  To "propagate" a work means to do anything with it that, without
permission, would make you directly or secondarily liable for
infringement under applicable copyright law, except executing it on a
computer or modifying a private copy.  Propagation includes copying,
distribution (with or without modification), making available to the
public, and in some countries other activities as well.

  To "convey" a work means any kind of propagation that enables other
parties to make or receive copies.  Mere interaction with a user through
a computer network, with no transfer of a copy, is not conveying.

  An interactive user interface displays "Appropriate Legal Notices"
to the extent that it includes a convenient and prominently visible
feature that (1) displays an appropriate copyright notice, and (2)
tells the user that there is no warranty for the work (except to the
extent that warranties are provided), that licensees may convey the
work under this License, and how to view a copy of this License.  If
the interface presents a list of user commands or options, such as a
menu, a prominent item in the list meets this criterion.

  1. Source Code.

  The "source code" for a work means the preferred form of the work
for making modifications to it.  "Object code" means any non-source
form of a work.

  A "Standard Interface" means an interface that either is an official
standard defined by a recognized standards body, or, in the case of
interfaces specified for a particular programming language, one that
is widely used among developers working in that language.

  The "System Libraries" of an executable work include anything, other
than the work as a whole, that (a) is included in the normal form of
packaging a Major Component, but which is not part of that Major
Component, and (b) serves only to enable use of the work with that
Major Component, or to implement a Standard Interface for which an
implementation is available to the public in source code form.  A
"Major Component", in this context, means a major essential component
(kernel, window system, and so on) of the specific operating system
(if any) on which the executable work runs, or a compiler used to
produce the work, or an object code interpreter used to run it.

  The "Corresponding Source" for a work in object code form means all
the source code needed to generate, install, and (for an executable
work) run the object code and to modify the work, including scripts to
control those activities.  However, it does not include the work's
System Libraries, or general-purpose tools or generally available free
programs which are used unmodified in performing those activities but
which are not part of the work.  For example, Corresponding Source
includes interface definition files associated with source files for
the work, and the source code for shared libraries and dynamically
linked subprograms that the work is specifically designed to require,
such as by intimate data communication or control flow between those
subprograms and other parts of the work.

  The Corresponding Source need not include anything that users
can regenerate automatically from other parts of the Corresponding
Source.

  The Corresponding Source for a work in source code form is that
same work.

  2. Basic Permissions.

  All rights granted under this License are granted for the term of
copyright on the Program, and are irrevocable provided the stated
conditions are met.  This License explicitly affirms your unlimited
permission to run the unmodified Program.  The output from running a
covered work is covered by this License only if the output, given its
content, constitutes a covered work.  This License acknowledges your
rights of fair use or other equivalent, as provided by copyright law.

  You may make, run and propagate covered works that you do not
convey, without conditions so long as your license otherwise remains
in force.  You may convey covered works to others for the sole purpose
of having them make modifications exclusively for you, or provide you
with facilities for running those works, provided that you comply with
the terms of this License in conveying all material for which you do
not control copyright.  Those thus making or running the covered works
for you must do so exclusively on your behalf, under your direction
and control, on terms that prohibit them from making any copies of
your copyrighted material outside their relationship with you.

  Conveying under any other circumstances is permitted solely under
the conditions stated below.  Sublicensing is not allowed; section 10
makes it unnecessary.

  3. Protecting Users' Legal Rights From Anti-Circumvention Law.

  No covered work shall be deemed part of an effective technological
measure under any applicable law fulfilling obligations under article
11 of the WIPO copyright treaty adopted on 20 December 1996, or
similar laws prohibiting or restricting circumvention of such
measures.

  When you convey a covered work, you waive any legal power to forbid
circumvention of technological measures to the extent such circumvention
is effected by exercising rights under this License with respect to
the covered work, and you disclaim any intention to limit operation or
modification of the work as a means of enforcing, against the work's
users, your or third parties' legal rights to forbid circumvention of
technological measures.

  4. Conveying Verbatim Copies.

  You may convey verbatim copies of the Program's source code as you
receive it, in any medium, provided that you conspicuously and
appropriately publish on each copy an appropriate copyright notice;
keep intact all notices stating that this License and any
non-permissive terms added in accord with section 7 apply to the code;
keep intact all notices of the absence of any warranty; and give all
recipients a copy of this License along with the Program.

  You may charge any price or no price for each copy that you convey,
and you may offer support or warranty protection for a fee.

  5. Conveying Modified Source Versions.

  You may convey a work based on the Program, or the modifications to
produce it from the Program, in the form of source code under the
terms of section 4, provided that you also meet all of these conditions:

    a) The work must carry prominent notices stating that you modified
    it, and giving a relevant date.

    b) The work must carry prominent notices stating that it is
    released under this License and any conditions added under section
    7.  This requirement modifies the requirement in section 4 to
    "keep intact all notices".

    c) You must license the entire work, as a whole, under this
    License to anyone who comes into possession of a copy.  This
    License will therefore apply, along with any applicable section 7
    additional terms, to the whole of the work, and all its parts,
    regardless of how they are packaged.  This License gives no
    permission to license the work in any other way, but it does not
    invalidate such permission if you have separately received it.

    d) If the work has interactive user interfaces, each must display
    Appropriate Legal Notices; however, if the Program has interactive
    interfaces that do not display Appropriate Legal Notices, your
    work need not make them do so.

  A compilation of a covered work with other separate and independent
works, which are not by their nature extensions of the covered work,
and which are not combined with it such as to form a larger program,
in or on a volume of a storage or distribution medium, is called an
"aggregate" if the compilation and its resulting copyright are not
used to limit the access or legal rights of the compilation's users
beyond what the individual works permit.  Inclusion of a covered work
in an aggregate does not cause this License to apply to the other
parts of the aggregate.

  6. Conveying Non-Source Forms.

  You may convey a covered work in object code form under the terms
of sections 4 and 5, provided that you also convey the
machine-readable Corresponding Source under the terms of this License,
in one of these ways:

    a) Convey the object code in, or embodied in, a physical product
    (including a physical distribution medium), accompanied by the
    Corresponding Source fixed on a durable physical medium
    customarily used for software interchange.

    b) Convey the object code in, or embodied in, a physical product
    (including a physical distribution medium), accompanied by a
    written offer, valid for at least three years and valid for as
    long as you offer spare parts or customer support for that product
    model, to give anyone who possesses the object code either (1) a
    copy of the Corresponding Source for all the software in the
    product that is covered by this License, on a durable physical
    medium customarily used for software interchange, for a price no
    more than your reasonable cost of physically performing this
    conveying of source, or (2) access to copy the
    Corresponding Source from a network server at no charge.

    c) Convey individual copies of the object code with a copy of the
    written offer to provide the Corresponding Source.  This
    alternative is allowed only occasionally and noncommercially, and
    only if you received the object code with such an offer, in accord
    with subsection 6b.

    d) Convey the object code by offering access from a designated
    place (gratis or for a charge), and offer equivalent access to the
    Corresponding Source in the same way through the same place at no
    further charge.  You need not require recipients to copy the
    Corresponding Source along with the object code.  If the place to
    copy the object code is a network server, the Corresponding Source
    may be on a different server (operated by you or a third party)
    that supports equivalent copying facilities, provided you maintain
    clear directions next to the object code saying where to find the
    Corresponding Source.  Regardless of what server hosts the
    Corresponding Source, you remain obligated to ensure that it is
    available for as long as needed to satisfy these requirements.

    e) Convey the object code using peer-to-peer transmission, provided
    you inform other peers where the object code and Corresponding
    Source of the work are being offered to the general public at no
    charge under subsection 6d.

  A separable portion of the object code, whose source code is excluded
from the Corresponding Source as a System Library, need not be
included in conveying the object code work.

  A "User Product" is either (1) a "consumer product", which means any
tangible personal property which is normally used for personal, family,
or household purposes, or (2) anything designed or sold for incorporation
into a dwelling.  In determining whether a product is a consumer product,
doubtful cases shall be resolved in favor of coverage.  For a particular
product received by a particular user, "normally used" refers to a
typical or common use of that class of product, regardless of the status
of the particular user or of the way in which the particular user
actually uses, or expects or is expected to use, the product.  A product
is a consumer product regardless of whether the product has substantial
commercial, industrial or non-consumer uses, unless such uses represent
the only significant mode of use of the product.

  "Installation Information" for a User Product means any methods,
procedures, authorization keys, or other information required to install
and execute modified versions of a covered work in that User Product from
a modified version of its Corresponding Source.  The information must
suffice to ensure that the continued functioning of the modified object
code is in no case prevented or interfered with solely because
modification has been made.

  If you convey an object code work under this section in, or with, or
specifically for use in, a User Product, and the conveying occurs as
part of a transaction in which the right of possession and use of the
User Product is transferred to the recipient in perpetuity or for a
fixed term (regardless of how the transaction is characterized), the
Corresponding Source conveyed under this section must be accompanied
by the Installation Information.  But this requirement does not apply
if neither you nor any third party retains the ability to install
modified object code on the User Product (for example, the work has
been installed in ROM).

  The requirement to provide Installation Information does not include a
requirement to continue to provide support service, warranty, or updates
for a work that has been modified or installed by the recipient, or for
the User Product in which it has been modified or installed.  Access to a
network may be denied when the modification itself materially and
adversely affects the operation of the network or violates the rules and
protocols for communication across the network.

  Corresponding Source conveyed, and Installation Information provided,
in accord with this section must be in a format that is publicly
documented (and with an implementation available to the public in
source code form), and must require no special password or key for
unpacking, reading or copying.

  7. Additional Terms.

  "Additional permissions" are terms that supplement the terms of this
License by making exceptions from one or more of its conditions.
Additional permissions that are applicable to the entire Program shall
be treated as though they were included in this License, to the extent
that they are valid under applicable law.  If additional permissions
apply only to part of the Program, that part may be used separately
under those permissions, but the entire Program remains governed by
this License without regard to the additional permissions.

  When you convey a copy of a covered work, you may at your option
remove any additional permissions from that copy, or from any part of
it.  (Additional permissions may be written to require their own
removal in certain cases when you modify the work.)  You may place
additional permissions on material, added by you to a covered work,
for which you have or can give appropriate copyright permission.

  Notwithstanding any other provision of this License, for material you
add to a covered work, you may (if authorized by the copyright holders of
that material) supplement the terms of this License with terms:

    a) Disclaiming warranty or limiting liability differently from the
    terms of sections 15 and 16 of this License; or

    b) Requiring preservation of specified reasonable legal notices or
    author attributions in that material or in the Appropriate Legal
    Notices displayed by works containing it; or

    c) Prohibiting misrepresentation of the origin of that material, or
    requiring that modified versions of such material be marked in
    reasonable ways as different from the original version; or

    d) Limiting the use for publicity purposes of names of licensors or
    authors of the material; or

    e) Declining to grant rights under trademark law for use of some
    trade names, trademarks, or service marks; or

    f) Requiring indemnification of licensors and authors of that
    material by anyone who conveys the material (or modified versions of
    it) with contractual assumptions of liability to the recipient, for
    any liability that these contractual assumptions directly impose on
    those licensors and authors.

  All other non-permissive additional terms are considered "further
restrictions" within the meaning of section 10.  If the Program as you
received it, or any part of it, contains a notice stating that it is
governed by this License along with a term that is a further
restriction, you may remove that term.  If a license document contains
a further restriction but permits relicensing or conveying under this
License, you may add to a covered work material governed by the terms
of that license document, provided that the further restriction does
not survive such relicensing or conveying.

  If you add terms to a covered work in accord with this section, you
must place, in the relevant source files, a statement of the
additional terms that apply to those files, or a notice indicating
where to find the applicable terms.

  Additional terms, permissive or non-permissive, may be stated in the
form of a separately written license, or stated as exceptions;
the above requirements apply either way.

  8. Termination.

  You may not propagate or modify a covered work except as expressly
provided under this License.  Any attempt otherwise to propagate or
modify it is void, and will automatically terminate your rights under
this License (including any patent licenses granted under the third
paragraph of section 11).

  However, if you cease all violation of this License, then your
license from a particular copyright holder is reinstated (a)
provisionally, unless and until the copyright holder explicitly and
finally terminates your license, and (b) permanently, if the copyright
holder fails to notify you of the violation by some reasonable means
prior to 60 days after the cessation.

  Moreover, your license from a particular copyright holder is
reinstated permanently if the copyright holder notifies you of the
violation by some reasonable means, this is the first time you have
received notice of violation of this License (for any work) from that
copyright holder, and you cure the violation prior to 30 days after
your receipt of the notice.

  Termination of your rights under this section does not terminate the
licenses of parties who have received copies or rights from you under
this License.  If your rights have been terminated and not permanently
reinstated, you do not qualify to receive new licenses for the same
material under section 10.

  9. Acceptance Not Required for Having Copies.

  You are not required to accept this License in order to receive or
run a copy of the Program.  Ancillary propagation of a covered work
occurring solely as a consequence of using peer-to-peer transmission
to receive a copy likewise does not require acceptance.  However,
nothing other than this License grants you permission to propagate or
modify any covered work.  These actions infringe copyright if you do
not accept this License.  Therefore, by modifying or propagating a
covered work, you indicate your acceptance of this License to do so.

  10. Automatic Licensing of Downstream Recipients.

  Each time you convey a covered work, the recipient automatically
receives a license from the original licensors, to run, modify and
propagate that work, subject to this License.  You are not responsible
for enforcing compliance by third parties with this License.

  An "entity transaction" is a transaction transferring control of an
organization, or substantially all assets of one, or subdividing an
organization, or merging organizations.  If propagation of a covered
work results from an entity transaction, each party to that
transaction who receives a copy of the work also receives whatever
licenses to the work the party's predecessor in interest had or could
give under the previous paragraph, plus a right to possession of the
Corresponding Source of the work from the predecessor in interest, if
the predecessor has it or can get it with reasonable efforts.

  You may not impose any further restrictions on the exercise of the
rights granted or affirmed under this License.  For example, you may
not impose a license fee, royalty, or other charge for exercise of
rights granted under this License, and you may not initiate litigation
(including a cross-claim or counterclaim in a lawsuit) alleging that
any patent claim is infringed by making, using, selling, offering for
sale, or importing the Program or any portion of it.

  11. Patents.

  A "contributor" is a copyright holder who authorizes use under this
License of the Program or a work on which the Program is based.  The
work thus licensed is called the contributor's "contributor version".

  A contributor's "essential patent claims" are all patent claims
owned or controlled by the contributor, whether already acquired or
hereafter acquired, that would be infringed by some manner, permitted
by this License, of making, using, or selling its contributor version,
but do not include claims that would be infringed only as a
consequence of further modification of the contributor version.  For
purposes of this definition, "control" includes the right to grant
patent sublicenses in a manner consistent with the requirements of
this License.

  Each contributor grants you a non-exclusive, worldwide, royalty-free
patent license under the contributor's essential patent claims, to
make, use, sell, offer for sale, import and otherwise run, modify and
propagate the contents of its contributor version.

  In the following three paragraphs, a "patent license" is any express
agreement or commitment, however denominated, not to enforce a patent
(such as an express permission to practice a patent or covenant not to
sue for patent infringement).  To "grant" such a patent license to a
party means to make such an agreement or commitment not to enforce a
patent against the party.

  If you convey a covered work, knowingly relying on a patent license,
and the Corresponding Source of the work is not available for anyone
to copy, free of charge and under the terms of this License, through a
publicly available network server or other readily accessible means,
then you must either (1) cause the Corresponding Source to be so
available, or (2) arrange to deprive yourself of the benefit of the
patent license for this particular work, or (3) arrange, in a manner
consistent with the requirements of this License, to extend the patent
license to downstream recipients.  "Knowingly relying" means you have
actual knowledge that, but for the patent license, your conveying the
covered work in a country, or your recipient's use of the covered work
in a country, would infringe one or more identifiable patents in that
country that you have reason to believe are valid.

  If, pursuant to or in connection with a single transaction or
arrangement, you convey, or propagate by procuring conveyance of, a
covered work, and grant a patent license to some of the parties
receiving the covered work authorizing them to use, propagate, modify
or convey a specific copy of the covered work, then the patent license
you grant is automatically extended to all recipients of the covered
work and works based on it.

  A patent license is "discriminatory" if it does not include within
the scope of its coverage, prohibits the exercise of, or is
conditioned on the non-exercise of one or more of the rights that are
specifically granted under this License.  You may not convey a covered
work if you are a party to an arrangement with a third party that is
in the business of distributing software, under which you make payment
to the third party based on the extent of your activity of conveying
the work, and under which the third party grants, to any of the
parties who would receive the covered work from you, a discriminatory
patent license (a) in connection with copies of the covered work
conveyed by you (or copies made from those copies), or (b) primarily
for and in connection with specific products or compilations that
contain the covered work, unless you entered into that arrangement,
or that patent license was granted, prior to 28 March 2007.

  Nothing in this License shall be construed as excluding or limiting
any implied license or other defenses to infringement that may
otherwise be available to you under applicable patent law.

  12. No Surrender of Others' Freedom.

  If conditions are imposed on you (whether by court order, agreement or
otherwise) that contradict the conditions of this License, they do not
excuse you from the conditions of this License.  If you cannot convey a
covered work so as to satisfy simultaneously your obligations under this
License and any other pertinent obligations, then as a consequence you may
not convey it at all.  For example, if you agree to terms that obligate you
to collect a royalty for further conveying from those to whom you convey
the Program, the only way you could satisfy both those terms and this
License would be to refrain entirely from conveying the Program.

  13. Remote Network Interaction; Use with the GNU General Public License.

  Notwithstanding any other provision of this License, if you modify the
Program, your modified version must prominently offer all users
interacting with it remotely through a computer network (if your version
supports such interaction) an opportunity to receive the Corresponding
Source of your version by providing access to the Corresponding Source
from a network server at no charge, through some standard or customary
means of facilitating copying of software.  This Corresponding Source
shall include the Corresponding Source for any work covered by version 3
of the GNU General Public License that is incorporated pursuant to the
following paragraph.

  Notwithstanding any other provision of this License, you have
permission to link or combine any covered work with a work licensed
under version 3 of the GNU General Public License into a single
combined work, and to convey the resulting work.  The terms of this
License will continue to apply to the part which is the covered work,
but the work with which it is combined will remain governed by version
3 of the GNU General Public License.

  14. Revised Versions of this License.

  The Free Software Foundation may publish revised and/or new versions of
the GNU Affero General Public License from time to time.  Such new versions
will be similar in spirit to the present version, but may differ in detail to
address new problems or concerns.

  Each version is given a distinguishing version number.  If the
Program specifies that a certain numbered version of the GNU Affero General
Public License "or any later version" applies to it, you have the
option of following the terms and conditions either of that numbered
version or of any later version published by the Free Software
Foundation.  If the Program does not specify a version number of the
GNU Affero General Public License, you may choose any version ever published
by the Free Software Foundation.

  If the Program specifies that a proxy can decide which future
versions of the GNU Affero General Public License can be used, that proxy's
public statement of acceptance of a version permanently authorizes you
to choose that version for the Program.

  Later license versions may give you additional or different
permissions.  However, no additional obligations are imposed on any
author or copyright holder as a result of your choosing to follow a
later version.

  15. Disclaimer of Warranty.

  THERE IS NO WARRANTY FOR THE PROGRAM, TO THE EXTENT PERMITTED BY
APPLICABLE LAW.  EXCEPT WHEN OTHERWISE STATED IN WRITING THE COPYRIGHT
HOLDERS AND/OR OTHER PARTIES PROVIDE THE PROGRAM "AS IS" WITHOUT WARRANTY
OF ANY KIND, EITHER EXPRESSED OR IMPLIED, INCLUDING, BUT NOT LIMITED TO,
THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
PURPOSE.  THE ENTIRE RISK AS TO THE QUALITY AND PERFORMANCE OF THE PROGRAM
IS WITH YOU.  SHOULD THE PROGRAM PROVE DEFECTIVE, YOU ASSUME THE COST OF
ALL NECESSARY SERVICING, REPAIR OR CORRECTION.

  16. Limitation of Liability.

  IN NO EVENT UNLESS REQUIRED BY APPLICABLE LAW OR AGREED TO IN WRITING
WILL ANY COPYRIGHT HOLDER, OR ANY OTHER PARTY WHO MODIFIES AND/OR CONVEYS
THE PROGRAM AS PERMITTED ABOVE, BE LIABLE TO YOU FOR DAMAGES, INCLUDING ANY
GENERAL, SPECIAL, INCIDENTAL OR CONSEQUENTIAL DAMAGES ARISING OUT OF THE
USE OR INABILITY TO USE THE PROGRAM (INCLUDING BUT NOT LIMITED TO LOSS OF
DATA OR DATA BEING RENDERED INACCURATE OR LOSSES SUSTAINED BY YOU OR THIRD
PARTIES OR A FAILURE OF THE PROGRAM TO OPERATE WITH ANY OTHER PROGRAMS),
EVEN IF SUCH HOLDER OR OTHER PARTY HAS BEEN ADVISED OF THE POSSIBILITY OF
SUCH DAMAGES.

  17. Interpretation of Sections 15 and 16.

  If the disclaimer of warranty and limitation of liability provided
above cannot be given local legal effect according to their terms,
reviewing courts shall apply local law that most closely approximates
an absolute waiver of all civil liability in connection with the
Program, unless a warranty or assumption of liability accompanies a
copy of the Program in return for a fee.

                     END OF TERMS AND CONDITIONS

            How to Apply These Terms to Your New Programs

  If you develop a new program, and you want it to be of the greatest
possible use to the public, the best way to achieve this is to make it
free software which everyone can redistribute and change under these terms.

  To do so, attach the following notices to the program.  It is safest
to attach them to the start of each source file to most effectively
state the exclusion of warranty; and each file should have at least
the "copyright" line and a pointer to where the full notice is found.

    <one line to give the program's name and a brief idea of what it does.>

    Copyright (C) {{ year }}  {{ organization }}

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU Affero General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU Affero General Public License for more details.

    You should have received a copy of the GNU Affero General Public License
    along with this program.  If not, see <http://www.gnu.org/licenses/>.

Also add information on how to contact you by electronic and paper mail.

  If your software can interact with users remotely through a computer
network, you should also make sure that it provides a way for users to
get its source.  For example, if your program is a web application, its
interface could display a "Source" link that leads users to an archive
of the code.  There are many ways you could offer source, and different
solutions will be better for different programs; see section 13 for the
specific requirements.

  You should also get your employer (if you work as a programmer) or school,
if any, to sign a "copyright disclaimer" for the program, if necessary.
For more information on this, and how to apply and follow the GNU AGPL, see
<http://www.gnu.org/licenses/>.
CASPIAN_LICENSE_EOF
}

notice_project() {
  cat <<'CASPIAN_LICENSE_EOF'
Caspian-BYOC
Copyright (C) 2026 Iman Samizadeh

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU Affero General Public License as published by the Free
Software Foundation, either version 3 of the License, or (at your option) any
later version. The full text is in LICENSE.

ADDITIONAL TERMS under section 7 of the AGPL
--------------------------------------------

These are the only additional terms, all three are of a kind section 7 permits,
and nothing here restricts what you may do with the software.

  (a) Under section 7(b), you must preserve the copyright notice above, this
      NOTICE file, and the attribution it carries, in all copies and in all
      modified versions you convey. Where the software presents a user
      interface, that interface must keep a visible reference to the Caspian
      project.

  (b) Under section 7(c), if you modify this software you must mark your
      version as changed, and you must not present it in a way that suggests
      the original authors endorse or produced your version.

  (c) Under section 7(d), you may not use the names of the licensors or the
      authors of this material for publicity purposes without their prior
      written permission. This includes, and is not limited to, using those
      names or the project name to solicit donations, sponsorship, grants or
      any other funding, and using them in a way that implies affiliation with
      or endorsement by the original project.

      This term restricts the use of NAMES. It does not restrict what you may
      do with the software. You remain free to run it, study it, modify it,
      and redistribute it under the AGPL, for any purpose including a
      commercial one. What you may not do is raise money in the authors' name.

      Section 7 permits this term and no more than this. If you receive the
      work with this term attached and you would rather not carry it, section 7
      lets you remove it from your copy; what you cannot do is remove the
      obligations it protects, because the project name and logo are not
      licensed to you by this licence at all. See TRADEMARK.md.

Why the AGPL rather than the GPL: this program is normally operated as a
service that other people connect to. Under the plain GPL, someone could modify
it, run it for others, and never publish the changes. Section 13 of the AGPL
closes that.

Why not a permissive licence: the binary statically links code under the GNU GPL
version 3 or later, so the combined work must be licensed on GPL-family terms.
MIT and Apache-2.0 are not available for the work as a whole. The GPL-3.0-or-later
modules are github.com/sagernet/sing and github.com/sagernet/sing-shadowsocks,
both reached through xray-core's Shadowsocks 2022 support. GPL-3.0-or-later is
one-way compatible with the AGPL, which is why AGPL-3.0-or-later is available
and a permissive licence is not.

THIRD-PARTY CODE
----------------

third_party/libxray-share/
    Share-link parser, taken from XTLS/libXray at tag v26.3.27, package
    "share". Licensed MIT, Copyright (c) 2023-2025 XTLS. The MIT text is kept
    beside the source in third_party/libxray-share/LICENSE. MIT is compatible
    with the AGPL, so these files keep their own notice while the combined work
    is licensed as above.

    Vendored rather than imported because the upstream module declares a path
    that Go cannot resolve at its major version. Pinned deliberately: upstream
    has already replaced its exported API wholesale and does not promise
    stability.

EVERY MODULE LINKED INTO THE BINARY

Each licence below was read from that module's own licence file rather than
recalled. Where a module ships more than one licence, the additional ones are
noted in brackets.

  COPYLEFT, and therefore the terms that decide the licence of the whole:

    github.com/sagernet/sing                    GPL-3.0-or-later
    github.com/sagernet/sing-shadowsocks        GPL-3.0-or-later
        Shadowsocks 2022, reached through xray-core.
    github.com/juju/ratelimit                   LGPL-3.0 WITH linking exception
        Token-bucket rate limiting, reached through github.com/xtls/reality
        rather than through xray-core. The linking exception waives the
        relinking obligation that would otherwise be awkward for a statically
        linked Go binary.
    github.com/xtls/xray-core                   MPL-2.0
        The proxy engine, linked in-process. MPL-2.0 is file-level copyleft and
        is GPL and AGPL compatible by its own section 3.3.
    github.com/xtls/reality                     MPL-2.0 (LICENSE-Go: BSD-3-Clause)
        The REALITY transport.

  PERMISSIVE:

    github.com/andybalholm/brotli               MIT
    github.com/apernet/quic-go                  MIT
    github.com/cloudflare/circl                 BSD-3-Clause
    github.com/ghodss/yaml                      MIT (portions BSD-3-Clause)
    github.com/google/btree                     Apache-2.0
    github.com/gorilla/websocket                BSD-2-Clause
    github.com/klauspost/compress               BSD-3-Clause (portions Apache-2.0, MIT)
    github.com/klauspost/cpuid/v2               MIT
    github.com/miekg/dns                        BSD-3-Clause
    github.com/pelletier/go-toml                MIT (two files Apache-2.0)
    github.com/pires/go-proxyproto              Apache-2.0
    github.com/quic-go/qpack                    MIT
    github.com/refraction-networking/utls       BSD-3-Clause
    github.com/vishvananda/netlink              Apache-2.0
    github.com/vishvananda/netns                Apache-2.0
    go4.org/netipx                              BSD-3-Clause
    golang.org/x/crypto                         BSD-3-Clause
    golang.org/x/exp                            BSD-3-Clause
    golang.org/x/net                            BSD-3-Clause
    golang.org/x/sync                           BSD-3-Clause
    golang.org/x/sys                            BSD-3-Clause
    golang.org/x/text                           BSD-3-Clause
    golang.org/x/time                           BSD-3-Clause
    golang.zx2c4.com/wintun                     MIT (Windows runtime binding)
    golang.zx2c4.com/wireguard                  MIT
    google.golang.org/genproto/googleapis/rpc   Apache-2.0
    google.golang.org/grpc                      Apache-2.0
    google.golang.org/protobuf                  BSD-3-Clause
    gopkg.in/yaml.v2                            Apache-2.0 (libyaml files MIT)
    gopkg.in/yaml.v3                            MIT AND Apache-2.0
    gvisor.dev/gvisor                           Apache-2.0 (some files MIT, BSD-3-Clause)
    lukechampine.com/blake3                     MIT

  Apache-2.0 is compatible with AGPL-3.0 and GPL-3.0 but not with GPL-2.0.
  Nothing here is GPL-2.0-only, so that incompatibility is inert. It would
  become live if a GPL-2.0-only dependency were ever added.

TEST AND BUILD TOOLING, NOT DISTRIBUTED

    github.com/stretchr/testify                 MIT
    github.com/davecgh/go-spew                  ISC
    github.com/pmezard/go-difflib               BSD-2-Clause
    golang.org/x/mod, golang.org/x/tools        BSD-3-Clause
    @cucumber/cucumber                          MIT
    selenium-webdriver                          Apache-2.0

    These never form part of the conveyed work, so their terms do not enter the
    combined licence.

PROGRAMS THE APPLIANCE RUNS BUT DOES NOT LINK

    hostapd, dnsmasq, nftables, iw, iproute2, procps-ng, NetworkManager,
    util-linux.

    These are separate programs invoked at arm's length, not libraries linked
    into this work, so their licences do not affect the licensing of Caspian.
    They are named because the appliance does not function without them.

WINDOWS DISTRIBUTION FILES
--------------------------

    wintun.dll                              Wintun Prebuilt Binaries License
        Caspian distributes the official signed Wintun 0.14.1 DLL without
        modification. The DLL is separate from caspian.exe and is loaded
        through the published Wintun API. The complete license is kept at
        third_party/wintun/PREBUILT-BINARIES-LICENSE.txt and is installed as
        WINTUN-LICENSE.txt beside the DLL.

    .NET runtime and Windows Forms          MIT
        caspian-tethering.exe and CaspianControl.exe are self-contained .NET
        applications. The runtime libraries form part of those single-file
        executables. The .NET license and third-party notices are kept under
        third_party/dotnet/.

    System.ServiceProcess.ServiceController 9.0.0  MIT
        This package lets CaspianControl.exe start and stop the two Windows
        services. Its license is kept at
        third_party/dotnet/System.ServiceProcess.ServiceController-LICENSE.txt.

    Microsoft.Windows.SDK.NET.Ref           build input only
        The Mobile Hotspot helper uses this reference package at build time.
        Caspian does not distribute it as a separate DLL.

SNI SPOOFING ATTRIBUTION
------------------------

    Primary code and idea: patterniha/SNI-Spoofing contributors
    https://github.com/patterniha/SNI-Spoofing
    Reviewed source: 13b78cf7e073f38d9cadcff542faf4a00b0a6de2
    License: GPL-3.0; full text in third_party/sni-spoofing/LICENSE.txt.

    Caspian translates the ClientHello template and adapts the handshake
    algorithm in internal/snispoof. Modifications by Iman Samizadeh (2026)
    add name validation, complete connection ownership, bounds checking,
    resource limits, service rollback, platform backends, and tests.
    The derived files retain GPL-3.0-only notices. GPLv3 section 13 permits
    combination with AGPLv3 code; each part retains its own license and the
    AGPL network-interaction requirements apply to the combination.
    Caspian's additional terms do not relicense the upstream material.
    This attribution does not claim upstream endorsement.

    WinDivert 2.2.2-A by Basil (basil00) and contributors
    https://github.com/basil00/WinDivert/tree/v2.2.2
    Windows x64 optional runtime: WinDivert.dll and WinDivert64.sys,
    unmodified, dynamically loaded. Caspian selects LGPL-3.0 from the
    upstream dual license. The complete license bundle and source reference
    are in third_party/windivert/ and installed beside the binaries.

    Research references, no copied code or bundled executable:
      selfishblackberry177/sni-spoof (no license found at reviewed commit)
      ValdikSS/GoodbyeDPI (Apache-2.0)
      bol-van/zapret (MIT)
      Floxu1/UAC-SNI-Spoofer-Android (no top-level app license found)
      therealaleph/sni-spoofing-rust (declares MIT; derived-code terms unverified)
    See docs/THIRD-PARTY.md and docs/THIRD-PARTY.md for scope and versions.
CASPIAN_LICENSE_EOF
}

license_sni() {
  cat <<'CASPIAN_LICENSE_EOF'
                    GNU GENERAL PUBLIC LICENSE
                       Version 3, 29 June 2007

 Copyright (C) 2007 Free Software Foundation, Inc. <https://fsf.org/>
 Everyone is permitted to copy and distribute verbatim copies
 of this license document, but changing it is not allowed.

                            Preamble

  The GNU General Public License is a free, copyleft license for
software and other kinds of works.

  The licenses for most software and other practical works are designed
to take away your freedom to share and change the works.  By contrast,
the GNU General Public License is intended to guarantee your freedom to
share and change all versions of a program--to make sure it remains free
software for all its users.  We, the Free Software Foundation, use the
GNU General Public License for most of our software; it applies also to
any other work released this way by its authors.  You can apply it to
your programs, too.

  When we speak of free software, we are referring to freedom, not
price.  Our General Public Licenses are designed to make sure that you
have the freedom to distribute copies of free software (and charge for
them if you wish), that you receive source code or can get it if you
want it, that you can change the software or use pieces of it in new
free programs, and that you know you can do these things.

  To protect your rights, we need to prevent others from denying you
these rights or asking you to surrender the rights.  Therefore, you have
certain responsibilities if you distribute copies of the software, or if
you modify it: responsibilities to respect the freedom of others.

  For example, if you distribute copies of such a program, whether
gratis or for a fee, you must pass on to the recipients the same
freedoms that you received.  You must make sure that they, too, receive
or can get the source code.  And you must show them these terms so they
know their rights.

  Developers that use the GNU GPL protect your rights with two steps:
(1) assert copyright on the software, and (2) offer you this License
giving you legal permission to copy, distribute and/or modify it.

  For the developers' and authors' protection, the GPL clearly explains
that there is no warranty for this free software.  For both users' and
authors' sake, the GPL requires that modified versions be marked as
changed, so that their problems will not be attributed erroneously to
authors of previous versions.

  Some devices are designed to deny users access to install or run
modified versions of the software inside them, although the manufacturer
can do so.  This is fundamentally incompatible with the aim of
protecting users' freedom to change the software.  The systematic
pattern of such abuse occurs in the area of products for individuals to
use, which is precisely where it is most unacceptable.  Therefore, we
have designed this version of the GPL to prohibit the practice for those
products.  If such problems arise substantially in other domains, we
stand ready to extend this provision to those domains in future versions
of the GPL, as needed to protect the freedom of users.

  Finally, every program is threatened constantly by software patents.
States should not allow patents to restrict development and use of
software on general-purpose computers, but in those that do, we wish to
avoid the special danger that patents applied to a free program could
make it effectively proprietary.  To prevent this, the GPL assures that
patents cannot be used to render the program non-free.

  The precise terms and conditions for copying, distribution and
modification follow.

                       TERMS AND CONDITIONS

  0. Definitions.

  "This License" refers to version 3 of the GNU General Public License.

  "Copyright" also means copyright-like laws that apply to other kinds of
works, such as semiconductor masks.

  "The Program" refers to any copyrightable work licensed under this
License.  Each licensee is addressed as "you".  "Licensees" and
"recipients" may be individuals or organizations.

  To "modify" a work means to copy from or adapt all or part of the work
in a fashion requiring copyright permission, other than the making of an
exact copy.  The resulting work is called a "modified version" of the
earlier work or a work "based on" the earlier work.

  A "covered work" means either the unmodified Program or a work based
on the Program.

  To "propagate" a work means to do anything with it that, without
permission, would make you directly or secondarily liable for
infringement under applicable copyright law, except executing it on a
computer or modifying a private copy.  Propagation includes copying,
distribution (with or without modification), making available to the
public, and in some countries other activities as well.

  To "convey" a work means any kind of propagation that enables other
parties to make or receive copies.  Mere interaction with a user through
a computer network, with no transfer of a copy, is not conveying.

  An interactive user interface displays "Appropriate Legal Notices"
to the extent that it includes a convenient and prominently visible
feature that (1) displays an appropriate copyright notice, and (2)
tells the user that there is no warranty for the work (except to the
extent that warranties are provided), that licensees may convey the
work under this License, and how to view a copy of this License.  If
the interface presents a list of user commands or options, such as a
menu, a prominent item in the list meets this criterion.

  1. Source Code.

  The "source code" for a work means the preferred form of the work
for making modifications to it.  "Object code" means any non-source
form of a work.

  A "Standard Interface" means an interface that either is an official
standard defined by a recognized standards body, or, in the case of
interfaces specified for a particular programming language, one that
is widely used among developers working in that language.

  The "System Libraries" of an executable work include anything, other
than the work as a whole, that (a) is included in the normal form of
packaging a Major Component, but which is not part of that Major
Component, and (b) serves only to enable use of the work with that
Major Component, or to implement a Standard Interface for which an
implementation is available to the public in source code form.  A
"Major Component", in this context, means a major essential component
(kernel, window system, and so on) of the specific operating system
(if any) on which the executable work runs, or a compiler used to
produce the work, or an object code interpreter used to run it.

  The "Corresponding Source" for a work in object code form means all
the source code needed to generate, install, and (for an executable
work) run the object code and to modify the work, including scripts to
control those activities.  However, it does not include the work's
System Libraries, or general-purpose tools or generally available free
programs which are used unmodified in performing those activities but
which are not part of the work.  For example, Corresponding Source
includes interface definition files associated with source files for
the work, and the source code for shared libraries and dynamically
linked subprograms that the work is specifically designed to require,
such as by intimate data communication or control flow between those
subprograms and other parts of the work.

  The Corresponding Source need not include anything that users
can regenerate automatically from other parts of the Corresponding
Source.

  The Corresponding Source for a work in source code form is that
same work.

  2. Basic Permissions.

  All rights granted under this License are granted for the term of
copyright on the Program, and are irrevocable provided the stated
conditions are met.  This License explicitly affirms your unlimited
permission to run the unmodified Program.  The output from running a
covered work is covered by this License only if the output, given its
content, constitutes a covered work.  This License acknowledges your
rights of fair use or other equivalent, as provided by copyright law.

  You may make, run and propagate covered works that you do not
convey, without conditions so long as your license otherwise remains
in force.  You may convey covered works to others for the sole purpose
of having them make modifications exclusively for you, or provide you
with facilities for running those works, provided that you comply with
the terms of this License in conveying all material for which you do
not control copyright.  Those thus making or running the covered works
for you must do so exclusively on your behalf, under your direction
and control, on terms that prohibit them from making any copies of
your copyrighted material outside their relationship with you.

  Conveying under any other circumstances is permitted solely under
the conditions stated below.  Sublicensing is not allowed; section 10
makes it unnecessary.

  3. Protecting Users' Legal Rights From Anti-Circumvention Law.

  No covered work shall be deemed part of an effective technological
measure under any applicable law fulfilling obligations under article
11 of the WIPO copyright treaty adopted on 20 December 1996, or
similar laws prohibiting or restricting circumvention of such
measures.

  When you convey a covered work, you waive any legal power to forbid
circumvention of technological measures to the extent such circumvention
is effected by exercising rights under this License with respect to
the covered work, and you disclaim any intention to limit operation or
modification of the work as a means of enforcing, against the work's
users, your or third parties' legal rights to forbid circumvention of
technological measures.

  4. Conveying Verbatim Copies.

  You may convey verbatim copies of the Program's source code as you
receive it, in any medium, provided that you conspicuously and
appropriately publish on each copy an appropriate copyright notice;
keep intact all notices stating that this License and any
non-permissive terms added in accord with section 7 apply to the code;
keep intact all notices of the absence of any warranty; and give all
recipients a copy of this License along with the Program.

  You may charge any price or no price for each copy that you convey,
and you may offer support or warranty protection for a fee.

  5. Conveying Modified Source Versions.

  You may convey a work based on the Program, or the modifications to
produce it from the Program, in the form of source code under the
terms of section 4, provided that you also meet all of these conditions:

    a) The work must carry prominent notices stating that you modified
    it, and giving a relevant date.

    b) The work must carry prominent notices stating that it is
    released under this License and any conditions added under section
    7.  This requirement modifies the requirement in section 4 to
    "keep intact all notices".

    c) You must license the entire work, as a whole, under this
    License to anyone who comes into possession of a copy.  This
    License will therefore apply, along with any applicable section 7
    additional terms, to the whole of the work, and all its parts,
    regardless of how they are packaged.  This License gives no
    permission to license the work in any other way, but it does not
    invalidate such permission if you have separately received it.

    d) If the work has interactive user interfaces, each must display
    Appropriate Legal Notices; however, if the Program has interactive
    interfaces that do not display Appropriate Legal Notices, your
    work need not make them do so.

  A compilation of a covered work with other separate and independent
works, which are not by their nature extensions of the covered work,
and which are not combined with it such as to form a larger program,
in or on a volume of a storage or distribution medium, is called an
"aggregate" if the compilation and its resulting copyright are not
used to limit the access or legal rights of the compilation's users
beyond what the individual works permit.  Inclusion of a covered work
in an aggregate does not cause this License to apply to the other
parts of the aggregate.

  6. Conveying Non-Source Forms.

  You may convey a covered work in object code form under the terms
of sections 4 and 5, provided that you also convey the
machine-readable Corresponding Source under the terms of this License,
in one of these ways:

    a) Convey the object code in, or embodied in, a physical product
    (including a physical distribution medium), accompanied by the
    Corresponding Source fixed on a durable physical medium
    customarily used for software interchange.

    b) Convey the object code in, or embodied in, a physical product
    (including a physical distribution medium), accompanied by a
    written offer, valid for at least three years and valid for as
    long as you offer spare parts or customer support for that product
    model, to give anyone who possesses the object code either (1) a
    copy of the Corresponding Source for all the software in the
    product that is covered by this License, on a durable physical
    medium customarily used for software interchange, for a price no
    more than your reasonable cost of physically performing this
    conveying of source, or (2) access to copy the
    Corresponding Source from a network server at no charge.

    c) Convey individual copies of the object code with a copy of the
    written offer to provide the Corresponding Source.  This
    alternative is allowed only occasionally and noncommercially, and
    only if you received the object code with such an offer, in accord
    with subsection 6b.

    d) Convey the object code by offering access from a designated
    place (gratis or for a charge), and offer equivalent access to the
    Corresponding Source in the same way through the same place at no
    further charge.  You need not require recipients to copy the
    Corresponding Source along with the object code.  If the place to
    copy the object code is a network server, the Corresponding Source
    may be on a different server (operated by you or a third party)
    that supports equivalent copying facilities, provided you maintain
    clear directions next to the object code saying where to find the
    Corresponding Source.  Regardless of what server hosts the
    Corresponding Source, you remain obligated to ensure that it is
    available for as long as needed to satisfy these requirements.

    e) Convey the object code using peer-to-peer transmission, provided
    you inform other peers where the object code and Corresponding
    Source of the work are being offered to the general public at no
    charge under subsection 6d.

  A separable portion of the object code, whose source code is excluded
from the Corresponding Source as a System Library, need not be
included in conveying the object code work.

  A "User Product" is either (1) a "consumer product", which means any
tangible personal property which is normally used for personal, family,
or household purposes, or (2) anything designed or sold for incorporation
into a dwelling.  In determining whether a product is a consumer product,
doubtful cases shall be resolved in favor of coverage.  For a particular
product received by a particular user, "normally used" refers to a
typical or common use of that class of product, regardless of the status
of the particular user or of the way in which the particular user
actually uses, or expects or is expected to use, the product.  A product
is a consumer product regardless of whether the product has substantial
commercial, industrial or non-consumer uses, unless such uses represent
the only significant mode of use of the product.

  "Installation Information" for a User Product means any methods,
procedures, authorization keys, or other information required to install
and execute modified versions of a covered work in that User Product from
a modified version of its Corresponding Source.  The information must
suffice to ensure that the continued functioning of the modified object
code is in no case prevented or interfered with solely because
modification has been made.

  If you convey an object code work under this section in, or with, or
specifically for use in, a User Product, and the conveying occurs as
part of a transaction in which the right of possession and use of the
User Product is transferred to the recipient in perpetuity or for a
fixed term (regardless of how the transaction is characterized), the
Corresponding Source conveyed under this section must be accompanied
by the Installation Information.  But this requirement does not apply
if neither you nor any third party retains the ability to install
modified object code on the User Product (for example, the work has
been installed in ROM).

  The requirement to provide Installation Information does not include a
requirement to continue to provide support service, warranty, or updates
for a work that has been modified or installed by the recipient, or for
the User Product in which it has been modified or installed.  Access to a
network may be denied when the modification itself materially and
adversely affects the operation of the network or violates the rules and
protocols for communication across the network.

  Corresponding Source conveyed, and Installation Information provided,
in accord with this section must be in a format that is publicly
documented (and with an implementation available to the public in
source code form), and must require no special password or key for
unpacking, reading or copying.

  7. Additional Terms.

  "Additional permissions" are terms that supplement the terms of this
License by making exceptions from one or more of its conditions.
Additional permissions that are applicable to the entire Program shall
be treated as though they were included in this License, to the extent
that they are valid under applicable law.  If additional permissions
apply only to part of the Program, that part may be used separately
under those permissions, but the entire Program remains governed by
this License without regard to the additional permissions.

  When you convey a copy of a covered work, you may at your option
remove any additional permissions from that copy, or from any part of
it.  (Additional permissions may be written to require their own
removal in certain cases when you modify the work.)  You may place
additional permissions on material, added by you to a covered work,
for which you have or can give appropriate copyright permission.

  Notwithstanding any other provision of this License, for material you
add to a covered work, you may (if authorized by the copyright holders of
that material) supplement the terms of this License with terms:

    a) Disclaiming warranty or limiting liability differently from the
    terms of sections 15 and 16 of this License; or

    b) Requiring preservation of specified reasonable legal notices or
    author attributions in that material or in the Appropriate Legal
    Notices displayed by works containing it; or

    c) Prohibiting misrepresentation of the origin of that material, or
    requiring that modified versions of such material be marked in
    reasonable ways as different from the original version; or

    d) Limiting the use for publicity purposes of names of licensors or
    authors of the material; or

    e) Declining to grant rights under trademark law for use of some
    trade names, trademarks, or service marks; or

    f) Requiring indemnification of licensors and authors of that
    material by anyone who conveys the material (or modified versions of
    it) with contractual assumptions of liability to the recipient, for
    any liability that these contractual assumptions directly impose on
    those licensors and authors.

  All other non-permissive additional terms are considered "further
restrictions" within the meaning of section 10.  If the Program as you
received it, or any part of it, contains a notice stating that it is
governed by this License along with a term that is a further
restriction, you may remove that term.  If a license document contains
a further restriction but permits relicensing or conveying under this
License, you may add to a covered work material governed by the terms
of that license document, provided that the further restriction does
not survive such relicensing or conveying.

  If you add terms to a covered work in accord with this section, you
must place, in the relevant source files, a statement of the
additional terms that apply to those files, or a notice indicating
where to find the applicable terms.

  Additional terms, permissive or non-permissive, may be stated in the
form of a separately written license, or stated as exceptions;
the above requirements apply either way.

  8. Termination.

  You may not propagate or modify a covered work except as expressly
provided under this License.  Any attempt otherwise to propagate or
modify it is void, and will automatically terminate your rights under
this License (including any patent licenses granted under the third
paragraph of section 11).

  However, if you cease all violation of this License, then your
license from a particular copyright holder is reinstated (a)
provisionally, unless and until the copyright holder explicitly and
finally terminates your license, and (b) permanently, if the copyright
holder fails to notify you of the violation by some reasonable means
prior to 60 days after the cessation.

  Moreover, your license from a particular copyright holder is
reinstated permanently if the copyright holder notifies you of the
violation by some reasonable means, this is the first time you have
received notice of violation of this License (for any work) from that
copyright holder, and you cure the violation prior to 30 days after
your receipt of the notice.

  Termination of your rights under this section does not terminate the
licenses of parties who have received copies or rights from you under
this License.  If your rights have been terminated and not permanently
reinstated, you do not qualify to receive new licenses for the same
material under section 10.

  9. Acceptance Not Required for Having Copies.

  You are not required to accept this License in order to receive or
run a copy of the Program.  Ancillary propagation of a covered work
occurring solely as a consequence of using peer-to-peer transmission
to receive a copy likewise does not require acceptance.  However,
nothing other than this License grants you permission to propagate or
modify any covered work.  These actions infringe copyright if you do
not accept this License.  Therefore, by modifying or propagating a
covered work, you indicate your acceptance of this License to do so.

  10. Automatic Licensing of Downstream Recipients.

  Each time you convey a covered work, the recipient automatically
receives a license from the original licensors, to run, modify and
propagate that work, subject to this License.  You are not responsible
for enforcing compliance by third parties with this License.

  An "entity transaction" is a transaction transferring control of an
organization, or substantially all assets of one, or subdividing an
organization, or merging organizations.  If propagation of a covered
work results from an entity transaction, each party to that
transaction who receives a copy of the work also receives whatever
licenses to the work the party's predecessor in interest had or could
give under the previous paragraph, plus a right to possession of the
Corresponding Source of the work from the predecessor in interest, if
the predecessor has it or can get it with reasonable efforts.

  You may not impose any further restrictions on the exercise of the
rights granted or affirmed under this License.  For example, you may
not impose a license fee, royalty, or other charge for exercise of
rights granted under this License, and you may not initiate litigation
(including a cross-claim or counterclaim in a lawsuit) alleging that
any patent claim is infringed by making, using, selling, offering for
sale, or importing the Program or any portion of it.

  11. Patents.

  A "contributor" is a copyright holder who authorizes use under this
License of the Program or a work on which the Program is based.  The
work thus licensed is called the contributor's "contributor version".

  A contributor's "essential patent claims" are all patent claims
owned or controlled by the contributor, whether already acquired or
hereafter acquired, that would be infringed by some manner, permitted
by this License, of making, using, or selling its contributor version,
but do not include claims that would be infringed only as a
consequence of further modification of the contributor version.  For
purposes of this definition, "control" includes the right to grant
patent sublicenses in a manner consistent with the requirements of
this License.

  Each contributor grants you a non-exclusive, worldwide, royalty-free
patent license under the contributor's essential patent claims, to
make, use, sell, offer for sale, import and otherwise run, modify and
propagate the contents of its contributor version.

  In the following three paragraphs, a "patent license" is any express
agreement or commitment, however denominated, not to enforce a patent
(such as an express permission to practice a patent or covenant not to
sue for patent infringement).  To "grant" such a patent license to a
party means to make such an agreement or commitment not to enforce a
patent against the party.

  If you convey a covered work, knowingly relying on a patent license,
and the Corresponding Source of the work is not available for anyone
to copy, free of charge and under the terms of this License, through a
publicly available network server or other readily accessible means,
then you must either (1) cause the Corresponding Source to be so
available, or (2) arrange to deprive yourself of the benefit of the
patent license for this particular work, or (3) arrange, in a manner
consistent with the requirements of this License, to extend the patent
license to downstream recipients.  "Knowingly relying" means you have
actual knowledge that, but for the patent license, your conveying the
covered work in a country, or your recipient's use of the covered work
in a country, would infringe one or more identifiable patents in that
country that you have reason to believe are valid.

  If, pursuant to or in connection with a single transaction or
arrangement, you convey, or propagate by procuring conveyance of, a
covered work, and grant a patent license to some of the parties
receiving the covered work authorizing them to use, propagate, modify
or convey a specific copy of the covered work, then the patent license
you grant is automatically extended to all recipients of the covered
work and works based on it.

  A patent license is "discriminatory" if it does not include within
the scope of its coverage, prohibits the exercise of, or is
conditioned on the non-exercise of one or more of the rights that are
specifically granted under this License.  You may not convey a covered
work if you are a party to an arrangement with a third party that is
in the business of distributing software, under which you make payment
to the third party based on the extent of your activity of conveying
the work, and under which the third party grants, to any of the
parties who would receive the covered work from you, a discriminatory
patent license (a) in connection with copies of the covered work
conveyed by you (or copies made from those copies), or (b) primarily
for and in connection with specific products or compilations that
contain the covered work, unless you entered into that arrangement,
or that patent license was granted, prior to 28 March 2007.

  Nothing in this License shall be construed as excluding or limiting
any implied license or other defenses to infringement that may
otherwise be available to you under applicable patent law.

  12. No Surrender of Others' Freedom.

  If conditions are imposed on you (whether by court order, agreement or
otherwise) that contradict the conditions of this License, they do not
excuse you from the conditions of this License.  If you cannot convey a
covered work so as to satisfy simultaneously your obligations under this
License and any other pertinent obligations, then as a consequence you may
not convey it at all.  For example, if you agree to terms that obligate you
to collect a royalty for further conveying from those to whom you convey
the Program, the only way you could satisfy both those terms and this
License would be to refrain entirely from conveying the Program.

  13. Use with the GNU Affero General Public License.

  Notwithstanding any other provision of this License, you have
permission to link or combine any covered work with a work licensed
under version 3 of the GNU Affero General Public License into a single
combined work, and to convey the resulting work.  The terms of this
License will continue to apply to the part which is the covered work,
but the special requirements of the GNU Affero General Public License,
section 13, concerning interaction through a network will apply to the
combination as such.

  14. Revised Versions of this License.

  The Free Software Foundation may publish revised and/or new versions of
the GNU General Public License from time to time.  Such new versions will
be similar in spirit to the present version, but may differ in detail to
address new problems or concerns.

  Each version is given a distinguishing version number.  If the
Program specifies that a certain numbered version of the GNU General
Public License "or any later version" applies to it, you have the
option of following the terms and conditions either of that numbered
version or of any later version published by the Free Software
Foundation.  If the Program does not specify a version number of the
GNU General Public License, you may choose any version ever published
by the Free Software Foundation.

  If the Program specifies that a proxy can decide which future
versions of the GNU General Public License can be used, that proxy's
public statement of acceptance of a version permanently authorizes you
to choose that version for the Program.

  Later license versions may give you additional or different
permissions.  However, no additional obligations are imposed on any
author or copyright holder as a result of your choosing to follow a
later version.

  15. Disclaimer of Warranty.

  THERE IS NO WARRANTY FOR THE PROGRAM, TO THE EXTENT PERMITTED BY
APPLICABLE LAW.  EXCEPT WHEN OTHERWISE STATED IN WRITING THE COPYRIGHT
HOLDERS AND/OR OTHER PARTIES PROVIDE THE PROGRAM "AS IS" WITHOUT WARRANTY
OF ANY KIND, EITHER EXPRESSED OR IMPLIED, INCLUDING, BUT NOT LIMITED TO,
THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
PURPOSE.  THE ENTIRE RISK AS TO THE QUALITY AND PERFORMANCE OF THE PROGRAM
IS WITH YOU.  SHOULD THE PROGRAM PROVE DEFECTIVE, YOU ASSUME THE COST OF
ALL NECESSARY SERVICING, REPAIR OR CORRECTION.

  16. Limitation of Liability.

  IN NO EVENT UNLESS REQUIRED BY APPLICABLE LAW OR AGREED TO IN WRITING
WILL ANY COPYRIGHT HOLDER, OR ANY OTHER PARTY WHO MODIFIES AND/OR CONVEYS
THE PROGRAM AS PERMITTED ABOVE, BE LIABLE TO YOU FOR DAMAGES, INCLUDING ANY
GENERAL, SPECIAL, INCIDENTAL OR CONSEQUENTIAL DAMAGES ARISING OUT OF THE
USE OR INABILITY TO USE THE PROGRAM (INCLUDING BUT NOT LIMITED TO LOSS OF
DATA OR DATA BEING RENDERED INACCURATE OR LOSSES SUSTAINED BY YOU OR THIRD
PARTIES OR A FAILURE OF THE PROGRAM TO OPERATE WITH ANY OTHER PROGRAMS),
EVEN IF SUCH HOLDER OR OTHER PARTY HAS BEEN ADVISED OF THE POSSIBILITY OF
SUCH DAMAGES.

  17. Interpretation of Sections 15 and 16.

  If the disclaimer of warranty and limitation of liability provided
above cannot be given local legal effect according to their terms,
reviewing courts shall apply local law that most closely approximates
an absolute waiver of all civil liability in connection with the
Program, unless a warranty or assumption of liability accompanies a
copy of the Program in return for a fee.

                     END OF TERMS AND CONDITIONS

            How to Apply These Terms to Your New Programs

  If you develop a new program, and you want it to be of the greatest
possible use to the public, the best way to achieve this is to make it
free software which everyone can redistribute and change under these terms.

  To do so, attach the following notices to the program.  It is safest
to attach them to the start of each source file to most effectively
state the exclusion of warranty; and each file should have at least
the "copyright" line and a pointer to where the full notice is found.

    <one line to give the program's name and a brief idea of what it does.>
    Copyright (C) <year>  <name of author>

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.

Also add information on how to contact you by electronic and paper mail.

  If the program does terminal interaction, make it output a short
notice like this when it starts in an interactive mode:

    <program>  Copyright (C) <year>  <name of author>
    This program comes with ABSOLUTELY NO WARRANTY; for details type `show w'.
    This is free software, and you are welcome to redistribute it
    under certain conditions; type `show c' for details.

The hypothetical commands `show w' and `show c' should show the appropriate
parts of the General Public License.  Of course, your program's commands
might be different; for a GUI interface, you would use an "about box".

  You should also get your employer (if you work as a programmer) or school,
if any, to sign a "copyright disclaimer" for the program, if necessary.
For more information on this, and how to apply and follow the GNU GPL, see
<https://www.gnu.org/licenses/>.

  The GNU General Public License does not permit incorporating your program
into proprietary programs.  If your program is a subroutine library, you
may consider it more useful to permit linking proprietary applications with
the library.  If this is what you want to do, use the GNU Lesser General
Public License instead of this License.  But first, please read
<https://www.gnu.org/licenses/why-not-lgpl.html>.
CASPIAN_LICENSE_EOF
}

notice_sni() {
  cat <<'CASPIAN_LICENSE_EOF'
# SNI spoofing reference attribution

The ClientHello template and handshake algorithm in `internal/snispoof` derive from
[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing),
commit `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`.
The original project uses GPL-3.0. Its license is in `LICENSE.txt`.
Caspian translates the packet layout into Go and adds validation, connection ownership,
resource limits, service integration, and tests.
The secondary Go repository was reviewed but its code was not copied.
CASPIAN_LICENSE_EOF
}

install_notices() {
  local doc_dir="${CASPIAN_SYSROOT}/usr/local/share/doc/caspian"
  run install -d -m 0755 -o root -g root "$doc_dir"
  license_project | write_file "$doc_dir/LICENSE" 0644 root root
  notice_project | write_file "$doc_dir/NOTICE" 0644 root root
  license_sni | write_file "$doc_dir/SNI-SPOOFING-LICENSE.txt" 0644 root root
  notice_sni | write_file "$doc_dir/SNI-SPOOFING-CREDITS.md" 0644 root root
}

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
  install_notices
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
