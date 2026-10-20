USCN GPU 插件 1.0.1

1. 登录 https://uscn.si/#/supply/gpu-listings，生成当前空间的一次性设备配对码。
2. 在每台实际出租服务器安装本包。
   macOS：双击 install.command，按提示输入本机管理员密码及配对码。
   Linux：解压后运行 sudo bash install.sh，按提示粘贴配对码。
   Windows：以管理员身份在 PowerShell 中运行 install.ps1。
3. 返回上架页，选择设备，查看并修正差异，再提交审核。

插件开机启动，主动连接平台。首次、重新采集和配置变化会即时上报；
基本设备报告每10分钟一份，连接心跳单独运行。断线期间只缓存设备元数据。
无需给插件提供SSH密码、第三方API Key或ChatGPT授权。
不自动安装驱动、重启、执行压力测试或模型请求。

Mac：包含Intel与Apple Silicon。Apple GPU/统一内存按实际类型报告，
不能作为NVIDIA显卡或专用NVIDIA显存申报。

升级时再次运行安装程序，保留原设备配对与待传报告。
停止或卸载插件不会取消现有订单，也不会解除履约义务。
安装包经HTTPS提供并附SHA256SUMS；未宣称具备Apple公证或Windows代码签名。

开源代码：https://github.com/beijingcao/uscn-gpu-agent（MIT）
命令行安装：Linux/macOS执行 curl -fsSL https://uscn.si/install/gpu-agent.sh | bash
Windows管理员PowerShell执行 irm https://uscn.si/install/gpu-agent.ps1 | iex
Linux/macOS安装后可运行 uscn-gpu-agent version 或 uscn-gpu-agent inspect。
