# Development and testing

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh)

[Caspian wiki](https://github.com/Iman/caspian/wiki/Home)

> This guide comes from the existing README. Its measurements retain their original dates; this documentation move does not report a new test run.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## Running it

Build the binary and hand it to the installer. This path needs no release, and
the installer takes it for a real install as well as a dry run:

    go build -o /tmp/caspian-linux-arm64 ./cmd/caspian
    sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

    env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
        CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
        bash install.sh --dry-run --yes

Drop `--dry-run` to install for real. Without `CASPIAN_LOCAL_CHECKSUMS` the
installer warns, in those words, that it is installing an unverified binary.
[`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) is the full runbook. It includes a fake `uname` harness for
walking the refusals on a machine that cannot be installed to.

The binary has four subcommands:

    caspian serve --privileged     root: routes, firewall, access point, engine
    caspian serve --panel          the caspian user: the web panel, nothing privileged
    caspian check                  report what this box looks like; changes nothing
    caspian version

There is deliberately no subcommand that applies a config or drives the switch.
The CLI says so itself: "After the installer has run, everything a person does
happens in the panel."

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) removes the units, the binary and the directories, and replays
the network journal so the box is left as it was found. Read [defect D5](https://github.com/Iman/caspian/wiki/Troubleshooting) before you rely on it.

## The rules this project holds itself to

These are not aspirations. Each one has a mechanism, and the mechanism is named.

**Nothing is called working without an exit IP captured from real traffic.**
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md), section 6. A connect is not a result. The hardware
harness grades UNPROVEN, not PASS, when no exit IP was captured, and it exits 1.

**A confident wrong sentence is worse than no sentence.** A reader who is told
something is handled correctly concludes there is nothing to check. So a
correction leaves a test behind rather than a better sentence.
`TestNothingInTheApplianceWatchesTheUplink` exists because two documents once
claimed the box watches its uplink and reloads the firewall when it moves.

**A started process is not evidence that it worked.** The hotspot interface is
read back from the kernel before anything binds to it, and the access point is
read back before the service reports itself running. Both readbacks were added
after one measured event in which every command had returned success.

**Every scenario has been watched failing.** `TestEveryScenarioCanFail` injects a
named defect into each behaviour and requires it to go red. A test nobody has
seen fail is a green light wired to nothing.

**The provenance of a fixture is in its filename.** `capture-pi5-` is byte
output of a real command on the target, `scenario-` is a machine nobody has
measured, and `golden-` is this project's own output. A test reading a
`capture-pi5-` file makes a claim about the target. A test reading a `scenario-`
file does not.

**A credential in a commit is permanent.** `test/goldenscan` sweeps every
committed fixture for registered sentinels and for credential shapes, and it
checks file names as well as file bodies. It has been watched catching a planted
secret of every class it knows.

**The coverage floors are a ratchet.** Every number in [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) is what
a package measured after the work that introduced it, not a target somebody
hoped for. A package with no row is not gated, and the absence of a row means
"no floor agreed yet" rather than "this package is covered".

**The privileged side trusts nothing the caller sends.** Every field of every
request is checked against what this machine detected for itself. A refusal is a
fault code from a closed set, never a sentence, and never a value the caller
sent.

**The box asks the internet for nothing.** No telemetry, no phone-home, no crash
upload, no web font, no geo data file, and no Google resolver in any default.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[Architecture](https://github.com/Iman/caspian/wiki/Architecture) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting)
