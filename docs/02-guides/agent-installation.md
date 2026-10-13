# Agent 安装

目标机器无需 Go 或源码。一行命令下载 GitHub 最新 Release 中匹配系统和架构的 Agent，校验 SHA256 后安装为开机自启服务。Linux/macOS 需要管理员权限；Windows 允许普通 PowerShell 启动，自动弹出 UAC，等待安装结束并返回结果。

## 平台范围

| 系统 | 架构 | 服务管理 | 默认目录 | 要求 |
| --- | --- | --- | --- | --- |
| Linux | x64 / ARM64 | systemd | `/opt/gaoming-agent` | 已运行的 systemd、curl、tar、sha256sum 或 shasum |
| macOS | Intel x64 / Apple Silicon ARM64 | launchd | `/usr/local/gaoming-agent` | curl、tar、shasum |
| Windows | x64 / ARM64 | Windows Service（WinSW） | `%ProgramFiles%\GaomingAgent` | Windows PowerShell 5.1 或 PowerShell 7；ARM64 使用 .NET Framework 4.6.1+ 的 WinSW 包装器 |

不支持 32 位系统、无 systemd 的 Linux 或容器内服务安装；这些环境不会被静默安装为无法自启的后台进程。目标机器需要访问 GitHub、raw.githubusercontent.com、GitHub Release 下载域名以及配置的服务端。

## 一行安装

Linux / macOS：

```sh
curl -fsSL https://raw.githubusercontent.com/gofxq/gaoming/master/deployments/install-agent.sh | sudo sh
```

Windows，在 PowerShell 中执行以下一行：

```powershell
$p=Join-Path $env:TEMP ('gaoming-install-'+[guid]::NewGuid()+'.ps1'); try { iwr https://raw.githubusercontent.com/gofxq/gaoming/master/deployments/install-agent.ps1 -UseBasicParsing -OutFile $p; powershell -NoProfile -ExecutionPolicy Bypass -File $p; if ($LASTEXITCODE -ne 0) { throw 'Agent installation failed' } } finally { Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue }
```

Windows 先将脚本下载到随机临时文件，然后以文件方式运行，避免 `iwr | iex` 在 UAC 提权时丢失脚本路径。`ExecutionPolicy Bypass` 只用于此次 PowerShell 进程。UAC 是操作系统授权，默认无配置交互不代表绕过 UAC。

默认 Web 地址 `https://gm-metric.gofxq.com/`，gRPC 地址 `gm-rpc.gofxq.com:443`，采集间隔 5 秒；tenant 留空时向 Web 服务申请，失败则生成本地随机 tenant。安装结束输出配置位置和 Dashboard 地址；查看 Dashboard 仍需完成服务端 setup / 登录及租户授权。

## 自建服务与配置

Linux / macOS 在命令末尾传递参数：

```sh
curl -fsSL https://raw.githubusercontent.com/gofxq/gaoming/master/deployments/install-agent.sh | sudo sh -s -- --web-url https://monitor.example.com --ingest-grpc-addr monitor.example.com:443 --tenant team
```

Windows 在上述一行的 `-File $p` 后增加 `-WebBaseUrl https://monitor.example.com -IngestGatewayGrpcAddr monitor.example.com:443 -TenantCode team`。内网直连可使用 `服务器地址:8091`。自建服务优先显式指定自己的 tenant。

| 用途 | Unix 参数 | PowerShell 参数 |
| --- | --- | --- |
| 固定版本 | `--version vX.Y.Z` | `-Version vX.Y.Z` |
| 配置交互 | `--interactive` | `-Interactive` |
| 仅更新（要求已安装） | `--update-binary` | `-UpdateBinary` |
| 重写配置 | `--reinstall` | `-Reinstall` |
| 采集间隔 | `--loop-interval-sec 5` | `-LoopIntervalSec 5` |
| 区域 / 环境 / 角色 | `--region` / `--env` / `--role` | `-Region` / `-EnvName` / `-Role` |

