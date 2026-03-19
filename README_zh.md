<div align="center">

# CCG

**CCG**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/visvioce/ccg?style=social)](https://github.com/visvioce/ccg)

基于 [CCR](https://github.com/musistudio/claude-code-router) 的 Go 语言重写版本

*轻量、快速、开箱即用*

[English](./README.md) | [中文文档](./README_zh.md)

</div>

---

## 关于项目

CCG 是 [CCR](https://github.com/musistudio/claude-code-router) 的 Go 语言重写版本。

**Vibe Coding 产物**

这是一个完全由 AI 编写的项目。整个代码库都是在与 AI 的对话中完成的——没有手动写过一行代码。如果你想体验类似的开发方式，欢迎试一试！

---

## 特性

| 特性 | 说明 |
|------|------|
| 🚀 **轻量快速** | Go 原生编译，无运行时依赖，启动秒开 |
| 💾 **低内存占用** | 服务端约 20-30MB，比 Node.js 版本节省 70%+ |
| 🖥️ **终端 UI** | 内置 TUI 界面，配置更方便 |
| 🔀 **智能路由** | 根据场景自动选择最优模型 |
| 🔌 **多提供商** | 支持 OpenAI、DeepSeek、Gemini 等多个提供商 |
| ⚡ **请求转换** | 自动适配不同提供商的 API 格式 |
| 📦 **预设管理** | 保存、分享、导入配置预设 |

---

## 快速开始

### 安装

```bash
# 一键安装（推荐）
curl -fsSL https://raw.githubusercontent.com/visvioce/ccg/master/install.sh | bash

# 或者从源码编译
git clone https://github.com/visvioce/ccg.git
cd ccr-go
make install
```

### 使用

```bash
# 启动服务
ccg start

# 打开终端 UI（配置更方便）
ccg tui

# 查看状态
ccg status

# 交互式选择模型
ccg model
```

---

## 配置

配置文件位于 `~/.ccg/config.json`

```json
{
  "PORT": "3456",
  "HOST": "127.0.0.1",
  "providers": [
    {
      "name": "openai",
      "api_base_url": "https://api.openai.com/v1/chat/completions",
      "api_key": "${OPENAI_API_KEY}",
      "models": ["gpt-4o", "gpt-4o-mini"],
      "transformer": { "use": [] }
    },
    {
      "name": "deepseek",
      "api_base_url": "https://api.deepseek.com/v1/chat/completions",
      "api_key": "${DEEPSEEK_API_KEY}",
      "models": ["deepseek-chat", "deepseek-coder"],
      "transformer": { "use": [] }
    }
  ],
  "Router": {
    "default": "openai,gpt-4o",
    "background": "openai,gpt-4o-mini",
    "think": "openai,gpt-4o",
    "longContext": "deepseek,deepseek-chat",
    "longContextThreshold": 60000,
    "webSearch": "openai,gpt-4o"
  }
}
```

---

## 命令参考

| 命令 | 说明 |
|------|------|
| `ccg start` | 启动服务 |
| `ccg stop` | 停止服务 |
| `ccg restart` | 重启服务 |
| `ccg status` | 查看状态 |
| `ccg model` | 交互式选择模型 |
| `ccg tui` | 打开终端 UI |
| `ccg preset list` | 列出预设 |
| `ccg preset export <name>` | 导出预设 |
| `ccg preset install <path>` | 安装预设 |
| `ccg activate` | 输出环境变量 |

---

## 与 CCR 的对比

<div align="center">

|  | CCR | CCG |
|--|-----|-----|
| **语言** | TypeScript/Node.js | Go |
| **内存占用** | ~150 MB | ~30 MB |
| **启动时间** | ~2 秒 | ~0.1 秒 |
| **管理界面** | Web UI (~150 MB) | TUI (~20 MB) |
| **安装方式** | `npm install -g` | 二进制下载 |
| **运行时依赖** | Node.js | 无 |

</div>

### 功能覆盖

| 功能 | CCR | CCG |
|------|:---:|:---:|
| 模型路由 | ✅ | ✅ |
| 多提供商 | ✅ | ✅ |
| 请求转换 | ✅ | ✅ |
| 流式响应 | ✅ | ✅ |
| 预设管理 | ✅ | ✅ |
| CLI 命令 | ✅ | ✅ |
| 管理界面 | Web UI | TUI |
| 自定义路由脚本 | ✅ | ❌ |
| 项目级别配置 | ✅ | ❌ |
| 日志轮转 | ✅ | ❌ |

---

## TUI 界面

CCG 内置了终端 UI，使用 `ccg tui` 即可打开：

```
┌─────────────────────────────────────────────────────────────┐
│ CCG  [Providers] [Router] [Transformers]  [Save] [Settings] │
├───────────────────────────────┬─────────────────────────────┤
│                               │  Router                     │
│  Providers                    │  ─────────────────────      │
│  ─────────────────────        │  Default:    openai,gpt-4o  │
│  [+ Add]                      │  Background: openai,gpt-4o  │
│                               │  Think:      openai,gpt-4o  │
│  ┌─────────────────────────┐  │  ...                        │
│  │ openai                  │  ├─────────────────────────────┤
│  │ api.openai.com          │  │  Transformers               │
│  │ 4 models                │  │  ─────────────────────      │
│  └─────────────────────────┘  │  [+ Add]                    │
│  ┌─────────────────────────┐  │  • maxtoken                 │
│  │ deepseek                │  │  • reasoning                │
│  │ api.deepseek.com        │  │                             │
│  │ 2 models                │  │                             │
│  └─────────────────────────┘  │                             │
├───────────────────────────────┴─────────────────────────────┤
│ 1-3: Switch | Tab: Next | ↑↓/j/k/Scroll: Navigate | q: Quit │
└─────────────────────────────────────────────────────────────┘
```

**操作方式：**
- `1` `2` `3` 切换面板
- `Tab` 下一个面板
- `↑/↓` 或 `j/k` 导航
- 滚轮滚动
- `n` 新增
- `d` 删除
- `s` 保存
- `q` 退出

---

## 路由场景

CCG 支持多种路由场景：

| 场景 | 说明 | 配置字段 |
|------|------|----------|
| **default** | 默认场景 | `Router.default` |
| **background** | 后台任务 | `Router.background` |
| **think** | 深度思考 | `Router.think` |
| **longContext** | 长上下文 | `Router.longContext` |
| **webSearch** | 网络搜索 | `Router.webSearch` |
| **image** | 图像处理 | `Router.image` |

---

## 内置 Transformers

| Transformer | 说明 |
|-------------|------|
| `anthropic` | Anthropic API 格式转换 |
| `openai` | OpenAI API 格式转换 |
| `deepseek` | DeepSeek 格式转换 |
| `gemini` | Google Gemini 格式转换 |
| `openrouter` | OpenRouter 格式转换 |
| `maxtoken` | 最大 token 限制 |
| `reasoning` | 推理模式 |
| `enhancetool` | 增强工具调用 |

---

## 环境变量

配置支持环境变量插值：

```json
{
  "api_key": "${OPENAI_API_KEY}"
}
```

会在运行时自动替换为环境变量的值。

---

## 许可证

[MIT License](LICENSE)

---

<div align="center">

**如果觉得有用，请给个 ⭐ Star 支持一下！**

[报告问题](https://github.com/visvioce/ccg/issues) · [功能建议](https://github.com/visvioce/ccg/issues) · [参与贡献](https://github.com/visvioce/ccg/pulls)

</div>
