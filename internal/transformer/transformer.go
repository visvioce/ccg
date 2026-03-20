package transformer

import (
	"encoding/json"
	"strings"
)

type RequestTransformer func(reqBody map[string]any, model string) map[string]any
type ResponseTransformer func(respBody []byte) []byte

type TransformerRegistry struct {
	requestTransformers  map[string]RequestTransformer
	responseTransformers map[string]ResponseTransformer
}

func NewRegistry() *TransformerRegistry {
	return &TransformerRegistry{
		requestTransformers:  make(map[string]RequestTransformer),
		responseTransformers: make(map[string]ResponseTransformer),
	}
}

func (r *TransformerRegistry) RegisterRequest(name string, fn RequestTransformer) {
	r.requestTransformers[strings.ToLower(name)] = fn
}

func (r *TransformerRegistry) RegisterResponse(name string, fn ResponseTransformer) {
	r.responseTransformers[strings.ToLower(name)] = fn
}

func (r *TransformerRegistry) TransformRequest(reqBody map[string]any, provider string, transforms []string) map[string]any {
	result := reqBody

	providerLower := strings.ToLower(provider)

	// 如果是Anthropic格式（有system字段或messages中的content是数组），默认转换为OpenAI格式
	isAnthropicFormat := false
	if _, hasSystem := reqBody["system"]; hasSystem {
		isAnthropicFormat = true
	}
	if messages, ok := reqBody["messages"].([]any); ok && len(messages) > 0 {
		if msg, ok := messages[0].(map[string]any); ok {
			if content, ok := msg["content"]; ok {
				if _, isArray := content.([]any); isArray {
					isAnthropicFormat = true
				}
			}
		}
	}

	// 应用指定的transforms
	for _, t := range transforms {
		tLower := strings.ToLower(t)
		if fn, ok := r.requestTransformers[tLower]; ok {
			result = fn(result, provider)
		}
	}

	// 应用provider特定的transformer
	if fn, ok := r.requestTransformers[providerLower]; ok {
		result = fn(result, provider)
	}

	// 如果没有provider特定的transformer，且请求是Anthropic格式，默认应用AnthropicToOpenAI转换
	if _, hasProviderTransform := r.requestTransformers[providerLower]; !hasProviderTransform && isAnthropicFormat {
		if fn, ok := r.requestTransformers["anthropic"]; ok {
			result = fn(result, provider)
		}
	}

	return result
}

func (r *TransformerRegistry) TransformResponse(respBody []byte, provider string, transforms []string) []byte {
	result := respBody

	// 检查是否需要将OpenAI格式转换为Anthropic格式
	// 如果响应包含OpenAI格式的字段（choices），则转换为Anthropic格式
	var respData map[string]any
	if err := json.Unmarshal(respBody, &respData); err == nil {
		if _, hasChoices := respData["choices"]; hasChoices {
			// 这是OpenAI格式，转换为Anthropic格式
			if fn, ok := r.responseTransformers["openai->anthropic"]; ok {
				result = fn(result)
			}
		}
	}

	return result
}

func AnthropicToOpenAI(req map[string]any, _ string) map[string]any {
	result := make(map[string]any)

	if model, ok := req["model"].(string); ok {
		result["model"] = model
	}

	// 转换messages，同时处理system消息
	var messages []any

	// 处理system消息 - Anthropic的system字段需要转换为OpenAI的messages中的system角色
	if system, ok := req["system"].(string); ok && system != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": system,
		})
	} else if system, ok := req["system"].([]any); ok {
		var sysContent []string
		for _, s := range system {
			if m, ok := s.(map[string]any); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						sysContent = append(sysContent, text)
					}
				}
			}
		}
		if len(sysContent) > 0 {
			messages = append(messages, map[string]any{
				"role":    "system",
				"content": strings.Join(sysContent, "\n"),
			})
		}
	}

	if reqMessages, ok := req["messages"].([]any); ok {
		transformedMessages := transformAnthropicMessages(reqMessages)
		messages = append(messages, transformedMessages...)
	}

	result["messages"] = messages

	if maxTokens, ok := req["max_tokens"].(float64); ok {
		result["max_tokens"] = int(maxTokens)
	} else if maxTokens, ok := req["max_tokens"].(int); ok {
		result["max_tokens"] = maxTokens
	}

	if stream, ok := req["stream"].(bool); ok {
		result["stream"] = stream
	}

	if tools, ok := req["tools"].([]any); ok {
		result["tools"] = transformAnthropicTools(tools)
	}

	if temperature, ok := req["temperature"].(float64); ok {
		result["temperature"] = temperature
	}

	if topP, ok := req["top_p"].(float64); ok {
		result["top_p"] = topP
	}

	return result
}

