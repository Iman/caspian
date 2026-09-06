# macOS development smoke test — 2026-09-05

Test machine: Intel Mac, macOS 26.6.2; USB Ethernet uplink `en6`, built-in
Wi-Fi `en0`, Internet Sharing AP `ap1` on `bridge100`.

## Verified

- Chrome DevTools MCP opened the installed panel, signed in and started Caspian.
- With the user's real configuration, `/status.json` reported `running: true`
  and `connected: true`, with no problem.
- HTTPS through the loopback SOCKS inbound succeeded with remote DNS resolution.
  The observed proxy exit address is recorded only in the local test output,
  not in this public repository.
- The panel listened on loopback and `10.83.51.1:8088`. A request bound to
  loopback against the latter address returned the expected login redirect.
  This is NOT a substitute for reachability from a Wi-Fi client.
- Live browser password change revoked the session (status endpoint returned
  401), the replaced password was rejected, and the current password logged in.
  The pre-test password was restored. Tunnel and hotspot configuration remained.
- `go test ./...` passed, including shared panel, networking, privileged-service,
  BDD, smoke, and golden checks.
- Cross-builds passed for Windows amd64, Linux arm64, Linux armv7, and macOS arm64.
  These are build checks, not physical Windows/Pi hardware tests.
- Intel DMG creation and integrity verification passed. Its binary ran, its
  Info.plist validated, and its AppleScript launcher compiled. The packaged icon
  was visually inspected against the panel's shield-and-signal artwork.
- The physical iPhone joined the built-in Wi-Fi AP and the user confirmed that
  browsing worked after the permanent macOS DNS-port correction.

## Fixes covered by regression tests

- Hexadecimal ifconfig flags containing letters no longer hide bridge/AP links.
- macOS portal detection uses the measured bridge address, not the station IP.
- macOS status no longer applies Linux's station-interface AP readback.
- Stopping from saved preferences targets NAT's own Enabled key rather than
  nested AirPort/PrimaryInterface keys.
- Password changes preserve settings, revoke sessions and reject bad inputs.
- Local password reset validates confirmation and input bounds and refuses a
  live panel or an uninitialized installation.

## Still required before release

- Phone with cellular disabled: verify the proxy exit address and captive-portal
  discovery rather than only direct browsing to a chosen site.
- Phone fail-closed checks: traffic cut, proxy outage and stop/start recovery.
- Fresh-Mac install and forgotten-password recovery via the packaged app.
  Existing-user browser password change is tested; fresh installation is not.
- Signing/notarization and Gatekeeper testing. This DMG is a development build.
- Physical Apple Silicon/Windows/Pi regression runs and any supported USB Wi-Fi
  configuration; this Mac has no USB Wi-Fi adapter attached.

## Follow-up: asynchronous controls and menu-bar app

- The updated panel passed a real Chrome Off → On cycle on loopback. Progress
  and disabled-submit state were visible; the same document remained alive.
- JavaScript tests exercise duplicate-submit prevention, bounded request waits,
  unknown outcomes on timeout, and disconnect recovery messaging.
- The native macOS control window replaces the dialog-only launcher. It provides
  service actions, password recovery, local panel tests, descriptions and a menu
  bar item. Network checks are asynchronous and subprocess I/O runs off the UI
  thread. Closing the window now hides it without releasing it; an automated
  close/reopen check kept the same process alive and restored the window.

## Follow-up: permanent macOS DNS correction

- The live PF anchor redirected client DNS to loopback port 53 even though
  Xray listened on port 5354. macOS lacks Linux's intermediate dnsmasq stage.
- Redirecting to the actual engine port restored browsing, confirmed by the
  iPhone user. This does not yet prove its exit IP or fail-closed behavior.
- `Config.netOptions` now selects `LocalDNSPort` for macOS alone. Regression
  tests check startup, traffic-cut and restore rules at both the default and
  a non-default engine port; Linux and Windows retain their existing ports.
- The corrected service binary and the installed control app's bundled binary
  are both updated, so a control-app reinstall does not restore the old code.
- After restarting both services and switching on through Chrome, the service
  regenerated TCP and UDP DNS redirects to port 5354 without the temporary
  override. The engine DNS listener resolved a test name successfully.

## Follow-up: automatic setup and update UX

- The control app now compares its bundled service binary with the installed
  `/usr/local/bin/caspian` before trusting panel reachability. Missing and
  different binaries start the administrator setup flow once per app launch;
  matching binaries do not prompt.
- Setup/update and Open panel are the only primary actions. Password reset and
  service controls are grouped under a clearly labelled bilingual Advanced
  options button, and a cancelled setup leaves its explanation and retry action
  visible. English appears above Persian from the same left edge while each
  language keeps its correct paragraph direction;
  large type and 72 point action targets are used throughout. The window stays
  at a stable size while variable content scrolls; random background clicks do
  not resize, magnify or reflow it.
- The footer has three equal columns. Build/release metadata is static, the two
  visible GitHub text links are the only clickable footer targets, and
  `Iman Samizadeh / ایمان سمیع زاده` appears in both languages. The columns do
  not overlap or compress.
  Tagged CI packaging rejects a version that differs from its GitHub tag;
  preview builds retain an explicit suffix and link to their base release.
- The Intel test DMG passed integrity, plist and deep ad-hoc signature checks.
  Its Applications shortcut resolves correctly, and its bundled version and
  SHA-256 differ from the older installed test binary, exercising the update
  detection condition without changing the live installation.
- The actual automatic authorization prompt still needs a user-observed run of
  this DMG, followed by a second launch confirming that no prompt is repeated.

## Recovery and service lifecycle

Automated checks cover the following changes:

- `caspian reset-password` generates a new panel password and preserves the
  proxy and hotspot settings. Restarting the panel clears existing sessions.
- The login page explains recovery in Persian and English.
- Service restart waits for the previous process to exit. A failed stop blocks
  restart. A temporary launchd bootstrap failure is retried.
- Changed hotspot credentials require an observed stop before the new network
  starts. An existing bridge alone cannot confirm the new credentials.

Run `bash packaging/darwin/test-service-action.sh` for the service command tests.
The repository build gate also runs these tests.

These checks still need an interactive Mac and iPhone:

1. Join the hotspot from the iPhone with the password shown in the panel.
2. Change the hotspot password. Forget the old network on the iPhone, then join
   with the new password. Make sure that the old password fails.
3. Choose **Quit Caspian and stop services**. Approve the macOS authorization
   prompt. Make sure that the app, services, and hotspot stop.
4. Open Caspian again. Make sure that it starts stopped services once. A manual
   **Stop services** action must stay stopped during later status checks.
5. Cancel authorization during Quit. Make sure that the app stays available.

The hotspot tests do not prove WPA authentication on hardware. The existing
question about macOS preferences versus Keychain storage remains open.
