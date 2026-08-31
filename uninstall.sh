#!/bin/bash
#
# Caspian-BYOC uninstaller.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Run the copy that install.sh left on the box:
#
#   sudo /usr/local/bin/caspian-uninstall
#
# or fetch it, the same way the installer is fetched:
#
#   /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/uninstall.sh)"
#
# Options:
#
#   --dry-run         print every action without taking any of them
#   --purge           delete /var/lib/caspian without asking
#   --keep-state      keep /var/lib/caspian without asking
#   --force           carry on even if the network journal cannot be replayed
#   --show-commands   print the replayed network commands in full (see below)
#   --yes, -y         do not ask anything
#   --help, -h        usage
#
# This script deliberately repeats a few helpers from install.sh rather than
# sharing them. Both are fetched and run on their own, so there is no library
# for them to share, and a shared file would be a third thing to download and
# verify.

set -euo pipefail

# ---------------------------------------------------------------------------
# The body is one function, called on the last line, for the same reason as in
# install.sh: a truncated download of a flat script executes its first half,
# and the first half of an uninstaller is a box with its services stopped, its
# network still rewritten, and no tool left to put it back.
# ---------------------------------------------------------------------------

# --- names and paths, all fixed by docs/LAYOUT.md --------------------------

readonly CASPIAN_BIN_PATH="/usr/local/bin/caspian"
readonly CASPIAN_STATE_DIR="/var/lib/caspian"
readonly CASPIAN_RUN_DIR="/run/caspian"
readonly CASPIAN_USER="caspian"
readonly CASPIAN_GROUP="caspian"
readonly CASPIAN_UNIT_PRIV="caspian.service"
readonly CASPIAN_UNIT_PANEL="caspian-panel.service"
readonly CASPIAN_UNIT_DIR="/etc/systemd/system"
readonly CASPIAN_TMPFILES_PATH="/etc/tmpfiles.d/caspian.conf"
readonly CASPIAN_MODULES_PATH="/etc/modules-load.d/caspian.conf"
readonly CASPIAN_UNINSTALL_PATH="/usr/local/bin/caspian-uninstall"
# The network journal. docs/LAYOUT.md and internal/netcfg/journal.go
# (DefaultJournalPath) agree on this name. They did not always: the file was
# called teardown.journal in the layout until 2026-08-30, and an uninstaller
# that guesses the name is one that silently leaves the box's routes, rules and
# firewall in place while telling the user the network was restored. If the
# name ever moves again, it moves in both places at once.
readonly CASPIAN_NETCFG_JOURNAL="/var/lib/caspian/netcfg.journal"

CASPIAN_SYSROOT="${CASPIAN_SYSROOT:-}"
CASPIAN_ASSUME_YES="${CASPIAN_ASSUME_YES:-0}"

DRY_RUN="0"
ASSUME_YES="0"
STATE_CHOICE=""
FORCE="0"
SHOW_COMMANDS="0"
WORK_DIR=""
REPLAY_PARTIAL="0"

DEST_BIN=""
DEST_STATE_DIR=""
DEST_RUN_DIR=""
DEST_UNIT_DIR=""
DEST_TMPFILES=""
DEST_MODULES=""
DEST_UNINSTALL=""
DEST_JOURNAL=""

# --- output ----------------------------------------------------------------

say() { printf '%s\n' "$*"; }
step() { printf '%s\n' "$*"; }
warn() { printf '%s\n' "warning: $*" >&2; }

die() {
  printf '%s\n' "error: $*" >&2
  exit 1
}

refuse() {
  printf '%s\n' "Caspian-BYOC cannot be uninstalled on this machine." >&2
  printf '%s\n' "$1" >&2
  printf '%s\n' "$2" >&2
  exit 1
}

usage() {
  cat <<'USAGE_EOF'
Caspian-BYOC uninstaller.

Usage:
  uninstall.sh [--dry-run] [--purge | --keep-state] [--force] [--show-commands] [--yes]

  --dry-run        Print every action that would be taken, take none of them.
  --purge          Delete /var/lib/caspian, including the saved config.
  --keep-state     Keep /var/lib/caspian.
  --force          Carry on even if the network journal cannot be replayed.
  --show-commands  Print each replayed network command in full. These contain
                   network addresses, including the address of the proxy
                   server from the saved config, so the default is off.
  --yes, -y        Do not ask anything. With neither --purge nor --keep-state
                   this keeps the state, which is the safe answer.
  --help, -h       This text.
USAGE_EOF
}

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

run() {
  if [ "$DRY_RUN" = "1" ]; then
    printf 'would run: %s\n' "$(show_cmd "$@")"
    return 0
  fi
  "$@"
}

