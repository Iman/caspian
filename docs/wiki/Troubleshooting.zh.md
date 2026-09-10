<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh) | [العربية](https://github.com/Iman/caspian/wiki/Troubleshooting.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Troubleshooting.tr) | [اردو](https://github.com/Iman/caspian/wiki/Troubleshooting.ur)

</div>

<a id="troubleshooting-for-home-users"></a>
# 家庭用户故障排除



首先从路由器到运行 Caspian 的计算机的以太网开始。使用该计算机的内置 Wi-Fi 作为热点，或使用 Linux 上兼容的 USB Wi-Fi 适配器。这为互联网连接和热点提供了单独的适配器。这是推荐的起始安排，而不是测量的速度保证。

<a id="wi-fi-country-is-not-set"></a>
## Wi-Fi 国家未设置

自动检测仍然是默认设置。将国家/地区留空，除非 Caspian 无法检测到它。如果您看到“Wi-Fi 国家/地区未设置”，请按照将 Wi-Fi 国家/地区设置为高级设置进行操作。输入计算机所在国家/地区的两个字母代码，保存，然后再次打开 Caspian。保存的选择在服务重新启动后仍然有效。清除该字段并保存以返回自动检测。 Caspian 不假定 IR 或从面板语言或代理服务器中选择国家/地区。

如果您安装的版本隐藏了“国家/地区”字段，则此恢复控制是问题 #3 更新的一部分。记录您的 Caspian 和 Linux 版本、适配器排列以及在更新的面板中选择国家/地区是否足够。不要发送私人配置或完整日志。手动 iw 命令有帮助的报告并不能证明该应用程序必须更改系统无线电设置。自动 iw reg 设置不属于此更改的一部分。


<a id="choose-your-connection"></a>
## 选择您的连接

在这些图中，[1] 是您的互联网路由器，[2] 是运行 Caspian 的计算机，[3] 是您的手机或其他设备。 ETH 表示以太网电缆。 USB 以太网适配器可连接互联网； USB Wi-Fi 适配器创建无线连接。他们从事不同的工作。

```text
A  [1] --ETH--> [2] --built-in Wi-Fi--> [3]
B  [1] --ETH--> [2] --USB Wi-Fi-------> [3]
C  [1] --Wi-Fi A--> [2] --Wi-Fi B----> [3]
D  [1] --Wi-Fi--> [2: one radio] --Wi-Fi--> [3]
```

| 互联网进入Caspian | 您的设备的热点 | Linux / 树莓派 | macOS |
|---|---|---|---|
| A、以太网 | 内置无线网络 | 当驱动程序可以创建热点时支持 | 支持的安排 |
| B、以太网 | 外部 USB 无线网络 | 需要具有接入点 (AP) 支持的 Linux 驱动程序 | Caspian 不支持作为热点 |
| C. Wi-Fi 适配器 A | 独立 Wi-Fi 适配器 B | 需要适配器 B 上的 AP 支持 | 不支持外部 USB Wi-Fi 热点 |
| D、无线网络 | 相同的 Wi-Fi 无线电 | 条件：驱动必须允许站和AP同时存在；该频道可能被共享 | 内置收音机不支持 |

这些是当前代码可以计划或拒绝的安排。他们并不对每个适配器、操作系统更新或笔记本电脑进行认证。可以加入家庭网络的 Wi-Fi 适配器可能仍然无法创建热点。 Linux USB配置有模型测试；现有的硬件记录并不能证明每个 USB 适配器都可以工作。在 macOS 上，使用以太网和内置 Wi-Fi 作为记录的路径。插入 USB Wi-Fi 并不会消除该限制。

<a id="connect-first-then-start"></a>
## 先连接，再启动

1. 在运行 Caspian 的计算机上工作，而不仅仅是在连接到其热点的手机上工作。停止热点会断开该电话的连接。

2. 保持凯斯宾电源关闭。将以太网电缆连接到工作路由器 LAN 端口和计算机。在打开应用程序之前连接任何 USB 以太网或支持的 USB Wi-Fi 适配器。

3. 检查计算机的网络设置：必须连接以太网。如果还连接了 Wi-Fi，仅工作网站并不能证明电缆可以承载互联网。断开家庭Wi-Fi网络并再次检查；保持 Wi-Fi 无线电可用于热点。

4. 在 Caspian 仍处于关闭状态的情况下，打开通常在此连接上运行的网站。如果失败，请先修复电缆、路由器连接或网络登录。 Caspian 需要现有的互联网连接。

5. 打开 Caspian 及其 Web 面板。在计算机本身上，如果面板服务正在运行，请使用 http://127.0.0.1:8088/。电话上的此地址指的是电话，而不是 Caspian 计算机。

6. 检查面板的互联网连接和热点适配器选择。从自动选择开始。如果 Caspian 选择了错误的连接，请选择以太网作为互联网，并选择预期的 Wi-Fi 适配器作为热点。 USB 适配器的名称各不相同；不要从其他人的指南中复制接口名称。

7. 保存您的代理配置和热点名称/密码。对于无法看到 5 GHz 的设备，从 2.4 GHz 频段开始。使用您的实际国家/地区代码并让频道保持自动，除非您有理由更改它。

8. 打开 Caspian 一次并等待结果。 Caspian Control 说“就绪”意味着其服务已响应；检查网络面板的隧道和热点状态。在一部手机上加入新的 Wi-Fi 网络，然后测试网站。

<a id="if-you-changed-a-cable-or-adapter"></a>
## 如果您更换了电缆或适配器

停止 Caspian，更改连接，并在重新开始之前重复上面的互联网检查。关闭浏览器或控制窗口并不一定会停止后台服务。重复单击不会修复不受支持的适配器。

在 Mac 上，打开 Caspian Control，选择“高级选项”，然后选择“重新启动服务”。等待结果，再次打开面板，如果 Caspian 关闭，则将其打开。重新启动会中断连接的设备并可能关闭面板。它保留保存的代理和热点设置。

在使用标准 systemd 安装程序安装的 Linux 上，以下命令将重新启动两个 Caspian 服务。使用本地终端或独立连接在 Caspian 计算机上运行它，该连接将在热点停止后继续存在。它不是适用于 macOS、Windows 或没有 systemd 的容器的命令。

```bash
sudo systemctl restart caspian.service caspian-panel.service
```

命令完成后，重新打开面板并根据需要打开 Caspian。如果服务仍然失败，请保留错误以用于下面的报告。避免重复重启；不要删除您的配置或禁用防火墙以使错误消失。在 Web 面板中，高级 > 将其放回并重新启动尝试使用保存的设置进行网络恢复；这与重新启动服务不同，设备可能会断开连接。

<a id="find-the-symptom"></a>
## 找到症状

| 你所看到的 | 接下来要检查什么 |
|---|---|
| 启动 Caspian 之前没有互联网 | 尝试使用其他电缆或路由器 LAN 端口。确认操作系统中以太网已连接。在 Caspian 关闭时完成任何网络登录。 |
| 唯一的 Wi-Fi 适配器已投入使用 | 在 Linux 上，使用以太网或单独的支持 AP 的适配器。一无线电 Wi-Fi 至 Wi-Fi 需要驱动程序支持，并且可能共享一个通道。在 macOS 上，使用以太网内置 Wi-Fi。 |
| 没有支持热点的适配器/适配器丢失 | 加入 Wi-Fi 并不能证明 AP 支持。在 Linux 上，检查适配器的 Linux 驱动程序和 AP 功能。在 Mac 上，外部 USB Wi-Fi 适配器不能成为 Caspian 的热点。 |
| 适配器繁忙或热点无法启动 | 阻止凯斯宾。断开目标热点适配器与其他网络的连接，而无需断开互联网适配器的连接。停止您启动的任何其他热点。关闭 Caspian 后，检查是否存在竞争 VPN，然后重试。 |
| 手机看不到热点 | 确认网络面板显示热点正在运行。靠近一点，尝试 2.4 GHz，然后检查国家/地区设置。固定频道跟随传入的 Wi-Fi；仅更改热点通道无法覆盖它。 |
| 手机可以看到 Wi-Fi 但无法加入 | 使用热点密码，而不是面板密码。重命名或更改密码后，请忘记旧保存的 Wi-Fi 条目，然后重新加入。如果一直停留在获取地址上，则重启Caspian一次，返回则报错。 |
| 手机已连接，但页面打不开 | 如果您打算允许流量，请选中流量切断并恢复流量。读取隧道错误。检查计算机的日期和时间。可读的配置可能指向不可用的服务器；询问您的提供商它是否仍然有效。测试时请暂时关闭手机移动数据，以免测试连接错误。 |
| 在 Control 中就绪，但在 Web 面板中呈红色 | 就绪确认后台服务响应。 Web 面板报告隧道和热点。记录其确切消息，而不是重复重新安装。 |
| 停止或重新启动后面板消失 | 服务运行后，从位于 http://127.0.0.1:8088/ 的 Caspian 计算机重新连接。当热点停止时，手机就会失去到面板的路由。默认情况下，本地网络访问处于关闭状态。 |
| 睡眠、拔出扩展坞或在网络之间移动后出现故障 | 唤醒计算机，重新连接电缆和适配器，在 Caspian 关闭的情况下确认互联网，然后重新启动。当其他设备依赖其热点时，保持主机处于唤醒状态。 |
| macOS 阻止应用程序 | 请遵循 macOS 安装指南。未经验证的开发人员警告和命名恶意软件检测需要不同的处理。请勿绕过名为 Trojan 或其他恶意软件的警报。 |

<a id="check-the-configuration-format"></a>
## 检查配置格式

Caspian 接受 VLESS、VMess、Shadowsocks、SOCKS、Trojan 和 Hysteria2 链接，包括 hy2 别名。它还接受受支持的 Clash/Clash.Meta YAML、Xray JSON、链接列表和 base64 订阅内容。它使用您选择的列表中的任何条目。订阅地址可以保存在配置旁边，并在您通过隧道按下按钮时刷新。向您的提供商询问实际支持的配置，而不是帐户密码或网页链接。

支持的传输名称包括 raw/tcp、ws、grpc、httpupgrade、xhttp/splithttp 和 kcp/mkcp。协议、传输和安全设置必须兼容；并非所有组合都有效。不支持 TUIC、WireGuard、SSR、AnyTLS 和 Hysteria v1 链接。不要重命名不受支持的协议以使其通过验证。有关限制和测试证据，请参阅方案指南。

<a id="ask-for-help-without-sharing-secrets"></a>
## 寻求帮助而不分享秘密

使用错误报告表并告诉我们：Caspian 版本、操作系统/发行版和版本、以太网到 Wi-Fi 或 Wi-Fi 到 Wi-Fi 的安排、每个适配器是内置的还是外部的、启动前互联网是否工作、确切的错误以及已经尝试过的步骤。如果您知道适配器芯片组或驱动程序名称，那么它会很有用；不包括序列号。

请勿发布代理链接、订阅内容、配置文件、密码、密钥、二维码、公共 IP 地址、家庭 Wi-Fi 名称、MAC/BSSID 地址、个人主机名或未经审核的日志/屏幕截图。复制短错误并删除标识符。请勿将面板暴露在互联网上或转发路由器端口以获取支持。

<a id="known-limitations-and-evidence"></a>
## 已知的限制和证据

缺陷寄存器记录安全和恢复差距。这些故障排除步骤不会关闭它们。例如，没有定期检查来恢复被其他程序删除的防火墙规则集。避免同时运行其他网络共享工具。下面的拓扑测试验证了受控输入的规划和拒绝；它们不是对您的硬件的新测试。

- [Installation](https://github.com/Iman/caspian/wiki/Installation.zh)
- [协议详情](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh)
- [报告问题](https://github.com/Iman/caspian/issues/new?template=bug_report.yml)
- [已知缺陷](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md)
- [代码和测试证据](https://github.com/Iman/caspian/blob/main/internal/netcfg/plan_test.go)
- [macOS：以太网/Wi-Fi](https://support.apple.com/en-ie/guide/mac-help/mchlp1540/mac)



<!-- Caspian guide navigation -->

Caspian指南：[设置和支持的协议](https://github.com/Iman/caspian/wiki/Home.zh)·[用于 DPI 规避的 SNI 欺骗：设置和限制](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh)。


<!-- English-source-sha256: d78e0c5791a008273b814aa9a9f9c4e6ba14c9875fd74ecd8a734c8e600f7ed4 -->
