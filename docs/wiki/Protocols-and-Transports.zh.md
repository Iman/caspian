<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh) | [العربية](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.tr) | [اردو](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ur)

</div>

<a id="protocols-and-transports"></a>
# 协议和传输



[Caspian维基](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南来自现有的自述文件。其测量结果保留其原始日期；此文档移动不会报告新的测试运行。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="what-you-can-paste-and-what-it-will-refuse"></a>
## 你可以粘贴什么，它会拒绝什么

你把配置带来。这是盒子接受的内容，取自代码
接受而不是从愿望清单中选择。每一行都根据
`internal/link` 和固定发动机。

|  | 它有效 | 被拒绝了 |
|---|---|---|
| 分享链接 | `vless://` `vmess://` `ss://` `socks://` `trojan://` `hysteria2://` `hy2://` | `tuic://` `ssr://` `wireguard://` `anytls://` `naive+https://` `hysteria://`（版本 1） |
| 粘贴文档 | Clash 和 Clash.Meta YAML、原始 xray JSON、每行一个链接列表、base64 订阅 blob | 订阅 URL、base64 包装的 Clash 文档、JSON 数组、第一行是注释的文本 |
| 交通 | `raw`（也写为 `tcp`）、`ws`、`grpc`、`httpupgrade`、`xhttp`（也写为 `splithttp`）、`kcp` 和`mkcp` | `h2`、`h3`、`http`、`quic`、`gun` |
| 安全 | `none`、`tls`、`reality` | `xtls`（传统类型）、`allowInsecure` |
| VLESS流量 | `xtls-rprx-vision`、`xtls-rprx-vision-udp443` 或无 | 所有其他值 |

该拒绝列中的 `h2` 和 `h3` 是传输名称。 HTTP/2 和 HTTP/3
本身携带：`type=xhttp` 和 `security=tls`，以及 TLS ALPN
决定哪个。参见【HTTP/2和HTTP/3都承载了，在不同的下
名称](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-and-http3-are-carried-under-a-different-name)。

有六件事让人们感到惊讶，所以它们在这里而不是在脚注中：

仅使用第一个链接。粘贴四十台服务器并配置一台；的
面板会告诉您找到了多少个。 `ss://`和`socks://`需要base64形式
他们的用户信息，简单的 `method:password@host` 拼写是
拒绝了。 REALITY 仅适用于 `raw`、`xhttp` 和 `grpc`，因此将其与
WebSocket 在粘贴时被引擎拒绝，而不是稍后失败。
`security=` 在这里必须是小写，即使引擎本身不是
注意，大写的 `TLS` 将返回给您 `none`。 `plugin=`
`ss://` 链路上的参数将被忽略，无需说明。以及订阅
粘贴到配置框中的 URL 被拒绝，因为该框采用配置：
地址放在它旁边的订阅字段中，Caspian 只获取它
当你按下按钮时，穿过隧道。