func transformAnthropicMessages(messages []any) []any {
	result := make([]any, len(messages))
	for i, m := range messages {
		if msg, ok := m.(map[string]any); ok {
			newMsg := make(map[string]any)
			if role, ok := msg["role"].(string); ok {
				newMsg["role"] = role
			}
			if content, ok := msg["content"].(string); ok {
				newMsg["content"] = content
			} else if content, ok := msg["content"].([]any); ok {
				var textContent string
				for _, c := range content {
					if c, ok := c.(map[string]any); ok {
						if ctype, ok := c["type"].(string); ok && ctype == "text" {
							if text, ok := c["text"].(string); ok {
								textContent += text
							}
						}
					}
				}
				newMsg["content"] = textContent
			}
			result[i] = newMsg
		}
	}
	return result
}

func transformAnthropicTools(tools []any) []any {
	result := make([]any, 0)
	for _, t := range tools {
		if tool, ok := t.(map[string]any); ok {
			newTool := make(map[string]any)
			if name, ok := tool["name"].(string); ok {
				newTool["function"] = map[string]any{
					"name":        name,
					"description": "",
				}
			}
			if inputSchema, ok := tool["input_schema"].(map[string]any); ok {
				if f, ok := newTool["function"].(map[string]any); ok {
					f["parameters"] = inputSchema
				}
			}
			result = append(result, newTool)
		}
	}
	return result
}

func OpenAIToAnthropic(req map[string]any, _ string) map[string]any {
	result := make(map[string]any)

	if model, ok := req["model"].(string); ok {
		result["model"] = model
	}

	if messages, ok := req["messages"].([]any); ok {
		result["messages"] = transformOpenAIMessages(messages)
	}

	if systemMessage, ok := req["system_message"].(string); ok {
		result["system"] = []any{
			map[string]any{"type": "text", "text": systemMessage},
		}
	}

	if maxTokens, ok := req["max_tokens"].(float64); ok {
		result["max_tokens"] = int(maxTokens)
	}

	if stream, ok := req["stream"].(bool); ok {
		result["stream"] = stream
	}

	if tools, ok := req["tools"].([]any); ok {
		result["tools"] = transformOpenAITools(tools)
	}

	return result
}

func transformOpenAIMessages(messages []any) []any {
	result := make([]any, len(messages))
	for i, m := range messages {
		if msg, ok := m.(map[string]any); ok {
			newMsg := make(map[string]any)
			if role, ok := msg["role"].(string); ok {
				newMsg["role"] = role
			}
			if content, ok := msg["content"].(string); ok {
				newMsg["content"] = content
			}
			result[i] = newMsg
		}
	}
	return result
}

func transformOpenAITools(tools []any) []any {
	result := make([]any, 0)
	for _, t := range tools {
		if tool, ok := t.(map[string]any); ok {
			newTool := make(map[string]any)
			if fn, ok := tool["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok {
					newTool["name"] = name
				}
				if description, ok := fn["description"].(string); ok {
					newTool["description"] = description
				}
				if parameters, ok := fn["parameters"].(map[string]any); ok {
					newTool["input_schema"] = parameters
				}
			}
			result = append(result, newTool)
		}
	}
	return result
}

