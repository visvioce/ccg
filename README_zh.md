<div align="center">

# CCG

**Claude Code Router Go**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

轻量级的 Claude Code Router Go 实现

*AI 模型智能路由*

[English](./README.md) | [中文文档](./README_zh.md)

</div>

---

## 关于

CCG 是 [CCR](https://github.com/musistudio/claude-code-router) 的 Go 语言实现版本。由于 WSL 经常因内存占用过高而崩溃，本项目旨在提供一个轻量级的替代方案。

**核心优势**
- **低内存**: ~30MB vs ~150MB (Node.js 版本)
- **快速启动**: ~0.1s vs ~2s (Node.js 版本)
- **无依赖**: 单二进制文件，无需运行时环境

---

## 安装

```bash
# 一键安装
curl -fsSL https://raw.githubusercontent.com/visvioce/ccg/master/install.sh | bash

# 源码编译
git clone https://github.com/visvioce/ccg.git
cd ccg
make install
```

---

## 快速开始

1. **配置环境变量**:
   ```bash
   export OPENAI_API_KEY="your-key"
   export DEEPSEEK_API_KEY="your-key"
   # 根据需要添加其他提供商密钥
   ```

2. **启动服务**:
   ```bash
   ccg start
   ```

3. **打开 Web UI**:
   ```bash
   ccg ui
   ```

---

## 命令

### 服务管理
| 命令 | 说明 |
|---------|-------------|
| `ccg start` | 启动服务 |
| `ccg stop` | 停止服务 |
| `ccg restart` | 重启服务 |
| `ccg status` | 显示服务状态 |
| `ccg ui` | 在浏览器中打开 Web UI |

### 信息查询
| 命令 | 说明 |
|---------|-------------|
| `ccg model` | 列出可用模型 |
| `ccg env` | 显示环境变量 |
| `ccg activate` | 输出环境变量供 shell 使用 |
| `ccg version` | 显示版本 |

### 执行
| 命令 | 说明 |
|---------|-------------|
| `ccg code <prompt>` | 执行 AI 命令 |

### 预设
| 命令 | 说明 |
|---------|-------------|
| `ccg preset list` | 列出预设 |
| `ccg preset info <name>` | 显示预设信息 |
| `ccg preset install <source>` | 安装预设 |
| `ccg preset export <name>` | 导出预设 |
| `ccg preset delete <name>` | 删除预设 |

### 市场
| 命令 | 说明 |
|---------|-------------|
| `ccg install <name>` | 从市场安装预设 |

---

## 配置

配置文件: `~/.ccg/config.json`

```json
{
  "HOST": "127.0.0.1",
  "PORT": "3456",
  "providers": [
    {
      "name": "openai",
      "api_base_url": "https://api.openai.com/v1/chat/completions",
      "api_key": "${OPENAI_API_KEY}",
      "models": ["gpt-4o", "gpt-4o-mini"],
      "transformer": { "use": [] }
    }
  ],
  "Router": {
    "default": "openai,gpt-4o",
    "background": "openai,gpt-4o-mini",
    "think": "openai,gpt-4o",
    "longContext": "openai,gpt-4o",
    "longContextThreshold": 60000,
    "webSearch": "openai,gpt-4o"
  }
}
```

CCG 兼容任何 OpenAI 格式的 API。在 `providers` 数组中配置提供商。

---

## 路由

根据场景路由请求:

| 场景 | 说明 |
|----------|-------------|
| `default` | 通用请求 |
| `background` | 低优先级任务 |
| `think` | 复杂推理 |
| `longContext` | 长上下文 (>阈值 tokens) |
| `webSearch` | 网页搜索 |

---

## API

```bash
POST http://localhost:3456/v1/chat/completions
GET  http://localhost:3456/v1/models
```

---

## 许可证

[MIT License](LICENSE)
