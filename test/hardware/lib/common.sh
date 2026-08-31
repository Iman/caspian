# shellcheck shell=bash
#
# Caspian-BYOC hardware harness: shared vocabulary.
#
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright (C) 2026 Iman Samizadeh
#
# Sourced by every script under test/hardware. It owns five things and nothing
# else: the verdict vocabulary, the step log, the dry-run switch, the run
# directory, and the redaction guard.
#
# Written for bash 3.2, because /bin/bash on the developer Mac is 3.2.57 and a
# script that only runs under a homebrew bash is a script that fails on the one
# machine the maintainer reaches for. So: no associative arrays, no mapfile, no
# ${var,,}. Measured 2026-08-30: /bin/bash --version is 3.2.57(1)-release.

# ---------------------------------------------------------------------------
# Verdicts.
#
# These are exit codes, not adjectives, so that an unattended run can be read by
# a wrapper and a human reading the log sees the same word the wrapper saw.
# LEAK outranks everything: a run that produces both a mismatch and a leak
# reports the leak.
# ---------------------------------------------------------------------------
# shellcheck disable=SC2034
# Each constant is read by the step files and by selftest/run.sh, never inside
# common.sh itself, so shellcheck cannot see the use from here.
HW_PASS=0        # real traffic traversed the tunnel and the exit IP matched
HW_UNPROVEN=1    # no exit IP was captured. NOT a pass, and never reported as one
HW_FAIL=2        # an exit IP was captured and it did not match the config
HW_LEAK=3        # the exit IP equals the untunnelled baseline, state stable
HW_PRECONDITION=4 # the run could not start, or the harness was misused
HW_VOID=5        # the tunnel, the app or the phone changed state mid-capture

hw_verdict_name() {
  case "${1:-}" in
    0) printf 'PASS' ;;
    1) printf 'UNPROVEN' ;;
    2) printf 'FAIL' ;;
    3) printf 'LEAK' ;;
    4) printf 'PRECONDITION' ;;
    5) printf 'VOID' ;;
    *) printf 'UNKNOWN(%s)' "${1:-}" ;;
  esac
}

# ---------------------------------------------------------------------------
# Output. No colour, no emoji: the log is read by grep as often as by a person,
# and the developer Mac aliases cat to a syntax highlighter, so anything that
# emits escape sequences is unreadable twice over.
#
# PROGRESS GOES TO STDERR, RESULTS GO TO STDOUT. Not a style choice: several
# helpers return their answer on stdout and are called inside "$( )", so a
# progress line written to stdout is swallowed into the answer instead of
# reaching the operator. That happened on the first dry run on 2026-08-30, and
# it produced a capture line reading "capture:   source A: driving Chrome...",
# with the real result nowhere. The split also means
# `caspian-hw prove x > result.txt` keeps the running commentary on the terminal.
# ---------------------------------------------------------------------------
HW_STEP_N=0

hw_say()  { printf '%s\n' "$*"; }
hw_info() { printf '     %s\n' "$*" >&2; }
hw_warn() { printf 'WARN %s\n' "$*" >&2; }
hw_err()  { printf 'FAIL %s\n' "$*" >&2; }

# hw_step announces what is ABOUT to happen, before it happens, so that a run
# that dies leaves the name of the thing it died in.
hw_step() {
  HW_STEP_N=$((HW_STEP_N + 1))
  printf '\n[%02d] %s\n' "$HW_STEP_N" "$*" >&2
}

# hw_die prints which step failed and what to check, then exits PRECONDITION.
# Every call site must give the check, because "it failed" with no next move is
# what makes an unattended run useless.
hw_die() {
  local what="$1" check="$2"
  hw_err "step [$(printf '%02d' "$HW_STEP_N")] $what"
  printf 'CHECK %s\n' "$check" >&2
  exit "$HW_PRECONDITION"
}

