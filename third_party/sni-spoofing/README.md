# SNI spoofing reference attribution

The ClientHello template and handshake algorithm in `internal/snispoof` derive from
[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing),
commit `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`.
The original project uses GPL-3.0. Its license is in `LICENSE.txt`.
Caspian translates the packet layout into Go and adds validation, connection ownership,
resource limits, service integration, and tests.
The secondary Go repository was reviewed but its code was not copied.
