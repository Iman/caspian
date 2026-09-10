<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) · [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) · [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) · [**简体中文**](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh) · [العربية](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Licence-and-Credits.tr) · [اردو](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ur)

</div>

<div dir="ltr" lang="zh">

[Caspian 文档](https://github.com/Iman/caspian/wiki/Home.zh) · [故障排除](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

</div>
<!-- wiki-navigation:end -->

<a id="licence-and-credits"></a>
# 许可证和学分

> 本指南来自现有的自述文件。其测量结果保留其原始日期；此文档移动不会报告新的测试运行。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="licence"></a>
## 许可证

AGPL-3.0-or-later，第 7 条下有三个附加条款。所有三个条款均属于
第 7 条允许且不限制您可以使用该软件执行的操作：
保留版权声明、归属权以及对
任何用户界面中的 Caspian 项目；如果您将您的版本标记为已更改
修改它；并且不要使用作者或项目的名称进行宣传，
其中包括以这些名义募集捐款、赞助或赠款。的
全文在 [`LICENSE`](https://github.com/Iman/caspian/blob/main/LICENSE) 中，术语在 [`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) 中。

第三个术语限制了 NAMES 的使用，仅此而已。您仍然可以自由地
根据 AGPL 运行、研究、修改和重新分发软件，以供任何人使用
目的包括商业目的。你不可以做的就是筹集资金
作者姓名。

AGPL 而不是 GPL，因为该程序通常作为
其他人连接到的服务，第 13 条缩小了普通 GPL 的差距
叶子。不是许可，因为二进制文件静态链接
GPL-3.0-or-later 代码：`github.com/sagernet/sing` 和
`github.com/sagernet/sing-shadowsocks`，均通过X射线核心到达。所以
组合作品必须遵循 GPL 系列条款，并且 MIT 或 Apache-2.0 不适用
可用它。

<a id="built-on"></a>
## 建立在

Caspian 是围绕其他人的工作编写的少量代码。发动机是
xray-core，共享链接解析器是 XTLS 的。两个项目都不认可这一点
一；他们受到赞扬是因为这项工作是他们的。

| 项目 | 许可证 | 它在这里做什么 |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | 代理引擎在进程内链接而不是作为单独的程序运行 |
| [libXray](https://github.com/XTLS/libXray) | MIT | 共享链接解析器，以 `third_party/libxray-share/` 名义出售 |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | TLS迷彩运输机 |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | TLS 指纹模仿 |
| [quic-go](https://github.com/apernet/quic-go) | MIT | QUIC 堆栈 Hysteria2 运行于 |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | TUN 入站使用的用户空间网络堆栈 |
| [sing](https://github.com/sagernet/sing) 和 [sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022，以及该项目是 Copyleft 的原因 |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | 接口、地址和路线 |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | DNS 消息处理 |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | WebSocket 传输 |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | 后量子密钥交换 |
| [Wintun](https://www.wintun.net/) | Wintun 预构建二进制文件许可证 | Windows 上签名的 `wintun.dll` 隧道驱动程序 |
| [.NET 运行时和 Windows 窗体](https://github.com/dotnet/runtime) | MIT | 独立的 Windows 帮助程序和托盘应用程序运行时 |
| `System.ServiceProcess.ServiceController` | MIT | 来自 `CaspianControl.exe` 的 Windows 服务控制 |

Windows 安装包括 `wintun.dll`。 SNI 版本还包括 Windows x64 上的 WinDivert。
Caspian 分发官方签名的 Wintun 0.14.1 二进制文件，未进行任何更改。
其许可证位于
[`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt) 并复制到
安装期间的 `C:\Program Files\Caspian\WINTUN-LICENSE.txt`。

`caspian-tethering.exe` 和 `CaspianControl.exe` 是独立的 .NET
程序。它们的 .NET 组件位于可执行文件内，而不是旁边
它们作为额外的 DLL。 .NET 许可证和声明位于 `third_party/dotnet/` 中。
Windows SDK 参考包是构建输入，不随
凯斯宾。

还需要

`hostapd`, `dnsmasq`, `nftables`, `iw`

和

`iproute2`

于
机。这些程序作为单独的程序运行而不是链接在一起，因此它们的
许可证不会影响这一点，但如果没有许可证，该设备就一无是处。

[`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) 携带完整记录：二进制文件中的每个模块、许可证读取
来自其自己的许可证文件和兼容性推理。

<!-- SNI upstream credits -->

SNI 欺骗来源：[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0)，以及 Windows x64 上的 WinDivert (LGPL-3.0)。
[第三方许可证、源版本和积分](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md)。

<a id="sni-idea-acknowledgements"></a>
## SNI 想法致谢

Caspian 还感谢这些项目的作者和贡献者的想法和实施比较，这些想法和实施比较为其 SNI 工作提供了信息。
他们的代码和可执行文件没有捆绑在一起。

- [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof)：去SNI转发对比。
- [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust)：Rust SNI 实现和功能比较。
- [bol-van/zapret](https://github.com/bol-van/zapret)：DPI规避策略和诊断。
- [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI)：DPI规避策略。
- [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android)：Android SNI集成思路。

改编后的代码保留其上游许可和通知。
想法确认并不授予复制代码的许可或暗示认可。
请参阅 [第三方信用](https://github.com/Iman/caspian/wiki/Third-Party-Credits.zh) 了解已审查的版本、许可证和使用范围。

[WinDivert — 巴兹尔 (basil00)](https://github.com/basil00/WinDivert/tree/v2.2.2)：Windows x64、LGPL-3.0。

<!-- English-source-sha256: 626ed23e3eab55bb351c5d12fc420fb06471c2cf6e2e08fb42edb6a8062571bf -->
