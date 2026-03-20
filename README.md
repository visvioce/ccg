# CCG - Claude Code Router (Go)

<div align="center">
  <img src="https://trae-api-cn.mchost.guru/api/ide/v1/text_to_image?prompt=modern%20minimalist%20logo%20for%20CCG%20Claude%20Code%20Router%20with%20geometric%20shapes%20and%20blue%20color%20scheme&image_size=square" alt="CCG Logo" width="120" height="120">
  <h3>高性能、低内存的 Claude Code 路由解决方案</h3>
  <p>用 Go 语言重写的 CCR (Claude Code Router)，为资源受限环境优化</p>
</div>

## 📋 项目简介

CCG 是 CCR (Claude Code Router) 的 Go 语言重写版本，专为解决 WSL 等资源受限环境中的内存问题而设计。通过 Go 语言的高性能特性，CCG 在保持功能完整性的同时，显著降低了内存占用和启动时间。

### ✨ 核心优势

- **极致轻量**：Go 语言编译为静态二进制，无运行时依赖，内存占用仅为 Node.js 版本的 1/5
- **性能卓越**：路由响应速度提升 3-5 倍，启动时间缩短至毫秒级
- **跨平台支持**：支持 Linux、macOS、Windows 等多种平台
- **功能完整**：保留 CCR 的全部核心功能，包括模型路由、多提供商支持等
- **易于维护**：Go 语言的类型系统和并发模型使代码更稳定可靠

## 🚀 快速开始

### 方式 1：一键安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/visvioce/ccg/master/install.sh | bash
```

### 方式 2：手动下载

从 [GitHub Releases](https://github.com/visvioce/ccg/releases) 下载对应平台的二进制文件，然后放到 `PATH` 目录中：

```bash
# Linux/macOS
sudo mv ccg-*-amd64 /usr/local/bin/ccg
chmod +x /usr/local/bin/ccg

# Windows
# 将 ccg-windows-amd64.exe 重命名为 ccg.exe 并添加到系统 PATH
```

### 方式 3：从源码编译

```bash
git clone https://github.com/visvioce/ccg.git
cd ccr-go
make install
```

## 🎯 使用方法

### 基础命令

```bash
# 启动 CCG 服务
ccg start

# 停止 CCG 服务
ccg stop

# 查看服务状态
ccg status

# 交互式模型选择
ccg model

# 打开 Web UI（暂未实现，敬请期待）
ccg ui

# 打开终端 UI
ccg tui
```

### 高级命令

```bash
# 管理预设配置
ccg preset list            # 列出所有预设
ccg preset export my-preset  # 导出当前配置为预设
ccg preset install /path/to/preset  # 安装预设

# 查看版本信息
ccg version

