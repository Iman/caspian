#!/usr/bin/env bash
# Prove the 32-bit Linux artefact was built for ARMv6, not ARMv7.
#
# WHY THIS EXISTS
#
# detect_arch in install.sh maps BOTH uname -m = armv7l AND uname -m = armv6l
# onto the single artefact caspian-linux-arm. That is only correct while the
# artefact is ARMv6, because ARMv6 code runs on an ARMv7 machine and ARMv7 code
# does not run on an ARMv6 one. Build it GOARM=7 and every Pi 1, Pi Zero and Pi
# Zero W dies with an illegal instruction on first run. This project has
# shipped that class of bug once already.
#
# WHY IT READS go version -m AND NOT readelf
#
# The first version of this check was "readelf -A <bin> | grep Tag_CPU_arch: v6"
# and it failed the release. Measured on this toolchain, GOOS=linux GOARCH=arm:
#
#   - The Go linker emits NO .ARM.attributes section at all. A GOARM=6 and a
#     GOARM=7 binary both have 26 sections and neither carries one. So that grep
#     could not pass for ANY input. It was not a strict check, it was a check
#     that always failed.
#   - ELF e_flags are byte-identical (0x05000002) between GOARM=6 and GOARM=7,
#     so the ELF header does not discriminate either.
#   - The DMB ISH byte pattern, ARMv7-only as an instruction, occurs 15 times in
#     BOTH binaries, so a naive opcode scan does not discriminate and would have
#     happily passed an ARMv7 build.
#
# What DOES discriminate is the build setting the toolchain records inside the
# binary, which "go version -m" reads back. That is the linker's own record of
# the GOARM it compiled for.
#
# The negative control below is the part that matters. The bug this replaces was
# a check that could never pass; a check that can never FAIL is the same class of
# defect. So this script proves it rejects something before it is trusted to
# accept anything.

set -euo pipefail

usage() {
  echo "usage: $0 <armv6-binary> [negative-control-binary]" >&2
  echo "  the second argument, when given, MUST NOT be an ARMv6 build." >&2
  exit 2
}

[ $# -ge 1 ] || usage

bin=$1
control=${2:-}

[ -f "$bin" ] || { echo "no such file: $bin" >&2; exit 2; }

# Prints the recorded GOARM, or nothing when the binary records none.
goarm_of() {
  go version -m "$1" 2>/dev/null |
    awk '$1 == "build" && $2 ~ /^GOARM=/ { sub(/^GOARM=/, "", $2); print $2 }'
}

# 1. NEGATIVE CONTROL FIRST. A check that cannot reject a known-bad input tells
#    us nothing when it accepts a good one, and we want to learn that here
#    rather than in a user's illegal-instruction crash.
if [ -n "$control" ]; then
  [ -f "$control" ] || { echo "no such control file: $control" >&2; exit 2; }
  if [ "$(goarm_of "$control")" = "6" ]; then
    echo "SELF-TEST FAILED: the negative control $control also reports GOARM=6." >&2
    echo "Either the control is wrong or this check accepts everything. Do not" >&2
    echo "trust the positive result below until this is resolved." >&2
    exit 1
  fi
  echo "negative control ok: $(basename "$control") is correctly not accepted as ARMv6"
fi

# 2. THE ACTUAL CHECK.
got=$(goarm_of "$bin")

if [ -z "$got" ]; then
  echo "FAIL: $bin records no GOARM at all." >&2
  echo "Either it is not a GOARCH=arm build, or its build info was stripped." >&2
  echo "install.sh maps armv6l onto this artefact, so an unproven build is a" >&2
  echo "broken Pi Zero install. Refusing to publish it." >&2
  # Only the build settings, not the dependency list, which buries them.
  go version -m "$bin" 2>&1 | grep -vE '^[[:space:]]+dep[[:space:]]' | head -12 >&2 || true
  exit 1
fi

if [ "$got" != "6" ]; then
  echo "FAIL: $bin was built GOARM=$got, not GOARM=6." >&2
  echo "install.sh maps BOTH armv7l and armv6l onto this one artefact, so an" >&2
  echo "ARMv7 build gives every Pi 1, Pi Zero and Pi Zero W an illegal" >&2
  echo "instruction on first run. Build it with GOARM=6." >&2
  exit 1
fi

echo "confirmed ARMv6: $(basename "$bin") records GOARM=6"