# run_tolerant is for steps whose failure must not stop the teardown. Removing
# something that is already gone is the normal case here, not an error.
run_tolerant() {
  if [ "$DRY_RUN" = "1" ]; then
    printf 'would run: %s\n' "$(show_cmd "$@")"
    return 0
  fi
  "$@" || warn "failed, continuing: $(show_cmd "$@")"
}

confirm() {
  local prompt="$1" reply=""
  if [ ! -r /dev/tty ] || [ ! -t 1 ]; then
    return 1
  fi
  printf '%s' "$prompt"
  read -r reply </dev/tty || reply=""
  case "$reply" in
    [Yy] | [Yy][Ee][Ss]) return 0 ;;
    *) return 1 ;;
  esac
}

check_platform() {
  local os
  os="$(uname -s)"
  if [ "$os" != "Linux" ]; then
    refuse "Found: $os." "Supported: Linux."
  fi
}

require_root() {
  if [ "$DRY_RUN" = "1" ]; then
    return 0
  fi
  if [ "$(id -u)" != "0" ]; then
    die "this uninstaller must run as root. Try: sudo ${CASPIAN_UNINSTALL_PATH}"
  fi
}

# --- replaying the network journal -----------------------------------------

# The journal at /var/lib/caspian/netcfg.journal holds the inverse of every
# network change the privileged service applied, written to disk as it went
# (design section 5.5). Replaying it in reverse is what returns the box to how
# it was found, and it has to work after a crash, which is exactly why the
# record is on disk rather than in the process that made it.
#
# The replay refuses to run anything outside the allowlist in
# internal/netcfg/command.go: ip, iw, nft, sysctl. That allowlist exists so
# that the privileged side never runs a command built from user input, and the
# same reasoning applies with more force here, where the input is a file that
# has been sitting on disk.
#
# The parsing and the execution are done by python3 rather than by this shell.
# The journal entries are JSON objects (the shape of netcfg.Command: path,
# args, stdin, why), a shell cannot parse JSON without help, and the argument
# vector must reach execve without passing through a shell at any point.
replay_program() {
  cat <<'PY_EOF'
# SPDX-License-Identifier: AGPL-3.0-or-later
# Replays a Caspian-BYOC network journal.
#
# The format is the one internal/netcfg/journal.go writes: JSON lines, one
# Record per line, several records per step, keyed by "seq".
#
#   {"seq":1,"phase":"begin","t":"...","op":"route","why":"...",
#    "do":{"path":"ip","args":[...]},"undo":{"path":"ip","args":[...]}}
#   {"seq":1,"phase":"done","t":"..."}
#
# The rules below are taken from LoadJournal and Entry.NeedsUndo in that file,
# and any change there has to be mirrored here:
#
#   - the "begin" record carries op, why, do and undo; later records for the
#     same seq only move the phase on
#   - an entry needs undoing unless its last phase is "undone"; "begin",
#     "done" and "failed" all still need it, because a command that was killed
#     halfway can have landed part of its effect
#   - an entry with neither a do nor an undo is dropped
#   - inverses are replayed newest first
#
# Exit codes, which the calling shell script depends on:
#   0  every inverse that was needed ran and succeeded
#   1  the journal was understood and something did not apply
#   2  the journal was refused, and nothing at all was run
import json
import os
import shutil
import subprocess
import sys

# internal/netcfg/command.go, allowedBinaries.
ALLOWED = ("ip", "iw", "nft", "sysctl")


def refuse(message):
    sys.stderr.write("journal: %s\n" % message)
    return 2


def command_of(obj):
    """Returns (path, args, stdin), None for an absent or zero Command, or the
    string "invalid" for one that is malformed."""
    if obj is None:
        return None
    if not isinstance(obj, dict):
        return "invalid"
    path = obj.get("path") or ""
    args = obj.get("args")
    if args is None:
        args = []
    stdin = obj.get("stdin") or ""
    if not isinstance(path, str) or not isinstance(stdin, str):
        return "invalid"
    if not isinstance(args, list) or not all(isinstance(a, str) for a in args):
        return "invalid"
    # netcfg.Command.IsZero.
    if path == "" and not args:
        return None
    return (path, args, stdin)


def load(path):
    """Returns (entries, skipped_lines) or (None, None) if the file is refused."""
    by_seq = {}
    skipped = 0
    with open(path, "r", encoding="utf-8") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
            except ValueError:
                # A truncated or corrupt tail. LoadJournal drops it and keeps
                # every complete record before it, because that case is
                # exactly the one the journal exists for. The count is
                # reported, so a silently shorter teardown is still visible.
                skipped += 1
                continue
            if not isinstance(record, dict) or not isinstance(record.get("seq"), int):
                skipped += 1
                continue
            seq = record["seq"]
            entry = by_seq.get(seq)
            if entry is None:
                entry = {"seq": seq, "op": "", "undo": None, "do": None, "phase": ""}
                by_seq[seq] = entry
            if record.get("phase") == "begin":
                entry["op"] = record.get("op") or ""
                entry["do"] = command_of(record.get("do"))
                entry["undo"] = command_of(record.get("undo"))
            entry["phase"] = record.get("phase") or ""

    entries = []
    for seq in sorted(by_seq):
        entry = by_seq[seq]
        if entry["do"] == "invalid" or entry["undo"] == "invalid":
            return None, refuse("entry %d has a malformed command" % seq)
        if entry["do"] is None and entry["undo"] is None:
            continue
        entries.append(entry)

    # The allowlist is checked over the whole file before anything runs. A line
    # that parses but names another binary is not a truncation artefact, it is
    # corruption or tampering, and running the entries before it would be
    # exactly the failure the allowlist exists to prevent.
    for entry in entries:
        if entry["undo"] is None:
            continue
        name = os.path.basename(entry["undo"][0])
        if name not in ALLOWED:
            return None, refuse(
                "entry %d undoes with %s, which is not one of %s"
                % (entry["seq"], name, ", ".join(ALLOWED))
            )
        if "/" in entry["undo"][0] and not entry["undo"][0].startswith("/"):
            return None, refuse("entry %d has a relative path with a slash" % entry["seq"])
    return entries, skipped


def main(argv):
    if len(argv) < 2:
        return refuse("no journal path given")
    path = argv[1]
    flags = argv[2:]
    dry = "--dry-run" in flags
    show = "--show-commands" in flags

    if not os.path.exists(path):
        return 0
    entries, skipped = load(path)
    if entries is None:
        return skipped

    todo = [e for e in entries if e["phase"] != "undone" and e["undo"] is not None]
    if not todo and not skipped:
        sys.stdout.write("nothing left to undo in %s\n" % path)
        return 0

    failures = 0
    for entry in reversed(todo):
        command, args, stdin = entry["undo"]
        # By default only the sequence number and the operation are printed.
        # Both come from a fixed vocabulary in internal/netcfg/route.go, so
        # neither can carry an address. The arguments can: one of the inverses
        # removes the pinned host route to the user's proxy server, and
        # docs/LAYOUT.md says that is never printed or logged.
        if show:
            label = "%d %s: %s" % (entry["seq"], entry["op"], " ".join([command] + args))
        else:
            label = "%d (%s)" % (entry["seq"], entry["op"] or os.path.basename(command))
        if dry:
            # Before the lookup below, on purpose: a dry run must describe the
            # journal on any machine, including one with no ip and no nft.
            sys.stdout.write("would replay entry %s\n" % label)
            continue
        resolved = command
        if "/" not in command:
            resolved = shutil.which(command) or ""
            if not resolved:
                sys.stderr.write("entry %d: %s is not installed, skipping\n" % (entry["seq"], command))
                failures += 1
                continue
        sys.stdout.write("replaying entry %s\n" % label)
        try:
            result = subprocess.run(
                [resolved] + args,
                input=stdin.encode("utf-8"),
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
        except OSError as exc:
            sys.stderr.write("entry %d failed to start: %s\n" % (entry["seq"], exc))
            failures += 1
            continue
        if result.returncode != 0:
            # Carrying on is deliberate, and matches Applier.Teardown: one
            # inverse that cannot be applied, usually because the thing it
            # undoes is already gone, must not strand every inverse after it.
            sys.stderr.write("entry %d exited %d, continuing\n" % (entry["seq"], result.returncode))
            failures += 1

    if skipped:
        sys.stderr.write("%d unreadable line(s) in %s were skipped\n" % (skipped, path))
    if failures:
        sys.stderr.write("%d of %d inverses did not apply\n" % (failures, len(todo)))
    if failures or skipped:
        return 1
    return 0


sys.exit(main(sys.argv))
PY_EOF
}

replay_teardown_journal() {
  local program flags="" status

  if [ ! -f "$DEST_JOURNAL" ]; then
    step "No journal; the network was never changed, or it has already been put back."
    return 0
  fi

  if ! command -v python3 >/dev/null 2>&1; then
    if [ "$FORCE" = "1" ]; then
      warn "python3 is not installed, so the journal cannot be replayed."
      warn "continuing because --force was given. The box's network is NOT being restored."
      REPLAY_PARTIAL="1"
      return 0
    fi
    die "python3 is not installed, and it is needed to read ${CASPIAN_NETCFG_JOURNAL}.
The journal is the record of every network change this software made, and replaying
it is what puts the box back. Nothing has been removed. Either install python3 and
run this again, or run it again with --force to remove the software and leave the
network as it is."
  fi

  program="${WORK_DIR}/replay.py"
  replay_program >"$program"

  if [ "$DRY_RUN" = "1" ]; then
    flags="--dry-run"
  fi
  if [ "$SHOW_COMMANDS" = "1" ]; then
    flags="${flags} --show-commands"
  fi

  set +e
  # shellcheck disable=SC2086
  # flags is a list this script built from its own options, never from input.
  python3 "$program" "$DEST_JOURNAL" $flags
  status=$?
  set -e

  case "$status" in
    0)
      step "Network journal replayed."
      ;;
    1)
      warn "some entries in ${DEST_JOURNAL} did not apply. See the lines above."
      REPLAY_PARTIAL="1"
      ;;
    *)
      if [ "$FORCE" = "1" ]; then
        warn "${DEST_JOURNAL} could not be read; continuing because --force was given."
        warn "the box's network is NOT being restored."
        REPLAY_PARTIAL="1"
        return 0
      fi
      die "the network journal at ${DEST_JOURNAL} was refused, so nothing was replayed
