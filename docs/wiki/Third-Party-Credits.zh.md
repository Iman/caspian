<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Third-Party-Credits) · [فارسی](https://github.com/Iman/caspian/wiki/Third-Party-Credits.fa) · [Русский](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ru) · [**简体中文**](https://github.com/Iman/caspian/wiki/Third-Party-Credits.zh) · [العربية](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/Third-Party-Credits.tr) · [اردو](https://github.com/Iman/caspian/wiki/Third-Party-Credits.ur)

</div>

<div dir="ltr" lang="zh">

[Caspian 文档](https://github.com/Iman/caspian/wiki/Home.zh) · [故障排除](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

</div>
<!-- wiki-navigation:end -->

<a id="third-party-code-and-credits"></a>
# 第三方代码和积分

[NOTICE](https://github.com/Iman/caspian/blob/feature/sni/NOTICE) 列出了 Caspian 的链接库和分发文件。
每个上游组件都有自己的许可证。
Caspian 的 AGPL 条款不会取代那些上游通知。

<a id="sni-spoofing"></a>
## SNI欺骗

主要代码和思想来自**[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)**。
Caspian 审查了提交 `13b78cf7e073f38d9cadcff542faf4a00b0a6de2`。
ClientHello 模板和握手算法通知 `internal/snispoof`。
Caspian 的更改添加了 Go 集成、验证、连接所有权、资源限制、回滚和测试。

派生的源文件保留 GPL-3.0-only 通知。
完整的 [上游 GPL 许可证](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/LICENSE.txt) 和 [署名信息](https://github.com/Iman/caspian/blob/feature/sni/third_party/sni-spoofing/README.md) 保留在存储库中。
GPLv3 第 13 节允许与 AGPLv3 代码组合，同时每个部分保留自己的条款。
分销商必须保留通知、标记更改并根据适用的许可提供相应的来源。
信用并不意味着上游作者的认可。

Windows x64 使用 Basil (basil00) 和贡献者的 **[WinDivert](https://github.com/basil00/WinDivert/tree/v2.2.2)**。
Caspian 从其双重许可证中选择 LGPL-3.0。
安装程序包含未经修改的驱动程序和 DLL、完整的许可证包、归属以及 v2.2.2 的源存档。
参见 [WinDivert 分发详细信息](https://github.com/Iman/caspian/blob/feature/sni/third_party/windivert/README.md)。
Windows ARM64 不包含 WinDivert，因此无法使用此 SNI 功能。

<a id="ideas-and-acknowledgements"></a>
## 想法和致谢

Caspian 还感谢这些项目的作者和贡献者的想法和实施比较，这些想法和实施比较为其 SNI 工作提供了信息。
他们的代码和可执行文件没有捆绑在一起。

| 项目 | 已审核修订 | 许可和使用 |
| --- | --- | --- |
| [selfishblackberry177/sni-spoof](https://github.com/selfishblackberry177/sni-spoof) | `a0d68c6627e0898cf13176e322420743f69d894f` | 未找到许可证文件；仅比较 |
| [ValdikSS/GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) | `f593a276f9ec753889f80208c6a7c5cf455df94a` | Apache-2.0；策略参考 |
| [bol-van/zapret](https://github.com/bol-van/zapret) | `87e058624c72863db53bdaf7fb6f16576dddb6ab` | MIT；策略和诊断参考 |
| [Floxu1/UAC-SNI-Spoofer-Android](https://github.com/Floxu1/UAC-SNI-Spoofer-Android) | `c68e350f5f9e9cff308c289db261d7c5d034efa2` | 未找到顶级应用许可证；仅想法 |
| [therealaleph/sni-spoofing-rust](https://github.com/therealaleph/sni-spoofing-rust) | `d2956025c31d96f0f0a341af4f1a8eda204857c7` | 声明 MIT； GPL 模板出处需要澄清；没有复制代码 |

<a id="other-distributed-components"></a>
## 其他分布式组件

[共享链接解析器](https://github.com/Iman/caspian/blob/feature/sni/third_party/libxray-share/LICENSE) 保留其 MIT 许可证。
Windows 安装程序还包含官方 Wintun 二进制文件和独立的 .NET 帮助程序。
他们的通知保留在 [third_party](https://github.com/Iman/caspian/blob/feature/sni/third_party) 下，并安装在应用程序旁边。
Go Wintun 绑定是 Windows 上的 MIT 运行时依赖项。

<!-- English-source-sha256: ceef83c2b7b0779eb04c1fa1c854f35aaf685978f4d17f439ddaa2fd2d01847c -->