# ---------------------------------------------------------------------------
# Dry run.
#
# HW_DRY=1 makes every hw_run print what it would do and do nothing. Capturing
# helpers return HW_DRY_SENTINEL rather than a plausible value, because a
# harness that invents a plausible exit IP under --dry-run is the exact failure
# this project exists to avoid. hw_require_measured refuses to grade a sentinel.
# ---------------------------------------------------------------------------
HW_DRY="${HW_DRY:-0}"
HW_DRY_SENTINEL='__DRY_RUN_NOT_MEASURED__'

# DRY notices go to stderr for the same reason progress does, and because a
# caller that redirects a command's stdout to /dev/null would otherwise hide the
# one line the dry run exists to print.
hw_run() {
  if [ "$HW_DRY" = "1" ]; then
    printf 'DRY  %s\n' "$*" >&2
    return 0
  fi
  "$@"
}

# hw_run_out runs a command and prints its stdout. Under --dry-run it prints the
# sentinel instead, so no caller can mistake it for a measurement.
hw_run_out() {
  if [ "$HW_DRY" = "1" ]; then
    printf 'DRY  %s\n' "$*" >&2
    printf '%s\n' "$HW_DRY_SENTINEL"
    return 0
  fi
  "$@"
}

hw_is_dry() { [ "$HW_DRY" = "1" ]; }

# hw_dry_stop ends a step under --dry-run WITHOUT a verdict.
#
# Every step calls this at the point where it would otherwise grade. A dry run
# printed the actions; it fetched nothing, so there is nothing to grade, and a
# green line here would be the harness inventing a result. It returns
# PRECONDITION rather than PASS so that no wrapper reads a dry run as a success.
hw_dry_stop() {
  local run="$1" name="$2"
  hw_ledger_end "$run" "$name" "DRY"
  hw_say ""
  hw_say "DRY RUN: nothing was fetched for '$name', so there is no verdict to give."
  return "$HW_PRECONDITION"
}

# hw_require_measured refuses to let a sentinel reach a verdict.
hw_require_measured() {
  case "$1" in
    "$HW_DRY_SENTINEL"|*"$HW_DRY_SENTINEL"*)
      hw_say "DRY  nothing was measured, so there is no verdict to give"
      exit "$HW_PRECONDITION"
      ;;
  esac
}

# ---------------------------------------------------------------------------
# Paths and the run directory.
#
# Every artefact goes under local/hardware-runs, which is inside the gitignored
# /local/ tree. That is not a convention, it is the mechanism: nothing the
# harness writes can reach a commit, because the directory it writes into is
# ignored by a rule that predates this harness (.gitignore:25, "/local/").
# hw_run_dir proves that with git check-ignore rather than asserting it.
# ---------------------------------------------------------------------------
# hw_repo_root is the checkout root. HW_HARNESS_DIR is set by the caspian-hw
# entry point to the directory holding it, which is <root>/test/hardware.
hw_repo_root() {
  [ -n "${HW_HARNESS_DIR:-}" ] || { printf '.\n'; return 0; }
  (cd -- "$HW_HARNESS_DIR/../.." && pwd)
}

