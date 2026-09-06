# Installation

[English](https://github.com/Iman/caspian/wiki/Installation) | [فارسی](https://github.com/Iman/caspian/wiki/Installation.fa) | [Русский](https://github.com/Iman/caspian/wiki/Installation.ru) | [中文](https://github.com/Iman/caspian/wiki/Installation.zh)

[Caspian wiki](https://github.com/Iman/caspian/wiki/Home)

> This guide comes from the existing README. Its measurements retain their original dates; this documentation move does not report a new test run.
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## Installing

Choose the operating system first. Windows and macOS have graphical installers;
Linux and Raspberry Pi can be installed automatically, checked first, or built
from source.

### Windows 10 and 11

Windows 10 support is experimental and not yet tested. On x64, the current code
requires version 2004 (build 19041) or later. Windows 10 ARM64 compatibility
is not yet verified.

The setup program installs everything that Caspian needs. You do not need
PowerShell, Go, or the .NET SDK.

#### What you need

- A Windows 11 computer, or Windows 10 version 2004 or later on x64 (experimental).
- An administrator account on that computer.
- A Wi-Fi adapter that supports Windows Mobile Hotspot.
- An internet connection.
- A supported proxy link or configuration.

#### Choose the correct download

Most Intel and AMD computers use the x64 installer:

- `CaspianSetup-0.2.1-windows-x64.exe`

Windows computers with a Snapdragon or another ARM processor use the ARM64
installer:

- `CaspianSetup-0.2.1-windows-arm64.exe`

If you do not know your computer type, open **Settings**. Select **System**,
then **About**. Read the **System type** line.

#### Install Caspian

1. Open the [Caspian release page](https://github.com/Iman/caspian/releases/latest).
2. Expand **Assets** under the newest release.
3. Download the correct Windows installer.
4. Double-click the downloaded file.
5. If SmartScreen appears, select **More info**.
6. Make sure that the publisher warning names the file that you downloaded.
7. Select **Run anyway**.
8. Select **Yes** when Windows requests administrator access.
9. Read the licence page, then continue.
10. Choose a password for the Caspian web panel.
11. Type the same password again.
12. Keep this password in a safe place.

The setup wizard also shows two optional choices:

- **Create a desktop shortcut**
- **Start Caspian Control when I sign in**

Both choices are off by default. Setup always creates a **Caspian Control**
shortcut in the Windows Start menu. When setup finishes, leave **Open Caspian
Control** selected and click **Finish**.

Windows can show an **Unknown publisher** warning until the installer has a
code-signing certificate. Check that the file came from the official Caspian
release page before you continue.

![Caspian Control on Windows](https://github.com/Iman/caspian/blob/main/docs/images/caspian-control-windows.png)

#### The two Caspian windows

Caspian has two different control screens on Windows.

| Screen | Where it opens | What it controls |
|---|---|---|
| **Caspian Control** | A small Windows app and notification-area icon | Starts, stops, or restarts the Caspian background services |
| **Caspian web panel** | Your web browser at `http://127.0.0.1:8088/` | Sets the Wi-Fi name, Wi-Fi password, frequency band, and proxy connection |

Use **Caspian Control** first. Wait for **Ready**, then select **Open panel**.
The web panel is the second screen. Use it to start the hotspot and tunnel.

**Ready** in Caspian Control means that the two background services answer.
It does not mean that the proxy tunnel is connected. The web panel turns green
when the hotspot and tunnel are ready.

#### First start

1. Open **Caspian Control** from the Windows Start menu or desktop.
2. Select **Yes** when Windows requests administrator access.
3. Select **Start all**.
4. Wait until the large card says **Ready**.
5. Select **Open panel**.
6. Type the panel password that you chose during setup.
7. Select **Sign in**.
8. Enter a name for the new Wi-Fi network.
9. Enter a Wi-Fi password with at least eight characters.
10. Keep **2.4 GHz** for the best support with old devices.
11. Paste your proxy link or configuration.
12. Select the switch to start Caspian.
13. Wait until the web panel status turns green.
14. Connect your phone or other device to the new Wi-Fi network.
15. Open a website on that device to test the connection.

The panel shows each connected device. Windows gives these devices addresses
from `192.168.137.0/24`. Caspian sends their internet traffic through the
`xray0` tunnel.

The panel password and Wi-Fi password are different. The panel password opens
the web panel. The Wi-Fi password connects phones and other devices.

#### What the Caspian Control buttons do

| Control | Result |
|---|---|
| **Start all** | Starts both Caspian background services |
| **Stop all** | Stops both services and keeps them stopped |
| **Restart all** | Stops and starts both services |
| **Open panel** | Opens the Caspian web panel in your browser |

The app stays in the Windows notification area after you close its window.
The notification area is beside the clock. Double-click the Caspian icon to
open the app again.

#### What to expect

Windows requests administrator access because Caspian changes network routes,
the firewall, Mobile Hotspot, and the Wintun network adapter.

Windows disconnects devices when you stop or restart the hotspot. Wait for the
web panel to turn green, then connect each device again.

If Caspian Control says **Ready** but the web panel is red, read the message in
the web panel. The web panel tests the hotspot and the proxy tunnel.

#### Developer requirements

The PowerShell method below is for developers. It builds Caspian from this
repository and installs the Windows services.

This method needs these additional programs:

- An administrator account.
- An active internet connection.
- A Wi-Fi adapter that supports Windows Mobile Hotspot.
- [Git for Windows](https://git-scm.com/download/win).
- [Go 1.26 or later](https://go.dev/dl/).
- [.NET 9 SDK](https://dotnet.microsoft.com/download/dotnet/9.0).

The setup program and developer method support x64 and ARM64 Windows systems.

#### Developer install

1. Open PowerShell.
2. Clone this repository.
3. Change to the repository directory.
4. Run the Windows installer.

```powershell
git clone https://github.com/Iman/caspian.git
Set-Location caspian
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\install.ps1
```

5. Click **Yes** when Windows requests administrator access.
6. Wait for the build and service installation to finish.

The installer does these tasks:

- Builds `caspian.exe` for the current computer.
- Builds the Windows Mobile Hotspot helper.
- Builds the `CaspianControl.exe` tray app.
- Downloads Wintun 0.14.1 when `wintun.dll` is absent.
- Compares the Wintun archive with its fixed SHA-256 value.
- Installs the programs in `C:\Program Files\Caspian`.
- Creates the `caspian` and `caspian-panel` Windows services.
- Sets both services to start automatically.
- Creates a **Caspian Control** desktop shortcut.
- Waits up to 45 seconds for the local panel.
- Opens Caspian Control after the panel answers.

Use `-NoOpen` to install without opening the tray app:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\install.ps1 -NoOpen
```

#### Repair or update

Run the same installer again. The installer stops the services, replaces the
programs, preserves the panel state, and starts the services again.

#### Uninstall

Open an administrator PowerShell in the repository. Then run:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\uninstall.ps1
```

The uninstaller removes the Windows services and installed programs. Read the
script before use when you need to preserve local state.

### macOS 13 or later

The macOS disk image contains the native **Caspian Control** app and the
Caspian engine. You do not need Terminal, Go, Homebrew, or another runtime.
You need an administrator account and, when the built-in Wi-Fi is the hotspot,
a wired Ethernet internet connection.

#### Choose the correct download

- Intel Macs use `Caspian-v0.2.4-macos-amd64.dmg`.
- Apple Silicon Macs (M1 or later) use `Caspian-v0.2.4-macos-arm64.dmg`.

Open **Apple menu → About This Mac** if you do not know whether the Mac has an
Intel processor or Apple silicon.

#### Install and approve the first opening

The v0.2.4 app is ad-hoc signed but is not yet signed with an Apple Developer
ID or notarized by Apple. Gatekeeper therefore shows **“Caspian” Not Opened**
and says that Apple could not verify that it is free of malware. This is not an
application crash. Override the warning only for a file downloaded from the
official Caspian release page.

1. Open the [latest Caspian release](https://github.com/Iman/caspian/releases/latest)
   and expand **Assets**.
2. Download the DMG for the Mac's processor and open it.
3. Drag `Caspian.app` into the **Applications** folder.
4. Open the copy in **Applications** once.
5. When Gatekeeper blocks it, click **Done**.
6. Open **Apple menu → System Settings → Privacy & Security**.
7. Scroll to **Security** and click **Open Anyway** beside Caspian. The button
   remains available for about one hour after the blocked opening attempt.
8. Enter the Mac login password, click **OK**, then confirm **Open**.

macOS saves this app as an exception, so later openings work by double-clicking
it normally. Apple documents the same process in
[Open an app by overriding security settings](https://support.apple.com/guide/mac-help/apple-cant-check-app-for-malicious-software-mchleab3a043/26/mac/26).

#### If macOS still blocks the background service

The installed background executable, `/usr/local/bin/caspian`, can retain a
quarantine flag after you approve `Caspian.app`. The warning names lowercase
`caspian`, and the control window can report **Caspian needs attention**.

**If the alert names a Trojan or reports malware, do not use the command below.**
Stop setup and report the exact alert, detection name, release version, and
download URL in a [GitHub issue](https://github.com/Iman/caspian/issues).
A malware detection needs investigation; an unsigned release alone does not
establish that the detection is false. See
[Apple's explanation of macOS security alerts](https://support.apple.com/en-ie/102445).

Use this fallback only for an unverified-developer or unnotarized-app warning,
after you trust the file and its source. Download the release from the official
Caspian release page and compare the DMG checksum with its published
`SHA256SUMS`. A matching checksum confirms the release file, not its safety.

1. Open **Terminal**.
2. Remove the quarantine flag from the installed background executable:

   ```bash
   sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
   ```

3. Enter your Mac login password. Terminal does not display the password as you type.
4. In Caspian, choose **Advanced options → Restart services**.

This command removes only the named file's quarantine attribute. It does not
scan, sign, or notarize the executable. If Terminal reports `No such xattr`, the
attribute is already absent. If the service still fails, report the error
instead of removing other security controls.

#### Let Caspian set itself up and save its password

1. Launch **Caspian Control**. It compares the bundled background service with
   the installed one before it checks the panel.
2. On the first launch, or when the DMG contains an update, setup starts
   automatically. Enter the administrator password in the macOS authorization
   dialog. When the installed version already matches, no password is requested.
3. Wait until the control window says **Caspian is ready**. If authorization is
   cancelled, **Set up Caspian** or **Update Caspian** remains visible for retry.
4. On a first installation, save the **first-run panel password** shown in the
   output. **Copy panel password** copies only that password.
5. Click **Open panel** and sign in with the saved panel password.
6. Enter the Wi-Fi name and password, paste the proxy configuration, then use
   the panel switch to start Caspian.

The Mac login password, Caspian panel password, and Wi-Fi password are three
different passwords. If the panel password is lost, use **Reset password** in
Caspian Control; administrator authorization is required, but the saved proxy
and hotspot settings remain. Closing the control window leaves its menu-bar
item running; choose **Open Caspian Control** there to reopen it.

On macOS, closing the control window keeps Caspian in the menu bar. Choosing
**Quit Caspian and stop services** stops the hotspot and background services.
If macOS authorization is cancelled or stopping fails, the app stays open.
On the next launch, Caspian starts stopped services once, with administrator authorization.

### Linux and Raspberry Pi

#### Automated: one line

    sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

The installer works out what machine it is on, downloads the matching binary
from the latest release, and refuses if the download does not match the
published checksum.

| `uname -m` | artefact | typical machine |
|---|---|---|
| `x86_64` | `caspian-linux-amd64` | a laptop or a mini PC |
| `aarch64` | `caspian-linux-arm64` | Raspberry Pi 3, 4, 5 on a 64-bit system |
| `armv7l` | `caspian-linux-arm` | Raspberry Pi 2 and 3 on a 32-bit system |
| `armv6l` | `caspian-linux-arm` | Raspberry Pi 1, Zero, Zero W |

It refuses rather than guesses when it cannot be sure. Not Linux, an
architecture not in that table, no systemd, or a checksum that does not match:
each is a refusal naming what it found. `armv8l`, a 32-bit userland on a 64-bit
kernel, is deliberately not mapped, because guessing there is how a previous
project shipped ARMv7 code to ARMv6 machines and left them dying with an
illegal instruction on first run.

Read the script before you pipe it into a shell. That advice is not a formality
for software of this kind, and the script is written to be read.

    curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | less

The command above displays the script; it does not install or update Caspian.
Run the installation command again to upgrade. The installer selects the latest
published release and keeps your saved settings. The portal is included in the
binary. Changes on `main` appear after a release includes them.

To install a specific release, replace the example tag below:

    sudo env CASPIAN_VERSION=v0.2.5 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"


### Forgotten panel password

On the Caspian computer, run this command in a terminal or SSH session:

```bash
sudo /usr/local/bin/caspian reset-password
```

The command prints a new panel password and restarts the panel. Your proxy and
Wi-Fi settings stay saved. Use the new password on the login page. On Windows,
run `& "$env:ProgramFiles\Caspian\caspian.exe" reset-password` in an administrator
PowerShell window. Reinstalling Caspian does not reset your password.

### Verifying a download yourself

Every release carries a `SHA256SUMS` file. The installer checks it for you, and
you can check it independently:

    curl -fsSLO https://github.com/Iman/caspian/releases/latest/download/caspian-linux-arm64
    curl -fsSLO https://github.com/Iman/caspian/releases/latest/download/SHA256SUMS
    sha256sum -c SHA256SUMS --ignore-missing

What that proves and what it does not: it proves the file you have is the file
the release published. It does not prove who built that release. The binaries
are built by GitHub Actions from a tagged commit, and the workflow that builds
them is in this repository at [`.github/workflows/release.yml`](https://github.com/Iman/caspian/blob/main/.github/workflows/release.yml), so the build is
readable even though it is not independently reproducible.

#### Manual: build it yourself

Nothing about the automated route is required. Building from source needs Go
1.26 or later and gives a binary identical in function.

    git clone https://github.com/Iman/caspian.git
    cd caspian
    go build -trimpath -o caspian ./cmd/caspian
    sudo CASPIAN_LOCAL_BINARY="$PWD/caspian" bash install.sh

`CASPIAN_LOCAL_BINARY` tells the installer to use the file you just built rather
than downloading one. Everything else the installer does, creating the service
account, the directories, the units and their permissions, happens the same way.

Cross-compiling for a Pi from another machine:

    GOOS=linux GOARCH=arm64 go build -trimpath -o caspian-linux-arm64 ./cmd/caspian
    GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -o caspian-linux-arm ./cmd/caspian

`GOARM=6` on the 32-bit build is not optional. Both `armv6l` and `armv7l`
machines install the same `arm` artefact, so an ARMv7 build breaks every Pi 1,
Zero and Zero W that installs it. The release workflow checks this with
`readelf` and fails rather than publishing an artefact that lies about its
architecture.

Before you trust a build, run the gate:

    bash scripts/gate.sh

It runs formatting, vet, the whole suite with the race detector, per-package
coverage floors, the golden regression layer, a privacy scan and a smoke
subset. It exits non-zero on failure. Do not pipe it anywhere: a shell pipeline
reports the status of its last command, so piping it into `tail` throws away
the answer you asked for.

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)