完整的图片，包括其中哪些携带了真实字节以及哪些
已被证明在硬件上端到端地捕获了退出地址，正在
[协议和传输](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#protocols-and-transports)。这是三个不同的
声明和这个项目不会让它们变得模糊。

<a id="protocols-and-transports-1"></a>
## 协议和传输

共享链接包含三个独立的内容，它有助于将它们分开：
代理协议、承载它的传输以及封装的加密层
围绕该交通工具。使用 TLS 的 WebSocket 上的 VLESS 链接和 VLESS 链接
通过普通 TCP 与 REALITY 是相同的协议，达到相同的类型
服务器通过两条不同的路线。他们以不同的方式失败。

代理协议是上面列出的七种方案。 VLESS 是最常见的一种
本文档的示例使用它，因为 REALITY 就是为此而构建的。什么都没有
在设备中是特定于它的。解析器产生一个描述，
`internal/xcfg` 围绕它编写了一个引擎文档，以及盒子的其余部分
不知道它承载的是哪个协议。

传输来自 xray-core，并在供应商的解析器中命名：

- `tcp`，也写作`raw`
- `ws`，用于 WebSocket
- `httpupgrade`
- `xhttp`，该协议以前称为 SplitHTTP。两种拼写都解析
- `grpc`
- `kcp` 和 `mkcp`，用于 mKCP

`h2`、`http`、`h3` 和 `quic` 不在该列表中。此引脚的引擎版本
删除了它们，因此请求链接的链接会被拒绝而不是进行，并且
`internal/link` 中的 `TestRemovedTransportsAreRefusedWithASentence` 认为
拒绝到位。

拒绝读起来的好坏取决于进入的路线。一份命名为“拒绝”的 Clash 文档
得到一个关于交通的句子。 `type=` 参数中的相同传输
在共享链接上，通用的“粘贴文本中没有任何内容是代理”
链接此框理解”，这是正确的但没有帮助。
`TestRemovedTransportInAURIIsReportedLessWell` 引脚不同，所以它是
已知的差距而不是惊喜。

<a id="http2-and-http3-are-carried-under-a-different-name"></a>
### HTTP/2 和 HTTP/3 以不同的名称进行承载

被拒绝 `type=h2` 或 `type=quic` 并不意味着盒子不能说出它们。
这意味着拼写发生了变化。 XHTTP 取代了两者，它选择它的 HTTP
来自 TLS ALPN 而不是来自传输名称的版本：

| 你想要什么 | 写什么 |
|---|---|
| HTTP/3，即 QUIC | `type=xhttp` 与 `security=tls`、`alpn=h3` 和 `mode=stream-one` |
| HTTP/2 | `type=xhttp` 与 `security=tls` 以及任何不完全是 `h3` 的 ALPN |
| QUIC，无 XHTTP | `hysteria2://` 链接，底层是 QUIC，需要 `alpn=h3` |

钥匙完好无损地到达发动机：`internal/xcfg` 将出站作为
不透明的 JSON 并且从不对其进行解码，因此 `alpn`、`mode`、`xmux` 和 QUIC 调整
块到达时与粘贴时完全相同。

四个细节决定你是得到h3还是默默得到别的东西：

`alpn` 必须恰好是一个值，并且该值必须是 `h3`。写作
`alpn=h3,h2` 为您提供 HTTP/2 且没有任何警告，因为引擎需要一个列表
任何其他长度作为版本 2 的请求。REALITY 每当
它存在，所以 REALITY 和 h3 是互斥的，将它们配对得到
你是h2而不是错误。必须显式设置 `mode`，因为
默认解析为 `packet-up` 而不是 `stream-one` 形状引擎
名称作为 QUIC 的替代品。和 `downloadSettings`，用于分割上传和
下载，与`mode: stream-one`一起被拒绝；该组合需要
`stream-up`。

词汇上的一个冲突值得直白地说明，因为它读起来就像
矛盾：`type=h3` 被拒绝，`alpn=h3` 为必填项。他们是
不同的领域。第一个命名了一种不再存在的传输；第二个
命名 TLS 内部协商的协议。

这些配置被盒子接受并验证。他们还没有
从这里开始针对实时服务器进行驱动，因此将该行视为引擎的行
能力，而不是这个项目所观察到的工作。

安全层为`reality`、`tls`或`none`。

并非每种组合都同样有用。 REALITY 通常与普通配对
TCP，因为它的整个方法是借用真实站点的TLS握手，所以
将其包装在另一个 TLS 层中就违背了这一点。 WebSocket、HTTPUpgrade 和
XHTTP 的存在看起来就像普通的 Web 流量一样，用于检查
连接，并且它们通常与 TLS 配对，原因与普通连接相同
网站是。带有 `security=none` 的 WebSocket 是一种需要三思而后行的形状
关于。它是线路上的明文，只有当有其他内容时才有意义
已经提供了加密，例如 CDN 在前面终止 TLS
服务器。

<a id="three-different-claims-kept-apart"></a>
### 三种不同的主张，分开

下面的区别是本文档中最重要的内容。阅读
列标题位于行之前。

| 索赔 | 它依靠什么 | 它的价值是什么 |
|---|---|---|
| 解析器接受它 | `internal/link`，以及承诺的黄金引擎文档 | 文档稳定。没有拨打任何电话 |
| 它携带字节 | `test/tunnel`，一个真正的环回xray核心服务器 | 流量通过协议移动。没有退出IP、没有设备、没有互联网 |
| 端到端已被证明 | `test/hardware`，热点上的真实手机 | 真实流量离开盒子，出口地址被捕获并命名 |

<a id="what-has-carried-bytes-through-a-real-server"></a>
### 什么通过真实服务器携带字节

由 `test/tunnel` 添加。解析器接受的每个方案都是端到端驱动的
针对真实的 xray-core 实例，根据该模块自身的依赖项构建，并且
通过 `internal/engine` 使用的相同加载器加载。客户端是
产品路径，未修改：`link.Parse`，然后 `xcfg.Build`，然后
`engine.Engine.Start`。没有配置是手写的。

| 协议 | 运输 | 安全 | 携带HTTP请求 |
|---|---|---|---|
| VLESS | TCP（原始） | 无 | 是的 |
| 虚拟梅斯 | TCP（原始） | 无 | 是的 |
| Shadowsocks，aes-256-gcm | TCP（原始） | 无 | 是的 |
| 袜子 | TCP（原始） | 无 | 是的 |
| Trojan | TCP（原始） | TLS，由摘要固定 | 是的 |
| Hysteria2 和 `hy2` 别名 | 奎克 | TLS，由摘要固定 | 是的 |

四个控件阻止跳过隧道通过的请求，并且所有四个控件
运行而不是在散文中断言。客户永远不会被告知在哪里
来源是，并被赋予 `.invalid` 名称和诱饵端口。名称
无法解析，如果机器上有解析器，套件会大声说出来
无论如何都会回答它。来源不仅检查请求的处理地点
它到达了。诱饵计算自己的点击次数，隧道请求必须添加
没有。 `TestEveryCarriageProofCanFail` 和
`TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel` 是什么让这些
控制证据而不是意图。

仔细阅读每一行。除 Hysteria2 之外的每一行都通过原始 TCP 运行。无行驱动器
REALITY，其服务器端需要一个真正的握手目标。 Shadowsocks 是
仅限 aes-256-gcm，因为 2022 密码采用不同的代码路径。每一行
携带 TCP 请求，UDP 关联关闭。一切都在环回，所以
没有捕获任何退出 IP，也不能捕获任何退出 IP。

`TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd` 读取已接受的方案
列出了 `internal/link` 的源，因此无法添加第八个方案
这里没有争论。

<a id="what-has-actually-been-proven-on-hardware"></a>
### 硬件上已经实际证明了什么

下表是捕获出口 IP 时所经过的实际流量。它
不是解析器接受的内容，也不是环回套件携带的内容。

| 协议 | 运输 | 安全 | 经过验证的端到端 |
|---|---|---|---|
| VLESS | TCP（原始） | REALITY | 是的，在三个独立的服务器上 |
| VLESS | ws（WebSocket） | 无，加上 VLESS 加密 | 是的 |
| VLESS | ws（WebSocket） | 传输层安全协议 | 是的，通过 CDN |
| VLESS | http升级 | 传输层安全协议 | 是的，通过 CDN |
| VLESS | xhttp | 传输层安全协议 | 是的 |
| VMess、Trojan、Shadowsocks、袜子、Hysteria2 | 任何 | 任何 | 不 |

每一个都通过在连接到网络的真实手机上驱动真实的浏览器来证明。
热点。退出地址是从两个独立的来源捕获并匹配的
到服务器的配置名称。使用了三个不同的服务器
每个返回不同的地址，因此不能重复或缓存读取
被误认为是工作隧道。

未经证实的行并不意味着它已被破坏。这是一个主张
没有人看过从远端发出的数据包，这是另一回事
这是该项目唯一将其视为证据的东西。引擎记录每个
运输产生的信息被固定为黄金文件，因此改变一个人的方式
组成显示为差异。这证明该文件是稳定的并且什么也没说
关于交通是否连接。

<a id="why-a-row-with-no-transport-security-is-still-encrypted"></a>
### 为什么没有传输安全的行仍然被加密

上面的 `security` 列是关于包裹在传输周围的层，并且
`none` 并不意味着“不加密”。这意味着没有 TLS，也没有 REALITY。那
值得精确说明，因为以其他方式阅读会令人震惊
读得太慷慨会更糟。

VLESS 本身不进行加密。这是一个无状态协议，期望
下面的层提供机密性，通常是 REALITY 或
TLS。通过 WebSocket 与 `security=none` 进行 VLESS 链接，仅此而已
线路上的明文，并且每个数据包的退出地址都会被证明
路径上的任何东西都可以读取。

使该行安全的是链接中携带的 VLESS 加密
`encryption=`参数。它是一种混合密钥交换，ML-KEM-768 用于
后量子电阻与 X25519 相结合，应用于 VLESS 层本身
而不是在它下面。所以流量是加密的，加密方式是
旨在防止今天记录它的攻击者的安全的东西
后来有了量子计算机。带有 `encryption=none` AND 的链接
`security=none` 两者都没有，那就是拒绝的组合。

这不是噪声协议框架 (noiseprotocol.org)。这里面什么都没有
设备、供应商的共享链接解析器或引擎中实现了噪声。
“噪音”这个词出现在 xray-core 的配置中，表示一些不相关的东西，
用随机字节填充流量以改变其在线上的形状，即
混淆而不是握手。赋予这一行它的东西
机密性是 VLESS 加密，名称很重要，因为两者
提供不同的保证。

2026 年 8 月 30 日测量而非假设。该软件包不会重建
逐个字段出站。它重新序列化解析器生成的内容，并且
协议设置作为一个不透明的斑点运行。这就是为什么参数
幸存下来。这也是为什么如果它停止生存，任何东西都不会损坏：没有领域
会丢失，类型不会改变，其他测试也不会注意到，而
隧道清晰地承载着用户的流量，每张支票仍然是绿色的。
`internal/link` 中的 `TestVLESSEncryptionSurvivesIntoTheEngineDocument` 是
守卫，并且人们看到它在之前的无声降级中失败了
它被保留了。

<a id="a-certificate-name-that-did-not-match-and-the-client-side-fix"></a>
### 证书名称不匹配，以及客户端修复

有一个结果值得记录，因为这是该设备正确的故障
拒绝用纸覆盖。两个配置指向服务器自己的地址
同时在其前面携带 CDN 的 TLS 名称。引擎报告：

传输/互联网/httpupgrade：无法拨打请求...
tls：无法验证证书：x509：证书的有效期为
      <the apex>, not <the cdn subdomain>

该证书与所要求的姓名确实不符，并且
拒绝是你想要的行为。接受它意味着隧道可以
被持有任何证书的任何事物终止。

原因和修复都在客户端，并且没有更改服务器
需要。共享链接带有两个人们认为必须匹配的名称
不是：

sni 名称 TLS 验证证书所依据的
host 服务器路由请求的名称，HTTP 标头

失败的链接在两者中都带有 CDN 的名称。通过有效的 CDN，
因为 CDN 拥有它的证书。直指原点吧
不能，因为起源仅持有顶点的证书。将 `sni` 设置为
证书实际携带的名称，并将 `host` 保留为证书的名称
服务器路由：

sni=example.com 主机=cdn.example.com

测量日期：2026 年 8 月 30 日。由于证书错误而失败的两个链接
在那一项更改之后，上面两者都连接了。退出地址捕获自
两个独立的来源并匹配到他们自己的服务器，并且 DNS 泄漏和
在同一运行中通过了失败关闭检查。

因此，如果传输仅在直接指向原点时失败，请比较 `sni`
在您怀疑之前对照原始证书的主体备用名称
运输。 `openssl s_client -connect <address>:443 -servername <name>`
打印服务器实际呈现的内容。

<a id="the-panel-takes-a-pasted-link-and-not-an-image"></a>
### 该面板采用粘贴的链接而不是图像

设计第 5.2 节中描述了删除 QR 图像，并且**不是
已实施**。 `internal/panel/qr`只是一个编码器，没有处理程序
`internal/panel` 读取分段上传。面板生成的二维码是
手机扫描以加入热点的那个。 [`internal/panel/view.go`](https://github.com/Iman/caspian/blob/main/internal/panel/view.go) 构建它
使用 `qr.Encode` 和 `qr.WiFiJoin`，因此没有图像库，也没有远程服务
参与。



[英语：HTTP/2、HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-and-http3-are-carried-under-a-different-name) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#protocols-and-transports) | [状态：HTTP/2、HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#http2-and-http3-are-carried-under-a-different-name) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#protocols-and-transports) | [编码：HTTP/2、HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#http2-and-http3-are-carried-under-a-different-name) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#protocols-and-transports) | [中文：HTTP/2、HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-and-http3-are-carried-under-a-different-name) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#protocols-and-transports)

<!-- Caspian guide navigation -->

Caspian指南：[设置和支持的协议](https://github.com/Iman/caspian/wiki/Home.zh)·[用于 DPI 规避的 SNI 欺骗：设置和限制](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh)。


<!-- English-source-sha256: caae2c1c2ed8f7b292b28b1371b95851d6f133b60ac1e202d1b6a74a314aeadc -->
