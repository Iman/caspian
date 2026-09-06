# 故障排查与已知缺陷

[English](https://github.com/Iman/caspian/wiki/Troubleshooting) | [فارسی](https://github.com/Iman/caspian/wiki/Troubleshooting.fa) | [Русский](https://github.com/Iman/caspian/wiki/Troubleshooting.ru) | [中文](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

[Caspian Wiki](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南从现有 README 迁移而来。测量结果保留原有日期；此次文档迁移不代表重新运行了测试。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## 未解决的缺陷

[`docs/DEFECTS.md`](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md) 是那份「已知、有证据、尚未修复」的清单，列明了实测到了什么、代价是什么、
以及要怎样才能关掉每一条。它们没有一条是客户端流量的泄漏。这里做个摘要，好让本文不至于成为
跳过那份文档的理由：

- **D1. 防火墙一旦加载就没有东西再去重新断言它。** 未解决。生产代码里任何地方都没有读取
  实时规则集的操作，也没有任何循环去复查它。所以任何在会话中途移除那张表的东西，都会让盒子
  继续转发、面板继续报告已连接。
- **D2. 对机器所做的两处改动没有逆操作。** 一处已关闭，另一处在进程内已关闭、但跨越进程被杀
  时仍未关闭。生成的配置文件现在会在停止时被删除，无线电的软阻断也会被放回去，并且会先重新
  读一遍设备状态，好让别人改过的无线电不被动。仍然未解决的部分很窄：哪些设备被解除过阻断，
  这份记录存在内存里，所以一个被杀掉而不是被停止的服务不会把它们重新阻断。
- **D3. 由本包创建的热点接口不会从 NetworkManager 手里释放。** 出于决定而未解决。接管既有
  接口的那些路径确实会释放。创建接口的那些路径不会，因为探测是在那个接口存在之前跑的。
  `TestACreatedHotspotInterfaceHasNoMeasuredManagerAndIsNotReleased` 把这个缺口钉住，让它保持
  是一个决定，而不是变成一次意外。
- **D4. 停止操作在什么都没撤销时也报告成功。** 未解决，只是报告层面的问题。一次每个逆操作都
  失败了的拆除仍然返回无错误，所以面板可以在盒子仍然完全配置着的时候，说它已经被还原成被发现
  时的样子。在那种状态下盒子仍然是失败即断开的，因为防火墙的逆操作被扣住了。
- **D5. 卸载脚本按它自己的规则回放日志。** 未解决。[`uninstall.sh`](https://github.com/Iman/caspian/blob/main/uninstall.sh) 带着一份独立的、用 Python
  重新实现的回放逻辑。它没有「当前面某个逆操作失败时扣住防火墙逆操作」这条规则的对应物，所以
  一次路由逆操作失败了的卸载，仍然会把那张表删掉。

[`docs/DEFECTS.md`](https://github.com/Iman/caspian/blob/main/docs/DEFECTS.md) 还列出了哪些是被修好而不是被记录下来的，好让这份未解决清单不被误当成全貌。

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)
