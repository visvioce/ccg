package tokenizer

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

var (
	enc     *tiktoken.Tiktoken
	initErr error
	mu      sync.Once
)

// Init 初始化 tokenizer
func Init() error {
	mu.Do(func() {
		enc, initErr = tiktoken.GetEncoding("cl100k_base")
		if initErr != nil {
			initErr = fmt.Errorf("failed to initialize tiktoken: %w", initErr)
		}
	})
	return initErr
}

// CountTokens 计算文本的 token 数量
func CountTokens(text string) int {
	if enc == nil {
		// 如果 tiktoken 未初始化，使用近似计算
		return len(text) / 4
	}
	tokens := enc.Encode(text, nil, nil)
	return len(tokens)
}

// Encode 将文本编码为 token IDs
func Encode(text string) []int {
	if enc == nil {
		return []int{}
	}
	return enc.Encode(text, nil, nil)
}

// Decode 将 token IDs 解码为文本
func Decode(tokens []int) string {
	if enc == nil {
		return ""
	}
	return enc.Decode(tokens)
}

// Message 消息结构
type Message struct {
	Role    string
	Content interface{}
}

// Tool 工具结构
type Tool struct {
	Type        string
	Name        string
	Description string
	InputSchema interface{}
}

// RequestBody 请求体结构
type RequestBody struct {
	Model    string
	Messages []Message
	System   interface{}
	Tools    []Tool
	Thinking interface{}
}

// CountRequestTokens 计算请求体的 token 数量
func CountRequestTokens(body *RequestBody) int {
	if enc == nil {
		// 如果 tiktoken 未初始化，使用近似计算
		return approximateTokenCount(body)
	}

	count := 0

	// 每条消息的基础 token 开销
	for _, msg := range body.Messages {
		count += 4 // 每条消息的基础开销
		count += countMessageTokens(&msg)
	}

	// System 消息
	if body.System != nil {
		count += 3 // system 的基础开销
		count += countSystemTokens(body.System)
	}

	// Tools
	for _, tool := range body.Tools {
		count += 15 // 每个工具的基础开销
		if tool.Name != "" || tool.Description != "" {
			count += CountTokens(tool.Name + tool.Description)
		}
		if tool.InputSchema != nil {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			count += CountTokens(string(schemaBytes))
		}
	}

	// 回复的基础开销
	count += 3

	return count
}

// countMessageTokens 计算单条消息的 token 数量
func countMessageTokens(msg *Message) int {
	count := 0

	// Role 的开销
	if msg.Role != "" {
		count += CountTokens(msg.Role)
	}

	// Content 的开销
	switch content := msg.Content.(type) {
	case string:
		count += CountTokens(content)
	case json.RawMessage:
		count += CountTokens(string(content))
	case []interface{}:
		// 处理多模态内容
		for _, item := range content {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					count += CountTokens(text)
				}
				// 图片内容估算
				if itemMap["type"] == "image" {
					count += 1024 // 图片的估算开销
				}
			}
		}
	}

	return count
}

// countSystemTokens 计算 system 字段的 token 数量
func countSystemTokens(system interface{}) int {
	count := 0

	switch s := system.(type) {
	case string:
		count += CountTokens(s)
	case []interface{}:
		for _, item := range s {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					count += CountTokens(text)
				}
			}
		}
	case json.RawMessage:
		count += CountTokens(string(s))
	}

	return count
}

// approximateTokenCount 近似计算 token 数量（当 tiktoken 不可用时）
func approximateTokenCount(body *RequestBody) int {
	count := 0

	for _, msg := range body.Messages {
		count += 4
		if text, ok := msg.Content.(string); ok {
			count += len(text) / 4
		}
	}

	if body.System != nil {
		count += 3
		if text, ok := body.System.(string); ok {
			count += len(text) / 4
		}
	}

	for _, tool := range body.Tools {
		count += 15
		count += len(tool.Name+tool.Description) / 4
	}

	count += 3

	return count
}

// CountTokensInMessages 计算消息列表的 token 数量
func CountTokensInMessages(messages []Message) int {
	if enc == nil {
		return 0
	}

	count := 0
	for _, msg := range messages {
		count += 4 // 基础开销
		count += countMessageTokens(&msg)
	}
	return count
}

// CountTokensInTools 计算工具列表的 token 数量
func CountTokensInTools(tools []Tool) int {
	if enc == nil {
		return 0
	}

	count := 0
	for _, tool := range tools {
		count += 15
		count += CountTokens(tool.Name + tool.Description)
		if tool.InputSchema != nil {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			count += CountTokens(string(schemaBytes))
		}
	}
	return count
}

// GetEncoding 获取当前的 encoding（用于调试）
func GetEncoding() *tiktoken.Tiktoken {
	return enc
}

// IsInitialized 检查 tokenizer 是否已初始化
func IsInitialized() bool {
	return enc != nil
}