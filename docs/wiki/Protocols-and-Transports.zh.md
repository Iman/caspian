# 协议与传输

[English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh)

[Caspian Wiki](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南从现有 README 迁移而来。测量结果保留原有日期；此次文档迁移不代表重新运行了测试。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## 您可以粘贴什么，什么会被拒绝

配置由您自己带来。下面这些是盒子接受的内容，取自真正负责接受的那段代码，而不是一份
愿望清单。每一行都是对着 `internal/link` 和固定版本的引擎实测出来的。

| | 可以用 | 会被拒绝 |
|---|---|---|
| 分享链接 | `vless://` `vmess://` `ss://` `socks://` `trojan://` `hysteria2://` `hy2://` | `tuic://` `ssr://` `wireguard://` `anytls://` `naive+https://` `hysteria://`（第 1 版） |
| 粘贴的文档 | Clash 和 Clash.Meta 的 YAML、原始的 xray JSON、每行一条的链接列表、base64 订阅数据块 | 订阅 URL、被 base64 包起来的 Clash 文档、JSON 数组、首行是注释的文本 |
| 传输方式 | `raw`（也写作 `tcp`）、`ws`、`grpc`、`httpupgrade`、`xhttp`（也写作 `splithttp`）、`kcp` 和 `mkcp` | `h2`、`h3`、`http`、`quic`、`gun` |
| 安全层 | `none`、`tls`、`reality` | `xtls`（旧的那种）、`allowInsecure` |
| VLESS 的 flow | `xtls-rprx-vision`、`xtls-rprx-vision-udp443`，或者不填 | 其他任何取值 |

`h2` 和 `h3` 在那个「会被拒绝」的列里是**传输的名字**。HTTP/2 和 HTTP/3 本身是被承载
的：`type=xhttp` 配上 `security=tls`，由 TLS 的 ALPN 决定是哪一个。见
[HTTP/2 与 HTTP/3 是被承载的，只是换了个名字](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-与-http3-是被承载的只是换了个名字)。

有六件事经常让人意外，所以写在正文里而不是脚注里：

只有第一条链接会被使用。粘贴四十个服务器，配置好的是一个；面板会告诉您它找到了
几条。`ss://` 和 `socks://` 需要用户信息的 base64 形式，纯 `method:password@host`
的写法会被拒绝。REALITY 只在 `raw`、`xhttp` 和 `grpc` 上可用，所以把它和 WebSocket
搭在一起，会在粘贴的那一刻就被引擎拒绝，而不是等到后面才失败。这里的 `security=`
必须小写，尽管引擎本身并不在意，大写的 `TLS` 会被报告成 `none`。`ss://` 链接上的
`plugin=` 参数会被忽略，而且不会告诉您。订阅 URL 会被拒绝，因为面板不从互联网上取
任何东西，这是刻意的性质，不是缺失的功能。

完整的情况，包括其中哪些真正搬运过字节、哪些在硬件上端到端证明过并抓到了出口地址，
在[协议与传输](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#协议与传输)一节。这是三种不同的主张，本项目不允许它们混为一谈。

## 协议与传输

一条分享链接携带三样彼此独立的东西，把它们分开看会有帮助：代理协议、承载它的传输，
以及包在那个传输外面的加密层。一条走 WebSocket 加 TLS 的 VLESS 链接，和一条走纯 TCP
加 REALITY 的 VLESS 链接，是同一个协议用两条不同的路线到达同一类服务器。它们失败的
方式不一样。

代理协议就是上面列出的那七种。VLESS 是本文大多数例子用的那个，因为 REALITY 就是为它
造的。这台设备里没有任何东西是专门为它写的。解析器产出一份描述，`internal/xcfg` 围绕
它组装出一份引擎配置文档，盒子的其余部分并不知道自己在载的是哪个协议。

传输方式来自 xray-core，在内置的解析器里点了名：

- `tcp`，也写作 `raw`
- `ws`，即 WebSocket
- `httpupgrade`
- `xhttp`，也就是从前叫 SplitHTTP 的那个协议。两种拼法都能解析
- `grpc`
- `kcp` 和 `mkcp`，即 mKCP

`h2`、`http`、`h3` 和 `quic` 不在那份名单里。本项目固定的这个引擎版本把它们删掉了，
所以一条要求其中之一的链接会被拒绝而不是被承载，`internal/link` 里的
`TestRemovedTransportsAreRefusedWithASentence` 把这条拒绝钉住。

这条拒绝读起来有多清楚，取决于是从哪条路进来的。一份 Clash 文档点名其中之一，会得到
一句关于该传输的说明。同一个传输若出现在分享链接的 `type=` 参数里，得到的却是那句
笼统的「nothing in the pasted text was a proxy link this box understands」，它没说错，
但没什么用。`TestRemovedTransportInAURIIsReportedLessWell` 把这个差别钉住，好让它是一个
已知的缺口，而不是一个意外。

### HTTP/2 与 HTTP/3 是被承载的，只是换了个名字

`type=h2` 或 `type=quic` 被拒绝，并不意味着盒子说不了它们。意思是拼写换了地方。XHTTP 把这
两者都取代了，而且它是从 TLS 的 ALPN 而不是从传输的名字来选择自己的 HTTP 版本的：

| 您想要什么 | 该怎么写 |
|---|---|
| HTTP/3，也就是 QUIC | `type=xhttp`，加上 `security=tls`、`alpn=h3` 和 `mode=stream-one` |
| HTTP/2 | `type=xhttp`，加上 `security=tls`，以及任何不正好是 `h3` 的 ALPN |
| QUIC，不走 XHTTP | 一条 `hysteria2://` 链接，它底下就是 QUIC，并且需要 `alpn=h3` |

这些键会原封不动地到达引擎：`internal/xcfg` 把 outbound 当作不透明的 JSON 携带，从不解码
它，所以 `alpn`、`mode`、`xmux` 以及 QUIC 调优的那一块，会和您粘贴时一模一样地送达。

有四个细节决定您拿到的是 h3，还是不声不响地拿到了别的东西：

`alpn` 必须正好是一个值，而且那个值必须是 `h3`。写成 `alpn=h3,h2` 会让您拿到 HTTP/2，而且
没有任何警告，因为引擎把任何其他长度的列表都当成是在要第 2 版。只要 REALITY 在场，它就会
强制 HTTP/2，所以 REALITY 和 h3 是互斥的，把它们搭在一起得到的是 h2 而不是一个错误。`mode`
必须显式设置，因为默认值解析成 `packet-up`，而不是引擎点名作为 QUIC 替代品的 `stream-one`
那种形态。还有，用于上传下载分离的 `downloadSettings`，和 `mode: stream-one` 一起使用会被
拒绝；那种组合需要 `stream-up`。

有一处词汇上的冲突值得明说，因为它读起来像自相矛盾：`type=h3` 会被拒绝，而 `alpn=h3` 是
必需的。它们是两个不同的字段。前者点名的是一个已经不存在的传输；后者点名的是在 TLS 内部
协商出来的协议。

这些配置盒子是接受并且校验的。它们还没有从这里对着一台在线的服务器驱动过，所以请把表里
那些行当作引擎的能力，而不是当作本项目看着它工作过的东西。

安全层是 `reality`、`tls` 或 `none`。

并非每种组合都同样有用。REALITY 通常和纯 TCP 搭配，因为它的整套办法就是借用一个真实
网站的 TLS 握手，所以再包一层 TLS 就把这件事的意义抵消了。WebSocket、HTTPUpgrade 和
XHTTP 的存在，是为了在检查连接的东西看来像普通的网页流量，所以它们通常和 TLS 搭配，
理由和一个普通网站用 TLS 是一样的。`security=none` 的 WebSocket 是唯一一种值得多想
一下的形态。它在线路上是明文，只有当别的东西已经提供了加密时它才说得通，比如在服务器
前面有一个 CDN 在终结 TLS。

### 三种不同的主张，分开来说

下面这个区分是本文最重要的东西。先读列标题，再读每一行。

| 主张 | 它依据什么 | 它值多少 |
|---|---|---|
| 解析器接受它 | `internal/link`，以及一份提交在仓库里的黄金引擎配置文档 | 文档是稳定的。什么都没有拨号 |
| 它搬运了字节 | `test/tunnel`，一个跑在环回地址上的真实 xray-core 服务端 | 流量确实穿过了这个协议。没有出口 IP，没有这台设备，没有互联网 |
| 它被端到端证明过 | `test/hardware`，一台连在热点上的真手机 | 真实流量离开了盒子，出口地址被抓到并点了名 |

### 什么真的通过一个真实服务端搬运过字节

由 `test/tunnel` 提供。解析器接受的每一种协议，都被端到端驱动去打一个真实的 xray-core
实例，该实例由本模块自己的依赖构建，并通过 `internal/engine` 用的同一个加载器加载。
客户端这一侧就是产品路径本身，没有改动：先 `link.Parse`，然后 `xcfg.Build`，然后
`engine.Engine.Start`。没有任何配置是手写的。

| 协议 | 传输 | 安全层 | 能送出一个 HTTP 请求 |
|---|---|---|---|
| VLESS | tcp (raw) | none | 是 |
| VMess | tcp (raw) | none | 是 |
| Shadowsocks，aes-256-gcm | tcp (raw) | none | 是 |
| SOCKS | tcp (raw) | none | 是 |
| Trojan | tcp (raw) | TLS，按摘要固定 | 是 |
| Hysteria2，以及 `hy2` 别名 | QUIC | TLS，按摘要固定 | 是 |

有四道控制措施能拦下一个绕过了隧道的请求，而且这四道都是真的在跑，不是在文字里断言。
客户端从不被告知源站在哪里，它拿到的是一个 `.invalid` 名字和一个诱饵的端口。那个名字
是解析不出来的，如果机器上有解析器居然应答了它，测试套件会大声说出来。源站检查的是
请求被寄往哪里，而不只是它到了。诱饵会数自己被打中了几次，一个走隧道的请求必须一次
都不加上去。`TestEveryCarriageProofCanFail` 和
`TestTheProofRejectsARequestThatDidNotGoThroughTheTunnel` 才是让这些控制措施成为证据
而不是意图的东西。

每一行都要读得窄一点。除 Hysteria2 外，每一行都跑在裸 TCP 上。没有一行驱动 REALITY，
因为它的服务端需要一个真实的握手目标。Shadowsocks 只有 aes-256-gcm，因为 2022 系列的
加密套件走的是另一条代码路径。每一行送的都是一个 TCP 请求，UDP associate 是关的。
一切都在环回地址上，所以没有抓到出口 IP，也不可能抓到。

`TestEveryProtocolTheParserAcceptsIsDrivenEndToEnd` 会从 `internal/link` 的源码里读出
那份被接受的协议名单，所以不可能加进第八种协议却不在这里加上一行。

### 什么在硬件上真的被证明过

下面这张表是真实流量走过、并且抓到了出口 IP 的那些。它不是解析器接受什么，也不是环回
测试套件搬运了什么。

| 协议 | 传输 | 安全层 | 端到端证明过 |
|---|---|---|---|
| VLESS | tcp (raw) | REALITY | 是，在三台不同的服务器上 |
| VLESS | ws (WebSocket) | none，外加 VLESS Encryption | 是 |
| VLESS | ws (WebSocket) | TLS | 是，经由一个 CDN |
| VLESS | httpupgrade | TLS | 是，经由一个 CDN |
| VLESS | xhttp | TLS | 是 |
| VMess、Trojan、Shadowsocks、SOCKS、Hysteria2 | 任意 | 任意 | 否 |

上面每一项，都是靠在一台连着热点的真手机上驱动一个真浏览器来证明的。出口地址由两个
彼此独立的来源抓到，并和配置里点名的那台服务器对上。用了三台不同的服务器，每一台
返回的地址都不同，所以一次重复的或缓存的读数不会被误当成隧道在工作。

一行没有被证明，不等于说它是坏的。它说的是还没有人看着一个数据包从远端出来，这是另一
回事，而且这是本项目唯一当作证据的东西。每一种传输产出的引擎配置文档**确实**被钉成了
黄金文件，所以组装方式一变就会显示成一处差异。那证明的是文档是稳定的，对这个传输能不能
连上则什么都没说。

### 为什么传输安全层为空的那一行仍然是加密的

上表里的 `security` 一列说的是**包在传输外面**的那一层，那里的 `none` 并不意味着
「没有加密」。它意味着没有 TLS 也没有 REALITY。这一点值得说准确，因为反着理解会让人
惊慌，而理解得太宽松则更糟。

VLESS 本身不携带加密。它是一个无状态协议，指望下面那一层提供机密性，通常是 REALITY
或 TLS。一条走 WebSocket、`security=none` 且再没有别的东西的 VLESS 链接，**确实**会在
线路上是明文，出口地址会被证明，同时路径上的任何东西都能读到每一个数据包。

让那一行安全的东西是 VLESS Encryption，它由链接的 `encryption=` 参数携带。那是一个
混合密钥交换，用 ML-KEM-768 提供抗量子能力，并与 X25519 结合，作用在 VLESS 这一层本身
而不是它下面。所以流量是加密的，而且加密它的东西被设计成：面对一个今天把流量录下来、
以后再拿到量子计算机的攻击者，仍然保持安全。一条同时带着 `encryption=none` 和
`security=none` 的链接两样都没有，而那正是应该拒绝的组合。

这**不是** Noise 协议框架（noiseprotocol.org）。这台设备里、内置的分享链接解析器里、
以及引擎里，都没有任何东西实现了 Noise。「noise」这个词出现在 xray-core 的配置里，指的
是另一件不相干的事，即用随机字节填充流量以改变它在线路上的形状，那是混淆而不是握手。
给这一行提供机密性的东西是 VLESS Encryption，名字很重要，因为这两者提供的保证并不相同。

这是在 2026-08-30 **实测**出来的，不是假定的。这个包并不逐字段重建 outbound。它把
解析器产出的东西重新序列化一遍，协议设置作为一个不透明的数据块搭车通过。这就是那个参数
能存活下来的原因。这也是为什么如果它哪天不再存活，什么都不会坏掉：不会缺任何字段，不会
有类型改变，也不会有别的测试注意到，而与此同时隧道正明文承载着用户的流量，所有检查却
依然全绿。`internal/link` 里的 `TestVLESSEncryptionSurvivesIntoTheEngineDocument` 就是
那道守卫，而且在被保留下来之前，有人亲眼看着它对着恰恰是这种静默降级失败过。

### 一个对不上的证书名，以及客户端一侧的修法

有一个结果值得记下来，因为它是这台设备正确地拒绝掩饰过去的一种故障。有两份配置指向
服务器自己的地址，却携带着它前面那个 CDN 的 TLS 名字。引擎报告：

    transport/internet/httpupgrade: failed to dial request ...
      tls: failed to verify certificate: x509: certificate is valid for
      <the apex>, not <the cdn subdomain>

那确实是一个和所请求的名字对不上的证书，拒绝它正是您想要的行为。接受它则意味着隧道
可以被任何持有任何证书的东西终结掉。

原因和修法都在客户端一侧，服务器不需要任何改动。一条分享链接携带两个名字，人们以为它们
必须相同，其实不必：

- `sni` 是 TLS 用来校验证书的那个名字
- `host` 是服务器据以路由请求的那个名字，一个 HTTP 头

那些失败的链接在**两个**里都填了 CDN 的名字。经由 CDN 时这样是行的，因为 CDN 持有该
名字的证书。直接指向源站时就不行了，因为源站只持有主域名的证书。把 `sni` 设成证书实际
携带的那个名字，并让 `host` 保持服务器据以路由的那个名字：

    sni=example.com          host=cdn.example.com

2026-08-30 **实测**。两条此前带着上面那个证书错误失败的链接，在做了这一处改动之后都
连上了。出口地址由两个彼此独立的来源抓到，并和它们各自的服务器对上，同一次运行里 DNS
泄漏检查和失败即断开检查也都通过了。

所以，如果某个传输只在直接指向源站时才失败，先拿 `sni` 和源站证书的主题备用名称比一比，
再去怀疑传输。`openssl s_client -connect <address>:443 -servername <name>` 会打印出
服务器实际出示的是什么。

### 面板接受粘贴的链接，不接受图片

拖入二维码图片这件事在设计文档 5.2 节里有描述，但**没有实现**。`internal/panel/qr`
只是一个编码器，`internal/panel` 里没有任何 handler 读取 multipart 上传。面板确实会
生成一个二维码，那是给手机扫来加入热点用的。[`internal/panel/view.go`](https://github.com/Iman/caspian/blob/main/internal/panel/view.go) 用 `qr.Encode` 和
`qr.WiFiJoin` 生成它，所以既不涉及图像库，也不涉及任何远程服务。

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[English: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#http2-and-http3-are-carried-under-a-different-name) | [English](https://github.com/Iman/caspian/wiki/Protocols-and-Transports#protocols-and-transports) | [فارسی: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#http2-و-http3-حمل-میشوند-با-نامی-دیگر) | [فارسی](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.fa#پروتکلها-و-ترابریها) | [Русский: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#http2-и-http3-переносятся-просто-под-другим-именем) | [Русский](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.ru#протоколы-и-транспорты) | [中文: HTTP/2, HTTP/3](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#http2-与-http3-是被承载的只是换了个名字) | [中文](https://github.com/Iman/caspian/wiki/Protocols-and-Transports.zh#协议与传输)
