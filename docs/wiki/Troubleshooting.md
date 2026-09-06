# Troubleshooting and known defects

[English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

[Caspian wiki](https://github.com/Iman/caspian/wiki/Home)

> This guide comes from the existing README. Its measurements retain their original dates; this documentation move does not report a new test run.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## Open defects

[`docs/DEFECTS.md`](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md) is the list of things that are known, evidenced and not fixed,
with what was measured, what it costs, and what would close each one. None of
them is a leak of client traffic. Summarised, so that this file is not a reason
to skip that one:

- **D1. Nothing re-asserts the firewall once it is loaded.** Open. There is no
  read of the live ruleset anywhere in production code, and no loop that
  re-checks it. So anything that removes the table mid-session leaves the box
  forwarding and the panel reporting connected.
- **D2. Two changes to the machine had no inverse.** One is closed and the other
  is closed in process and open across a kill. The generated configuration files
  are now removed on stop, and the radio soft-block is put back, re-reading the
  device state first so a radio somebody else changed is left alone. What is
  still open is narrow: the record of which devices were unblocked lives in
  memory, so a service that is killed rather than stopped does not re-block
  them.
- **D3. A hotspot interface this package created is not released from
  NetworkManager.** Open by decision. The paths that take over an existing
  interface do release it. The paths that create one do not, because detection
  ran before that interface existed.
  `TestACreatedHotspotInterfaceHasNoMeasuredManagerAndIsNotReleased` pins the
  gap so it stays a decision rather than becoming an accident.
- **D4. Stop reports success when it undid nothing.** Open, reporting only. A
  teardown in which every inverse failed still returns no error, so the panel
  can say the box was returned to how it was found while it is still fully
  configured. The box stays fail-closed in that state, because the firewall's
  inverse is held.
- **D5. The uninstaller replays the journal by its own rules.** Open.
  [`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) carries an independent Python reimplementation of the replay.
  It has no equivalent of the rule that holds the firewall's inverse when an
  earlier one fails, so an uninstall whose routing inverses fail still deletes
  the table.

[`docs/DEFECTS.md`](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md) also lists what was closed rather than recorded, so that the
open list is not mistaken for the whole picture.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)
