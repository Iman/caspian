<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Development-and-Testing) | [فارسی](https://github.com/Iman/caspian/wiki/Development-and-Testing.fa) | [Русский](https://github.com/Iman/caspian/wiki/Development-and-Testing.ru) | [中文](https://github.com/Iman/caspian/wiki/Development-and-Testing.zh) | [العربية](https://github.com/Iman/caspian/wiki/Development-and-Testing.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Development-and-Testing.tr) | [اردو](https://github.com/Iman/caspian/wiki/Development-and-Testing.ur)

</div>

<a id="development-and-testing"></a>
# 开发与测试



[Caspian维基](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南来自现有的自述文件。其测量结果保留其原始日期；此文档移动不会报告新的测试运行。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="running-it"></a>
## 运行它

构建二进制文件并将其交给安装程序。该路径不需要释放，并且
安装程序将其用于实际安装和试运行：

去构建-o /tmp/caspian-linux-arm64 ./cmd/caspian
sha256sum /tmp/caspian-linux-arm64 | sha256sum /tmp/caspian-linux-arm64 | sed 's|/tmp/||' > /tmp/SHA256SUMS

env CASPIAN_LOCAL_BINARY=/tmp/caspian-linux-arm64 \
CASPIAN_LOCAL_CHECKSUMS=/tmp/SHA256SUMS \
bash install.sh --dry-run --yes

删除`--dry-run`即可真正安装。没有 `CASPIAN_LOCAL_CHECKSUMS` 的
换句话说，安装程序警告它正在安装未经验证的二进制文件。
[`docs/INSTALL.md`](https://github.com/Iman/caspian/blob/main/docs/INSTALL.md) 是完整的操作手册。它包括一个假 `uname` 安全带，用于
在无法安装的机器上行走拒绝。

该二进制文件有四个子命令：

caspian服务--特权根：路由、防火墙、接入点、引擎
caspianserve --panel caspian 用户：Web 面板，没有任何特权
Caspian检查报告这个盒子是什么样子；没有改变任何东西
Caspian版本

故意没有应用配置或驱动交换机的子命令。
CLI 本身是这样说的：“安装程序运行后，一个人所做的一切
发生在面板中。”

[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) 删除单元、二进制文件和目录并重播
网络日志，因此盒子保持被发现时的样子。在依赖它之前，请先阅读 [缺陷D5](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)。

<a id="the-rules-this-project-holds-itself-to"></a>
## 该项目遵守的规则

这些都不是愿望。每一种都有一种机制，并且该机制被命名。

**如果没有从真实流量中捕获的退出 IP，任何东西都不能称为工作。**
[`docs/2026-08-29-design.md`](https://github.com/Iman/caspian/blob/main/docs/2026-08-29-design.md)，第 6 节。连接不是结果。硬件
当未捕获任何退出 IP 并且退出 1 时，线束等级为 UNPROVEN，而不是 PASS。

**自信的错误句子比没有句子更糟糕。**被告知的读者
某些事情处理正确就得出结论，没有什么需要检查的。所以一个
修正留下的是测试而不是更好的句子。
`TestNothingInTheApplianceWatchesTheUplink` 存在，因为两个文档一次
声称该盒子会监视其上行链路并在移动时重新加载防火墙。

**已启动的进程并不能证明它有效。** 热点接口是
在任何东西绑定到内核之前从内核读回，并且访问点是
在服务报告自身运行之前回读。添加了两个读回
在一次测量事件之后，每个命令都返回成功。

**每个场景都被观察到失败。** `TestEveryScenarioCanFail` 注入
将每个行为命名为缺陷，并要求其变为红色。无人能及的测试
所见失败是绿灯没有连接。

**夹具的出处位于其文件名中。** `capture-pi5-` 是字节
目标上真实命令的输出，`scenario-` 是一台无人拥有的机器
测得，`golden-`是该项目自己的输出。测试阅读a
`capture-pi5-` 文件对目标做出了声明。读取 `scenario-` 的测试
文件没有。

**提交中的凭证是永久的。** `test/goldenscan` 扫描每个
注册哨兵和凭证形状的承诺固定装置，并且它
检查文件名和文件体。有人看到它捕捉了一个种植的
它知道的每个类别的秘密。

**覆盖楼层是一个棘轮。** [`scripts/gate.sh`](https://github.com/Iman/caspian/blob/main/scripts/gate.sh) 中的每个数字是什么
在引入它的工作之后测量的包，而不是目标某人
希望。没有行的包不是门控的，没有行意味着
“尚未达成一致”而不是“涵盖此一揽子计划”。

**特权方不信任调用者发送的任何内容。**每个字段
请求会根据该机器自身检测到的内容进行检查。拒绝是一个
来自闭集的错误代码，从来不是一个句子，也从来不是调用者的值
已发送。

**该盒子会向互联网询问您未要求的任何信息。** 没有遥测，没有回拨，没有崩溃
上传，没有网络字体，没有地理数据文件，并且在任何默认情况下都没有 Google 解析器。



[Architecture](https://github.com/Iman/caspian/wiki/Architecture.zh) | [Panel-and-Configuration](https://github.com/Iman/caspian/wiki/Panel-and-Configuration.zh) | [Troubleshooting](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

<!-- Caspian guide navigation -->

Caspian指南：[设置和支持的协议](https://github.com/Iman/caspian/wiki/Home.zh)·[用于 DPI 规避的 SNI 欺骗：设置和限制](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh)。


<!-- English-source-sha256: 0b014c20f10040f03746de1a758beef6a154eef8fa3c308a7a8ca9f7b44707a3 -->
