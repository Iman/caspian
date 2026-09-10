<!-- wiki-navigation:start -->
<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/SNI-Spoofing) · [فارسی](https://github.com/Iman/caspian/wiki/SNI-Spoofing.fa) · [Русский](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ru) · [**简体中文**](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh) · [العربية](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ar) · [Türkçe](https://github.com/Iman/caspian/wiki/SNI-Spoofing.tr) · [اردو](https://github.com/Iman/caspian/wiki/SNI-Spoofing.ur)

</div>

<div dir="ltr" lang="zh">

[Caspian 文档](https://github.com/Iman/caspian/wiki/Home.zh) · [故障排除](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

</div>
<!-- wiki-navigation:end -->

<a id="caspian-sni-spoofing-and-tls-splitting-for-dpi-circumvention"></a>
# Caspian SNI 欺骗和 TLS 分裂以规避 DPI

本指南涵盖了 `feature/sni` 中可选的深度数据包检测 (DPI) 规避，而不是当前发布的安装程序。

SNI 是 TLS 问候语中的服务器名称。
此功能会在真正的代理流之前发送带有欺骗名称的额外问候语。
导入的配置中的真实 TLS 或 REALITY 名称保持不变。
导入的 `sni` 值不会自动启用欺骗。

<a id="set-a-spoof-name"></a>
## 设置一个恶搞名称

1. 在面板中保存您的代理配置。
2. 在仪表板上打开 **DPI 规避（可选）**。
3. 输入域名，例如本地测试的`cover.example.invalid`。
4. 保存设置。

使用适合您实际网络的域名；示例域无法解析。
如果 Caspian 已打开，则保存会将其与新设置重新连接。
要禁用欺骗，请清除该字段并保存。
替换配置会清除欺骗名称和两个拆分设置。
选择另一个条目或刷新订阅会保留它。

<a id="independent-tcp-split-and-tls-record-split"></a>
## 独立的 TCP 分割和 TLS 记录分割

在保存的配置旁边打开 **DPI 规避（可选）**：

- **假服务器名称**：留空以关闭假 SNI。
- **TCP 分割**：在 SNI 主机名中间附近的两次写入中发送初始 TLS 问候语。
- **TLS记录分割**：在该位置分割一条TLS问候语记录，而不改变握手内容。

每个选项都是独立的；您可以将它们组合起来或将所有三个都保留。
如果没有 SNI 扩展，则拆分将使用握手类型之后的位置。
TCP 分割是尽力而为：单独的写入并不能保证每个操作系统和网络上的单独数据包。
TLS 记录分割会更改记录边界，并且在某些服务器或 CDN 上可能会失败。
目前，这两种拆分选项都需要通过支持的 IPv4 TCP 传输使用 VLESS、VMess 或 Trojan 的普通 TLS。
REALITY 拆分未启用； REALITY仍然可以单独使用假SNI。
仅分割最初的ClientHello；后续应用流量正常转发。
仅拆分模式不需要 WinDivert、数据包套接字或 BPF 访问。
当还选择假 SNI 时，其数据包确认必须在发送任何真实问候语之前完成。
格式错误、不完整、过大或停滞的问候语会关闭连接；不重试会静默删除已启用的选项。
保存重新连接活动隧道；配置替换会清除所有三个选择。

状态字段为 `spoof_sni`、`tcp_split` 和 `tls_record_split`。
升级版本 4 文件会保留其假名并关闭两个拆分设置。

<a id="state-compatibility"></a>
## 状态兼容性

该分支编写状态模式版本 5。
较旧的版本拒绝此状态文件以防止数据丢失。
如果您需要返回到较旧的版本，请保留升级前的状态备份。

<a id="limits"></a>
## 限制

此版本支持基于 IPv4 TCP 的 VLESS、VMess 和 Trojan，包括 WebSocket、HTTPUpgrade 和 gRPC。
它不支持 Hysteria2、QUIC、SOCKS、Shadowsocks、XHTTP 或纯 IPv6 服务器。
SOCKS 可以协商此 TCP 转发器无法覆盖的单独 UDP 端点。
本机 Shadowsocks UDP（包括隧道 DNS 流量）无法使用此纯 TCP 转发器。
转发器从检测到的服务器地址中选择第一个可用的 IPv4 地址。
如果确认失败，则关闭连接而不转发真实流。
它不会退回到普通连接。

Windows x64 上的假 SNI 需要本地安装程序版本中包含的 WinDivert 文件。
Windows ARM64无法使用假SNI；仅分割模式不使用数据包后端。
Linux 上的假 SNI 需要数据包套接字权限； macOS 需要通过以太网接口访问 BPF 设备。
特权 Caspian 服务拥有这些资源，并在停止时关闭它们。

Windows环回测试证明服务器接收到的真实流没有变化。
它并不能证明欺骗行为会违反您的提供商的过滤。
Linux 和 macOS 版本还需要在其目标系统上进行实时数据包测试。

<a id="credits"></a>
## 制作人员

主要来源和想法是 [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)，已获得 GPL-3.0 许可。
参见 [第三方信用](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md)。

<a id="dpi-bypass-sni-spoofing-and-security"></a>
## DPI 绕过、SNI 欺骗和安全

<a id="is-caspian-dpi-safe"></a>
### Caspian DPI 安全吗？

不存在通用的 DPI 安全保证。可选的 SNI 欺骗尝试影响过滤系统读取初始 TCP 流量的方式。
它不会隐藏服务器 IP、流量或时间，并且提供商仍然可以阻止连接。
`feature/sni` 实现保留真实的 TLS 身份并拒绝失败的欺骗确认，而不是直接发送真实流。

<a id="does-caspian-include-goodbyedpi-or-zapret"></a>
### Caspian 是否包含 GoodbyeDPI 或 zapret？

第[GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI)和[zapret](https://github.com/bol-van/zapret)是未来可能策略的研究参考。
主要的 SNI 代码和想法来自 [patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing)，保留了 GPL 归属。
Caspian 没有捆绑这些其他项目，也没有声称它们的作者认可它。

<a id="does-a-config-with-sni-enable-dpi-bypass-automatically"></a>
### 具有 SNI 的配置是否会自动启用 DPI 绕过？

不会，导入的SNI才是真实的服务器身份。设置单独的可选欺骗名称以启用此模式。
启用之前请先阅读 [SNI 设置和限制](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh)。

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

[验证结果和剩余硬件测试](https://github.com/Iman/caspian/blob/feature/sni/docs/SNI-VALIDATION.md)。

<!-- English-source-sha256: 0c3a086c7127a1bec0deacf690b2aed7696a340ce94ec55379e1f233bee629f0 -->
