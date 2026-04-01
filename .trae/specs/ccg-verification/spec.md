# CCG (Claude Code Router Go) - 验证产品需求文档

## 概述
- **摘要**：对 CCG 项目进行全面验证，确保其功能与原始 CCR (Claude Code Router) TypeScript 项目一致，并且所有代码能够正常编译和运行。
- **目的**：验证 CCG 项目的完整性、正确性和与 CCR 的一致性，确保用户可以无缝迁移到 Go 版本。
- **目标用户**：开发团队和最终用户，确保 CCG 能够满足所有预期功能。

## 目标
- 验证 CCG 项目的所有核心功能是否与 CCR 一致
- 确保所有代码能够成功编译和测试
- 验证配置系统、路由系统、转换系统和插件系统的功能
- 确保 API 端点与 CCR 兼容
- 验证 CLI 命令的功能完整性

## 非目标（范围外）
- 性能优化：本次验证不关注性能优化，只关注功能完整性
- 新功能开发：本次验证只关注与 CCR 一致的功能，不添加新功能
- 文档完善：虽然需要基本文档，但不要求详细的用户文档

## 背景与上下文
- CCG 是 CCR (Claude Code Router) 的 Go 语言重写版本
- CCR 是一个用于路由 AI 模型请求的 Node.js 实现
- CCG 旨在提供与 CCR 相同的功能，但使用 Go 语言实现，以提高性能和可靠性
- 已完成的主要功能包括：代理和超时配置、子代理标签处理、fallback 机制、token 速度测量、图像代理功能等

## 功能需求
- **FR-1**：配置系统 - 确保配置文件路径、格式和加载机制与 CCR 一致
- **FR-2**：路由系统 - 确保模型路由逻辑、子代理标签处理与 CCR 一致
- **FR-3**：转换系统 - 确保 API 格式转换（Anthropic ↔ OpenAI）与 CCR 一致
- **FR-4**：插件系统 - 确保插件加载和管理与 CCR 一致
- **FR-5**：图像代理 - 确保图像分析和生成功能与 CCR 一致
- **FR-6**：API 端点 - 确保所有 API 端点与 CCR 兼容
- **FR-7**：CLI 命令 - 确保所有 CLI 命令与 CCR 一致

## 非功能需求
- **NFR-1**：编译成功 - 所有代码能够成功编译，无编译错误
- **NFR-2**：测试通过 - 所有测试能够成功运行（如果有测试文件）
- **NFR-3**：代码质量 - 代码结构清晰，符合 Go 语言最佳实践
- **NFR-4**：配置兼容性 - 能够使用与 CCR 相同的配置文件格式

## 约束
- **技术**：Go 语言，Gin 框架，tiktoken-go 库
- **依赖**：外部 API 服务（如 OpenAI、Anthropic 等）
- **平台**：支持 Linux、macOS、Windows 等主要操作系统

## 假设
- 假设用户已经安装了 Go 环境（Go 1.18+）
- 假设用户了解 CCR 的基本功能和配置方式
- 假设外部 API 服务（如 OpenAI、Anthropic）是可访问的

## 验收标准

### AC-1：配置系统验证
- **Given**：用户创建了与 CCR 格式相同的配置文件
- **When**：用户启动 CCG 服务器
- **Then**：CCG 能够正确加载配置文件并应用配置
- **Verification**：`programmatic`
- **Notes**：配置文件路径应为 `~/.claude-code-router/config.json`

### AC-2：路由系统验证
- **Given**：用户发送包含子代理标签的请求
- **When**：CCG 处理请求
- **Then**：CCG 能够正确识别和处理子代理标签，支持 `<CCR-SUBAGENT-MODEL>` 和 `<CCG-SUBAGENT-MODEL>` 格式
- **Verification**：`programmatic`

### AC-3：转换系统验证
- **Given**：用户发送 OpenAI 格式的请求
- **When**：CCG 处理请求
- **Then**：CCG 能够正确转换为 Anthropic 格式并发送，然后将响应转换回 OpenAI 格式
- **Verification**：`programmatic`

### AC-4：插件系统验证
- **Given**：用户启用了 token 速度插件
- **When**：CCG 处理请求
- **Then**：CCG 能够正确记录和报告 token 速度
- **Verification**：`programmatic`

### AC-5：图像代理验证
- **Given**：用户发送包含图像的请求
- **When**：CCG 处理请求
- **Then**：CCG 能够正确分析图像并返回结果
- **Verification**：`programmatic`

### AC-6：API 端点验证
- **Given**：用户发送请求到 CCG 的 API 端点
- **When**：CCG 处理请求
- **Then**：CCG 能够正确响应所有 API 端点，包括 `/v1/messages`、`/v1/chat/completions` 等
- **Verification**：`programmatic`

### AC-7：CLI 命令验证
- **Given**：用户运行 CCG 的 CLI 命令
- **When**：CCG 执行命令
- **Then**：CCG 能够正确执行所有 CLI 命令，包括 `start`、`stop`、`status`、`preset` 等
- **Verification**：`programmatic`

### AC-8：编译和测试验证
- **Given**：用户编译 CCG 项目
- **When**：Go 编译器编译代码
- **Then**：代码能够成功编译，无编译错误，并且测试能够通过
- **Verification**：`programmatic`

## 未解决的问题
- [ ] 是否需要添加更多测试文件来验证功能？
- [ ] 是否需要添加更多文档来描述 CCG 的使用方法？
- [ ] 是否需要支持更多的插件类型？