and nothing has been removed. The journal is still there. Run again with --force to
remove the software anyway and leave the network as it is."
      ;;
  esac
}

# --- removal ---------------------------------------------------------------

unit_installed() {
  [ -f "${DEST_UNIT_DIR}/$1" ]
}

# stop_services runs before the journal is replayed. A running privileged
# service would put back what the replay has just taken away.
stop_services() {
  local u
  for u in "$CASPIAN_UNIT_PANEL" "$CASPIAN_UNIT_PRIV"; do
    if unit_installed "$u"; then
      run_tolerant systemctl disable --now "$u"
    fi
  done
}

remove_units() {
  local f
  for f in "${DEST_UNIT_DIR}/${CASPIAN_UNIT_PANEL}" "${DEST_UNIT_DIR}/${CASPIAN_UNIT_PRIV}" \
    "$DEST_TMPFILES" "$DEST_MODULES"; do
    if [ -f "$f" ]; then
      run_tolerant rm -f "$f"
    fi
  done
  if command -v systemctl >/dev/null 2>&1; then
    run_tolerant systemctl daemon-reload
    # Clears the "unit file is gone but the failed state is remembered" entry
    # that would otherwise sit in the unit list until the next reboot.
    run_tolerant systemctl reset-failed
  fi
}

remove_binaries() {
  if [ -f "$DEST_BIN" ]; then
    run_tolerant rm -f "$DEST_BIN"
  fi
  # Last, and tolerantly: this may be the file currently executing. Removing a
  # running script's file is safe on Linux because the inode stays alive until
  # the process exits.
  if [ -f "$DEST_UNINSTALL" ]; then
    run_tolerant rm -f "$DEST_UNINSTALL"
  fi
}

