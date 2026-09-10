<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [فارسی](https://github.com/Iman/caspian/wiki/Security-and-Privacy.fa) | [Русский](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ru) | [中文](https://github.com/Iman/caspian/wiki/Security-and-Privacy.zh) | [العربية](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Security-and-Privacy.tr) | [اردو](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ur)

</div>

<a id="security-and-privacy"></a>
# 安全和隐私



[Caspian维基](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南来自现有的自述文件。其测量结果保留其原始日期；此文档移动不会报告新的测试运行。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="what-it-guarantees"></a>
## 它保证什么

这里的每个标题都由生成的防火墙输出支持
`internal/netcfg/testdata/`，通过指定的测试，或通过记录在
存储库。 [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) 是可读的承诺列表。每个
其中的标题是`test/bdd/`中的场景名称，每个场景都有一个
匹配注入的缺陷。所以“这个测试可以检测到它声称的东西
检测”本身就是一个测试结果。

<a id="forwarded-client-traffic-fails-closed-and-the-block-does-not-need-the-tunnel"></a>
### 转发的客户端流量关闭失败，块不需要隧道

正向链的策略是`drop`。其中的第一条规则是泄漏阻止，
它仅命名热点和上行链路：

iifname“wlan0”oifname“eth0”删除评论“失败关闭：客户端流量永远不会通过上行链路离开”

每条允许客户端流量的规则都会对隧道设备进行命名，因此当
隧道消失，那些规则停止匹配，并且策略会丢弃所有内容。的
当隧道消失时，块本身不能停止工作，因为它不会
提一下。每个接口都按名称匹配，而不是按索引匹配，因此
规则集加载时不存在隧道，而这正是需要的时候。的
后路由链故意为空。

场景：“隧道消失后，客户端流量无法通过上行链路流出”。
配套分析仪测试：`TestWithoutInterfaceRemovesOnlyTheRulesNamingIt`。

<a id="the-kill-switch-covers-the-boxs-own-traffic-too"></a>
### 终止开关也覆盖了盒子自己的流量

输出链是 `policy drop`，带有指定的许可列表。许可证是
通过枚举目标上实际运行的内容而不是通过采样得出
流量，生成的规则集中的每个许可证都带有以下读数：
证明它的合理性：NetworkManager 的 DHCP 客户端套接字、systemd-timesyncd、DNS、
环回、隧道设备、IPv6 邻居发现和代理服务器
**通过地址**而不是通过端口允许，因此 UDP-on-443 传输不是
悄然破碎。一项许可是通过推理而不是测量而添加的，并且
是这样说的：盒子应答 DHCP 作为热点上的服务器，它连接跟踪
无法覆盖，因为 DHCP 回复及其请求不共享元组。

挑衅，更重要的是，消极控制都记录在
`PROVENANCE.md`，“三个挑衅，加载策略运行”。测试：
`TestRestrictedEgress_PermitList`，
`TestRestrictedEgress_AcceptsEstablishedBeforeItDropsAnything`，
`TestRestrictedEgress_ServerIsPermittedByAddressNotPort`。

成本在规则集自己的标头中说明，而不是发现：`apt
当设备打开时，从盒子上的 shell 进行更新失败。

<a id="there-is-an-emergency-cut-that-does-not-take-the-hotspot-with-it"></a>
### 有紧急切断，不会带走热点

关闭设备会关闭热点，从而断开手机连接
按住按钮。因此有一个单独的控件可以丢弃转发的客户端
流量，同时保持热点、DHCP、DNS 和面板开启。看
[`internal/privsvc/cut.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/cut.go) 和 [`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go) 中的面板动作 `cut`。

剪切是运行时状态，永远不会写入磁盘，因此拔掉插头
撤消它。测试：`TestCuttingClientTrafficLeavesTheWayBack`，
`TestACutIsNeverWrittenDown`、`TestACutDoesNotSurviveARestart`、
`TestForwardCut_StopsClientsAndKeepsThePanelReachable`。

有关控件以及每个控件停止的内容，请参阅 [面板及配置](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.zh)。

<a id="it-reads-the-interface-back-from-the-kernel-instead-of-trusting-a-process"></a>
### 它从内核读回接口而不是信任进程

已启动的流程并不能证明其有效。这是一次真正的失败。这
[`internal/privsvc/readback.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/readback.go) 的标头记录了 2026-08-30 的服务
记录自己在开启热点的情况下运行

`wlan0`

尽管

`wlan0`

仍然是一个
家庭网络上的电台。 hostapd 是一个实时进程，其控制套接字
没有回答。房间里的一部电话列出了十一个网络，我们的不在其中
他们。 dnsmasq 正在回答其他人 LAN 上的陌生人设备
DHCPNAK。

不允许任何内容绑定到热点接口，直到
`netcfg.AssertHotspotInterfaceReleased` 证明它是免费的，没有任何报告
自身运行直到 `AssertHotspotIsAccessPoint` 读回接入点
广播预期的名称。测试：
`TestNothingBindsToTheHotspotInterfaceUntilItIsProvedFree`，
`TestTheServiceDoesNotReportRunningUntilTheAccessPointReadsBackAsOne`，
`TestAnAccessPointBroadcastingAnotherNameIsNotOurs`，
`TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter`。

<a id="you-get-your-wifi-back"></a>
### 您将恢复 WiFi

每个网络更改都会写入 `/var/lib/caspian/netcfg.journal` 及其
**在**制作之前进行反向播放，关闭会反向重播这些内容。这
记录位于磁盘上而不是内存中，因此进程被终止或断电会导致
不要失去它。一个在变化过程中死亡的盒子会在查看之前重播记录
机器或应用任何新的东西。

WiFi接口的接管是一步一步记录下来的。前锋
序列及其逆序列已在目标上运行并记录在
`PROVENANCE.md`，“释放序列已在目标上运行”。四个
命令发出后，反向操作将盒子放回到它自己的网络上
八秒后自己的地址。

一项改变没有故意的相反，并且
`TestPlan_InvariantsHoldOnEveryModelledMachine` 断言它是唯一的：
打开热点界面。出去时把收音机拿下来更糟糕
比保留它，因为机器自己的WiFi，以及用户的面板
读书，可以就可以了。

场景：“关闭开关会返回盒子所做的每一个更改”、“a
从被终止进程的日志中重播的拆卸会撤消相同的更改”，
“一个中途被杀死的盒子会在做其他事情之前清理干净”。测试：
`TestJournal_RecordsInverseBeforeTheChange`，
`TestTeardown_ReplaysInExactReverseOrder`，
`TestRecover_UndoesAJournalLeftByAKilledProcess`，
`TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease`。

如果逆操作失败，防火墙自己的逆操作将被**保留**而不是运行，因此
无法撤消其路线的盒子将保持其阻塞状态。测试：
`TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed`。

<a id="the-pasted-config-never-reaches-a-screen-a-log-or-a-readable-file"></a>
### 粘贴的配置永远不会到达屏幕、日志或可读文件

搜索流程生成的所有内容以查找粘贴的凭据：

- 每个错误以及向用户显示的每条消息
- 设备的日志行
- 面板的配置描述
- 为诊断而渲染时保存的设置
- 生成的防火墙
- 生成的 DHCP 和 DNS 配置
- 引擎自己的日志
- 磁盘上的日志
- 从非特权面板跨越到特权服务的请求

它们都不存在。经过检查，它位于必须位于的两个位置，因此
由于配置丢失，测试无法通过。

场景：“粘贴的凭证永远不会到达屏幕、日志或可读的
文件”，“热点密码到达接入点，仅此而已”。测试：
`TestPastedConfigNeverAppearsInAResponseOrALog`，
`TestFailedConfigPathsDoNotEchoTheInput`, `TestStartRequestRedactsItself`,
`TestNoCredentialReachesTheAdvancedView`，
`TestTheServerAddressNeverAppearsInADiagnosticLine`。

<a id="the-panel-asks-the-internet-for-nothing-on-its-own"></a>
### 该小组本身并没有向互联网索要任何东西

浏览器加载的每个样式表、脚本和图标都被编译成二进制文件
与 `go:embed`。参见 [`internal/panel/assets.go`](https://github.com/Iman/caspian/blob/main/internal/panel/assets.go)。根本没有网络字体：
样式表的字体堆栈完全是系统字体，支持波斯语的字体
首先。

隐私原因是远程资产告诉第三方其地址
每个打开面板的人。更强有力的原因是可用性。面板有
当隧道关闭时加载，这正是有人需要它的时候。

两种机制，而不是一种。 `TestNoAssetReferencesAnExternalURL` 和
`TestNoRenderedPageReferencesAnExternalURL` 扫描资产和每个渲染
绝对 URL 的页面。 `setSecurityHeaders` 发送 `default-src 'none'`
每个列出的源都设置为 `'self'`，因此浏览器会拒绝通过该设置的源
测试。 `internal/panel` 中除其外的任何位置都不存在出站 HTTP 客户端
自己的测试。

Caspian 对内容提出的唯一请求是订阅刷新，并且
人按下它。它运行在特权半区而不是面板中，它是
在任何套接字打开之前被拒绝，除非引擎正在运行，并且它被拨号
通过引擎自身的环回 SOCKS 入站将主机名传递给
代理而不是在这里解析，因此字节和名称查找都不是
联系此人的 ISP。没有定时器，开机无刷新，无刷新
当隧道出现时。

生成的配置也没有在任何地方命名 Google 解析器，并且不使用
`geoip:` 或 `geosite:` 规则，因为任何一个都会将下载重新引入到
产品的整个安装过程是一个经过验证的二进制文件。测试：
`TestNoGoogleAnywhereInGeneratedConfigs`，
`TestGoogleResolverIsRejectedAtTheSource`。场景：“盒子无需下载
并且不会向 Google 服务器询问任何事情”。

<a id="privilege-is-split"></a>
### 特权被分割

`caspian serve --privileged` 以 root 身份运行并拥有路由、防火墙、
接入点和引擎。它接受一个简短的命名操作列表
unix 套接字，而不是根据用户输入构建的命令。 `caspian serve --panel`
以非特权 `caspian` 帐户运行并拥有 Web 界面和
没有别的。词汇表和帧格式参见[Architecture](https://github.com/Iman/caspian/wiki/Architecture.zh)。

面板密码使用 argon2id 进行哈希处理。参见 [`internal/state/password.go`](https://github.com/Iman/caspian/blob/main/internal/state/password.go)。它
是盒子上的本地密码。其他地方没有账户。

<a id="the-clock-is-checked-before-anything-handshakes"></a>
### 在握手之前检查时钟

Pi 没有电池时钟，两个独立的机制依赖于挂钟。
REALITY 将其写入握手中，并配置 xray-core **接受**
取决于日期。因此，时钟走错的盒子不仅仅无法
连接。它接受一个配置，一旦时钟被设置，相同的二进制文件就会拒绝
已更正。

该检查在验证之前和尝试任何操作之前运行。参见
[`internal/privsvc/clock.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/clock.go)，从 `Service.Start` 作为步骤 1 调用
`applyLocked`。它会引发明显的故障，因此面板不会责怪用户
配置。测试：`TestClockFailureIsNotBlamedOnTheConfig`。

<a id="three-config-failures-are-told-apart"></a>
### 区分三种配置失败

“无法读取该链接”，“读取它，并且不能按书面形式使用”，以及
“链接正常，服务器没有应答”需要三种不同的操作
来自用户，第三种是最常见的。首先归咎于配置是什么
让人扔掉一个从未被破坏的配置。机器上什么都没有
在读取粘贴的文本之前触摸。场景：“不是链接的文本
在任何内容被触及之前都被拒绝”，“引擎不会的链接
接受被告知与不会解析的链接分开”，“服务器永远不会解析的链接
答案不归咎于链接”。

<a id="what-it-does-not-guarantee"></a>
## 它不保证什么

这份清单值得仔细阅读。

<a id="dns-over-https-on-port-443-is-carried-not-blocked-and-nothing-here-can-see-it"></a>
### 443端口上的DNS over HTTPS被携带，没有被阻止，这里没有任何东西可以看到它

端口 53 上的客户端 DNS 通过两种协议重定向到此框，而不是
只是允许的。因此，这里回答了带有硬编码解析器的设备，而不是
比允许到达它被告知使用的那个。 853 上的 DNS over TLS 是
通过 TCP 重置被拒绝，因此设备回退到重定向端口。域名系统
853 上的 QUIC 已被丢弃。

端口 443 上的 HTTPS 上的 DNS 与任何其他 HTTPS 无法区分，并且
像其他东西一样穿过隧道。使用它的客户端位于
隧道且不漏水。它也是看不见的。该项目中没有任何内容，并且
硬件线束中没有任何东西可以观察到它。这是设计的限制。
它在生成的规则集本身、[`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) 和
DNS 泄漏检查的打印输出而不仅仅是此处。

<a id="ipv6-is-blocked-and-the-ipv6-path-is-not-finished"></a>
### IPv6受阻，IPv6路未走完

没有 IPv6 隧道。具有工作 IPv6 路径的设备比 IPv4 更喜欢它
并且会完全绕过隧道，因此默认策略是阻止。四
事情确实如此。 `IPv6Block` 是 `netcfg.DefaultOptions` 中的默认值。盒子
不转发 IPv6。防火墙会丢弃热点上转发的 IPv6
方向。并且针对热点的路由器通告被丢弃，因此
设备无法给自己一个地址。场景：“客户永远不会被提供
隧道无法承载 IPv6”。

`IPv6Forward` 作为一个选项存在，它自己的注释表明不要设置它。的
引擎的 TUN 入站尚未显示在目标上携带 IPv6。它还
特意向前向链添加不允许规则：IPv4 允许名称
两个方向上的热点子网，任何地方都没有 v6 前缀
计划命名，并且仅匹配两个接口名称的规则将接受任何
客户端写入的源地址。 `TestRuleset_NoUnconstrainedIPv6AcceptInForward`
保持那条线。

**“阻止”是关于路由，而不是关于 DNS，并且差异很重要。**
来自已加入设备的 AAAA 查询不会被抑制，也不会回答为空。它
前往引擎，穿过隧道，带回真正的 AAAA 记录，
因为引擎文档要求 `UseIP` 而 dnsmasq 没有设置 `filter-AAAA`。
因此，设备会学习它无法到达的 IPv6 地址，然后回退
到 IPv4。

这是无害的，但没有任何东西可以给客户端提供 v6 地址，这就是
如果有任何事情发生的话，第一件事就不再是无害的，因为客户
具有有效的 v6 路径更喜欢 AAAA 答案，并且会通过以下路线离开
盒子不带。它写在这里而不是留下来作为惊喜，并且
`TestAAAAQueriesAreAnsweredAndNotSuppressed` 固定两半，以便改变
这必须是一个决定。

**硬件设备根本无法对 IPv6 进行评分，因此没有 IPv6 结果意味着** [`test/hardware/README.md`](https://github.com/Iman/caspian/blob/main/test/hardware/README.md) 记录在“这个优势不能做什么”下
等级：IPv6”，电话仅携带链路本地地址，即
`ip -6 route show default` 在手机和 Pi 上都是空的，并且
与 IPv6 文字的连接回答“网络无法访问”。没有 IPv6
根本不在该 LAN 上，因此在那里运行的 IPv6 泄漏检查无需
设备做任何事情。该项目的每个硬件成果都是 IPv4
结果。任何在具有有效 IPv6 的网络上运行此程序的人都必须将其视为
新问题，不是涵盖的问题，并且必须期望编写测试而不是
启用一个。

<a id="the-boxs-own-traffic-is-outside-the-fail-closed-promise-by-design"></a>
### 根据设计，盒子自身的流量超出了故障关闭承诺范围

该承诺是关于**转发的客户端流量**。盒子自己的连接
您的服务器必须直接到达上行链路，否则根本没有隧道，并且
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md) 第 7 节将盒子自身的流量置于
出于这个原因的保证。

输出链终止开关缩小了它的范围，但并没有关闭它。生成的
规则集在其自己的标头中声明了残差。 DNS 是一个洞：任何东西
盒子仍然可以通过端口 53 到达网络，并且服务器的主机名是
在任何隧道存在之前，在本地网络上以明文方式解决。两者都不是
客户端流量的泄漏，并且终止开关不会使情况变得更糟。

输入链的策略是 `accept`，也是设计使然，也在
规则集。早期版本为 `drop`，`PROVENANCE.md` 记录了什么
当它在目标上进行测量时发生。每个新的入站连接
拒绝并且 SSH 停止应答，而已经打开的会话继续工作，
在无头机器上，这与崩溃没有区别。唯一的地方
输入链限制热点端的任何内容，其中连接的设备
到达 DHCP、DNS、面板和 ICMP 回显，盒子上没有其他内容。

<a id="client-isolation-is-a-rule-not-a-measurement"></a>
### 客户端隔离是一个规则，而不是一个衡量标准

该规则集包含 `iifname "wlan0" oifname "wlan0" drop`。规则是
目前已被检查。它的工作原理并非如此。

<a id="nothing-in-this-repository-captures-an-exit-ip"></a>
### 此存储库中没有任何内容捕获退出 IP

`test/tunnel` 通过真实的 xray 核心服务器移动真实的字节，以及所有内容
它处于环回状态，因此它无法捕获任何退出 IP。 `test/bdd` 无
网络，没有无线电，没有根，没有隧道设备。它运行真正的引擎
通过真实的配置加载器进行处理，因此“引擎接受了此配置并
已启动”的意思就是它所说的，但入站隧道已关闭。

所以这个存储库中没有任何内容满足项目自己的调用标准
有效的东西。 [`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) 以一段结尾，“这个套件是什么？
不能证明”，列出仍欠的金额。将其作为套件的一部分来阅读。

<a id="nothing-re-checks-the-firewall-once-it-is-loaded"></a>
### 一旦加载，就不会重新检查防火墙

参见 [缺陷D1](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)。如果在设备运行时有东西冲水桌子
运行，盒子保持转发，面板保持报告连接，并且
没有什么注意到。

<a id="nothing-watches-the-uplink"></a>
### 没有任何东西监视上行链路

互联网移动，因为电缆被拔掉或租约更新
不同的是，这是一个没有什么会失败的改变。固定的路线
服务器仍然存在，并且仍然指向一个不再出路的地址。
盒子没有注意到，隧道保持停止状态，直到有人按下
再次切换。

代价是可用性，而不是隐私。客户端流量保持阻塞
自始至终，因为转发策略是 drop 并且其中的每个接受都命名为
隧道。 `netcfg.WatchUplink` 和 `Plan.RederiveForUplink` 存在并工作，并且没有
发送的代码调用也可以。 `TestNothingInTheApplianceWatchesTheUplink`是什么
阻止相反的句子回到文档中原来的位置
直至 2026 年 8 月 30 日。

<a id="mode-b-has-never-been-run-on-real-hardware"></a>
### 模式 B 从未在真实硬件上运行过

每个 B 模式灯具都是经过创作的。 `PROVENANCE.md` 记录目标已
一台收音机，没有USB适配器，所以这个产品的安排告诉人们
购买适配器的字节数经过验证，无人测量。



[Architecture](https://github.com/Iman/caspian/wiki/Architecture.zh) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.zh) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

<!-- SNI upstream credits -->

SNI 欺骗来源：[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0)，以及 Windows x64 上的 WinDivert (LGPL-3.0)。
[第三方许可证、源版本和积分](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md)。

<!-- Caspian guide navigation -->

Caspian指南：[设置和支持的协议](https://github.com/Iman/caspian/wiki/Home.zh)·[用于 DPI 规避的 SNI 欺骗：设置和限制](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh)。


<!-- English-source-sha256: 535cf4665f69f332fe7b3455f5d65126b1a3ebbe759ae17c46983ea6ab766450 -->
