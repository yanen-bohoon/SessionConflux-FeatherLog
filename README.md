# SessionConflux

跨机器同步 AI 会话记录，基于[飞书云空间](https://open.feishu.cn/)传输——无需自建服务器。

配合 [AgentsView](https://github.com/wesm/agentsview) 实现本地浏览。将会话 JSONL 文件上传到飞书云空间，其他机器下载后由 AgentsView 自动发现。

## 安装

从 [Releases](https://github.com/yanen-bohoon/SessionConflux-FeatherLog/releases) 下载对应平台的二进制文件。

或源码编译：

```
git clone https://github.com/yanen-bohoon/SessionConflux-FeatherLog.git
cd SessionConflux-FeatherLog
make build
```

## 快速开始

```sh
# 1. 配置飞书凭证
session-conflux config

# 2. 查看本地会话
session-conflux list

# 3. 上传到飞书云空间
session-conflux upload

# 4. 在另一台机器上下载全部会话
session-conflux download --all

# 5. 或启动守护进程，每天自动同步
session-conflux sync
```

## 命令

| 命令 | 说明 |
|---------|-------------|
| `config` | 交互式配置飞书凭证 |
| `list` | 列出所有本地 AI 会话 |
| `upload` | 上传有变动的会话到飞书云空间 |
| `download` | 从飞书云空间下载会话（`--all` / `--session <key>`） |
| `sync` | 守护进程模式，每天定时自动同步 |
| `version` | 显示版本号 |

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
飞书云空间:
  SessionConflux/                  # L1
    ├─ mac-studio/                 # L2 (机器名)
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
[feishu]
app_id = "cli_xxx"
app_secret = "xxx"

[sync]
schedule = "02:00"
direction = "both"

[agents]
exclude = ["warp"]

[compression]
level = 3
```

## License

MIT
