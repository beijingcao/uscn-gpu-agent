# USCN GPU Agent

USCN GPU 服务器设备插件。自动采集硬件信息，为 GPU 资源上架核对和租赁状态记录提供元数据。

- **Linux / Windows / macOS**：GPU、CPU、操作系统、内存、磁盘；可读取的 GPU 利用率、显存使用、温度和功耗。
- 每10分钟一份基本报告，独立连接心跳、配置变化即时报告和有界离线补传。
- 一次性设备配对、Ed25519签名、持久序列；设备密钥留在本机。
- 不安装驱动、不运行模型、不读取用户文档、不接受任意远程命令。

此仓库仅包含 GPU 插件、协议、安装器和测试。USCN 平台服务端与网页不在本仓库内。独立本地 `inspect` 无需账号；连接 https://uscn.si 需要你自己的正常平台账号和设备配对码。

## 命令行安装

先登录 [GPU上架页](https://uscn.si/#/supply/gpu-listings)，生成当前空间的一次性配对码；然后在**每台实际出租服务器**执行对应命令。

### Linux / macOS

```sh
curl -fsSL https://uscn.si/install/gpu-agent.sh | bash
```

安装程序选择固定版本安装包、校验SHA256，并由你确认系统服务安装权限；按提示粘贴配对码。请在交互终端执行，以便输入配对码。Mac通用包包含Intel与Apple Silicon。

也可使用GitHub上的同一版本安装入口：

```sh
curl -fsSL https://raw.githubusercontent.com/beijingcao/uscn-gpu-agent/v1.0.1/install.sh | bash -s -- --github
```

仅下载并核对，不安装服务：

```sh
curl -fsSL https://uscn.si/install/gpu-agent.sh | bash -s -- --download-only
```

### Windows

以管理员身份打开PowerShell：

```powershell
irm https://uscn.si/install/gpu-agent.ps1 | iex
```

按提示输入设备配对码。安装原生Windows Service，保留原设备身份和已有待传报告。

安装完成后，回到GPU上架页选择设备，修正标出的差异，再核对并提交审核。

## CLI 使用

Linux/macOS安装后：

```sh
uscn-gpu-agent version
uscn-gpu-agent inspect
```

Windows安装后：

```powershell
& "$env:ProgramFiles\USCN GPU Agent\uscn-gpu-agent.exe" version
& "$env:ProgramFiles\USCN GPU Agent\uscn-gpu-agent.exe" inspect
```

已有Go环境也可以仅安装命令行工具：

```sh
go install github.com/beijingcao/uscn-gpu-agent/cmd/uscn-gpu-agent@v1.0.1
```

从源码安装CLI不会自动安装开机服务。需要服务常驻时使用上面的平台安装器。

如需手动前台运行，配置必须位于源码目录之外，配对码从标准输入读取：

```sh
uscn-gpu-agent enroll --config "$HOME/.config/uscn-gpu-agent/device.json"
uscn-gpu-agent run --config "$HOME/.config/uscn-gpu-agent/device.json"
```

默认平台地址是 `https://uscn.si`。仅在你信任的本地网络内显式配置私网、loopback或CGNAT网格IP时允许HTTP；公网HTTP地址被拒绝。不要通过命令参数、URL或公开issue发送配对码、设备密钥或SSH密码。

## 构建和测试

Go版本固定在 `go.mod`，没有外部Go模块依赖。

```sh
make fmt
make lint
make test
make build
make smoke
```

在macOS维护机上构建Linux x86_64、Windows x86_64和macOS Universal三个安装包：

```sh
python3 scripts/build-release.py --out dist/release
python3 scripts/render-installers.py --packages dist/release
```

发布目录包含安装包、各自的SHA256和构建清单；根目录`install.sh`/`install.ps1`由该清单生成并固定制品摘要。发布前运行 `python3 scripts/test-installers.py`，不在维护机安装服务。升级保留设备配置和待传报告，不会自动取消订单或解除履约义务。

## 验证范围

macOS Apple Silicon已做只读硬件采集；Intel架构、Linux/Windows制品已交叉构建。真实NVIDIA服务器、Windows/Intel Mac原生服务安装、连续真实10分钟报告和实际GPU购买交付仍需获授权设备验收。Mac包为ad-hoc签名，未Apple公证；Windows包未代码签名。

Apple GPU及统一内存独立标示，不作为NVIDIA显卡或专用显存申报。当前NVIDIA整机上架规则不能批准Apple GPU；虚拟机/WSL/容器也不能冒充物理整机。

签名报告不是防篡改硬件证明，也不单独证明SSH、模型工作负载或租赁服务可用。缺报需结合平台故障、网络、交付及双方证据复核，不自动归责或退款。

## License

[MIT](LICENSE) · Copyright 2026 beijingcao.
