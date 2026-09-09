# 家庭用户故障排查

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh) | [العربية](https://github.com/Iman/caspian/wiki/Troubleshooting.ar) | [اردو](https://github.com/Iman/caspian/wiki/Troubleshooting.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Troubleshooting.tr)

</div>

建议先用以太网线连接路由器和运行 Caspian 的电脑，再用电脑内置 Wi-Fi 建立热点。Linux 也可以使用兼容的 USB Wi-Fi 适配器。这样，互联网接入和热点各用一个适配器。这是推荐的起步方案，并不是经过测速的性能保证。

## 未设置 Wi-Fi 国家或地区

默认仍为自动检测。如果 Caspian 能检测到国家或地区，请将 Country 留空。出现“Wi-Fi country is not set”时，点击 Set Wi-Fi country。在高级设置中输入电脑实际所在国家或地区的两字母代码，保存后重新开启 Caspian。重启服务后仍保留此选择。清空该字段并保存即可恢复自动检测。Caspian 不会默认使用 IR，也不会根据面板语言或代理服务器选择国家或地区。

如果已安装版本隐藏了 Country 字段，可编辑字段属于 issue #3 的更新。请提供 Caspian 和 Linux 版本、网卡连接方式，以及在更新后的面板中选择国家或地区是否有效。不要发送私人配置或完整日志。手动执行 iw 命令有效，并不能证明应用必须修改系统无线设置。本次更改不包含自动执行 iw reg set。


## 选择连接方式

图中 [1] 是互联网路由器，[2] 是运行 Caspian 的电脑，[3] 是手机或其他设备。ETH 表示以太网线。USB 以太网适配器负责接入互联网；USB Wi-Fi 适配器负责无线连接。两者用途不同。

```text
A  [1] --ETH--> [2] --built-in Wi-Fi--> [3]
B  [1] --ETH--> [2] --USB Wi-Fi-------> [3]
C  [1] --Wi-Fi A--> [2] --Wi-Fi B----> [3]
D  [1] --Wi-Fi--> [2: one radio] --Wi-Fi--> [3]
```

| Caspian 的互联网来源 | 供设备连接的热点 | Linux / Raspberry Pi | macOS |
|---|---|---|---|
| A. 以太网 | 内置 Wi-Fi | 驱动支持建立热点时可用 | 支持的连接方式 |
| B. 以太网 | 外置 USB Wi-Fi | 需要支持接入点（AP）模式的 Linux 驱动 | Caspian 不支持将其用作热点 |
| C. Wi-Fi 适配器 A | 独立的 Wi-Fi 适配器 B | 适配器 B 需要支持 AP | 不支持外置 USB Wi-Fi 热点 |
| D. Wi-Fi | 同一个 Wi-Fi 无线模块 | 有条件支持：驱动须允许客户端和 AP 同时运行，可能共用信道 | 不支持用内置无线模块这样连接 |

此表说明当前代码能够规划或拒绝的连接方式，不代表所有适配器、系统更新或笔记本电脑都经过验证。能连接家庭 Wi-Fi 的适配器，不一定能建立热点。Linux 的 USB 连接方案有模型测试，但现有硬件记录不能证明所有 USB 适配器都可用。在 Mac 上请使用文档规定的以太网接入和内置 Wi-Fi 热点。添加 USB Wi-Fi 无法解除这个限制。

## 先连接网络，再启动

1. 请在运行 Caspian 的电脑上操作，不要只用连接热点的手机。停止热点会断开手机连接。

2. 保持 Caspian 关闭。将网线连接到路由器正常工作的 LAN 端口和电脑。打开应用前，先插好 USB 以太网适配器或受支持的 USB Wi-Fi 适配器。

3. 在电脑的网络设置中确认以太网已连接。如果 Wi-Fi 也连接着，能打开网页并不能证明网线有互联网。先断开家庭 Wi-Fi 连接再测试，同时保留 Wi-Fi 无线模块供热点使用。

4. 在 Caspian 关闭时，打开一个平时能通过该网络访问的网站。如果打不开，先解决网线、路由器连接或网络登录问题。Caspian 需要已有的互联网连接。

5. 打开 Caspian 和网页面板。如果面板服务已运行，可在这台电脑上访问 http://127.0.0.1:8088/。在手机上，这个地址指的是手机本身，并不是 Caspian 电脑。

6. 检查面板中的互联网连接和热点适配器选择。先使用自动选择；如果选错，再将互联网设为以太网，将热点设为预期的 Wi-Fi 适配器。USB 适配器的接口名称各不相同，不要照抄别人的名称。

7. 保存代理配置、热点名称和密码。对于看不到 5 GHz 的设备，先用 2.4 GHz。选择实际所在国家的代码，没有明确原因时让信道保持自动。

8. 只开启一次 Caspian，然后等待结果。Caspian Control 显示 Ready 仅代表后台服务能响应；隧道和热点状态请看网页面板。先让一台手机加入新 Wi-Fi，再打开网站测试。

## 更换网线或适配器后

停止 Caspian，完成连接变更，再按上述方法确认互联网可用，然后重新启动。关闭浏览器或控制窗口不一定会停止后台服务。反复点击不能修复不受支持的适配器。

在 Mac 上打开 Caspian Control，选择 Advanced options，再选择 Restart services。等待结果后重新打开面板；如果 Caspian 处于关闭状态，再将其开启。重启会中断客户端连接，也可能关闭面板，但保留已保存的代理和热点设置。

对于使用标准 systemd 安装程序安装的 Linux，以下命令会重启两个 Caspian 服务。请在 Caspian 电脑的本地终端执行，或通过不会随热点停止而中断的独立连接执行。它不适用于 macOS、Windows 或没有 systemd 的容器。

```bash
sudo systemctl restart caspian.service caspian-panel.service
```

命令完成后，重新打开面板，按需开启 Caspian。如果服务仍失败，请保留错误文字用于报告。不要反复重启，也不要为了消除错误而删除配置或禁用防火墙。网页面板中的 Advanced > Put it back and start again 会尝试使用保存的设置恢复网络；这与重启服务不同，设备可能断开连接。

## 按现象排查

| 现象 | 下一步检查 |
|---|---|
| 启动 Caspian 前就没有互联网 | 换一根网线或路由器 LAN 端口。在系统中确认以太网已连接。需要网络登录时，在 Caspian 关闭状态下完成。 |
| 唯一的 Wi-Fi 适配器已被占用 | Linux 可改用以太网或独立的 AP 适配器。同一无线模块同时接入和共享 Wi-Fi 需要驱动支持，且可能共用信道。macOS 请使用以太网到内置 Wi-Fi。 |
| 没有热点适配器或找不到适配器 | 能连接 Wi-Fi 不代表支持 AP。Linux 请检查驱动和 AP 能力。Mac 的外置 USB Wi-Fi 不能用作 Caspian 热点。 |
| 适配器忙或热点无法启动 | 停止 Caspian。让热点适配器断开其他网络，但不要断开互联网适配器。停止自己开启的其他热点。在 Caspian 关闭时检查是否有另一个 VPN 正在运行，再重试。 |
| 手机看不到热点 | 确认网页面板显示热点运行中。靠近电脑，尝试 2.4 GHz 并检查国家设置。被固定的信道跟随互联网来源 Wi-Fi，单独更改热点信道无法覆盖它。 |
| 手机能看到 Wi-Fi，但无法加入 | 使用热点密码，而不是面板密码。修改网络名称或密码后，忘记旧的已保存网络，再重新加入。如果一直停在获取地址，重启 Caspian 一次；再次发生时报告错误。 |
| 手机已连接，但网页打不开 | 检查 Traffic cut；如果要允许联网，恢复流量。阅读隧道错误，检查电脑日期和时间。配置可被读取，不代表服务器可用，请向提供者确认。测试时暂时关闭手机移动数据，避免测到另一个连接。 |
| Control 显示 Ready，但网页面板是红色 | Ready 只确认服务响应。网页面板才显示隧道和热点状态。记录确切消息，不要反复重新安装。 |
| 停止或重启后面板消失 | 服务运行后，在 Caspian 电脑上打开 http://127.0.0.1:8088/。热点停止时，手机就失去访问面板的路径。默认关闭从本地网络访问面板。 |
| 睡眠、拔掉扩展坞或切换网络后失败 | 唤醒电脑，重新连接网线和适配器，在 Caspian 关闭时确认互联网正常，再启动。其他设备依赖热点时，请让电脑保持唤醒。 |
| macOS 阻止应用运行 | 按照 Mac 安装指南操作。未验证开发者警告与明确的恶意软件检测不同。不要绕过点名木马或其他恶意软件的警报。 |

## 检查配置格式

Caspian 接受 VLESS、VMess、Shadowsocks、SOCKS、Trojan 和 Hysteria2 链接，包括 hy2 别名。也接受受支持的 Clash/Clash.Meta YAML、Xray JSON、链接列表和 base64 订阅内容。列表只使用第一条链接；面板不会下载订阅 URL。请向提供者索取实际支持的配置，而不是账户密码或网页链接。

支持的传输名称包括 raw/tcp、ws、grpc、httpupgrade、xhttp/splithttp 和 kcp/mkcp。协议、传输和安全参数必须兼容，并非任意组合都能使用。不支持 TUIC、WireGuard、SSR、AnyTLS 和 Hysteria v1 链接。不要通过改名让不支持的协议通过验证。限制和测试证据见协议指南。

## 求助时保护隐私

使用问题报告表单，提供 Caspian 版本、系统或发行版及版本、以太网到 Wi-Fi 或 Wi-Fi 到 Wi-Fi 的连接方式、各适配器是内置还是外置、启动前互联网是否可用、确切错误和已尝试的步骤。如果知道适配器芯片组或驱动名称，也可以提供；不要附上序列号。

不要发布代理链接、订阅内容、配置文件、密码、密钥、二维码、公网 IP、家庭 Wi-Fi 名称、MAC/BSSID、个人主机名或未经检查的日志和截图。复制简短错误并删除标识信息。不要为了获得支持而把面板暴露到互联网，或设置路由器端口转发。

## 已知限制与验证依据

缺陷记录列出了安全和恢复方面的不足，本指南不会消除这些缺陷。例如，目前没有定期检查来恢复被其他程序删除的防火墙规则。请避免同时运行另一个网络共享工具。下面的拓扑测试通过受控输入验证规划和拒绝行为，并不是在您的硬件上进行的新测试。

- [安装](https://github.com/Iman/caspian/wiki/Installation.zh)
- [协议详情](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh)
- [报告问题](https://github.com/Iman/caspian/issues/new?template=bug_report.yml)
- [已知缺陷](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md)
- [代码与测试依据](https://github.com/Iman/caspian/blob/main/internal/netcfg/plan_test.go)
- [macOS: Ethernet / Wi-Fi](https://support.apple.com/en-ie/guide/mac-help/mchlp1540/mac)

<div dir="ltr" align="left">

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md) | [العربية](https://github.com/Iman/caspian/wiki/Home.ar) | [اردو](https://github.com/Iman/caspian/wiki/Home.ur) | [Türkçe](https://github.com/Iman/caspian/wiki/Home.tr)

</div>

<!-- Caspian guide navigation -->

Caspian 指南：[设置与支持的协议](https://github.com/Iman/caspian/wiki/Home.zh) · [SNI 欺骗与 DPI 规避：设置和限制（English）](https://github.com/Iman/caspian/wiki/SNI-Spoofing).