默认重复执行仅更新程序，配置文件字节内容保持不变；传入新配置参数时需搭配 `--reinstall` / `-Reinstall` 才会生效。交互方式会以已有配置作为默认值。更新前先校验下载内容并解包，随后停止旧服务、原子替换程序并重启；升级失败时尝试恢复旧文件和服务，并返回失败结果。安装成功表示服务已启动，不代表已经成功连接远端；通过日志和 Dashboard 确认上报。

Windows 会在默认新目录尚无配置时复制 `%LOCALAPPDATA%\GaomingAgent\agent-config.yaml`，保留旧 tenant；服务改为使用 Program Files。旧目录不会自动删除，可在确认新服务正常后手动清理。通过其他管理员账户进行 UAC 授权时，请先手动迁移原账户的配置。自定义 Windows 安装目录必须位于 Program Files 的子目录，禁止目录联接和符号链接。

Linux 服务使用独立用户，程序和安装目录由 root 持有，只有配置和日志对服务用户可写。macOS 的 launchd 服务以 root 运行。Windows 默认服务以 SYSTEM 运行，安装目录仅管理员和 SYSTEM 可修改。

## 查看服务与卸载

Linux：`sudo systemctl status gaoming-agent`，日志位于 `/opt/gaoming-agent/logs/agent.log`。macOS：`sudo launchctl print system/com.gofxq.gaoming-agent`，日志位于 `/usr/local/gaoming-agent/logs/agent.log`。Windows：`Get-Service GaomingAgent`，日志位于 `%ProgramFiles%\GaomingAgent\logs`。

Linux / macOS 卸载：

```sh
curl -fsSL https://raw.githubusercontent.com/gofxq/gaoming/master/deployments/uninstall-agent.sh | sudo sh
```

Windows PowerShell 卸载：

```powershell
$p=Join-Path $env:TEMP ('gaoming-uninstall-'+[guid]::NewGuid()+'.ps1'); try { iwr https://raw.githubusercontent.com/gofxq/gaoming/master/deployments/uninstall-agent.ps1 -UseBasicParsing -OutFile $p; powershell -NoProfile -ExecutionPolicy Bypass -File $p; if ($LASTEXITCODE -ne 0) { throw 'Agent uninstallation failed' } } finally { Remove-Item -LiteralPath $p -Force -ErrorAction SilentlyContinue }
```

卸载删除服务及其安装目录中的配置和日志，需要保留时请提前备份。

## 发布和验证

`.github/workflows/build-agent.yml` 在三个操作系统上测试安装脚本，构建六份二进制归档；推送 `v*` tag 时发布二进制、安装/卸载脚本和 `checksums.txt`。推送 master 会运行测试和构建，只有推送 `v*` tag 才会创建 Release。安装命令从 master 下载脚本、从最新 Release 下载二进制；已有 Release 包含对应二进制和 `checksums.txt` 时，更新安装脚本只需合入 master，无需重新发布。修改 Agent 程序或需要将脚本一起归档到 Release 时，再发布新的 tag。也可从指定 Release 下载同版本脚本，避免主分支脚本与历史版本差异。

本地执行 `make test-agent-installers` 和 `make test`。PowerShell 测试执行 `powershell -NoProfile -ExecutionPolicy Bypass -File deployments/tests/install-agent.Tests.ps1` 或 `pwsh -NoProfile -File deployments/tests/install-agent.Tests.ps1`。测试不修改宿主机服务；真实 UAC、systemd/launchd/SCM 安装及上报需在目标机器验收。

## 安全提醒

一行安装会以管理员权限执行远程脚本，请只使用可信仓库；需要审阅时先下载并检查脚本再执行。Agent 归档通过 Release 的 SHA256 清单校验，[WinSW v2.12.0](https://github.com/winsw/winsw/releases/tag/v2.12.0) 使用脚本内固定 SHA256。SHA256 防止下载损坏，不能替代对发布账户和脚本来源的信任。
