# 安装

[English](https://github.com/Iman/caspian/wiki/Installation) | [فارسی](https://github.com/Iman/caspian/wiki/Installation.fa) | [Русский](https://github.com/Iman/caspian/wiki/Installation.ru) | [中文](https://github.com/Iman/caspian/wiki/Installation.zh)

[Caspian Wiki](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南从现有 README 迁移而来。测量结果保留原有日期；此次文档迁移不代表重新运行了测试。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

## 安装

请先选择您的操作系统。macOS 使用图形化 DMG；Linux 和 Raspberry Pi 可以自动安装，
也可以先检查或自己构建。

### Windows 10 和 11

Windows 10 支持仍处于实验阶段，尚未测试。当前代码在 x64 上要求 Windows 10
版本 2004（内部版本 19041）或更高版本。Windows 10 ARM64 的兼容性尚未验证。

您需要 Windows 11 电脑，或运行 Windows 10 版本 2004 或更高版本的 x64 电脑
（实验性支持）、管理员账户、支持 Windows Mobile Hotspot 的 Wi-Fi 适配器和互联网连接。
安装程序包含所需组件，无需安装 Go 或 .NET SDK。
详细步骤请参阅[英文 Windows 安装说明](https://github.com/Iman/caspian/wiki/Installation#windows-10-and-11)。

### macOS 13 或更高版本

macOS 磁盘映像包含原生的 **Caspian Control** 应用和 Caspian 引擎。无需 Terminal、
Go、Homebrew 或其他运行时。您需要管理员账户；当内置 Wi-Fi 用作热点时，Mac 还需要
通过有线 Ethernet 接入互联网。

#### 选择正确的下载文件

- Intel Mac 使用 `Caspian-v0.2.4-macos-amd64.dmg`。
- Apple Silicon Mac（M1 或更新型号）使用
  `Caspian-v0.2.4-macos-arm64.dmg`。

如果不确定处理器类型，请打开 **Apple 菜单 → 关于本机**。

#### 安装并批准首次打开

v0.2.4 应用带有 ad-hoc 签名，但尚未使用 Apple Developer ID 签名，也未经过 Apple
公证。因此 Gatekeeper 会显示 **“Caspian” Not Opened**，并提示 Apple 无法验证它是否
不含恶意软件。这不是应用崩溃。只有当文件来自 Caspian 官方发布页时才应绕过此警告。

1. 打开 [Caspian 最新发布](https://github.com/Iman/caspian/releases/latest)，展开
   **Assets**。
2. 下载适合 Mac 处理器的 DMG，并将它打开。
3. 将 `Caspian.app` 拖入 **Applications（应用程序）**文件夹。
4. 尝试打开 **Applications** 中的副本一次。
5. Gatekeeper 阻止它时，点击 **Done**。
6. 打开 **Apple 菜单 → System Settings → Privacy & Security**。
7. 向下滚动到 **Security**，在 Caspian 旁点击 **Open Anyway**。首次打开被阻止后，
   这个按钮大约只显示一小时。
8. 输入 Mac 登录密码，点击 **OK**，然后确认 **Open**。

macOS 会把这个应用保存为例外，以后可以正常双击打开。Apple 的官方说明见
[通过覆盖安全设置打开应用](https://support.apple.com/guide/mac-help/apple-cant-check-app-for-malicious-software-mchleab3a043/26/mac/26)。

#### 如果 macOS 仍然阻止后台服务

即使你已批准 `Caspian.app`，已安装的后台服务可执行文件
`/usr/local/bin/caspian` 仍可能保留隔离属性。此时警告中的名称是小写的
`caspian`，控制窗口可能显示 **Caspian needs attention**。

**如果警告提到木马或报告检测到恶意软件，请勿执行下面的命令。**
停止安装，并在 [GitHub 问题报告](https://github.com/Iman/caspian/issues)中提供
完整警告文本、检测到的威胁名称、发布版本和下载地址。
恶意软件检测结果需要调查；仅凭发布文件缺少签名，不能认定检测结果是误报。
请参阅 [Apple 对 macOS 安全警告的说明](https://support.apple.com/en-ie/102445)。

此备用方法仅适用于无法验证开发者或应用未经公证的警告，且你必须信任文件及其来源。
从 Caspian 官方发布页下载发布文件，并将 DMG 的校验和与该版本公布的
`SHA256SUMS` 进行比较。校验和一致只能确认文件与发布文件相同，不能证明文件安全。

1. 打开 **Terminal（终端）**。
2. 移除已安装的后台服务可执行文件的隔离属性：

   ```bash
   sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
   ```

3. 输入 Mac 登录密码。Terminal 不会显示你输入的密码。
4. 在 Caspian 中选择 **Advanced options → Restart services**。

此命令仅移除指定文件的隔离属性，不会扫描、签名或公证该可执行文件。
如果 Terminal 显示 `No such xattr`，说明该属性已不存在。
如果服务仍无法启动，请报告错误，不要关闭其他安全防护措施。

#### 安装 Caspian 并保存密码

1. 在 **Caspian Control** 中点击 **Install / Update**。
2. 在 macOS 授权窗口中输入管理员密码。
3. 等待控制窗口显示 **Action completed**。
4. 首次安装时，保存输出中显示的 **first-run panel password**。
   **Copy panel password** 只会复制这个密码。
5. 点击 **Open panel**，使用保存的面板密码登录。
6. 输入 Wi-Fi 名称和密码，粘贴代理配置，然后使用面板开关启动 Caspian。

Mac 登录密码、Caspian 面板密码和 Wi-Fi 密码是三个不同的密码。如果面板密码丢失，
请在 Caspian Control 中使用 **Reset password**；这需要管理员授权，但会保留代理配置
和热点设置。关闭控制窗口后，应用仍留在 macOS 顶部菜单栏；从那里选择
**Open Caspian Control** 即可重新打开窗口。

### Linux 和 Raspberry Pi

#### 自动：一行命令

    sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

安装脚本会判断自己在什么机器上，从最新的发布里下载对应的二进制文件，如果下载的内容
和已公布的校验和不符就拒绝继续。

| `uname -m` | 构建产物 | 典型机器 |
|---|---|---|
| `x86_64` | `caspian-linux-amd64` | 一台笔记本或迷你主机 |
| `aarch64` | `caspian-linux-arm64` | 64 位系统上的 Raspberry Pi 3、4、5 |
| `armv7l` | `caspian-linux-arm` | 32 位系统上的 Raspberry Pi 2 和 3 |
| `armv6l` | `caspian-linux-arm` | Raspberry Pi 1、Zero、Zero W |

当它不能确定时，它拒绝，而不是猜。不是 Linux、架构不在那张表里、没有 systemd、
校验和对不上：每一种情况都是一次拒绝，并说清楚它发现了什么。`armv8l`，也就是 64 位
内核上的 32 位用户态，是刻意不做映射的，因为在那里靠猜，正是从前某个项目把 ARMv7
代码发到 ARMv6 机器上、让它们第一次运行就死于非法指令的原因。

在把脚本喂给 shell 之前先读一遍。对这一类软件来说，这句话不是客套，脚本本身也是照着
「要被人读」来写的。

    curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh | less

如果想固定某个版本而不是取最新版：

    sudo env CASPIAN_VERSION=v0.2.5 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

#### 自己验证下载的文件

每次发布都带一个 `SHA256SUMS` 文件。安装脚本会替您核对，您也可以自己独立核对：

    curl -fsSLO https://github.com/Iman/caspian/releases/latest/download/caspian-linux-arm64
    curl -fsSLO https://github.com/Iman/caspian/releases/latest/download/SHA256SUMS
    sha256sum -c SHA256SUMS --ignore-missing

这能证明什么，不能证明什么：它证明您手上的文件就是那次发布公布的文件。它不能证明那次
发布是谁构建的。这些二进制文件由 GitHub Actions 从一个打了标签的提交构建，构建它们的
工作流就在本仓库的 [`.github/workflows/release.yml`](https://github.com/Iman/caspian/blob/main/.github/workflows/release.yml) 里，所以构建过程是可读的，尽管它
并不是可独立复现的。

#### 手动：自己构建

自动那条路一点也不是必须的。从源码构建需要 Go 1.26 或更高版本，得到的二进制在功能上
完全一样。

    git clone https://github.com/Iman/caspian.git
    cd caspian
    go build -trimpath -o caspian ./cmd/caspian
    sudo CASPIAN_LOCAL_BINARY="$PWD/caspian" bash install.sh

`CASPIAN_LOCAL_BINARY` 告诉安装脚本使用您刚构建出来的文件，而不是去下载一个。安装
脚本做的其他事情，创建服务账号、目录、systemd 单元以及它们的权限，都照旧进行。

在另一台机器上为 Pi 做交叉编译：

    GOOS=linux GOARCH=arm64 go build -trimpath -o caspian-linux-arm64 ./cmd/caspian
    GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -o caspian-linux-arm ./cmd/caspian

32 位构建上的 `GOARM=6` 不是可选项。`armv6l` 和 `armv7l` 两种机器装的是同一个 `arm`
产物，所以一个按 ARMv7 构建的版本会让每一台装上它的 Pi 1、Zero 和 Zero W 都用不了。
发布工作流用 `readelf` 检查这一点，宁可失败也不发布一个谎报自己架构的产物。

在信任一个构建之前，先跑一遍门禁：

    bash scripts/gate.sh

它会跑格式检查、vet、带竞态检测器的完整测试套件、每个包的覆盖率下限、黄金回归层、
一次隐私扫描，以及一部分冒烟测试。失败时它以非零状态退出。不要把它接到管道里：shell
管道报告的是最后一条命令的状态，所以把它接给 `tail` 会把您要问的答案扔掉。

[English](https://github.com/Iman/caspian/blob/main/README.md) | [فارسی](https://github.com/Iman/caspian/blob/main/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/main/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/main/README.zh.md)
