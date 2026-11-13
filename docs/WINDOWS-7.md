# Caspian on Windows 7 and older Windows, through VirtualBox

[فارسی](WINDOWS-7.fa.md)

> **Not yet tested on Windows 7.** Every step below uses tools that exist and
> download today, and Caspian's own part is the normal Linux install. Nobody has
> run the whole path on a Windows 7 PC yet. If you do, please open an issue and
> say what worked and what did not.

The Windows app of Caspian needs Windows 10 (version 1607 or later) or Windows 11.
On Windows 7 you run Caspian inside a small Linux virtual machine instead. The
virtual machine gets the internet from your PC, and a USB Wi-Fi adapter that you
give to the virtual machine becomes the hotspot.

```text
[router] --cable or Wi-Fi--> [Windows 7 PC] --VirtualBox--> [Ubuntu + Caspian] --USB Wi-Fi--> [your phone, TV, ...]
```

## Why not Docker Toolbox

Docker Toolbox was the old way to run containers on Windows 7. It was retired in
2020, and its Linux image (boot2docker) was archived on 30 June 2020. Its kernel
has no nftables support and no USB Wi-Fi drivers, and Caspian needs both: the
firewall that stops leaks is an nftables ruleset, and the hotspot needs a Wi-Fi
driver. The Caspian container image is also only for looking at the panel, not
for running the gateway (see the `Dockerfile`). So this guide uses a full Linux
virtual machine instead. Docker Toolbox also ran its containers inside a
VirtualBox machine, so the idea is the same. Only the Linux inside is different.

## What you need

| Item | Detail |
|---|---|
| PC | Windows 7 SP1, **64-bit**. VirtualBox 6.x does not run on 32-bit Windows. |
| CPU setting | Virtualisation (Intel VT-x or AMD-V) switched on in the BIOS. |
| Memory | 4 GB in the PC; the virtual machine uses 2 GB. |
| Disk | 25 GB free. |
| Internet into the PC | An Ethernet cable from the router is best. The PC's own Wi-Fi also works. |
| USB Wi-Fi adapter | One whose chip Linux can run as an access point (AP). See below. |
| A config | Your VLESS, VMess, Trojan, Shadowsocks, SOCKS or Hysteria2 link, Clash YAML, Xray JSON or subscription. |

### Which USB Wi-Fi adapter

The adapter must work as an access point under Linux. These chip families have
access point support in the standard Linux kernel:

- Atheros AR9271 (driver `ath9k_htc`)
- Ralink RT5370 and RT5372 (driver `rt2800usb`)
- MediaTek MT7610U and MT7612U (drivers `mt76x0u`, `mt76x2u`)

Look for the chip name in the shop listing, not only the brand. Many cheap
Realtek adapters do not run as an access point under Linux. After the install,
`caspian check` tells you if your adapter can make a hotspot.

## Downloads

Download each file on the Windows 7 PC. Check each file against the checksums
on the same page when one is given.

| What | Version | Where |
|---|---|---|
| VirtualBox for Windows hosts | 6.1.50, the last release that supports Windows 7 | https://www.virtualbox.org/wiki/Download_Old_Builds_6_1 (file `VirtualBox-6.1.50-161033-Win.exe`) |
| VirtualBox Extension Pack | 6.1.50, same version as VirtualBox. Needed for USB 2.0. | https://download.virtualbox.org/virtualbox/6.1.50/Oracle_VM_VirtualBox_Extension_Pack-6.1.50.vbox-extpack |
| Ubuntu Server | 24.04 LTS, 64-bit | https://ubuntu.com/download/server (file `ubuntu-24.04.x-live-server-amd64.iso`) |
| PuTTY, to reach the panel | latest | https://www.chiark.greenend.org.uk/~sgtatham/putty/latest.html |

VirtualBox 6.1 has had no security updates since January 2024. Use it only for
this machine and keep the virtual machine's ports closed to the outside, as the
steps below do.

## Steps

### 1. Install VirtualBox and the Extension Pack

1. Run `VirtualBox-6.1.50-161033-Win.exe` and accept the defaults. Allow the USB
   and network drivers when Windows asks.
2. Double-click the `.vbox-extpack` file. VirtualBox opens and installs it.

### 2. Make the virtual machine

1. In VirtualBox, choose **New**. Name: `Caspian`. Type: **Linux**. Version:
   **Ubuntu (64-bit)**. If only 32-bit versions are listed, switch virtualisation
   on in the BIOS first.
2. Memory: **2048 MB**. Disk: **25 GB**, VDI, dynamically allocated.
3. Open **Settings** for the new machine:
   - **Storage**: put the Ubuntu Server ISO in the empty optical drive.
   - **Network**, Adapter 1: **NAT**. Open **Advanced**, **Port Forwarding**, and
     add one rule: name `ssh`, protocol TCP, host IP `127.0.0.1`, host port
     `2222`, guest port `22`. With host IP `127.0.0.1`, only this PC can use the
     forward.
   - **USB**: choose **USB 2.0 (EHCI) Controller**. Plug in the USB Wi-Fi
     adapter, press the plus icon, and choose the adapter. This filter gives the
     adapter to the virtual machine every time it runs.

### 3. Install Ubuntu Server

1. Start the virtual machine. Install Ubuntu Server with the defaults.
2. When the installer asks, tick **Install OpenSSH server**.
3. Choose a user name and a strong password. Restart when it finishes and remove
   the ISO if VirtualBox asks.

### 4. Install Caspian

1. On Windows, open PuTTY. Host name: `127.0.0.1`. Port: `2222`.
2. Before you press Open, go to **Connection**, **SSH**, **Tunnels**. Source port:
   `8088`. Destination: `127.0.0.1:8088`. Press **Add**.
3. Go back to **Session**, type `Caspian` under **Saved Sessions** and press
   **Save**. You use this saved session every day.
4. Press **Open** and log in with your Ubuntu user.
5. Run the normal Linux installer:

       sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

6. Write down the panel password that the installer prints.
7. Run `caspian check`. It must show that the USB adapter can make a hotspot. If
   it cannot, the adapter's chip is the problem, not Windows 7.

### 5. Open the panel and switch on

1. Keep the PuTTY window open. In a browser on Windows, open
   `http://127.0.0.1:8088/` and log in with the panel password.
2. Paste your config, set the hotspot name and password, and press the switch.
3. Join the new Wi-Fi network from your phone, TV or any other device. Each
   device uses the one config. Nothing is installed on them.

The panel now also answers on the hotspot, so after the first setup you can use
it from any phone that has joined.

## Every day

1. Start the virtual machine in VirtualBox. **Headless start** runs it without a
   window.
2. Caspian comes back switched off after every restart. This is by design: a
   restart never turns the tunnel on by itself.
3. Open the saved `Caspian` session in PuTTY, open `http://127.0.0.1:8088/`, and
   press the switch.

To make Windows start the virtual machine when you log in, put a shortcut to
`"C:\Program Files\Oracle\VirtualBox\VBoxManage.exe" startvm Caspian --type headless`
in the Windows Startup folder.

## Removing it

In the virtual machine: `sudo /usr/local/bin/caspian-uninstall`. Or delete the
whole virtual machine in VirtualBox, with **Delete all files**. Windows 7 itself
is not changed by any of these steps, apart from the VirtualBox install.

## Limits

- Not yet tested end to end on Windows 7. See the note at the top.
- The hotspot is only as good as the USB adapter and its Linux driver.
- A USB Wi-Fi hotspot on Linux (arrangement B in the README) has modelled tests.
  The hardware record does not show that every USB adapter works.
- The virtual machine needs the PC to stay on.