# remove_runtime_dirs takes /run/caspian and everything under it, which
# includes the dnsmasq directory. There is no /etc/caspian: docs/LAYOUT.md
# dropped it on 2026-08-30, because the generated hostapd and dnsmasq files it
# was supposed to hold live under /run, where a WPA2 passphrase does not
# survive a power cut into a file nobody knows is there.
remove_runtime_dirs() {
  if [ -d "$DEST_RUN_DIR" ]; then
    run_tolerant rm -rf "$DEST_RUN_DIR"
  fi
}

# decide_state_choice settles what happens to /var/lib/caspian. It asks only
# when nothing on the command line has already answered, and the answer to
# silence is always keep: a deleted config cannot be got back, and a kept one
# costs a directory.
decide_state_choice() {
  if [ -n "$STATE_CHOICE" ]; then
    return 0
  fi
  if [ ! -d "$DEST_STATE_DIR" ]; then
    STATE_CHOICE="keep"
    return 0
  fi
  # Nothing to ask with, or told not to ask: keep. The paragraph below is only
  # printed when there is somebody there to answer it.
  if [ "$ASSUME_YES" = "1" ] || [ ! -r /dev/tty ] || [ ! -t 1 ]; then
    STATE_CHOICE="keep"
    step "Nothing was asked, so ${CASPIAN_STATE_DIR} is kept. Use --purge to delete it."
    return 0
  fi
  say ""
  say "${CASPIAN_STATE_DIR} holds the saved proxy config, the hotspot name and password,"
  say "and the panel password. Deleting it cannot be undone."
  if confirm "Delete it? [y/N] "; then
    STATE_CHOICE="purge"
  else
    STATE_CHOICE="keep"
  fi
}