func MaxTokenTransformer(req map[string]any, _ string) map[string]any {
	if maxTokens, ok := req["max_tokens"].(float64); ok {
		if req["max_completion_tokens"] == nil {
			req["max_completion_tokens"] = maxTokens
		}
	}
	return req
}

func MaxCompletionTokensTransformer(req map[string]any, _ string) map[string]any {
	if maxCompletionTokens, ok := req["max_completion_tokens"].(float64); ok {
		if req["max_tokens"] == nil {
			req["max_tokens"] = maxCompletionTokens
		}
	}
	return req
}

func ForceReasoningTransformer(req map[string]any, _ string) map[string]any {
	if thinking, ok := req["thinking"].(map[string]any); ok {
		if enabled, ok := thinking["enabled"].(bool); ok && enabled {
			if _, ok := req["extra_body"]; !ok {
				req["extra_body"] = map[string]any{}
			}
			if eb, ok := req["extra_body"].(map[string]any); ok {
				eb["thinking"] = map[string]any{
					"type": "enabled",
				}
			}
		}
	}
	return req
}

func SamplingTransformer(req map[string]any, _ string) map[string]any {
	if temperature, ok := req["temperature"].(float64); ok {
		if req["temperature"] == nil {
			req["temperature"] = temperature
		}
	}
	if topP, ok := req["top_p"].(float64); ok {
		if req["top_p"] == nil {
			req["top_p"] = topP
		}
	}
	return req
}

func StreamOptionsTransformer(req map[string]any, _ string) map[string]any {
	if streamOptions, ok := req["stream_options"].(map[string]any); ok {
		if include, ok := streamOptions["include_usage"].(bool); ok && include {
			req["stream"] = true
		}
	}
	return req
}

func CleanCacheTransformer(req map[string]any, _ string) map[string]any {
	if _, ok := req["extra_body"]; !ok {
		req["extra_body"] = map[string]any{}
	}
	if eb, ok := req["extra_body"].(map[string]any); ok {
		eb["extra_body"] = map[string]any{
			"web_search_options": map[string]any{
				"enable": false,
			},
		}
	}
	return req
}

func EnhanceToolTransformer(req map[string]any, _ string) map[string]any {
	if tools, ok := req["tools"].([]any); ok {
		for _, t := range tools {
			if tool, ok := t.(map[string]any); ok {
				if fn, ok := tool["function"].(map[string]any); ok {
					if _, ok := fn["parameters"]; !ok {
						fn["parameters"] = map[string]any{
							"type":       "object",
							"properties": map[string]any{},
						}
					}
				}
			}
		}
	}
	return req
}

func OpenAIStreamHandler(data []byte) []byte {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return data
	}

	if choices, ok := raw["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if delta, ok := choice["delta"].(map[string]any); ok {
				if content, ok := delta["content"].(string); ok {
					return []byte("data: " + `{"content":"` + content + `"}` + "\n\n")
				}
			}
		}
	}

	return data
}

func DeepSeekTransformer(req map[string]any, model string) map[string]any {
	if strings.Contains(model, "deepseek-coder") {
		if req["tools"] != nil {
		}
	}
	return req
}

func GeminiTransformer(req map[string]any, _ string) map[string]any {
	if system, ok := req["system_message"].(string); ok {
		req["systemInstruction"] = map[string]any{
			"parts": []any{
				map[string]any{"text": system},
			},
		}
		delete(req, "system_message")
	}

	if tools, ok := req["tools"].([]any); ok {
		var toolsList []any
		for _, t := range tools {
			if tool, ok := t.(map[string]any); ok {
				if fn, ok := tool["function"].(map[string]any); ok {
					toolsList = append(toolsList, map[string]any{
						"functionDeclarations": []any{fn},
					})
				}
			}
		}
		if len(toolsList) > 0 {
			req["tools"] = toolsList
		}
	}

	return req
}

func GroqTransformer(req map[string]any, _ string) map[string]any {
	return req
}

func VercelTransformer(req map[string]any, _ string) map[string]any {
	if _, ok := req["extra_body"]; !ok {
		req["extra_body"] = map[string]any{}
	}
	return req
}

