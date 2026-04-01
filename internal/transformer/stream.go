package transformer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StreamChunk 流式数据块
type StreamChunk struct {
	Type    string                 `json:"type,omitempty"`
	ID      string                 `json:"id,omitempty"`
	Object  string                 `json:"object,omitempty"`
	Model   string                 `json:"model,omitempty"`
	Choices []StreamChoice         `json:"choices,omitempty"`
	Usage   *StreamUsage           `json:"usage,omitempty"`
	Delta   map[string]interface{} `json:"delta,omitempty"`
}

type StreamChoice struct {
	Index        int         `json:"index"`
	Delta        StreamDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type StreamDelta struct {
	Role      string                   `json:"role,omitempty"`
	Content   string                   `json:"content,omitempty"`
	ToolCalls []map[string]interface{} `json:"tool_calls,omitempty"`
}

type StreamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AnthropicStreamChunk Anthropic 格式的流式数据块
type AnthropicStreamChunk struct {
	Type    string                 `json:"type"`
	Index   int                    `json:"index,omitempty"`
	Delta   map[string]interface{} `json:"delta,omitempty"`
	Content []StreamContent        `json:"content,omitempty"`
	Usage   *StreamUsage           `json:"usage,omitempty"`
}

type StreamContent struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// StreamTransformer 流式转换器
type StreamTransformer struct {
	provider  string
	transform string
	buffer    string
}

// NewStreamTransformer 创建新的流式转换器
func NewStreamTransformer(provider, transform string) *StreamTransformer {
	return &StreamTransformer{
		provider:  provider,
		transform: transform,
		buffer:    "",
	}
}

// TransformOpenAIToAnthropic 将 OpenAI 格式的流式响应转换为 Anthropic 格式
func (st *StreamTransformer) TransformOpenAIToAnthropic(reader io.Reader, writer io.Writer) error {
	scanner := bufio.NewScanner(reader)
	messageIndex := 0
	contentBlockIndex := 0
	currentText := ""

	for scanner.Scan() {
		line := scanner.Text()

		// 跳过空行
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 处理 SSE 格式的数据行
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// 处理 [DONE] 标记
			if data == "[DONE]" {
				// 发送结束消息
				finishChunk := AnthropicStreamChunk{
					Type: "message_stop",
				}
				if err := st.writeChunk(writer, finishChunk); err != nil {
					return err
				}
				continue
			}

			// 解析 OpenAI 格式的 chunk
			var openaiChunk StreamChunk
			if err := json.Unmarshal([]byte(data), &openaiChunk); err != nil {
				// 解析失败，原样转发
				fmt.Fprintf(writer, "data: %s\n\n", data)
				continue
			}

			// 转换为 Anthropic 格式
			anthropicChunks := st.convertChunk(&openaiChunk, &messageIndex, &contentBlockIndex, &currentText)
			for _, chunk := range anthropicChunks {
				if err := st.writeChunk(writer, chunk); err != nil {
					return err
				}
			}
		}
	}

	return scanner.Err()
}

// convertChunk 转换单个 chunk
func (st *StreamTransformer) convertChunk(openaiChunk *StreamChunk, messageIndex, contentBlockIndex *int, currentText *string) []AnthropicStreamChunk {
	chunks := make([]AnthropicStreamChunk, 0)

	// 处理消息开始
	if *messageIndex == 0 {
		chunks = append(chunks, AnthropicStreamChunk{
			Type: "message_start",
			Delta: map[string]interface{}{
				"role": "assistant",
			},
		})
		*messageIndex++
	}

	// 处理 content_block_start
	if *contentBlockIndex == 0 && len(openaiChunk.Choices) > 0 {
		chunks = append(chunks, AnthropicStreamChunk{
			Type:  "content_block_start",
			Index: 0,
			Delta: map[string]interface{}{
				"type": "text",
			},
		})
		*contentBlockIndex++
	}

	// 处理 delta
	if len(openaiChunk.Choices) > 0 {
		choice := openaiChunk.Choices[0]

		// 处理文本内容
		if choice.Delta.Content != "" {
			*currentText += choice.Delta.Content
			chunks = append(chunks, AnthropicStreamChunk{
				Type:  "content_block_delta",
				Index: 0,
				Delta: map[string]interface{}{
					"type": "text_delta",
					"text": choice.Delta.Content,
				},
			})
		}

		// 处理工具调用
		if len(choice.Delta.ToolCalls) > 0 {
			for _, toolCall := range choice.Delta.ToolCalls {
				if toolCallType, ok := toolCall["type"].(string); ok && toolCallType == "function" {
					if fn, ok := toolCall["function"].(map[string]interface{}); ok {
						name, _ := fn["name"].(string)
						arguments, _ := fn["arguments"].(string)

						// 解析 arguments
						var input map[string]interface{}
						if arguments != "" {
							json.Unmarshal([]byte(arguments), &input)
						}
						if input == nil {
							input = make(map[string]interface{})
						}

						chunks = append(chunks, AnthropicStreamChunk{
							Type:  "content_block_delta",
							Index: 1,
							Delta: map[string]interface{}{
								"type":  "tool_use_delta",
								"id":    toolCall["id"],
								"name":  name,
								"input": input,
							},
						})
					}
				}
			}
		}

		// 处理完成原因
		if choice.FinishReason != nil && *choice.FinishReason != "" {
			stopReason := st.mapFinishReason(*choice.FinishReason)
			chunks = append(chunks, AnthropicStreamChunk{
				Type: "message_delta",
				Delta: map[string]interface{}{
					"stop_reason":   stopReason,
					"stop_sequence": nil,
				},
			})
		}
	}

	// 处理 usage
	if openaiChunk.Usage != nil {
		chunks = append(chunks, AnthropicStreamChunk{
			Type: "message_delta",
			Usage: &StreamUsage{
				PromptTokens:     openaiChunk.Usage.PromptTokens,
				CompletionTokens: openaiChunk.Usage.CompletionTokens,
				TotalTokens:      openaiChunk.Usage.TotalTokens,
			},
		})
	}

	return chunks
}

