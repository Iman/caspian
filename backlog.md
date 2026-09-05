# Backlog

This file records deferred work that changes Caspian's security boundary. A
checked interim item is not evidence for the stronger item below it.

## macOS host traffic

- [x] Option 2 — interim system proxy: after Xray has started, point every
  enabled macOS network service's SOCKS setting at Caspian's loopback listener
  (`127.0.0.1:10808`). Read the previous endpoint, enabled state, authentication
  flag, and bypass domains before changing anything; journal their inverses;
  restore the effective setting on stop, failed start, and crash recovery. A
  previously configured endpoint is restored; a service that had no endpoint
  is disabled because `networksetup` has no endpoint-clear verb. Refuse to
  overwrite an authenticated proxy because macOS does not reveal the password
  needed to restore it.
- [ ] Option 1 — full-system tunnel: make traffic from the Mac itself use the
  configured tunnel even when an application ignores system proxy settings.
  This is not complete until DNS is tunnelled too. Use the supported macOS
  packet-tunnel/Network Extension path, or graduate `StrategySplitDefault` only
  after the same properties are proved on a live Mac.

Option 1 acceptance tests:

- A normal macOS application with no explicit proxy setting has the configured
  tunnel's public exit IP.
- TCP and UDP from applications that ignore the system proxy cannot use the
  physical uplink directly.
- System DNS and direct UDP/TCP port 53 are captured and resolved through the
  configured tunnel. No fallback resolver is reachable on the physical uplink.
- IPv6 is either carried by the tunnel or blocked fail-closed; it cannot bypass
  the IPv4 tunnel.
- Stopping or crashing Xray stops protected host traffic instead of exposing
  the physical connection, while the pinned route to the proxy server remains
  outside the tunnel and cannot loop.
- Stop, failed start, uninstall, and crash recovery restore routes, DNS, proxy,
  and Network Extension state exactly.
- The live test uses an ordinary proxy-unaware process and packet capture on the
  physical uplink. An explicit `curl --socks5-hostname` test is useful but does
  not satisfy this item by itself.

Reference: the current JavidGorz macOS app implements the same interim
system-SOCKS pattern. Its archived Packet Tunnel branch is research, not
production proof for Option 1.
