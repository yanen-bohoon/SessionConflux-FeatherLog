# SessionConflux

跨机器同步 AI 会话记录，支持[飞书云空间](https://open.feishu.cn/)和 SSH/SFTP 两种传输方式——无需自建服务。

内置 [AgentsView](https://github.com/wesm/agentsview) 浏览端，支持 Web 界面和 macOS 桌面应用。云同步功能已深度集成，在 GUI 中一键上传/下载会话，配置定时自动同步。

## 安装

### 1. 安装 Go

**macOS**

```bash
brew install go
```

**Linux**

```bash
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

**Windows**

[下载安装包](https://go.dev/dl/) 或 `winget install GoLang.Go`

验证安装：

```bash
go version  # 需显示 go1.21 以上
```

### 2. 克隆并构建

```bash
# 1. 克隆仓库
git clone https://github.com/yanen-bohoon/SessionConflux-FeatherLog.git

# 2. 进入项目目录
cd SessionConflux-FeatherLog

# 3. 构建并安装（含同步功能 + Web UI）
make install
```

`session-conflux` 统一 binary 安装到 `~/SessionConflux-FeatherLog/`。

### 3. （可选）macOS 桌面应用

下载 `SessionConflux.app`，双击运行，菜单栏出现图标。无需安装 Go 或 Node.js。

```bash
# 构建 .app（需 Rust 工具链）
make desktop-macos-app
# .app 输出到 dist/desktop/macos/SessionConflux.app
```

桌面应用内置云同步界面：菜单栏图标 → 设置 → 云同步，配置飞书或 SSH 后一键上传/下载。

## 快速开始

```sh
# 1. 将安装目录加入 PATH
export PATH="$HOME/SessionConflux-FeatherLog:$PATH"

# 2. 配置传输方式（飞书或SSH）及端口
# 重跑 setup 即可修改配置，每项按回车保留原值
session-conflux setup

# 3. 查看本地会话
session-conflux list

# 4. 上传会话到远端
session-conflux upload

# 5. 下载其他机器的会话（可选）
session-conflux download --all

# 6. 启动守护进程，每天自动同步（可选）
session-conflux sync

# 7. 启动本地浏览端（端口在 setup 时已配置，默认 8080）
session-conflux serve
# 浏览器打开 http://127.0.0.1:8080
# 自动监听 agent 目录，新会话实时入库，无需手动刷新
```

## 命令

### 云同步

| 命令 | 说明 |
|---------|-------------|
| `setup` | 交互式配置传输后端（飞书或SSH） |
| `list` | 列出所有本地 AI 会话 |
| `status` | 查看同步状态摘要 |
| `upload` | 上传有变动的会话 |
| `download` | 下载会话（`--all` / `--session <key>`） |
| `sync` | 守护进程模式，每天定时自动同步 |

### 核心/管理

| 命令 | 说明 |
|------|------|
| `serve` | 启动本地 Web 浏览端，默认 `127.0.0.1:8080` |
| `file-sync` | 手动触发本地文件扫描与索引 |
| `projects` | 列出所有项目及其会话数 |
| `health` | 检查会话完整性与信号 |
| `stats` | 生成工作空间分析报告 |
| `usage` | Token 消耗与成本统计 |
| `version` | 显示版本号 |

Web 端顶栏"云同步"按钮提供可视化上传/下载/状态查看，设置页可配置全部同步参数。桌面端右键菜单栏图标同样可用。

## 支持的 Agent

| Agent | 说明 |
|-------|------|
| Claude Code | Anthropic 官方 CLI agent |
| Codex | OpenAI 官方 CLI agent |
| Gemini CLI | Google 官方 CLI agent |
| Copilot | GitHub Copilot Chat |
| Cursor | Cursor 编辑器 AI 会话 |
| OpenCode | 开源 CLI coding agent |
| OpenHands | 开源 CLI coding agent |
| Amp | CLI coding agent |
| Zencoder | CLI coding agent |
| iFlow | CLI coding agent |
| VS Code Copilot | VS Code 内 Copilot Chat 会话 |
| Pi | CLI coding agent |
| OpenClaw | 开源 agent 框架 |
| Kimi | Moonshot AI agent |
| Claude.ai | claude.ai 网页版 |
| ChatGPT | chatgpt.com 网页版 |
| Kiro | 开源 CLI agent |
| Kiro IDE | Kiro IDE 内置 AI |
| Cortex | Snowflake CLI agent |
| Hermes | 开源 agent 框架 |
| Warp | Warp 终端 AI |
| Positron | Positron IDE 内置 AI |
| CodeBuddy | 腾讯 CodeBuddy CLI agent |
| WorkBuddy | 腾讯 WorkBuddy 桌面端 agent |

## 工作原理

1. **扫描** — 发现 AI agent 目录下的 JSONL 会话文件
2. **打包** — 首次运行将所有会话打成 tar.zst 压缩包上传（基线）
3. **增量** — 后续仅上传文件大小有变化的单个会话
4. **下载** — 合并基线压缩包 + 增量文件，写入对应 agent 目录
5. **AgentsView** — fsnotify 自动发现新文件，解析入库

### 文件夹结构

```
远程存储（飞书云空间 或 SSH 服务器）:
  SessionConflux/                  # 根目录
    ├─ mac-studio/                 # 机器名
    │  ├─ baseline/                # 基线压缩包
    │  │  └─ bundle.tar.zst.partNN
    │  └─ incremental/             # 增量文件
    │     └─ claude/session_id.jsonl.zst
    └─ thinkpad/
       └─ ...
```

## 配置
配置统一存储在 `~/.session-conflux/config.toml`：

```toml
[sync]
enabled = true
```

schedule = "02:00"
direction = "both"
compression_level = 3

[sync.transport]
backend = "feishu"           # "feishu" 或 "ssh"

[sync.transport.feishu]
app_id = "cli_xxx"
app_secret = "xxx"
folder_token = ""            # 可选，留空自动创建

[sync.transport.ssh]
host = "192.168.1.100"
port = 22
user = "your_username"
key_file = "~/.ssh/id_ed25519"
remote_path = "/data/session-conflux"
```

运行 `session-conflux setup` 交互式配置，或通过 Web 设置页（http://127.0.0.1:8080 → 设置 → 云同步）可视化配置。

旧版 `~/.session-conflux/config.toml` 和 `[feishu]` 配置块会在首次启动时自动迁移到 `~/.agentsview/config.toml` 的 `[sync]` 块，原文件加 `.bak` 后缀保留。

## 致谢

[AgentsView](https://github.com/wesm/agentsview) — 优秀的 AI 会话本地浏览工具，本项目内置了其功能。

## License

MIT