// mapFinishReason 映射完成原因
func (st *StreamTransformer) mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "content_filter":
		return "content_filter"
	default:
		return reason
	}
}

// writeChunk 写入 chunk
func (st *StreamTransformer) writeChunk(writer io.Writer, chunk AnthropicStreamChunk) error {
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "data: %s\n\n", data)
	return err
}

// TransformStream 流式转换主函数
func TransformStream(reader io.Reader, writer io.Writer, provider string, transforms []string) error {
	transformer := NewStreamTransformer(provider, "")
	return transformer.TransformOpenAIToAnthropic(reader, writer)
}

// ProcessStreamChunk 处理单个流式 chunk（用于处理 buffer 中的数据）
func ProcessStreamChunk(data []byte, provider string) []byte {
	// 如果是 [DONE]，直接返回
	if string(data) == "[DONE]" {
		return data
	}

	// 尝试解析为 OpenAI 格式
	var openaiChunk map[string]any
	if err := json.Unmarshal(data, &openaiChunk); err != nil {
		// 解析失败，原样返回
		return data
	}

	// 如果是 Anthropic 格式响应，直接返回
	if openaiChunk["object"] == "message" || openaiChunk["type"] == "message" {
		return data
	}

	// 处理 choices
	choices, ok := openaiChunk["choices"].([]any)
	if !ok || len(choices) == 0 {
		return data
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return data
	}

	delta, ok := choice["delta"].(map[string]any)
	if !ok {
		return data
	}

	// 处理 reasoning_content (DeepSeek 等模型的思考链)
	if reasoningContent, ok := delta["reasoning_content"].(string); ok && reasoningContent != "" {
		anthropicChunk := map[string]any{
			"type": "content_block_delta",
			"delta": map[string]any{
				"type": "thinking_delta",
				"thinking": map[string]any{
					"content": reasoningContent,
				},
			},
		}
		result, _ := json.Marshal(anthropicChunk)
		return result
	}

	// 处理普通文本内容
	textContent, _ := delta["content"].(string)
	if textContent == "" {
		return data
	}

	anthropicChunk := map[string]any{
		"type": "content_block_delta",
		"delta": map[string]any{
			"type": "text_delta",
			"text": textContent,
		},
	}

	result, _ := json.Marshal(anthropicChunk)
	return result
}

// IsStreamResponse 判断是否为流式响应
func IsStreamResponse(contentType string) bool {
	return strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "stream")
}

// ConvertStreamFormat 转换流格式（OpenAI -> Anthropic）
func ConvertStreamFormat(input []byte, provider string) []byte {
	lines := bytes.Split(input, []byte("\n"))
	output := make([][]byte, 0)

	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if bytes.HasPrefix(line, []byte("data: ")) {
			data := bytes.TrimPrefix(line, []byte("data: "))

			if string(data) == "[DONE]" {
				output = append(output, []byte("data: [DONE]\n"))
				continue
			}

			converted := ProcessStreamChunk(data, provider)
			output = append(output, []byte("data: "))
			output = append(output, converted)
			output = append(output, []byte("\n"))
		}
	}

	return bytes.Join(output, []byte("\n"))
}
