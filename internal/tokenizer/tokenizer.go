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
		return 0
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
		return 0
	}

	count := 0

	// Count messages
	for _, msg := range body.Messages {
		count += countMessageTokens(&msg)
	}

	// Count system
	if body.System != nil {
		count += countSystemTokens(body.System)
	}

	// Count tools
	for _, tool := range body.Tools {
		if tool.Name != "" || tool.Description != "" {
			count += CountTokens(tool.Name + tool.Description)
		}
		if tool.InputSchema != nil {
			schemaBytes, _ := json.Marshal(tool.InputSchema)
			count += CountTokens(string(schemaBytes))
		}
	}

	return count
}

// countMessageTokens 计算单条消息的 token 数量
func countMessageTokens(msg *Message) int {
	count := 0

	// Content 的开销
	switch content := msg.Content.(type) {
	case string:
		count += CountTokens(content)
	case json.RawMessage:
		count += CountTokens(string(content))
	case []interface{}:
		for _, item := range content {
			if itemMap, ok := item.(map[string]interface{}); ok {
				itemType, _ := itemMap["type"].(string)
				switch itemType {
				case "text":
					if text, ok := itemMap["text"].(string); ok {
						count += CountTokens(text)
					}
				case "tool_use":
					if input, ok := itemMap["input"]; ok {
						inputBytes, _ := json.Marshal(input)
						count += CountTokens(string(inputBytes))
					}
				case "tool_result":
					var contentStr string
					if c, ok := itemMap["content"].(string); ok {
						contentStr = c
					} else if itemMap["content"] != nil {
						contentBytes, _ := json.Marshal(itemMap["content"])
						contentStr = string(contentBytes)
					}
					count += CountTokens(contentStr)
				case "image":
					count += 1024
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
				if itemMap["type"] != "text" {
					continue
				}
				if text, ok := itemMap["text"].(string); ok {
					count += CountTokens(text)
				} else if textArray, ok := itemMap["text"].([]interface{}); ok {
					for _, textPart := range textArray {
						count += CountTokens(fmt.Sprint(textPart))
					}
				}
			}
		}
	case json.RawMessage:
		count += CountTokens(string(s))
	}

	return count
}

// CountTokensInMessages 计算消息列表的 token 数量
func CountTokensInMessages(messages []Message) int {
	if enc == nil {
		return 0
	}

	count := 0
	for _, msg := range messages {
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
