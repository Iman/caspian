# 开发与测试

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh)

[Caspian Wiki](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南从现有 README 迁移而来。测量结果保留原有日期；此次文档迁移不代表重新运行了测试。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## 运行它

构建二进制，然后把它交给安装脚本。这条路不需要任何发布版，而且安装脚本既接受它做真装，
也接受它做试运行：

    go build -o /tmp/caspian-linux-arm64 ./cmd/caspian
    sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

    env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
        CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
        bash install.sh --dry-run --yes

去掉 `--dry-run` 就是真装。不带 `CASPIAN_LOCAL_CHECKSUMS` 时，安装脚本会用这样的措辞
警告您，它正在安装一个未经验证的二进制。[`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) 是完整的操作手册。它包含一个
假的 `uname` 装置，可以在一台根本装不上的机器上把各种拒绝走一遍。

这个二进制有四个子命令：

    caspian serve --privileged     root: routes, firewall, access point, engine
    caspian serve --panel          the caspian user: the web panel, nothing privileged
    caspian check                  report what this box looks like; changes nothing
    caspian version

刻意没有任何一个子命令能应用配置或者拨动那个开关。CLI 自己就是这么说的：「After the
installer has run, everything a person does happens in the panel.」

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) 会移除 systemd 单元、二进制和目录，并回放网络日志，好让盒子回到被发现时
的样子。在依赖它之前，先读下面的缺陷 D5。

## 这个项目给自己定的规矩

这些不是愿景。每一条都有一个机制，而且机制被点了名。

**没有从真实流量里抓到出口 IP，就不叫可以用。** [`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md) 第 6 节。
连上不等于结果。当没有抓到出口 IP 时，硬件测试装置给出的评级是 UNPROVEN 而不是 PASS，
并且以 1 退出。

**一个自信的错句子比没有句子更糟。** 一个被告知某件事已经处理好的读者，会得出结论说
这里没什么需要检查的。所以一次纠正留下的是一个测试，而不是一句更好的话。
`TestNothingInTheApplianceWatchesTheUplink` 之所以存在，是因为曾经有两份文档声称盒子会
盯着自己的上行链路，并在它变动时重新加载防火墙。

**一个进程被启动了，不等于它起作用了。** 热点接口在任何东西绑定到它之前，会先从内核
回读一遍；接入点在服务报告自己正在运行之前，也会先回读一遍。这两次回读都是在一次被实测到
的事件之后加上的，那次事件里每一条命令都返回了成功。

**每一个场景都被看着失败过。** `TestEveryScenarioCanFail` 会往每一项行为里注入一个点了名
的缺陷，并要求它变红。一个没有人见它失败过的测试，是一盏接在什么都没有上的绿灯。

**一份测试数据的来源写在它的文件名里。** `capture-pi5-` 是目标机器上一条真实命令的字节
输出，`scenario-` 是一台没有人实测过的机器，`golden-` 是本项目自己的输出。一个读
`capture-pi5-` 文件的测试，做出的是关于目标机器的主张。一个读 `scenario-` 文件的测试则
不是。

**一份凭据一旦进了提交就是永久的。** `test/goldenscan` 会对每一份提交进来的测试数据扫描
已登记的哨兵值和各种凭据形态，而且它检查文件名，不只是文件内容。它已经被看着抓住了它认识的
每一类被人故意种进去的秘密。

**覆盖率下限是一把棘轮。** [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) 里的每一个数字，都是某个包在引入它的那次工作
之后实际测出来的值，不是谁希望达到的目标。没有对应行的包就是没有被设下限，而没有行的意思是
「还没有商定下限」，不是「这个包有覆盖」。

**特权侧不相信调用方送来的任何东西。** 每个请求的每个字段都会和这台机器自己探测到的结果
核对。一次拒绝是一个来自封闭集合的故障码，绝不是一句话，也绝不是调用方送来的某个值。

**盒子不向互联网要任何东西。** 没有遥测，不回传，不上传崩溃报告，没有网络字体，没有地理
数据文件，任何默认配置里也没有 Google 的解析器。

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)

[Architecture](https://github.com/Iman/caspian/wiki/Architecture) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting)
