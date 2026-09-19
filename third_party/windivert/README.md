# WinDivert distribution

Caspian's Windows x64 installer includes the unmodified WinDivert 2.2.2-A
`WinDivert.dll` and `WinDivert64.sys` by Basil (basil00) and contributors.
Caspian selects the LGPL-3.0 option of the upstream dual license.
The complete upstream license bundle is preserved in `LICENSE.txt`.

- Project and source: https://github.com/basil00/WinDivert/tree/v2.2.2
- Binary archive: https://github.com/basil00/WinDivert/releases/download/v2.2.2/WinDivert-2.2.2-A.zip
- SHA256: `63cb41763bb4b20f600b6de04e991a9c2be73279e317d4d82f237b150c5f3f15`

The library is loaded dynamically beside `caspian.exe`. Users can replace it
with a compatible modified library; Caspian imposes no restriction on debugging
those changes. The Windows driver must meet the operating system's signing rules.
The installer also includes `WinDivert-source.zip` from the exact v2.2.2 tag.
Its SHA256 is `65ec79c9e6afa99f648a3f4d1f6db794640b40d0b65bd438770ea503ee14ecb7`.
Distributors must provide the corresponding library source as required by LGPL-3.0,
including when they mirror the binaries; an upstream link is not a substitute
for their own source-distribution obligations.
WinDivert is used only when the user enables SNI spoofing. Windows ARM64 does not
include this x64 driver and cannot enable this feature.
