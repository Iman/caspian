# 安全与隐私

[English](https://github.com/Iman/caspian/wiki/Security-and-Privacy) | [فارسی](https://github.com/Iman/caspian/wiki/Security-and-Privacy.fa) | [Русский](https://github.com/Iman/caspian/wiki/Security-and-Privacy.ru) | [中文](https://github.com/Iman/caspian/wiki/Security-and-Privacy.zh)

[Caspian Wiki](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南从现有 README 迁移而来。测量结果保留原有日期；此次文档迁移不代表重新运行了测试。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## 它保证什么

这里的每个标题，背后要么是 `internal/netcfg/testdata/` 里生成的防火墙输出，要么是一个
被点名的测试，要么是记录在仓库里的一次实测。[`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) 是那份可读的承诺清单。
它里面的每一个标题都是 `test/bdd/` 里一个场景的名字，而每一个场景都有一个与之匹配的
注入缺陷。所以「这个测试能检测出它声称能检测的东西」这件事本身，也是一个测试结果。

### 转发的客户端流量失败即断开，而且这道阻断不需要隧道

forward 链的策略是 `drop`。它里面的第一条规则就是泄漏阻断规则，而且它只点名了热点和上行
接口：

    iifname "wlan0" oifname "eth0" drop comment "fail-closed: client traffic never leaves by the uplink"

每一条放行客户端流量的规则都点名了隧道设备，所以隧道一消失，那些规则就不再匹配，策略把
一切丢弃。阻断规则本身不可能因为隧道没了就失效，因为它压根没提隧道。每个接口都按名字
匹配、从不按索引，所以规则集在没有隧道的情况下也能加载，而那正是需要它的时候。postrouting
链是刻意空着的。

场景：「with the tunnel gone, nothing lets client traffic out by the uplink」。配套的
分析器测试：`TestWithoutInterfaceRemovesOnlyTheRulesNamingIt`。

### 这个断网开关也覆盖盒子自己的流量

output 链是 `policy drop` 加上一份点了名的放行清单。这些放行项是靠枚举目标机器上真正在跑
的东西推导出来的，而不是靠采样流量，而且生成的规则集里每一条放行都带着支撑它的那次读数：
NetworkManager 的 DHCP 客户端 socket、systemd-timesyncd、DNS、环回、隧道设备、IPv6 邻居
发现，以及那台代理服务器，后者是**按地址**放行而不是按端口，这样一个跑在 UDP 443 上的传输
才不会被悄悄弄坏。有一条放行是靠推理而不是实测加上去的，并且它明说了这一点：盒子在热点上
作为服务器应答 DHCP，这件事 conntrack 覆盖不了，因为一个 DHCP 应答和它的请求不共享同一个
五元组。

那些主动挑衅测试，以及更重要的那些阴性对照，都记录在 `PROVENANCE.md` 的「The three
provocations, run with the policy loaded」一节。测试：`TestRestrictedEgress_PermitList`、
`TestRestrictedEgress_AcceptsEstablishedBeforeItDropsAnything`、
`TestRestrictedEgress_ServerIsPermittedByAddressNotPort`。

代价写在规则集自己的头部，而不是留给人去发现：在设备开着的时候，从盒子上的 shell 里跑
`apt update` 会失败。

### 有一个不会把热点一起带走的紧急切断

把设备关掉会让热点也下线，也就断开了拿着按钮的那台手机。所以另有一个控制，它丢弃转发的
客户端流量，同时让热点、DHCP、DNS 和面板保持在线。见 [`internal/privsvc/cut.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/cut.go) 和
[`internal/panel/priv.go`](https://github.com/Iman/caspian/blob/main/internal/panel/priv.go) 里的面板动作 `cut`。

切断是运行时状态，永远不写到磁盘上，所以拔电源就能撤销它。测试：
`TestCuttingClientTrafficLeavesTheWayBack`、`TestACutIsNeverWrittenDown`、
`TestACutDoesNotSurviveARestart`、`TestForwardCut_StopsClientsAndKeepsThePanelReachable`。

两者中该按哪一个、以及各自会拆掉什么，见上面的「那几个控制，以及该按哪一个」。

### 它从内核回读接口，而不是相信一个进程

一个进程被启动了，不等于它起作用了。这是一次真实的失败。[`internal/privsvc/readback.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/readback.go)
的头部记录着：2026-08-30，服务把自己记录成正在运行、热点在 `wlan0` 上，而 `wlan0` 当时
还是家庭网络上的一个 station。hostapd 是一个活着的进程，但它的控制 socket 不应答。房间里
的一台手机列出了十一个网络，我们的不在其中。而 dnsmasq 正在别人局域网上，用 DHCPNAK 应答
一台陌生人的设备。

在 `netcfg.AssertHotspotInterfaceReleased` 证明热点接口是空闲的之前，不允许任何东西绑定到
它；在 `AssertHotspotIsAccessPoint` 回读到一个正在广播预期名称的接入点之前，不允许任何
东西报告自己正在运行。测试：`TestNothingBindsToTheHotspotInterfaceUntilItIsProvedFree`、
`TestTheServiceDoesNotReportRunningUntilTheAccessPointReadsBackAsOne`、
`TestAnAccessPointBroadcastingAnotherNameIsNotOurs`、
`TestTheReleaseIsReadBackBeforeAnythingBindsAndTheAccessPointAfter`。

### 您能把自己的 WiFi 拿回来

每一次网络改动，都会在做出改动**之前**连同它的逆操作一起写进
`/var/lib/caspian/netcfg.journal`，而关闭时会倒着回放这些记录。记录在磁盘上而不是在内存
里，所以一个被杀掉的进程或者一次断电不会把它弄丢。一台在改动中途死掉的盒子，会先回放
记录，然后才去看这台机器、才去应用任何新的东西。

接管一个 WiFi 接口是一步一步记进日志的。正向序列和它的逆操作都在目标机器上跑过，记录在
`PROVENANCE.md` 的「The release sequence has been run on the target」一节。四条命令发了
出去，逆操作在八秒后把盒子放回了它自己的网络，带着它自己的地址。

有一次改动是刻意没有逆操作的，而且
`TestPlan_InvariantsHoldOnEveryModelledMachine` 断言它是唯一的一个：把热点接口拉起来。
在退出时把一个无线电关掉，比让它开着更糟，因为机器自己的 WiFi、以及用户正在看的那个面板，
都可能在它上面。

场景：「turning the switch off returns every change the box made」、「a teardown replayed
from the journal of a killed process undoes the same changes」、「a box killed halfway
through cleans up before it does anything else」。测试：
`TestJournal_RecordsInverseBeforeTheChange`、
`TestTeardown_ReplaysInExactReverseOrder`、
`TestRecover_UndoesAJournalLeftByAKilledProcess`、
`TestTheTakeoverReleasesTheInterfaceItSaysItWillRelease`。

如果某个逆操作失败了，防火墙自己的逆操作会被**扣住**而不是执行，这样一台没能撤销自己路由
的盒子仍然保有它的阻断。测试：`TestTheFirewallIsNotRemovedWhenAnEarlierInverseFailed`。

### 粘贴的配置不会出现在屏幕上、日志里或任何可读的文件里

这条流程产出的每一样东西，都会被拿去搜索那份粘贴进来的凭据：

- 每一个错误，以及每一条显示给用户的消息
- 这台设备的日志行
- 面板对配置的描述
- 保存的设置在诊断视图里渲染出来的样子
- 生成的防火墙规则
- 生成的 DHCP 和 DNS 配置
- 引擎自己的日志
- 磁盘上的日志文件
- 从非特权的面板发往特权服务的那个请求

它一个都不在里面。同时也会检查它确实在它必须在的那两个地方，这样测试就不会因为配置本身
不见了而通过。

场景：「the pasted credential never reaches a screen, a log or a readable file」、「the
hotspot password reaches the access point and nothing else」。测试：
`TestPastedConfigNeverAppearsInAResponseOrALog`、
`TestFailedConfigPathsDoNotEchoTheInput`、`TestStartRequestRedactsItself`、
`TestNoCredentialReachesTheAdvancedView`、
`TestTheServerAddressNeverAppearsInADiagnosticLine`。

### 面板不向互联网要任何东西

浏览器加载的每一份样式表、脚本和图标，都用 `go:embed` 编进了二进制。见
[`internal/panel/assets.go`](https://github.com/Iman/caspian/blob/main/internal/panel/assets.go)。完全没有网络字体：样式表的字体栈全是系统字体，波斯语可用的
排在前面。

隐私上的理由是，一个远程资源会把每一个打开面板的人的地址告诉第三方。更强的理由是可用性。
面板必须在隧道断掉时也能加载，而那恰恰是有人需要它的时候。

是两道机制，不是一道。`TestNoAssetReferencesAnExternalURL` 和
`TestNoRenderedPageReferencesAnExternalURL` 会扫描静态资源和每一个渲染出来的页面，看有没有
绝对 URL。`setSecurityHeaders` 会发送 `default-src 'none'`，并把列出的每一个源都设为
`'self'`，这样即使有漏网之鱼躲过了测试，浏览器也会拒绝它。`internal/panel` 里除了它自己的
测试之外，任何地方都不存在向外发起 HTTP 的客户端。

生成的配置里同样任何地方都不出现 Google 的解析器，也不使用任何 `geoip:` 或 `geosite:`
规则，因为这两样都会给一个整个安装故事就是「一个经过校验的二进制」的产品重新引入下载。
测试：`TestNoGoogleAnywhereInGeneratedConfigs`、
`TestGoogleResolverIsRejectedAtTheSource`。场景：「the box needs no download and asks no
Google server anything」。

### 权限是分开的

`caspian serve --privileged` 以 root 运行，掌管路由、防火墙、接入点和引擎。它通过一个
unix socket 接受一份简短的、点了名的动作清单，绝不接受由用户输入拼出来的命令。
`caspian serve --panel` 以非特权的 `caspian` 账号运行，只掌管网页界面，别的什么都不管。
词汇表和帧格式见上面的「架构」。

面板密码用 argon2id 哈希。见 [`internal/state/password.go`](https://github.com/Iman/caspian/blob/main/internal/state/password.go)。它是盒子上的一个本地密码。
别处不存在任何账号。

### 在任何握手之前先检查时钟

Pi 没有带电池的时钟，而有两个彼此独立的机制依赖挂钟时间。REALITY 会把它写进握手，而
xray-core **接受**哪些配置取决于日期。所以一台时钟起来就是错的盒子，不只是连不上而已。
它会接受一份同一个二进制在时钟被校正之后就会拒绝的配置。

这项检查在校验之前、在尝试任何东西之前就运行。见 [`internal/privsvc/clock.go`](https://github.com/Iman/caspian/blob/main/internal/privsvc/clock.go)，它作为
`applyLocked` 的第 1 步由 `Service.Start` 调用。它会抛出一个独立的故障码，好让面板不去
怪罪用户的配置。测试：`TestClockFailureIsNotBlamedOnTheConfig`。

### 三种配置失败被区分开来

「这条链接读不了」、「读懂了，但照这样写没法用」、「链接没问题，是服务器没有应答」这三
种情况需要用户做三件不同的事，而第三种最常见。先怪配置，正是让人扔掉一份从来没坏过的配置
的原因。在那段粘贴进来的文本被读之前，机器上的任何东西都不会被碰。场景：「text that is
not a link at all is refused before anything is touched」、「a link the engine will not
accept is told apart from one that would not parse」、「a link whose server never answers
is not blamed on the link」。

## 它不保证什么

这份清单才是要仔细读的那份。

### 443 端口上的 DNS over HTTPS 是被承载的，不是被阻断的，而且这里没有东西能看见它

客户端在 53 端口上的 DNS，在两种协议上都是被重定向到这个盒子，而不只是被放行。所以一台
把解析器硬编码在自己身上的设备是在这里被应答的，而不是被放出去找它被告知要用的那一个。
853 端口上的 DNS over TLS 会被 TCP reset 拒绝，于是设备退回到被重定向的那个端口。853 上的
DNS over QUIC 会被丢弃。

443 端口上的 DNS over HTTPS 和任何别的 HTTPS 都区分不开，会像别的东西一样经隧道承载。一个
使用它的客户端在隧道里面，不算泄漏。它同时也是不可见的。本项目里没有任何东西、硬件测试
装置里也没有任何东西能观察到它。这是设计上的一个界限。这一点写在生成的规则集本身里、写在
[`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) 里，也写在 DNS 泄漏检查打印出来的输出里，而不是只写在这里。

### IPv6 是被阻断的，而且 IPv6 那条路还没做完

没有 IPv6 隧道。一台有可用 IPv6 路径的设备会优先走 IPv6，从而完全绕开隧道，所以默认策略
是阻断。有四样东西撑着这一点。`IPv6Block` 是 `netcfg.DefaultOptions` 里的默认值。盒子不
转发 IPv6。防火墙在热点上双向丢弃转发的 IPv6。而朝热点的路由通告会被丢弃，所以设备没法
给自己弄一个地址。场景：「clients are never offered the IPv6 the tunnel cannot carry」。

`IPv6Forward` 作为一个选项存在，而它自己的注释就说不要打开它。引擎的 TUN 入站还没有在目标
机器上被证明能承载 IPv6。它也刻意不往 forward 链里加任何放行规则：IPv4 的那两条放行规则在
两个方向上都指名了热点子网，而整份方案里根本没有任何 v6 前缀可供指名，一条只匹配那两个接口
名字的规则会接受客户端写上去的任何源地址。`TestRuleset_NoUnconstrainedIPv6AcceptInForward`
守着这条线。

**「阻断」说的是路由，不是 DNS，而这个区别很重要。** 一台已接入设备发出的 AAAA 查询不会被
抑制，也不会被回以空答案。它会走到引擎，穿过隧道，然后带着真实的 AAAA 记录回来，因为引擎配
置文档要求 `UseIP`，而 dnsmasq 没有设 `filter-AAAA`。于是设备学到了一批它根本无法到达的
IPv6 地址，然后退回到 IPv4。

在没有任何东西能给客户端一个 v6 地址之前，这是无害的；而一旦有什么东西能给了，这就是第一件
不再无害的事，因为一个有可用 v6 路径的客户端会优先采用那个 AAAA 答案，并从一条这个盒子并不
承载的路径离开。这件事被写在这里，而不是留成一个意外，而且
`TestAAAAQueriesAreAnsweredAndNotSuppressed` 把这两半都钉住了，所以要改它就必须是一个决定。

**硬件测试台根本没法给 IPv6 打分，所以从它那里得到的任何 IPv6 结果都没有意义。**
[`test/hardware/README.md`](https://github.com/Iman/caspian/blob/main/test/hardware/README.md) 在「What this vantage cannot grade: IPv6」一节里记录着：手机只有
一个链路本地地址，`ip -6 route show default` 在手机上和在 Pi 上都是空的，而连接一个 IPv6
字面地址会得到「Network is unreachable」。那个局域网上根本没有 IPv6，所以在那里跑一次 IPv6
泄漏检查，即使这台设备什么都不做也会通过。本项目手上的每一个硬件结果都是 IPv4 结果。任何
在一个 IPv6 可用的网络上运行它的人，都必须把这当成一个新问题，而不是一个已经覆盖了的问题，
并且要准备好自己去写那个测试，而不是打开一个现成的。

### 盒子自己的流量按设计不在失败即断开的承诺范围内

那个承诺说的是**转发的客户端流量**。盒子自己通往您服务器的那条连接必须直接到达上行接口，
否则根本就没有隧道，而 [`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md) 第 7 节正是因为这个原因把盒子自己的流量
放在保证范围之外。

output 链的断网开关缩小了这个范围，但没有把它关上。生成的规则集在自己的头部写明了残留风险。
DNS 是一个口子：盒子上的任何东西仍然能在 53 端口上够到网络，而且在任何隧道存在之前，服务器
的主机名就已经在本地网络上被明文解析了。这两样都不是客户端流量的泄漏，而且断网开关也没有
让它们变得更糟。

input 链的策略是 `accept`，这同样是按设计如此，也同样写在规则集里。早先的一个版本把它设为
`drop`，`PROVENANCE.md` 记录了在目标机器上实测时发生了什么。每一个新的入站连接都被拒绝，
SSH 不再应答，而已经打开的那个会话还能用，这在一台无头机器上和崩溃是分不清的。input 链
唯一限制了东西的地方是热点这一侧，在那里一台加入的设备只能够到 DHCP、DNS、面板和 ICMP
echo，盒子上别的什么都够不到。

### 客户端隔离是一条规则，不是一次实测

规则集里有 `iifname "wlan0" oifname "wlan0" drop`。这条规则在不在，是有检查的。它管不管用，
没有。

### 这个仓库里没有任何东西会抓取出口 IP

`test/tunnel` 让真实字节穿过一个真实的 xray-core 服务端，而它里面的一切都在环回地址上，
所以它抓不到出口 IP，也不可能抓到。`test/bdd` 没有网络、没有无线电、没有 root、也没有隧道
设备。它在进程内通过真实的配置加载器运行真实的引擎，所以「引擎接受了这份配置并启动了」这句
话是字面意思，但隧道入站是关掉的。

所以这个仓库里没有任何东西满足本项目自己给「可以用」定下的标准。[`docs/BEHAVIOUR.md`](https://github.com/Iman/caspian/blob/main/docs/BEHAVIOUR.md) 以一节
「What this suite does not prove」结尾，列出还欠着什么。请把它当作测试套件的一部分来读。

### 防火墙一旦加载就没有东西再复查它

见下面的缺陷 D1。如果在设备运行期间有东西把表清空了，盒子会继续转发，面板会继续报告已连接，
而没有任何东西会察觉。

### 没有东西盯着上行链路

互联网这一头挪了位置，比如线被拔了或者租约换了个方式续，这是一种没有任何东西会大声报错的
变化。那条钉住的、通往服务器的路由仍然存在，仍然指着一个已经不再是出路的地址。盒子不会
察觉，隧道会一直停着，直到有人再按一次开关。

这付出的代价是可用性，不是隐私。客户端流量在整个过程中一直被阻断，因为 forward 策略是 drop，
而它里面每一条 accept 都点名了隧道。`netcfg.WatchUplink` 和 `Plan.RederiveForUplink` 存在
而且能用，但没有任何已发布的代码调用它们中的任何一个。
`TestNothingInTheApplianceWatchesTheUplink` 就是那个防止相反的句子飘回文档里的东西，那句话
在文档里一直待到 2026-08-30。

### 模式 B 从未在真实硬件上跑过

每一份模式 B 的测试数据都是写出来的。`PROVENANCE.md` 记录了目标机器只有一个无线电、也没有
USB 网卡，所以这个产品叫人去买网卡才能用的那种摆法，是对着没有人实测过的字节做的证明。

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[Architecture](https://github.com/Iman/caspian/wiki/Architecture) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting)