hw_new_run_dir() {
  local root base id dir
  root="$(hw_repo_root)"
  base="${CASPIAN_HW_RUNS:-$root/local/hardware-runs}"
  id="run-$(date -u '+%Y%m%dT%H%M%SZ')"
  dir="$base/$id"

  # Refuse to write anywhere git would track. This is the privacy guarantee and
  # it is checked, not assumed.
  #
  # Only when the destination is INSIDE the checkout. A path outside it cannot
  # be committed by this repository, so git has no opinion and check-ignore
  # would answer "not ignored" for the uninteresting reason that it is not in
  # the tree at all.
  case "$base" in
    "$root"/*)
      if command -v git >/dev/null 2>&1 && [ -d "$root/.git" ]; then
        if ! (cd "$root" && git check-ignore -q "$base/probe") 2>/dev/null; then
          hw_die "the run directory $base is inside the checkout and is NOT gitignored" \
            "add it to .gitignore before running anything. Artefacts hold the phone's real exit IP."
        fi
      fi
      ;;
  esac

  mkdir -p "$dir" || hw_die "could not create $dir" "check the path is writable"
  printf '%s\n' "$dir"
}

# ---------------------------------------------------------------------------
# The step ledger.
#
# A partial run must LOOK partial. Every step writes a row when it starts and
# rewrites it when it ends, so a run killed halfway leaves its last step marked
# RUNNING rather than leaving a short file that reads like a clean finish.
# ---------------------------------------------------------------------------
hw_ledger_init() {
  local dir="$1" total="$2"
  printf 'step\tstatus\tstarted\tended\n' > "$dir/steps.tsv"
  printf '%s\n' "$total" > "$dir/steps.expected"
}

hw_ledger_start() {
  local dir="$1" name="$2"
  printf '%s\tRUNNING\t%s\t-\n' "$name" "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" >> "$dir/steps.tsv"
}

hw_ledger_end() {
  local dir="$1" name="$2" status="$3" tmp
  tmp="$dir/.steps.tsv.$$"
  awk -F'\t' -v OFS='\t' -v n="$name" -v s="$status" -v e="$(date -u '+%Y-%m-%dT%H:%M:%SZ')" '
    $1 == n && $2 == "RUNNING" { $2 = s; $4 = e }
    { print }
  ' "$dir/steps.tsv" > "$tmp" && mv "$tmp" "$dir/steps.tsv"
}

# hw_ledger_report says PARTIAL out loud when fewer steps ran than were planned,
# or when any step is still marked RUNNING.
hw_ledger_report() {
  local dir="$1" expected ran running
  expected="$(cat "$dir/steps.expected" 2>/dev/null || printf '0')"
  ran="$(awk -F'\t' 'NR>1 && $2!="RUNNING"' "$dir/steps.tsv" 2>/dev/null | wc -l | tr -d ' ')"
  running="$(awk -F'\t' 'NR>1 && $2=="RUNNING"' "$dir/steps.tsv" 2>/dev/null | wc -l | tr -d ' ')"
  hw_say ""
  hw_say "steps: $ran of $expected finished, $running still marked RUNNING"
  if [ "$ran" != "$expected" ] || [ "$running" != "0" ]; then
    hw_say "RESULT PARTIAL. This run did not complete. Do not read it as a clean result."
    return 1
  fi
  return 0
}

# ---------------------------------------------------------------------------
# Redaction.
#
# The harness reads real configs out of /local/. Nothing it writes may carry a
# config, a server address, a user id or a key, in a file, a log line or a
# filename. Two layers, because one is a filter and filters have holes:
#
#   hw_secret        registers a string and the placeholder that replaces it
#   hw_redact        substitutes, longest secret first, by plain string match
#   hw_guard         re-reads the written file and refuses to continue if any
#                    registered secret survived
#
# Substitution is index-based in awk rather than regex-based in sed, so a secret
# containing a regex metacharacter (a REALITY short id can, a base64 blob does)
# is matched literally and needs no escaping. Escaping is where this kind of
# code goes wrong quietly.
# ---------------------------------------------------------------------------
HW_SECRETS=""   # path to a TSV of secret<TAB>placeholder
HW_PROTECT=""   # path to a list of strings a secret may never be a substring of

hw_secrets_init() {
  HW_SECRETS="${1:?hw_secrets_init needs a path}"
  HW_PROTECT="$HW_SECRETS.protect"
  : > "$HW_SECRETS"
  : > "$HW_PROTECT"
}

# hw_protect registers a string that must survive redaction: the config labels.
#
# This exists because of a defect the selftest caught on 2026-08-30. A share
# link carries security=reality, so "reality" was registered as a secret; the
# label of that config was "reality-a"; and every artefact naming the box came
# out as "<param:reality-a>-a". The redaction had eaten the one thing the report
# is required to print. A secret that is a substring of a label is not a secret
# worth having, so it is dropped, loudly.
hw_protect() {
  [ -n "$HW_PROTECT" ] || return 0
  [ -n "$1" ] || return 0
  printf '%s\n' "$1" >> "$HW_PROTECT"
}

# hw_secret registers one secret. Two refusals, both deliberate:
#
#   too short   a 3-character secret would redact half the English language out
#               of the log and hide the very evidence the run exists to produce
#   in a label  see hw_protect
hw_secret() {
  local value="$1" placeholder="$2" min="${3:-4}"
  [ -n "$HW_SECRETS" ] || return 0
  [ -n "$value" ] || return 0
  [ "${#value}" -ge "$min" ] || return 0
  if [ -n "$HW_PROTECT" ] && [ -s "$HW_PROTECT" ]; then
    if awk -v v="$value" 'index($0, v) > 0 { found = 1 } END { exit !found }' "$HW_PROTECT"; then
      return 0
    fi
  fi
  printf '%s\t%s\n' "$value" "$placeholder" >> "$HW_SECRETS"
}

# hw_redact filters stdin to stdout.
#
# Substitution is index-based and PLACEHOLDER-FREEZING. Freezing is the second
# half of the same defect: once a stretch of the line has been replaced, no
# later secret may match inside it. Without that, secret B matching inside the
# placeholder that secret A just wrote produces nested nonsense, and the guard
# then trips on the harness's own output.
hw_redact() {
  if [ -z "$HW_SECRETS" ] || [ ! -s "$HW_SECRETS" ]; then
    cat
    return 0
  fi
  # Longest first, so a hostname that is a substring of a full URI is replaced
  # as part of the URI and not left as a half-redacted fragment.
  awk -F'\t' '{ print length($1) "\t" $0 }' "$HW_SECRETS" \
    | sort -rn -k1,1 \
    | cut -f2- \
    > "$HW_SECRETS.sorted"
  awk -F'\t' '
    NR == FNR { if (length($1) > 0) { s[++n] = $1; p[n] = $2 } ; next }
    {
      nseg = 1; seg[1] = $0; frz[1] = 0
      for (i = 1; i <= n; i++) {
        m = 0
        for (j = 1; j <= nseg; j++) {
          if (frz[j]) { m++; ns[m] = seg[j]; nf[m] = 1; continue }
          rest = seg[j]
          while ((q = index(rest, s[i])) > 0) {
            pre = substr(rest, 1, q - 1)
            if (length(pre) > 0) { m++; ns[m] = pre; nf[m] = 0 }
            m++; ns[m] = p[i]; nf[m] = 1
            rest = substr(rest, q + length(s[i]))
          }
          if (length(rest) > 0) { m++; ns[m] = rest; nf[m] = 0 }
        }
        if (m == 0) { m = 1; ns[1] = ""; nf[1] = 0 }
        nseg = m
        for (j = 1; j <= nseg; j++) { seg[j] = ns[j]; frz[j] = nf[j] }
      }
      line = ""
      for (j = 1; j <= nseg; j++) line = line seg[j]
      print line
    }
  ' "$HW_SECRETS.sorted" -
}

# hw_guard re-reads a file the harness wrote and fails the whole run if any
# registered secret is still in it. This is the check the constraint asks for,
# run against the bytes on disk rather than against intent.
#
# Placeholders are stripped before the check, so the guard cannot trip on text
# the redactor itself wrote.
hw_guard() {
  local file="$1" hit
  [ -f "$file" ] || return 0
  [ -n "$HW_SECRETS" ] && [ -s "$HW_SECRETS" ] || return 0
  hit="$(awk -F'\t' '
    NR == FNR { if (length($1) > 0) { s[++n] = $1 } ; next }
    {
      probe = $0
      gsub(/<[a-z]+:[^<>]*>/, "", probe)
      for (i = 1; i <= n; i++) if (index(probe, s[i]) > 0) { print FILENAME ":" FNR; exit }
    }
  ' "$HW_SECRETS" "$file")"
  if [ -n "$hit" ]; then
    hw_err "a registered secret survived redaction in $hit"
    hw_err "the harness will not leave that file behind"
    rm -f "$file"
    exit "$HW_PRECONDITION"
  fi
  return 0
}

# hw_leakscan re-reads EVERY file under a directory and reports any registered
# secret that survived, including in the FILE NAMES.
#
# hw_guard already checks each file as it is written. This is the separate,
# after-the-fact sweep the constraint asks for in its own words: check your own
# output before you finish. It catches anything written by a path that did not
# go through hw_write, which is the only way a leak gets out.
hw_leakscan() {
  local dir="$1" bad=0 f
  [ -d "$dir" ] || { hw_err "no such directory: $dir"; return "$HW_PRECONDITION"; }
  if [ -z "$HW_SECRETS" ] || [ ! -s "$HW_SECRETS" ]; then
    hw_err "no secrets are registered, so a clean scan would prove nothing"
    return "$HW_PRECONDITION"
  fi

  # EVERY entry, not just files: a directory named after a config label is
  # exactly the shape hw_guard_name exists to prevent, and -type f would miss it.
  find "$dir" -print > "$HW_SECRETS.names"

  # File names first. A label that carried an address would show up here.
  if awk -F'\t' '
      NR == FNR { if (length($1) > 0) { s[++n] = $1 } ; next }
      { for (i = 1; i <= n; i++) if (index($0, s[i]) > 0) { print "NAME " $0; found = 1; break } }
      END { exit !found }
    ' "$HW_SECRETS" "$HW_SECRETS.names"; then
    bad=1
  fi

  # Then contents, files only.
  while IFS= read -r f; do
    [ -f "$f" ] || continue
    case "$f" in *.png) continue ;; esac
    if awk -F'\t' '
        NR == FNR { if (length($1) > 0) { s[++n] = $1 } ; next }
        {
          probe = $0
          gsub(/<[a-z]+:[^<>]*>/, "", probe)
          for (i = 1; i <= n; i++) {
            # One report per line. Naming which secret matched would print it.
            if (index(probe, s[i]) > 0) { print FILENAME ":" FNR; found = 1; break }
          }
        }
        END { exit !found }
      ' "$HW_SECRETS" "$f"; then
      bad=1
    fi
  done < "$HW_SECRETS.names"

  if [ "$bad" = "0" ]; then
    hw_say "LEAK SCAN CLEAN: no registered secret appears in any file name or file body under"
    hw_say "  $dir"
    hw_say "  $(wc -l < "$HW_SECRETS" | tr -d ' ') secrets were searched for."
    return 0
  fi
  hw_err "LEAK SCAN FAILED. The lines above name the file and the line."
  return "$HW_FAIL"
}

# hw_guard_name refuses a filename that carries anything but the safe alphabet.
# Labels come from filenames in /local/, which the operator chose, so a label
# could be "vless-198.51.100.4". This is what stops that reaching a path.
hw_guard_name() {
  local name="$1"
  case "$name" in
    *[!A-Za-z0-9._-]*)
      hw_die "the label '$name' has characters outside A-Za-z0-9._-" \
        "rename the config file in local/configs/ to a plain label. The filename becomes a directory name in the run output."
      ;;
  esac
  # A label that looks like an address is a label that leaks one.
  if printf '%s' "$name" | grep -Eq '([0-9]{1,3}\.){3}[0-9]{1,3}'; then
    hw_die "the label '$name' contains something shaped like an IPv4 address" \
      "rename the config file in local/configs/. Labels are written into filenames and summaries; addresses are not."
  fi
}

# hw_write writes stdin to a file, redacted, then guards it. Every artefact the
# harness produces goes through here. There is no other writer.
hw_write() {
  local file="$1"
  mkdir -p "$(dirname "$file")"
  hw_redact > "$file"
  hw_guard "$file"
}

# ---------------------------------------------------------------------------
# Small shared helpers.
# ---------------------------------------------------------------------------

# hw_first_ip prints the first IPv4 or IPv6 literal on stdin, or nothing.
# "or nothing" is load-bearing: an empty result is UNPROVEN, never a pass.
hw_first_ip() {
  grep -oE '([0-9]{1,3}\.){3}[0-9]{1,3}|([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}' \
    | grep -vE '^0\.0\.0\.0$' \
    | head -1
}

hw_need() {
  command -v "$1" >/dev/null 2>&1 || hw_die "the command '$1' is not on PATH" "$2"
}