remove_state() {
  if [ "$STATE_CHOICE" != "purge" ]; then
    if [ -d "$DEST_STATE_DIR" ]; then
      step "Keeping ${CASPIAN_STATE_DIR}."
    fi
    return 0
  fi
  if [ -d "$DEST_STATE_DIR" ]; then
    run_tolerant rm -rf "$DEST_STATE_DIR"
  fi
}

# remove_account removes the service account, but only when the state
# directory went with it. Removing the account while its directory is still
# there would leave files owned by a numeric id that the next account created
# on this box could inherit.
remove_account() {
  if [ "$STATE_CHOICE" != "purge" ]; then
    step "Keeping the ${CASPIAN_USER} account, because ${CASPIAN_STATE_DIR} is still there."
    return 0
  fi
  if getent passwd "$CASPIAN_USER" >/dev/null 2>&1; then
    if command -v userdel >/dev/null 2>&1; then
      run_tolerant userdel "$CASPIAN_USER"
    elif command -v deluser >/dev/null 2>&1; then
      run_tolerant deluser "$CASPIAN_USER"
    fi
  fi
  if getent group "$CASPIAN_GROUP" >/dev/null 2>&1; then
    if command -v groupdel >/dev/null 2>&1; then
      run_tolerant groupdel "$CASPIAN_GROUP"
    elif command -v delgroup >/dev/null 2>&1; then
      run_tolerant delgroup "$CASPIAN_GROUP"
    fi
  fi
}

final_message() {
  printf '\n'
  if [ "$DRY_RUN" = "1" ]; then
    say "Dry run finished. Nothing was changed."
    say ""
    say "On a real run the closing message would be:"
  fi
  if [ "$REPLAY_PARTIAL" = "1" ]; then
    say "Caspian-BYOC is removed, but the network was not fully restored."
    say "A reboot returns the routes, rules and firewall to the box's own configuration."
    return 0
  fi
  say "Caspian-BYOC is removed and the network is back to how it was found."
}

# --- wiring ----------------------------------------------------------------

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --dry-run) DRY_RUN="1" ;;
      --purge) STATE_CHOICE="purge" ;;
      --keep-state) STATE_CHOICE="keep" ;;
      --force) FORCE="1" ;;
      --show-commands) SHOW_COMMANDS="1" ;;
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

setup_dest_paths() {
  if [ -n "$CASPIAN_SYSROOT" ] && [ "$DRY_RUN" != "1" ]; then
    die "CASPIAN_SYSROOT is a testing hook and is only allowed together with --dry-run"
  fi
  DEST_BIN="${CASPIAN_SYSROOT}${CASPIAN_BIN_PATH}"
  DEST_STATE_DIR="${CASPIAN_SYSROOT}${CASPIAN_STATE_DIR}"
  DEST_RUN_DIR="${CASPIAN_SYSROOT}${CASPIAN_RUN_DIR}"
  DEST_UNIT_DIR="${CASPIAN_SYSROOT}${CASPIAN_UNIT_DIR}"
  DEST_TMPFILES="${CASPIAN_SYSROOT}${CASPIAN_TMPFILES_PATH}"
  DEST_MODULES="${CASPIAN_SYSROOT}${CASPIAN_MODULES_PATH}"
  DEST_UNINSTALL="${CASPIAN_SYSROOT}${CASPIAN_UNINSTALL_PATH}"
  DEST_JOURNAL="${CASPIAN_SYSROOT}${CASPIAN_NETCFG_JOURNAL}"
}

make_work_dir() {
  WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/caspian-uninstall.XXXXXX")"
  trap 'rm -rf "$WORK_DIR"' EXIT
}

caspian_uninstall_main() {
  if [ "${CASPIAN_SOURCE_ONLY:-0}" = "1" ]; then
    return 0
  fi

  parse_args "$@"
  umask 022
  PATH="${PATH}:/usr/local/sbin:/usr/sbin:/sbin"

  if [ "$DRY_RUN" = "1" ]; then
    say "Dry run. Nothing will be changed."
  fi

  check_platform
  require_root
  setup_dest_paths
  make_work_dir

  # The order matters and is the reverse of the install: stop first so nothing
  # re-applies what is about to be undone, put the network back while the
  # journal is still readable, and only then remove the files.
  stop_services
  replay_teardown_journal
  remove_units
  remove_binaries
  remove_runtime_dirs
  decide_state_choice
  remove_state
  remove_account

  final_message
}

# The single call, on the last line. See the note at the top.
caspian_uninstall_main "$@"
