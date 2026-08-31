# Security policy

[🇮🇷 فارسی](SECURITY.fa.md) | 🇬🇧 **English** | [🇷🇺 Русский](README.ru.md) | [🇨🇳 中文](README.zh.md)

> ### [فارسی: سیاست امنیتی](SECURITY.fa.md)
>
> **[Read this in Persian](SECURITY.fa.md)**

## Reporting a vulnerability

Report privately, through the contact on the project's own website. Do not open
a public issue for a vulnerability.

This is censorship-circumvention software. A public report is readable by the
people the tool exists to protect users from, and it is readable by them
immediately, while a fix is still being written and long before anybody has
updated. That asymmetry is the reason for private reporting here, and it is
sharper than it would be for ordinary software.

Please include what you did, what happened, and what you expected. A way to
reproduce it is worth more than a description. If you have a packet capture,
say so before sending one: captures of real traffic are exactly the data this
project works to avoid collecting, and there is usually a way to demonstrate the
problem without it.

## What counts

Anything that lets a joined device's traffic leave the box untunnelled. Anything
that reveals to a network observer that the box is what it is. Anything that
exposes a proxy configuration, a key, a passphrase or a user identifier.
Anything that lets an unauthenticated request reach the privileged service.
Anything that lets a device on the hotspot reach the machine beyond the services
it is meant to have.

## What does not count, and why it is written down

The panel is authenticated by one password and is meant to be reachable from the
hotspot. A person already on your hotspot with your panel password is not a
vulnerability, they are the intended operator.

The box's own DNS resolution is outside the guarantee. The design says so, the
firewall permits it deliberately, and a report that the appliance itself
resolves names in the clear is describing a documented decision.

DNS over HTTPS from a client is carried and invisible. It is not distinguishable
from other HTTPS inside the tunnel. This is a limit of the design and is
recorded as one.

## What this project promises back

An acknowledgement, an honest assessment including when the answer is that it is
not a vulnerability, and credit if you want it.

What it does not promise is a bounty. There is no money here.

## Scope

This repository. Vulnerabilities in xray-core, in the Linux kernel, in hostapd or
in dnsmasq belong to their own projects, and reporting them here delays the fix.
