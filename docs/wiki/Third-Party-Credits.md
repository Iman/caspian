[English](https://github.com/Iman/caspian/wiki/Third-Party-Credits) · [فارسی](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) · [Русский](https://github.com/Iman/caspian/blob/feature/sni/README.ru.md) · [中文](https://github.com/Iman/caspian/blob/feature/sni/README.zh.md)

# Third-party code and credits

[NOTICE](https://github.com/Iman/caspian/blob/feature/sni/NOTICE) lists Caspian's linked libraries and distribution files.
Each upstream component keeps its own license.
Caspian's AGPL terms do not replace those upstream notices.

## SNI spoofing

The primary code and idea come from **[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)**.
Caspian reviewed commit `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`.
The ClientHello template and handshake algorithm inform `internal/snispoof`.
Caspian's changes add Go integration, validation, connection ownership, resource limits, rollback, and tests.

The derived source files retain GPL-3.0-only notices.
The complete [upstream GPL license](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/LICENSE.txt) and [attribution](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/README.md) remain in the repository.
GPLv3 section 13 permits combination with AGPLv3 code while each part retains its own terms.
Distributors must preserve notices, mark changes, and provide corresponding source under the applicable licenses.
Credit does not imply endorsement by the upstream authors.

Windows x64 uses **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)** by Basil (basil00) and contributors.
Caspian selects LGPL-3.0 from its dual license.
The installer contains the unmodified driver and DLL, complete license bundle, attribution, and source archive for v2.2.2.
See [WinDivert distribution details](https://github.com/Iman/caspian/blob/feature/sni/third_party/windivert/README.md).
Windows ARM64 does not include WinDivert and cannot use this SNI feature.

## Ideas and acknowledgements

Caspian also credits the authors and contributors of these projects for ideas and implementation comparisons that informed its SNI work.
Their code and executables are not bundled.

| Project | Reviewed revision | License and use |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | No license file found; comparison only |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0; strategy reference |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT; strategy and diagnostic reference |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | No top-level app license found; ideas only |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | Declares MIT; GPL template provenance needs clarification; no code copied |


## Other distributed components

The [share-link parser](https://github.com/Iman/caspian/blob/feature/sni/third_party/libxray-share/LICENSE) retains its MIT license.
Windows installers also contain official Wintun binaries and self-contained .NET helpers.
Their notices remain under [third_party](https://github.com/Iman/caspian/blob/feature/sni/third_party) and are installed beside the application.
The Go Wintun binding is an MIT runtime dependency on Windows.

<!-- Caspian guide navigation -->

Caspian guides: [setup and supported protocols](https://github.com/Iman/caspian/wiki/Home) · [SNI spoofing for DPI circumvention: setup and limits](https://github.com/Iman/caspian/wiki/SNI-Spoofing).
