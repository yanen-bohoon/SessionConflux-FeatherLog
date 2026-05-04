# SessionConflux

跨机器同步 AI 会话记录，支持[飞书云空间](https://open.feishu.cn/)和 SSH/SFTP 两种传输方式——无需自建服务。

配合 [AgentsView](https://github.com/wesm/agentsview) 实现本地浏览。将会话 JSONL 文件上传到飞书云空间或远程服务器，其他机器下载后由 AgentsView 自动发现。

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

# 3. 构建并安装（含 session-conflux + agentsview）
make install
```

`session-conflux` 和 `agentsview` 安装到 `~/SessionConflux-FeatherLog/`。

## 快速开始

```sh
# 1. 将安装目录加入 PATH
export PATH="$HOME/SessionConflux-FeatherLog:$PATH"

# 2. 配置传输方式（飞书或SSH）及 agentsview 端口
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
agentsview serve
# 浏览器打开 http://127.0.0.1:8080
# agentsview 自动监听 agent 目录，新会话实时入库，无需手动刷新
```

## 命令

### 同步工具

| 命令 | 说明 |
|---------|-------------|
| `setup` | 交互式配置传输后端（飞书或SSH） |
| `list` | 列出所有本地 AI 会话 |
| `status` | 查看同步状态摘要 |
| `upload` | 上传有变动的会话 |
| `download` | 下载会话（`--all` / `--session <key>`） |
| `sync` | 守护进程模式，每天定时自动同步 |
| `version` | 显示版本号 |

### 浏览端

| 命令 | 说明 |
|------|------|
| `agentsview serve` | 启动本地 Web 浏览端，默认 `127.0.0.1:8080` |

## 支持的 Agent

Claude Code、Codex、Gemini CLI、Copilot、Cursor、OpenCode、OpenHands、Amp、Zencoder、iFlow、VS Code Copilot、Pi、OpenClaw、Kimi、Claude.ai、ChatGPT、Kiro、Kiro IDE、Cortex、Hermes、Warp、Positron。

## 工作原理

1. **扫描** — 发现 21 个 AI agent 目录下的 JSONL 会话文件
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

`~/.session-conflux/config.toml`:

```toml
[transport]
backend = "feishu"           # "feishu" 或 "ssh"

[transport.feishu]
app_id = "cli_xxx"
app_secret = "xxx"
folder_token = ""            # 可选，留空自动创建

[transport.ssh]
host = "192.168.1.100"
port = 22
user = "your_username"
key_file = "~/.ssh/id_ed25519"
remote_path = "/data/session-conflux"

[sync]
schedule = "02:00"
direction = "both"

[agents]
exclude = ["warp"]

[compression]
level = 3
```

旧版 `[feishu]` 配置块会在首次加载时自动迁移到 `[transport]` 格式。

## 致谢

[AgentsView](https://github.com/wesm/agentsview) — 优秀的 AI 会话本地浏览工具，本项目内置了其前端浏览端。

## License

MIT
