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

	for _, t := range transforms {
		tLower := strings.ToLower(t)
		if fn, ok := r.requestTransformers[tLower]; ok {
			result = fn(result, provider)
		}
	}

	if fn, ok := r.requestTransformers[providerLower]; ok {
		result = fn(result, provider)
	}

	return result
}

func (r *TransformerRegistry) TransformResponse(respBody []byte, provider string, transforms []string) []byte {
	result := respBody

	providerLower := strings.ToLower(provider)

	for _, t := range transforms {
		tLower := strings.ToLower(t)
		if fn, ok := r.responseTransformers[tLower]; ok {
			result = fn(result)
		}
	}

	if fn, ok := r.responseTransformers[providerLower]; ok {
		result = fn(result)
	}

	return result
}

func AnthropicToOpenAI(req map[string]any, _ string) map[string]any {
	result := make(map[string]any)

	if model, ok := req["model"].(string); ok {
		result["model"] = model
	}

	if messages, ok := req["messages"].([]any); ok {
		result["messages"] = transformAnthropicMessages(messages)
	}

	if system, ok := req["system"].(string); ok {
		result["system_message"] = system
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
		result["system_message"] = strings.Join(sysContent, "\n")
	}

	if maxTokens, ok := req["max_tokens"].(float64); ok {
		result["max_tokens"] = int(maxTokens)
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
							"type": "object",
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

	return registry
}
