# 许可证与致谢

[English](https://github.com/Iman/caspian/wiki/Licence-and-Credits) | [فارسی](https://github.com/Iman/caspian/wiki/Licence-and-Credits.fa) | [Русский](https://github.com/Iman/caspian/wiki/Licence-and-Credits.ru) | [中文](https://github.com/Iman/caspian/wiki/Licence-and-Credits.zh)

[Caspian Wiki](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南从现有 README 迁移而来。测量结果保留原有日期；此次文档迁移不代表重新运行了测试。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## 许可证

AGPL-3.0-or-later，并附有第 7 节允许的三条附加条款。三条都属于第 7 节允许的种类，没有一条
限制您可以用这个软件做什么：保留版权声明、这份署名，以及在任何用户界面中对 Caspian 项目的
可见引用；如果您修改了它，就标明您的版本已被更改；以及不要用作者或项目的名义做宣传，其中包括
以那些名义募捐、拉赞助或申请资助。完整文本在 [`LICENSE`](https://github.com/Iman/caspian/blob/main/LICENSE) 里，条款在 [`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) 里。

第三条限制的是**名称**的使用，别的什么都不限制。您仍然可以自由地运行、研究、修改和再分发这个
软件，用于任何目的，包括商业目的。您不能做的是用作者的名义去筹钱。

之所以用 AGPL 而不是 GPL，是因为这个程序通常是作为一项服务运行、由别人连上来使用的，而第 13
节堵上了普通 GPL 留下的那个口子。之所以不用宽松许可证，是因为这个二进制静态链接了
GPL-3.0-or-later 的代码：`github.com/sagernet/sing` 和 `github.com/sagernet/sing-shadowsocks`，
两者都是经由 xray-core 引入的。所以组合作品必须以 GPL 家族的条款发布，MIT 或 Apache-2.0 对它
不可用。

## 构建于

Caspian 是围绕别人的工作写的一小段代码。引擎是 xray-core，分享链接解析器是 XTLS 的。这两个
项目都不为本项目背书；它们被列出来，是因为那些工作是他们的。

| 项目 | 许可证 | 它在这里做什么 |
|---|---|---|
| [xray-core](https://github.com/xtls/xray-core) | MPL-2.0 | 代理引擎，在进程内链接，而不是作为一个独立程序运行 |
| [libXray](https://github.com/XTLS/libXray) | MIT | 分享链接解析器，内置在 `third_party/libxray-share/` 下 |
| [REALITY](https://github.com/xtls/reality) | MPL-2.0 | TLS 伪装传输 |
| [uTLS](https://github.com/refraction-networking/utls) | BSD-3-Clause | TLS 指纹模仿 |
| [quic-go](https://github.com/apernet/quic-go) | MIT | Hysteria2 所运行的 QUIC 栈 |
| [gVisor](https://github.com/google/gvisor) | Apache-2.0 | TUN 入站使用的用户态网络栈 |
| [sing](https://github.com/sagernet/sing) 和 [sing-shadowsocks](https://github.com/sagernet/sing-shadowsocks) | GPL-3.0-or-later | Shadowsocks 2022，以及本项目采用 copyleft 的原因 |
| [netlink](https://github.com/vishvananda/netlink) | Apache-2.0 | 接口、地址和路由 |
| [miekg/dns](https://github.com/miekg/dns) | BSD-3-Clause | DNS 报文处理 |
| [gorilla/websocket](https://github.com/gorilla/websocket) | BSD-2-Clause | WebSocket 传输 |
| [CIRCL](https://github.com/cloudflare/circl) | BSD-3-Clause | 后量子密钥交换 |

它还需要机器上有 `hostapd`、`dnsmasq`、`nftables`、`iw` 和 `iproute2`。这些是作为独立程序运行
的，不是被链接进来的，所以它们的许可证不影响本项目的许可证，但没有它们这台设备什么都不是。

[`NOTICE`](https://github.com/Iman/caspian/blob/main/NOTICE) 里有完整的记录：二进制里的每一个模块、从它自己的许可证文件里读到的许可证，以及兼容性
方面的推理。

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

## Windows 组件

| 项目 | 许可证 | 用途 |
|---|---|---|
| [Wintun](https://www.wintun.net/) | Wintun Prebuilt Binaries License | 已签名的隧道驱动 `wintun.dll` |
| [.NET runtime and Windows Forms](https://github.com/dotnet/runtime) | MIT | Windows 控制程序随附的运行时 |
| `System.ServiceProcess.ServiceController` | MIT | 由 `CaspianControl.exe` 控制服务 |

Windows 安装包含独立文件 `wintun.dll`。项目原样分发官方签名的 Wintun 0.14.1。
许可证位于 [`third_party/wintun/PREBUILT-BINARIES-LICENSE.txt`](https://github.com/Iman/caspian/blob/main/third_party/wintun/PREBUILT-BINARIES-LICENSE.txt)，安装时复制到 `C:\Program Files\Caspian\WINTUN-LICENSE.txt`。

`caspian-tethering.exe` 和 `CaspianControl.exe` 的 .NET 运行时包含在可执行文件内部。
相关许可证和声明位于 `third_party/dotnet/`。Windows SDK 引用包仅用于构建，不随 Caspian 安装。
