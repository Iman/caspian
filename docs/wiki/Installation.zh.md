<div dir="ltr">

[English](https://github.com/Iman/caspian/wiki/Installation) | [فارسی](https://github.com/Iman/caspian/wiki/Installation.fa) | [Русский](https://github.com/Iman/caspian/wiki/Installation.ru) | [中文](https://github.com/Iman/caspian/wiki/Installation.zh) | [العربية](https://github.com/Iman/caspian/wiki/Installation.ar) | [Türkçe](https://github.com/Iman/caspian/wiki/Installation.tr) | [اردو](https://github.com/Iman/caspian/wiki/Installation.ur)

</div>

<a id="installation"></a>
# 安装



[有关连接图、电缆优先设置、服务重新启动和常见错误的信息，请阅读家庭用户故障排除指南。](https://github.com/Iman/caspian/wiki/Troubleshooting.zh)

CPU 和 RAM：Caspian 尚未测量最低 RAM、CPU 核心数或时钟速度。资源使用取决于流量、代理协议和同时连接。在发布最低要求之前，需要进行空闲和负载基准测试。

Linux 发行版二进制文件面向 x86-64、ARM64 和 ARMv6/ARMv7。仅架构兼容性并不能建立可用的性能。

[Caspian维基](https://github.com/Iman/caspian/wiki/Home.zh)

> 本指南来自现有的自述文件。其测量结果保留其原始日期；此文档移动不会报告新的测试运行。
> [English](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.md) | [فارسی](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.fa.md) | [Русский](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.ru.md) | [中文](https://github.com/Iman/caspian/blob/108567a6a529be05b577ee68b65b48790f07e43d/README.zh.md)

<a id="installing"></a>
## 安装中

首先选择操作系统。 Windows 和 macOS 都有图形安装程序；
Linux和Raspberry Pi可以自动安装，先检查，或构建
从源头。

<a id="windows-10-and-11"></a>
### Windows 10 和 11

安装程序会安装 Caspian 所需的一切。你不需要
PowerShell、Go 或 .NET SDK。

<a id="what-you-need"></a>
#### 你需要什么

- 在 x64 或 ARM64 上运行 Windows 10 版本 2004（内部版本 19041）或更高版本或 Windows 11 的计算机。
安装程序拒绝任何早于 Windows 10 版本 1607 的版本，这是
首先是移动热点。 1607 年至 1909 年间 Caspian 安装和面板
打开，但连接被拒绝，并显示一条命名版本的消息：那些版本
无法通过隧道发送名称查找，Caspian 不会运行
名称泄漏或停止解析的状态。
- 该计算机上的管理员帐户。
- 支持 Windows Mobile 热点的 Wi-Fi 适配器。
- 互联网连接。
- 支持的代理链接或配置。

<a id="choose-the-correct-download"></a>
#### 选择正确的下载

大多数 Intel 和 AMD 计算机使用 x64 安装程序：

- `CaspianSetup-0.2.1-windows-x64.exe`

配备 Snapdragon 或其他 ARM 处理器的 Windows 计算机使用 ARM64
安装人员：

- `CaspianSetup-0.2.1-windows-arm64.exe`

如果您不知道自己的计算机类型，请打开**设置**。选择**系统**，
然后**关于**。阅读 **系统类型** 行。

<a id="install-caspian"></a>
#### 安装Caspian

1. 打开[Caspian发布页面](https://github.com/Iman/caspian/releases/latest)。
2. 展开最新版本下的**资产**。
3. 下载正确的 Windows 安装程序。
4. 双击下载的文件。
5. 如果出现 SmartScreen，请选择 **更多信息**。
6. 确保发布者警告为您下载的文件命名。
7. 选择**仍然运行**。
8. 当 Windows 请求管理员访问权限时，选择 **是**。
9. 阅读许可证页面，然后继续。
10. 选择 Caspian Web 面板的密码。
11. 再次输入相同的密码。
12. 将此密码保存在安全的地方。

设置向导还显示两个可选选项：

- **创建桌面快捷方式**
- **登录时启动 Caspian Control**

默认情况下，这两个选项均处于关闭状态。安装程序始终创建 **Caspian Control**
Windows 开始菜单中的快捷方式。设置完成后，离开 **Open Caspian
选择 Control** 并单击 **完成**。

Windows 可以显示 **未知发布者** 警告，直到安装程序有
代码签名证书。检查该文件是否来自 Caspian 官方
在继续之前释放页面。

![Caspian Control on Windows](https://github.com/Iman/caspian/blob/main/docs/images/caspian-control-windows.png)

<a id="the-two-caspian-windows"></a>
#### 两扇Caspian之窗

Caspian 在 Windows 上有两个不同的控制屏幕。

| 屏幕 | 在哪里打开 | 它控制什么 |
|---|---|---|
| **Caspian控制** | 一个小的 Windows 应用程序和通知区域图标 | 启动、停止或重新启动 Caspian 后台服务 |
| **Caspian网络面板** | 您的网络浏览器位于 `http://127.0.0.1:8088/` | 设置 Wi-Fi 名称、Wi-Fi 密码、频段和代理连接 |

首先使用 **Caspian Control**。等待**就绪**，然后选择**打开面板**。
网络面板是第二个屏幕。用它来启动热点和隧道。

Caspian Control 中的 **Ready** 意味着两个后台服务已应答。
这并不意味着代理隧道已连接。网页面板变成绿色
当热点和隧道准备就绪时。

<a id="first-start"></a>
#### 第一次开始

1. 从 Windows 开始菜单或桌面打开 **Caspian Control**。
2. 当 Windows 请求管理员访问权限时，选择 **是**。
3. 选择**全部启动**。
4. 等到大卡片显示“**就绪**”。
5. 选择**打开面板**。
6. 输入您在设置过程中选择的面板密码。
7. 选择**登录**。
8. 输入新 Wi-Fi 网络的名称。
9. 输入至少八个字符的 Wi-Fi 密码。
10. 保留 **2.4 GHz** 以获得对旧设备的最佳支持。
11. 粘贴您的代理链接或配置。
12. 选择启动 Caspian 的开关。
13. 等待网络面板状态变为绿色。
14. 将您的手机或其他设备连接到新的 Wi-Fi 网络。
15. 在该设备上打开一个网站来测试连接。

该面板显示每个连接的设备。 Windows 给出这些设备地址
来自 `192.168.137.0/24`。 Caspian 通过以下方式发送互联网流量
`xray0`隧道。

面板密码和Wi-Fi密码不同。面板密码打开
网络面板。 Wi-Fi 密码连接手机和其他设备。

<a id="what-the-caspian-control-buttons-do"></a>
#### Caspian 控制按钮的作用

| 控制 | 结果 |
|---|---|
| **开始全部** | 启动两个 Caspian 后台服务 |
| **全部停止** | 停止这两项服务并使它们保持停止状态 |
| **全部重新启动** | 停止和启动这两个服务 |
| **打开面板** | 在浏览器中打开 Caspian 网页面板 |

关闭窗口后，该应用程序将保留在 Windows 通知区域中。
通知区域位于时钟旁边。双击 Caspian 图标
再次打开应用程序。

<a id="what-to-expect"></a>
#### 会发生什么

Windows 请求管理员访问权限，因为 Caspian 更改了网络路由，
防火墙、移动热点和 Wintun 网络适配器。

当您停止或重新启动热点时，Windows 会断开设备连接。等待
Web 面板变为绿色，然后重新连接每个设备。

如果 Caspian Control 显示 **Ready** 但 Web 面板为红色，请阅读以下消息：
网络面板。 Web 面板测试热点和代理隧道。

<a id="developer-requirements"></a>
#### 开发者要求

下面的 PowerShell 方法适用于开发人员。它由此构建了 Caspian
存储库并安装 Windows 服务。

此方法需要这些附加程序：

- 管理员帐户。
- 有效的互联网连接。
- 支持 Windows Mobile 热点的 Wi-Fi 适配器。
- [适用于 Windows 的 Git](https://git-scm.com/download/win)。
- [转到 1.26 或更高版本](https://go.dev/dl/)。
- [.NET 9 SDK](https://dotnet.microsoft.com/download/dotnet/9.0)。

安装程序和开发者方法支持x64和ARM64 Windows系统。

<a id="developer-install"></a>
#### 开发者安装

1. 打开 PowerShell。
2. 克隆此存储库。
3. 更改到存储库目录。
4. 运行 Windows 安装程序。

```powershell
git clone https://github.com/Iman/caspian.git
Set-Location caspian
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\install.ps1
```

5. 当 Windows 请求管理员访问权限时，单击 **是**。
6. 等待构建和服务安装完成。

安装程序执行以下任务：

- 为当前计算机构建 `caspian.exe`。
- 构建 Windows Mobile 热点帮助程序。
- 构建 `CaspianControl.exe` 托盘应用程序。
- 当 `wintun.dll` 不存在时下载 Wintun 0.14.1。
- 将 Wintun 存档与其固定 SHA-256 值进行比较。
- 安装`C:\Program Files\Caspian`中的程序。
- 创建 `caspian` 和 `caspian-panel` Windows 服务。
- 将这两个服务设置为自动启动。
- 创建 **Caspian Control** 桌面快捷方式。
- 等待本地面板最多 45 秒。
- 面板应答后打开 Caspian Control。

使用 `-NoOpen` 无需打开托盘应用程序即可安装：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\install.ps1 -NoOpen
```

<a id="repair-or-update"></a>
#### 修复或更新

再次运行相同的安装程序。安装程序停止服务，替换
程序，保留面板状态，并再次启动服务。

<a id="uninstall"></a>
#### 卸载

在存储库中打开管理员 PowerShell。然后运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\packaging\windows\uninstall.ps1
```

卸载程序会删除 Windows 服务和已安装的程序。阅读
当您需要保留本地状态时，请在使用前编写脚本。

<a id="macos-13-or-later"></a>
### macOS 13 或更高版本

macOS 磁盘映像包含本机 **Caspian Control** 应用程序和
Caspian发动机。您不需要 Terminal、Go、Homebrew 或其他运行时。
您需要一个管理员帐户，并且当内置 Wi-Fi 为热点时，
有线以太网互联网连接。

<a id="choose-the-correct-download-1"></a>
#### 选择正确的下载

- Intel Mac 使用 `Caspian-v0.2.4-macos-amd64.dmg`。
- Apple Silicon Mac（M1 或更高版本）使用 `Caspian-v0.2.4-macos-arm64.dmg`。

如果您不知道 Mac 是否有，请打开 **Apple 菜单 → 关于本机**
英特尔处理器或苹果芯片。

<a id="install-and-approve-the-first-opening"></a>
#### 安装并批准首次开放

v0.2.4 应用程序是临时签名的，但尚未与 Apple 开发人员签名
ID 或经过 Apple 公证。因此，Gatekeeper 显示**“Caspian”未打开**
并表示苹果无法验证其是否没有恶意软件。这不是一个
应用程序崩溃。仅针对从以下位置下载的文件覆盖警告
Caspian 官方发布页面。

1. 打开[最新的Caspian版本](https://github.com/Iman/caspian/releases/latest)
并展开**资产**。
2. 下载适用于 Mac 处理器的 DMG 并打开它。
3. 将 `Caspian.app` 拖到 **Applications** 文件夹中。
4. 在**应用程序**中打开副本一次。
5. 当 Gatekeeper 阻止它时，单击 **完成**。
6. 打开 **Apple 菜单 → 系统设置 → 隐私和安全**。
7. 滚动到 **安全**，然后单击 Caspian 旁边的 **仍然打开**。按钮
在尝试打开被阻止后大约一小时内仍然可用。
8. 输入Mac登录密码，点击**确定**，然后确认**打开**。

macOS 将此应用程序保存为例外，因此以后可以通过双击打开
正常情况下。苹果公司在文档中记录了相同的过程
[通过覆盖安全设置打开应用程序](https://support.apple.com/guide/mac-help/apple-cant-check-app-for-malicious-software-mchleab3a043/26/mac/26)。

<a id="if-macos-still-blocks-the-background-service"></a>
#### 如果 macOS 仍然阻止后台服务

安装的后台可执行文件 `/usr/local/bin/caspian` 可以保留
您批准 `Caspian.app` 后的隔离标志。警告名称小写
`caspian`，控制窗口可以报告**Caspian需要注意**。

**如果警报命名为 Trojan 或报告恶意软件，请勿使用以下命令。**
停止安装并报告确切的警报、检测名称、发布版本和
下载地址为[GitHub问题](https://github.com/Iman/caspian/issues)。
恶意软件检测需要调查；单独未签名的版本并不
确定检测是错误的。参见
[Apple 对 macOS 安全警报的解释](https://support.apple.com/en-ie/102445)。

仅针对未经验证的开发人员或未经公证的应用程序警告使用此后备，
在您信任该文件及其来源之后。从官方下载release版本
Caspian 发布页面并将 DMG 校验和与其发布的进行比较
`SHA256SUMS`。匹配的校验和确认发布文件，而不是其安全性。

1. 打开**终端**。
2. 从已安装的后台可执行文件中删除隔离标志：

   ```bash
   sudo xattr -d com.apple.quarantine /usr/local/bin/caspian
   ```

3. 输入您的 Mac 登录密码。终端不会在您键入时显示密码。
4. 在 Caspian 中，选择 **高级选项 → 重新启动服务**。

此命令仅删除指定文件的隔离属性。它不
扫描、签署或公证可执行文件。如果终端报告`No such xattr`，则
属性已经不存在。如果服务还是失败，报错
而不是删除其他安全控制。

<a id="let-caspian-set-itself-up-and-save-its-password"></a>
#### 让 Caspian 设置自身并保存其密码

1. 启动**Caspian控制**。它将捆绑的后台服务与
在检查面板之前安装的一个。
2. 首次启动时，或当 DMG 包含更新时，安装程序开始
自动。在macOS授权中输入管理员密码
对话框。当安装的版本已经匹配时，不需要密码。
3. 等到控制窗口显示 **Caspian 已准备就绪**。如果授权是
已取消，**设置 Caspian** 或 **更新 Caspian** 仍然可见以供重试。
4. 首次安装时，保存显示在的**首次运行面板密码**
输出。 **复制面板密码** 仅复制该密码。
5. 单击“**打开面板**”并使用保存的面板密码登录。
6. 输入Wi-Fi名称和密码，粘贴代理配置，然后使用
面板开关启动 Caspian。

Mac登录密码、Caspian面板密码、Wi-Fi密码三个
不同的密码。如果面板密码丢失，请使用**重置密码**
Caspian控制；需要管理员授权，但是保存的代理
并保留热点设置。关闭控制窗口保留其菜单栏
项目运行；选择 **Open Caspian Control** 来重新打开它。

在 macOS 上，关闭控制窗口会使 Caspian 保留在菜单栏中。选择
**退出 Caspian 并停止服务** 停止热点和后台服务。
如果 macOS 授权被取消或停止失败，应用程序将保持打开状态。
下次启动时，Caspian 将在管理员授权的情况下启动一次已停止的服务。

<a id="linux-and-raspberry-pi"></a>
### Linux 和树莓派

<a id="automated-one-line"></a>
#### 自动化：一条线

sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"

安装程序确定它所在的机器，下载匹配的二进制文件
从最新版本开始，如果下载与版本不符则拒绝
发布的校验和。

| `uname -m` | 人工制品 | 典型机器 |
|---|---|---|
| `x86_64` | `caspian-linux-amd64` | 笔记本电脑或迷你电脑 |
| `aarch64` | `caspian-linux-arm64` | 64 位系统上的 Raspberry Pi 3、4、5 |
| `armv7l` | `caspian-linux-arm` | 32 位系统上的 Raspberry Pi 2 和 3 |
| `armv6l` | `caspian-linux-arm` | Raspberry Pi 1、零、零 W |

当它不能确定时，它会拒绝而不是猜测。不是 Linux，而是
架构不在该表中，没有 systemd，或者校验和不匹配：
每一个都是拒绝命名它所发现的东西。 `armv8l`，64位上的32位用户区
内核，故意不映射，因为猜测以前是如何
项目将 ARMv7 代码发送到 ARMv6 机器，并让它们死去
第一次运行时的非法指令。

在将脚本通过管道传输到 shell 之前，请先阅读该脚本。这个建议不是一个形式
对于此类软件，脚本是为了可读而编写的。

卷曲-fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh |少

上面的命令显示脚本；它不会安装或更新 Caspian。
再次运行安装命令进行升级。安装程序选择最新的
发布版本并保留您保存的设置。该门户包含在
二进制。 `main` 上的更改会在发布包含这些更改后出现。

要安装特定版本，请替换下面的示例标签：

sudo env CASPIAN_VERSION=v0.2.5 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Iman/caspian/main/install.sh)"


<a id="forgotten-panel-password"></a>
### 忘记面板密码

在 Caspian 计算机上，在终端或 SSH 会话中运行以下命令：

```bash
sudo /usr/local/bin/caspian reset-password
```

该命令打印新的面板密码并重新启动面板。您的代理人和
Wi-Fi 设置保持保存。在登录页面上使用新密码。在 Windows 上，
以管理员身份运行`& "$env:ProgramFiles\Caspian\caspian.exe" reset-password`
PowerShell 窗口。重新安装 Caspian 不会重置您的密码。

<a id="verifying-a-download-yourself"></a>
### 自行验证下载

每个版本都带有 `SHA256SUMS` 文件。安装程序会为您检查，然后
您可以独立检查：

卷曲-fsSLO https://github.com/Iman/caspian/releases/latest/download/caspian-linux-arm64
卷曲-fsSLO https://github.com/Iman/caspian/releases/latest/download/SHA256SUMS
sha256sum -c SHA256SUMS --忽略缺失

这证明了什么，没有证明什么：它证明你拥有的文件就是文件
发布的版本。它并不能证明是谁构建了该版本。二进制文件
由 GitHub Actions 从标记的提交构建，以及构建的工作流程
它们位于 [`.github/workflows/release.yml`](https://github.com/Iman/caspian/blob/main/.github/workflows/release.yml) 的存储库中，因此构建是
即使它不能独立重现，但仍具有可读性。

<a id="manual-build-it-yourself"></a>
#### 手册：自己构建

不需要任何关于自动路线的信息。从源代码构建需要 Go
1.26 或更高版本并给出功能相同的二进制文件。

git 克隆 https://github.com/Iman/caspian.git
CaspianCD
去构建-trimpath -o caspian ./cmd/caspian
sudo CASPIAN_LOCAL_BINARY="$PWD/caspian" bash install.sh

`CASPIAN_LOCAL_BINARY` 告诉安装程序使用您刚刚构建的文件
比下载一个。安装程序执行的其他所有操作（创建服务）
帐户、目录、单元及其权限也是如此。

从另一台机器交叉编译 Pi：

GOOS=linux GOARCH=arm64 go build -trimpath -o caspian-linux-arm64 ./cmd/caspian
GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -o caspian-linux-arm ./cmd/caspian

32 位版本上的 `GOARM=6` 不是可选的。 `armv6l` 和 `armv7l`
机器安装相同的 `arm` 工件，因此 ARMv7 构建会破坏每个 Pi 1，
安装它的零和零W。发布工作流程会检查这一点
`readelf` 并且失败了，而不是发布关于其谎言的人工制品
架构。

在信任构建之前，请运行gate：

bash 脚本/gate.sh

它运行格式化、审查、带有种族检测器的整个套件、每个包
覆盖层、黄金回归层、隐私扫描和烟雾
子集。失败时它以非零值退出。不要在任何地方进行管道传输：外壳管道
报告其最后一个命令的状态，因此将其通过管道输送到 `tail` 会被丢弃
你所要求的答案。



<!-- SNI upstream credits -->

SNI 欺骗来源：[patterniha/SNI-Spoofing](https://github.com/patterniha/SNI-Spoofing) (GPL-3.0)，以及 Windows x64 上的 WinDivert (LGPL-3.0)。
[第三方许可证、源版本和积分](https://github.com/Iman/caspian/blob/feature/sni/docs/THIRD-PARTY.md)。

<!-- Caspian guide navigation -->

Caspian指南：[设置和支持的协议](https://github.com/Iman/caspian/wiki/Home.zh)·[用于 DPI 规避的 SNI 欺骗：设置和限制](https://github.com/Iman/caspian/wiki/SNI-Spoofing.zh)。


<!-- English-source-sha256: b5ed00f600b06aa55c5250f3ac9465f05c5905e24ac9ae230aa23c100d6626c5 -->
