# CCG (Claude Code Router Go) - 验证清单

## 配置系统
- [x] 配置文件路径是否为 `~/.claude-code-router/config.json`
- [x] 配置加载机制是否与 CCR 一致
- [x] 配置文件格式是否兼容 CCR
- [x] 配置项是否能够正确加载和应用

## 路由系统
- [x] 是否支持 `<CCR-SUBAGENT-MODEL>` 标签
- [x] 是否支持 `<CCG-SUBAGENT-MODEL>` 标签
- [x] 模型路由逻辑是否与 CCR 一致
- [x] fallback 机制是否正常工作

## 转换系统
- [x] OpenAI 格式到 Anthropic 格式的转换是否正确
- [x] Anthropic 格式到 OpenAI 格式的转换是否正确
- [x] 响应转换是否正确
- [x] 各种请求格式是否都能正确处理

## 插件系统
- [x] token 速度插件是否正常工作
- [x] 插件加载和管理机制是否正常
- [x] 插件配置是否与 CCR 一致

## 图像代理
- [x] 图像分析功能是否正常工作
- [x] 图像生成功能是否正常工作
- [x] HTTP 客户端配置是否正确

## API 端点
- [x] `/v1/messages` 端点是否正常工作
- [x] `/v1/chat/completions` 端点是否正常工作
- [x] `/v1/messages/count_tokens` 端点是否正常工作
- [x] `/health` 端点是否正常工作
- [x] `/v1/models` 端点是否正常工作
- [x] `/api` 下的所有端点是否正常工作

## CLI 命令
- [x] `start` 命令是否正常工作
- [x] `stop` 命令是否正常工作
- [x] `restart` 命令是否正常工作
- [x] `status` 命令是否正常工作
- [x] `model` 命令是否正常工作
- [x] `ui` 命令是否正常工作
- [x] `preset` 命令是否正常工作
- [x] `install` 命令是否正常工作
- [x] `activate` 命令是否正常工作
- [x] `env` 命令是否正常工作
- [x] `code` 命令是否正常工作
- [x] `statusline` 命令是否正常工作

## 编译和测试
- [x] 所有代码是否能够成功编译
- [x] 所有测试是否能够通过
- [x] 代码结构是否清晰
- [x] 代码是否符合 Go 语言最佳实践
