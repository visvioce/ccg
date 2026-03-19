<div align="center">

# CCG

**Claude Code Gateway (Go)**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub Stars](https://img.shields.io/github/stars/visvioce/ccg?style=social)](https://github.com/visvioce/ccg)

A Go rewrite of Claude Code Router, based on [CCR (Claude Code Router)](https://github.com/musistudio/claude-code-router)

*Lightweight, Fast, Ready to Use*

[English](./README.md) | [中文文档](./README_zh.md)

</div>

---

## About

CCG is a Go language rewrite of [CCR (Claude Code Router)](https://github.com/musistudio/claude-code-router).

**Vibe Coding Product**

This project was entirely written by AI. The entire codebase was completed through conversation with AI - not a single line of code was written manually. If you want to experience a similar development approach, feel free to give it a try!

---

## Features

| Feature | Description |
|---------|-------------|
| 🚀 **Lightweight & Fast** | Native Go compilation, no runtime dependencies, instant startup |
| 💾 **Low Memory** | ~20-30MB server memory, 70%+ savings vs Node.js version |
| 🖥️ **Terminal UI** | Built-in TUI for easy configuration |
| 🔀 **Smart Routing** | Automatically selects optimal model based on scenario |
| 🔌 **Multi-Provider** | Supports OpenAI, DeepSeek, Gemini and more |
| ⚡ **Request Transform** | Auto-adapts to different provider API formats |
| 📦 **Preset Management** | Save, share, and import configuration presets |

---

## Quick Start

### Installation

```bash
# One-line install (recommended)
curl -fsSL https://raw.githubusercontent.com/visvioce/ccg/main/install.sh | bash

# Or build from source
git clone https://github.com/visvioce/ccg.git
cd ccr-go
make install
```

### Usage

```bash
# Start server
ccg start

# Open terminal UI
ccg tui

# Check status
ccg status

# Interactive model selection
ccg model
```

---

## Configuration

Config file location: `~/.ccg/config.json`

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

## Command Reference

| Command | Description |
|---------|-------------|
| `ccg start` | Start server |
| `ccg stop` | Stop server |
| `ccg restart` | Restart server |
| `ccg status` | Show status |
| `ccg model` | Interactive model selection |
| `ccg tui` | Open terminal UI |
| `ccg preset list` | List presets |
| `ccg preset export <name>` | Export preset |
| `ccg preset install <path>` | Install preset |
| `ccg activate` | Output environment variables |

---

## CCR vs CCG

<div align="center">

|  | CCR | CCG |
|--|-----|-----|
| **Language** | TypeScript/Node.js | Go |
| **Memory Usage** | ~150 MB | ~30 MB |
| **Startup Time** | ~2 sec | ~0.1 sec |
| **Management UI** | Web UI (~150 MB) | TUI (~20 MB) |
| **Install Method** | `npm install -g` | Binary download |
| **Runtime Deps** | Node.js | None |

</div>

### Feature Coverage

| Feature | CCR | CCG |
|---------|:---:|:---:|
| Model Routing | ✅ | ✅ |
| Multi-Provider | ✅ | ✅ |
| Request Transform | ✅ | ✅ |
| Streaming Response | ✅ | ✅ |
| Preset Management | ✅ | ✅ |
| CLI Commands | ✅ | ✅ |
| Management UI | Web UI | TUI |
| Custom Router Script | ✅ | ❌ |
| Project-Level Config | ✅ | ❌ |
| Log Rotation | ✅ | ❌ |

---

## TUI Interface

CCG has a built-in terminal UI, use `ccg tui` to open:

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

**Controls:**
- `1` `2` `3` Switch panels
- `Tab` Next panel
- `↑/↓` or `j/k` Navigate
- Mouse wheel scroll
- Mouse click select
- `n` New
- `d` Delete
- `s` Save
- `q` Quit

---

## Routing Scenarios

CCG supports multiple routing scenarios:

| Scenario | Description | Config Field |
|----------|-------------|--------------|
| **default** | Default scenario | `Router.default` |
| **background** | Background tasks | `Router.background` |
| **think** | Deep thinking | `Router.think` |
| **longContext** | Long context | `Router.longContext` |
| **webSearch** | Web search | `Router.webSearch` |
| **image** | Image processing | `Router.image` |

---

## Built-in Transformers

| Transformer | Description |
|-------------|-------------|
| `anthropic` | Anthropic API format conversion |
| `openai` | OpenAI API format conversion |
| `deepseek` | DeepSeek format conversion |
| `gemini` | Google Gemini format conversion |
| `openrouter` | OpenRouter format conversion |
| `maxtoken` | Max token limit |
| `reasoning` | Reasoning mode |
| `enhancetool` | Enhanced tool calls |

---

## Environment Variables

Configuration supports environment variable interpolation:

```json
{
  "api_key": "${OPENAI_API_KEY}"
}
```

Will be automatically replaced with the environment variable value at runtime.

---

## License

[MIT License](LICENSE)

---

<div align="center">

**If you find this useful, please give a ⭐ Star!**

[Report Issues](https://github.com/visvioce/ccg/issues) · [Feature Requests](https://github.com/visvioce/ccg/issues) · [Contribute](https://github.com/visvioce/ccg/pulls)

</div>