func CustomParamsTransformer(req map[string]any, _ string) map[string]any {
	return req
}

func OpenAIToAnthropicResponse(respBody []byte) []byte {
	var openaiResp map[string]any
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return respBody
	}

	anthropicResp := make(map[string]any)

	if id, ok := openaiResp["id"].(string); ok {
		anthropicResp["id"] = id
	}

	anthropicResp["type"] = "message"
	anthropicResp["role"] = "assistant"

	if model, ok := openaiResp["model"].(string); ok {
		anthropicResp["model"] = model
	}

	// 转换choices到content
	content := []any{}
	stopReason := "end_turn"

	if choices, ok := openaiResp["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if text, ok := message["content"].(string); ok && text != "" {
					content = append(content, map[string]any{
						"type": "text",
						"text": text,
					})
				}

				// 处理tool_calls
				if toolCalls, ok := message["tool_calls"].([]any); ok {
					for _, tc := range toolCalls {
						if toolCall, ok := tc.(map[string]any); ok {
							toolUse := map[string]any{
								"type": "tool_use",
								"id":   toolCall["id"],
							}
							if fn, ok := toolCall["function"].(map[string]any); ok {
								toolUse["name"] = fn["name"]
								var input map[string]any
								if args, ok := fn["arguments"].(string); ok {
									json.Unmarshal([]byte(args), &input)
								}
								if input == nil {
									input = make(map[string]any)
								}
								toolUse["input"] = input
							}
							content = append(content, toolUse)
						}
					}
				}
			}

			// 处理finish_reason
			if finishReason, ok := choice["finish_reason"].(string); ok {
				switch finishReason {
				case "stop":
					stopReason = "end_turn"
				case "length":
					stopReason = "max_tokens"
				case "tool_calls":
					stopReason = "tool_use"
				default:
					stopReason = "end_turn"
				}
			}
		}
	}

	anthropicResp["content"] = content
	anthropicResp["stop_reason"] = stopReason
	anthropicResp["stop_sequence"] = nil

	// 转换usage
	if usage, ok := openaiResp["usage"].(map[string]any); ok {
		anthropicResp["usage"] = map[string]any{
			"input_tokens":  usage["prompt_tokens"],
			"output_tokens": usage["completion_tokens"],
		}
	}

	result, _ := json.Marshal(anthropicResp)
	return result
}

func BuildDefaultRegistry() *TransformerRegistry {
	registry := NewRegistry()

	registry.RegisterRequest("anthropic", AnthropicToOpenAI)
	registry.RegisterRequest("anthropic->openai", AnthropicToOpenAI)
	registry.RegisterRequest("openai->anthropic", OpenAIToAnthropic)
	registry.RegisterRequest("openai", func(req map[string]any, _ string) map[string]any { return req })
	registry.RegisterRequest("deepseek", DeepSeekTransformer)
	registry.RegisterRequest("gemini", GeminiTransformer)
	registry.RegisterRequest("google", GeminiTransformer)
	registry.RegisterRequest("groq", GroqTransformer)
	registry.RegisterRequest("vercel", VercelTransformer)
	registry.RegisterRequest("openrouter", func(req map[string]any, _ string) map[string]any { return req })
	registry.RegisterRequest("cerebras", func(req map[string]any, _ string) map[string]any { return req })

	registry.RegisterRequest("maxtoken", MaxTokenTransformer)
	registry.RegisterRequest("maxcompletiontokens", MaxCompletionTokensTransformer)
	registry.RegisterRequest("forcereasoning", ForceReasoningTransformer)
	registry.RegisterRequest("sampling", SamplingTransformer)
	registry.RegisterRequest("streamoptions", StreamOptionsTransformer)
	registry.RegisterRequest("cleancache", CleanCacheTransformer)
	registry.RegisterRequest("enhancetool", EnhanceToolTransformer)
	registry.RegisterRequest("customparams", CustomParamsTransformer)

	// 注册response transformers
	registry.RegisterResponse("openai->anthropic", OpenAIToAnthropicResponse)

	return registry
}