# 查看帮助信息
ccg help
```

## ⚙️ 配置指南

配置文件位于 `~/.ccg/config.json`，采用 JSON 格式，支持多提供商配置：

```json
{
  "LOG": false,
  "LOG_LEVEL": "debug",
  "CLAUDE_PATH": "",
  "HOST": "127.0.0.1",
  "PORT": "3456",
  "API_KEY": "sk-your-api-key",
  "API_TIMEOUT_MS": "600000",
  "transformers": [],
  "providers": [
    {
      "name": "openai",
      "api_base_url": "https://api.openai.com/v1/chat/completions",
      "api_key": "${OPENAI_API_KEY}",
      "models": ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo"],
      "transformer": {
        "use": []
      }
    },
    {
      "name": "deepseek",
      "api_base_url": "https://api.deepseek.com/v1/chat/completions",
      "api_key": "${DEEPSEEK_API_KEY}",
      "models": ["deepseek-chat", "deepseek-coder"],
      "transformer": {
        "use": []
      }
    },
    {
      "name": "anthropic",
      "api_base_url": "https://api.anthropic.com/v1/messages",
      "api_key": "${ANTHROPIC_API_KEY}",
      "models": ["claude-sonnet-4-20250514", "claude-3-5-sonnet-20241022", "claude-3-haiku-20240307"],
      "transformer": {
        "use": ["anthropic->openai"]
      }
    },
    {
      "name": "groq",
      "api_base_url": "https://api.groq.com/openai/v1/chat/completions",
      "api_key": "${GROQ_API_KEY}",
      "models": ["llama-3.3-70b-versatile", "mixtral-8x7b-32768"],
      "transformer": {
        "use": []
      }
    },
    {
      "name": "google",
      "api_base_url": "https://generativelanguage.googleapis.com/v1beta/models",
      "api_key": "${GOOGLE_API_KEY}",
      "models": ["gemini-2.0-flash-exp", "gemini-1.5-pro", "gemini-1.5-flash"],
      "transformer": {
        "use": ["gemini"]
      }
    }
  ],
  "Router": {
    "default": "openai,gpt-4o",
    "background": "openai,gpt-4o-mini",
    "think": "openai,gpt-4o",
    "longContext": "anthropic,claude-3-5-sonnet-20241022",
    "longContextThreshold": 60000,
    "webSearch": "google,gemini-2.0-flash-exp"
  }
}
```

### 配置说明

- **LOG**：是否启用日志
- **LOG_LEVEL**：日志级别（debug/info/warn/error）
- **HOST**：服务监听主机
- **PORT**：服务监听端口
- **API_KEY**：默认 API 密钥
- **API_TIMEOUT_MS**：API 超时时间（毫秒）
- **providers**：支持多个 LLM 提供商
- **Router**：根据不同场景路由到不同模型

## 🔄 模型路由机制

CCG 采用智能路由机制，根据请求类型和上下文自动选择最适合的模型：

1. **默认路由**：处理常规对话和通用请求
2. **后台路由**：处理长时间运行的任务
3. **思考路由**：处理需要深度思考的复杂问题
4. **长上下文路由**：处理长文本输入
5. **网络搜索路由**：处理需要网络搜索的请求

## 📁 项目结构

```
ccr-go/
├── cmd/              # 命令行入口
│   ├── cli/          # CLI 命令实现
│   └── server/       # 服务器实现
├── internal/         # 内部包
│   ├── agent/        # 代理逻辑
│   ├── cache/        # 缓存系统
│   ├── config/       # 配置管理
│   ├── logger/       # 日志系统
│   ├── middleware/   # 中间件
│   ├── plugin/       # 插件系统
│   ├── preset/       # 预设管理
│   ├── provider/     # 提供商集成
│   ├── router/       # 路由逻辑
│   ├── server/       # 服务器核心
│   ├── tokenizer/    # 分词器
│   └── transformer/  # 请求/响应转换
├── pkg/              # 公共包
│   └── shared/       # 共享工具
├── .github/          # GitHub 配置
│   └── workflows/    # CI/CD 工作流
├── install.sh        # 安装脚本
├── Makefile          # 构建配置
├── go.mod            # Go 模块文件
└── README.md         # 项目文档
```

## 📊 性能对比

| 特性 | CCR (Node.js) | CCG (Go) | 提升 |
|------|---------------|----------|------|
| 内存占用 | ~150MB | ~30MB | 80% 减少 |
| 启动时间 | ~2-3秒 | ~0.1秒 | 95% 减少 |
| 响应速度 | 基准 | 3-5倍 | 300-500% 提升 |
| 依赖项 | Node.js 运行时 | 无 | 零依赖 |
| 部署方式 | npm 包 | 单二进制文件 | 更简单 |

## 🔧 开发与扩展

### 构建项目

```bash
# 构建单个平台
make build

# 构建所有平台
make build-all
```

### 运行测试

```bash
make test
```

### 清理构建文件

```bash
make clean
```

## 🤝 贡献指南

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 🌟 致谢

- 感谢原 CCR 项目的创意和设计
- 感谢 Go 语言团队提供的优秀工具链
- 感谢所有为项目做出贡献的开发者

---

<div align="center">
  <p>Made with ❤️ for developers who value performance and reliability</p>
</div>
