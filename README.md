<div align="center">

# CCG

**Claude Code Router Go**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight Go implementation of Claude Code Router

*Smart Routing for AI Models*

[English](./README.md) | [中文文档](./README_zh.md)

</div>

---

## About

CCG is a Go-based implementation of [CCR](https://github.com/musistudio/claude-code-router). Due to WSL frequently crashing from high memory usage, this project was created to provide a lightweight alternative.

**Key Benefits**
- **Low Memory**: ~30MB vs ~150MB (Node.js version)
- **Fast Startup**: ~0.1s vs ~2s (Node.js version)
- **No Dependencies**: Single binary, no runtime required

---

## The Story Behind CCG

**This project is a product of Vibe Coding.**

I don't know how to write Go code. Not a single line was written by hand - the entire codebase was generated through conversations with AI. I just described what I needed for my WSL environment and let AI handle the implementation. (Yes, this sentence was also written by AI.)

---

## Installation

```bash
# One-line install
curl -fsSL https://raw.githubusercontent.com/visvioce/ccg/master/install.sh | bash

# Build from source
git clone https://github.com/visvioce/ccg.git
cd ccg
make install
```

---

## Quick Start

1. **Configure environment variables**:
   ```bash
   export OPENAI_API_KEY="your-key"
   export DEEPSEEK_API_KEY="your-key"
   # Add other provider keys as needed
   ```

2. **Start the server**:
   ```bash
   ccg start
   ```

3. **Open Web UI**:
   ```bash
   ccg ui
   ```

---

## Commands

### Server Management
| Command | Description |
|---------|-------------|
| `ccg start` | Start server |
| `ccg stop` | Stop server |
| `ccg restart` | Restart server |
| `ccg status` | Show server status |
| `ccg ui` | Open Web UI in browser |

### Information
| Command | Description |
|---------|-------------|
| `ccg model` | List available models |
| `ccg env` | Show environment variables |
| `ccg activate` | Output env vars for shell |
| `ccg version` | Show version |

### Execution
| Command | Description |
|---------|-------------|
| `ccg code <prompt>` | Execute AI command |

### Presets
| Command | Description |
|---------|-------------|
| `ccg preset list` | List presets |
| `ccg preset info <name>` | Show preset info |
| `ccg preset install <source>` | Install preset |
| `ccg preset export <name>` | Export preset |
| `ccg preset delete <name>` | Delete preset |

### Marketplace
| Command | Description |
|---------|-------------|
| `ccg install <name>` | Install preset from marketplace |

---

## Configuration

File: `~/.claude-code-router/config.json`

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

CCG works with any OpenAI-compatible API. Configure providers in the `providers` array.

---

## Routing

Routes requests based on scenario:

| Scenario | Description |
|----------|-------------|
| `default` | General requests |
| `background` | Low-priority tasks |
| `think` | Complex reasoning |
| `longContext` | Long context (>threshold tokens) |
| `webSearch` | Web search |

---

## API

```bash
POST http://localhost:3456/v1/chat/completions
GET  http://localhost:3456/v1/models
```

---

## License

[MIT License](LICENSE)
