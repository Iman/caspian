<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh) | [العربية](https://github.com/Iman/caspian/wiki/Troubleshooting.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Troubleshooting.tr) | [اردو](https://github.com/Iman/caspian/wiki/Troubleshooting.ur)

</div>

# Troubleshooting for home users



Start with Ethernet from your router to the computer running Caspian. Use that computer's built-in Wi-Fi for the hotspot, or a compatible USB Wi-Fi adapter on Linux. This gives the internet connection and hotspot separate adapters. It is the recommended starting arrangement, not a measured speed guarantee.

## Wi-Fi country is not set

Automatic detection remains the default. Leave Country blank unless Caspian cannot detect it. If you see “Wi-Fi country is not set”, follow Set Wi-Fi country to Advanced settings. Enter the two-letter code for the country where the computer is located, save, then switch Caspian on again. The saved choice survives a service restart. Clear the field and save to return to automatic detection. Caspian does not assume IR or choose a country from the panel language or proxy server.

If your installed version hides the Country field, this recovery control is part of the issue #3 update. Record your Caspian and Linux versions, adapter arrangement, and whether choosing Country in the updated panel is enough. Do not send private configuration or complete logs. A report that a manual iw command helped does not prove that the app must change the system radio settings. Automatic iw reg set is not part of this change.


## Choose your connection

In these diagrams, [1] is your internet router, [2] is the computer running Caspian, and [3] is your phone or another device. ETH means an Ethernet cable. A USB Ethernet adapter brings internet in; a USB Wi-Fi adapter creates a wireless connection. They do different jobs.

```text
A  [1] --ETH--> [2] --built-in Wi-Fi--> [3]
B  [1] --ETH--> [2] --USB Wi-Fi-------> [3]
C  [1] --Wi-Fi A--> [2] --Wi-Fi B----> [3]
D  [1] --Wi-Fi--> [2: one radio] --Wi-Fi--> [3]
```

| Internet into Caspian | Hotspot to your devices | Linux / Raspberry Pi | macOS |
|---|---|---|---|
| A. Ethernet | Built-in Wi-Fi | Supported when the driver can create a hotspot | Supported arrangement |
| B. Ethernet | External USB Wi-Fi | Requires a Linux driver with access point (AP) support | Not supported as the hotspot by Caspian |
| C. Wi-Fi adapter A | Separate Wi-Fi adapter B | Requires AP support on adapter B | An external USB Wi-Fi hotspot is not supported |
| D. Wi-Fi | The same Wi-Fi radio | Conditional: the driver must allow a station and AP together; the channel may be shared | Not supported on the built-in radio |

These are the arrangements the current code can plan or refuse. They do not certify every adapter, OS update, or laptop. A Wi-Fi adapter that can join your home network may still be unable to create a hotspot. Linux USB arrangements have modelled tests; the existing hardware record does not establish that every USB adapter works. On macOS, use Ethernet and built-in Wi-Fi for the documented path. Plugging in USB Wi-Fi does not remove that restriction.

## Connect first, then start

1. Work on the computer running Caspian, not only on a phone connected to its hotspot. Stopping the hotspot disconnects that phone.

2. Keep Caspian switched off. Connect the Ethernet cable to a working router LAN port and to the computer. Attach any USB Ethernet or supported USB Wi-Fi adapter before opening the app.

3. Check the computer's Network settings: Ethernet must be connected. If Wi-Fi is also connected, a working website alone does not prove the cable carries internet. Disconnect from the home Wi-Fi network and check again; keep the Wi-Fi radio available for the hotspot.

4. With Caspian still off, open a website that normally works on this connection. If it fails, fix the cable, router connection, or network sign-in first. Caspian needs an existing internet connection.

5. Open Caspian and its web panel. On the computer itself, use http://127.0.0.1:8088/ if the panel service is running. This address on a phone refers to the phone, not the Caspian computer.

6. Check the panel's internet connection and hotspot adapter choices. Start with automatic selection. If Caspian picks the wrong connection, select Ethernet for internet and the intended Wi-Fi adapter for the hotspot. A USB adapter's name varies; do not copy an interface name from somebody else's guide.

7. Save your proxy configuration and hotspot name/password. Start with the 2.4 GHz band for devices that cannot see 5 GHz. Use your actual country code and leave the channel automatic unless you have a reason to change it.

8. Switch Caspian on once and wait for the result. Caspian Control saying Ready means its services answer; check the web panel for the tunnel and hotspot state. Join the new Wi-Fi network on one phone, then test a website.

## If you changed a cable or adapter

Stop Caspian, make the connection change, and repeat the internet check above before starting again. Closing the browser or the control window does not necessarily stop the background services. Repeated clicks will not repair an unsupported adapter.

On a Mac, open Caspian Control, choose Advanced options, then Restart services. Wait for the result, open the panel again, and switch Caspian on if it is off. A restart interrupts connected devices and may close the panel. It keeps saved proxy and hotspot settings.

On Linux installed with the standard systemd installer, the following command restarts both Caspian services. Run it on the Caspian computer, using a local terminal or a separate connection that will survive the hotspot stopping. It is not a command for macOS, Windows, or a container without systemd.

```bash
sudo systemctl restart caspian.service caspian-panel.service
```

After the command finishes, reopen the panel and switch Caspian on if needed. If a service still fails, keep the error for the report below. Avoid repeated restarts; do not delete your configuration or disable the firewall to make the error disappear. In the web panel, Advanced > Put it back and start again attempts network recovery with saved settings; this is different from restarting the services, and devices may disconnect.

## Find the symptom

| What you see | What to check next |
|---|---|
| No internet before starting Caspian | Try another cable or router LAN port. Confirm Ethernet has connected in the OS. Complete any network sign-in while Caspian is off. |
| The only Wi-Fi adapter is already in use | On Linux, use Ethernet or a separate AP-capable adapter. One-radio Wi-Fi-to-Wi-Fi needs driver support and may share one channel. On macOS, use Ethernet to built-in Wi-Fi. |
| No hotspot-capable adapter / adapter missing | Joining Wi-Fi is not proof of AP support. On Linux, check the adapter's Linux driver and AP capability. On a Mac, an external USB Wi-Fi adapter cannot be Caspian's hotspot. |
| Adapter busy or hotspot will not start | Stop Caspian. Disconnect the intended hotspot adapter from its other network, without disconnecting your internet adapter. Stop any other hotspot you started. With Caspian off, check for a competing VPN, then try again. |
| Phone cannot see the hotspot | Confirm the web panel says the hotspot is running. Move closer, try 2.4 GHz, and check the country setting. A pinned channel follows the incoming Wi-Fi; changing the hotspot channel alone cannot override it. |
| Phone sees Wi-Fi but cannot join | Use the hotspot password, not the panel password. Forget the old saved Wi-Fi entry after renaming or changing the password, then join again. If it stays on obtaining an address, restart Caspian once and report the error if it returns. |
| Phone joined, but pages do not open | Check Traffic cut and resume it if you meant to allow traffic. Read the tunnel error. Check the computer's date and time. A readable configuration may point to an unavailable server; ask your provider whether it still works. Test with phone mobile data temporarily off to avoid testing the wrong connection. |
| Ready in Control, but red in the web panel | Ready confirms the background services respond. The web panel reports the tunnel and hotspot. Record its exact message instead of repeatedly reinstalling. |
| Panel disappeared after stopping or restarting | Reconnect from the Caspian computer at http://127.0.0.1:8088/ once services are running. A phone loses its route to the panel when the hotspot stops. Local-network access is off by default. |
| Failure after sleep, unplugging a dock, or moving between networks | Wake the computer, reconnect the cable and adapters, confirm internet with Caspian off, then start again. Keep the host awake while other devices depend on its hotspot. |
| macOS blocks the application | Follow the macOS installation guide. An unverified-developer warning and a named malware detection need different handling. Do not bypass an alert that names a Trojan or other malware. |

## Check the configuration format

Caspian accepts VLESS, VMess, Shadowsocks, SOCKS, Trojan, and Hysteria2 links, including the hy2 alias. It also accepts supported Clash/Clash.Meta YAML, Xray JSON, lists of links, and base64 subscription content. It uses whichever entry of a list you choose. A subscription address can be saved beside the config and refreshed when you press the button, through the tunnel. Ask your provider for the actual supported configuration, not an account password or a web page link.

Supported transport names include raw/tcp, ws, grpc, httpupgrade, xhttp/splithttp, and kcp/mkcp. Protocol, transport, and security settings must be compatible; not every combination works. TUIC, WireGuard, SSR, AnyTLS, and Hysteria v1 links are not supported. Do not rename an unsupported protocol to make it pass validation. See the protocol guide for restrictions and test evidence.

## Ask for help without sharing secrets

Use the bug report form and tell us: Caspian version, OS/distribution and version, Ethernet-to-Wi-Fi or Wi-Fi-to-Wi-Fi arrangement, whether each adapter is built in or external, whether internet worked before starting, the exact error, and the steps already tried. The adapter chipset or driver name is useful if you know it; do not include a serial number.

Do not post proxy links, subscription contents, configuration files, passwords, keys, QR codes, public IP addresses, home Wi-Fi names, MAC/BSSID addresses, personal hostnames, or unreviewed logs/screenshots. Copy the short error and remove identifiers. Do not expose the panel to the internet or forward a router port for support.

## Known limitations and evidence

The defect register records security and recovery gaps. These troubleshooting steps do not close them. For example, there is no periodic check that restores a firewall ruleset removed by another program. Avoid running another network-sharing tool at the same time. The topology tests below verify planning and refusals with controlled inputs; they are not a fresh test on your hardware.

- [Installation](https://github.com/Iman/caspian/wiki/Installation)
- [Protocol details](https://github.com/Iman/caspian/wiki/Protocols-and-Transports)
- [Report a problem](https://github.com/Iman/caspian/issues/new?template=bug_report.yml)
- [Known defects](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md)
- [Code and test evidence](https://github.com/Iman/caspian/blob/main/internal/netcfg/plan_test.go)
- [macOS: Ethernet / Wi-Fi](https://support.apple.com/en-ie/guide/mac-help/mchlp1540/mac)



<!-- Caspian guide navigation -->

Caspian guides: [setup and supported protocols](https://github.com/Iman/caspian/wiki/Home) · [SNI spoofing for DPI circumvention: setup and limits](https://github.com/Iman/caspian/wiki/SNI-Spoofing).
