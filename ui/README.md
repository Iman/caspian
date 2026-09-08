# Caspian application

[English](README.md) | [فارسی](README.fa.md)

This Flutter application combines Caspian's desktop controls and web panel.
It uses the panel's colours, layout, wording, and English/Persian direction rules.
Windows, macOS, and Linux use native windows. Raspberry Pi serves the same UI
as a web build, accessible through its permitted IP address.

The Go backend owns configuration, authentication, hotspot control, and the
Xray engine. Flutter calls its authenticated API. Desktop service recovery
uses restricted, OS-authorized helpers so recovery remains available when the
HTTP service stops. Closing the application leaves background services running.

Open the application and follow its setup guide. On macOS, first copy
`Caspian.app` into Applications and open that copy. If the service is unavailable,
choose **Install and continue**, then approve the operating system prompt.
If services are already installed but stopped, choose **Start existing services**.
The guide waits for the service to answer before offering sign-in.
If only that check fails, retry checks readiness without reinstalling services.

Save the generated Caspian password and confirm that you saved it before
continuing. This password differs from your administrator password. On Windows,
the installer or elevated repair window shows the password; keep it before
closing that window. In-app installation and repair are available on Windows,
macOS, and Linux. An existing service keeps its normal sign-in flow after the
welcome guide is complete.

After signing in to a new installation, follow the connection checklist:
add a configuration, choose the hotspot name and password, switch Caspian on,
and join its hotspot from another device. The checklist waits for a connected
service and a reported device before marking setup ready. Browser access uses
this same checklist, without desktop installation controls. Welcome and checklist
completion are saved on that device; passwords are not stored in these preferences.

Use Flutter 3.41.6 and Go 1.26 for release builds. Build each native package on
its target OS. The Go and platform-helper requirements also apply; Flutter's
minimum OS setting alone does not establish full gateway compatibility.
Windows 1507 is not supported by the current hotspot helper, which targets
Windows 10 build 19041. Older macOS versions need separate compatibility work.

Run these commands from the repository root:

```sh
bash scripts/test-ui.sh
bash scripts/build-ui.sh web dev
bash bdd/flutter/run.sh
bash scripts/test-integration.sh macos
```

The unit gate requires at least 80% measured Flutter native Dart line coverage,
overall Go statement coverage, and Go panel statement coverage. It also requires
fresh test results and a nonzero executed test count. Existing stricter Go package floors still apply
through `scripts/gate.sh`. Web-only transport code is identified separately;
native coverage does not measure it.

BDD drives the compiled Flutter web application and real Go HTTP handlers and
state store through a browser. Native integration drives the same app and Go
fixture through a desktop window. Both use a fake privileged machine boundary.
They do not prove real hotspot traffic, administrator prompts, or installation
on another operating system. The existing `test/tunnel` suite exercises real
Xray protocol carriage; hardware validation remains a separate gate.

Build packages with:

```sh
bash packaging/darwin/build-dmg.sh dev
bash packaging/linux/build-desktop.sh dev
```

On Windows, run `packaging/windows/installer/build-installer.ps1` with the
parameters documented in that script. Production builds stage local Flutter
web assets before compiling Go with `-tags flutterui`. That build serves only
the Flutter UI and authenticated API. Builds without the tag retain the legacy
HTML harness for regression tests during migration.

Fonts and renderer assets are bundled. The web loader points font fallback
requests at the same server; it does not use a font CDN. Font licences are in
`assets/fonts`. UI golden images use fictional network values and the existing
QR encoder. They contain no real network captures.
