package transformer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// VertexClaudeTransformer Vertex AI Claude 转换器
type VertexClaudeTransformer struct{}

// NewVertexClaudeTransformer 创建新的 Vertex Claude 转换器
func NewVertexClaudeTransformer() *VertexClaudeTransformer {
	return &VertexClaudeTransformer{}
}

// TransformRequest 转换请求
func (t *VertexClaudeTransformer) TransformRequest(req map[string]interface{}, model string) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制基本字段
	for k, v := range req {
		result[k] = v
	}

	// 获取项目 ID
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectId == "" {
		if credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credPath != "" {
			if data, err := os.ReadFile(credPath); err == nil {
				var creds map[string]interface{}
				if json.Unmarshal(data, &creds) == nil {
					if pid, ok := creds["project_id"].(string); ok {
						projectId = pid
					}
				}
			}
		}
	}

	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if location == "" {
		location = "us-east5"
	}

	// 构建 Vertex AI 端点 URL
	stream := false
	if s, ok := req["stream"].(bool); ok {
		stream = s
	}

	endpoint := "rawPredict"
	if stream {
		endpoint = "streamRawPredict"
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:%s",
		location, projectId, location, model, endpoint)

	// 添加自定义 header 信息（实际请求时使用）
	result["_vertex_url"] = url
	result["_vertex_project"] = projectId
	result["_vertex_location"] = location

	return result
}

// VertexGeminiTransformer Vertex AI Gemini 转换器
type VertexGeminiTransformer struct{}

// NewVertexGeminiTransformer 创建新的 Vertex Gemini 转换器
func NewVertexGeminiTransformer() *VertexGeminiTransformer {
	return &VertexGeminiTransformer{}
}

// TransformRequest 转换请求
func (t *VertexGeminiTransformer) TransformRequest(req map[string]interface{}, model string) map[string]interface{} {
	result := make(map[string]interface{})

	// 获取项目 ID
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectId == "" {
		if credPath := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credPath != "" {
			if data, err := os.ReadFile(credPath); err == nil {
				var creds map[string]interface{}
				if json.Unmarshal(data, &creds) == nil {
					if pid, ok := creds["project_id"].(string); ok {
						projectId = pid
					}
				}
			}
		}
	}

	location := os.Getenv("GOOGLE_CLOUD_LOCATION")
	if location == "" {
		location = "us-central1"
	}

	// 构建 Gemini 格式的请求体
	stream := false
	if s, ok := req["stream"].(bool); ok {
		stream = s
	}

	// 转换消息格式
	if messages, ok := req["messages"].([]interface{}); ok {
		contents := make([]map[string]interface{}, 0)
		for _, msg := range messages {
			if msgMap, ok := msg.(map[string]interface{}); ok {
				role, _ := msgMap["role"].(string)
				content, _ := msgMap["content"]

				parts := make([]map[string]interface{}, 0)

				switch c := content.(type) {
				case string:
					parts = append(parts, map[string]interface{}{
						"text": c,
					})
				case []interface{}:
					for _, item := range c {
						if itemMap, ok := item.(map[string]interface{}); ok {
							if text, ok := itemMap["text"].(string); ok {
								parts = append(parts, map[string]interface{}{
									"text": text,
								})
							}
						}
					}
				}

				geminiRole := "user"
				if role == "assistant" {
					geminiRole = "model"
				}

				contents = append(contents, map[string]interface{}{
					"role":  geminiRole,
					"parts": parts,
				})
			}
		}
		result["contents"] = contents
	}

	// 转换 system 消息
	if system, ok := req["system"].(string); ok && system != "" {
		result["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": system},
			},
		}
	}

	// 转换工具
	if tools, ok := req["tools"].([]interface{}); ok && len(tools) > 0 {
		functions := make([]map[string]interface{}, 0)
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				if fn, ok := toolMap["function"].(map[string]interface{}); ok {
					functions = append(functions, fn)
				}
			}
		}
		if len(functions) > 0 {
			result["tools"] = []map[string]interface{}{
				{
					"functionDeclarations": functions,
				},
			}
		}
	}

	// 转换生成配置
	generationConfig := make(map[string]interface{})
	if temp, ok := req["temperature"].(float64); ok {
		generationConfig["temperature"] = temp
	}
	if maxTokens, ok := req["max_tokens"].(float64); ok {
		generationConfig["maxOutputTokens"] = int(maxTokens)
	}
	if topP, ok := req["top_p"].(float64); ok {
		generationConfig["topP"] = topP
	}
	if len(generationConfig) > 0 {
		result["generationConfig"] = generationConfig
	}

	// 构建端点 URL
	endpoint := "generateContent"
	if stream {
		endpoint = "streamGenerateContent"
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1beta1/projects/%s/locations/%s/publishers/google/models/%s:%s",
		location, projectId, location, model, endpoint)

	result["_vertex_url"] = url
	result["_vertex_project"] = projectId
	result["_vertex_location"] = location
	result["_vertex_stream"] = stream

	return result
}

// CerebrasTransformer Cerebras 转换器
type CerebrasTransformer struct{}

// NewCerebrasTransformer 创建新的 Cerebras 转换器
func NewCerebrasTransformer() *CerebrasTransformer {
	return &CerebrasTransformer{}
}

// TransformRequest 转换请求
func (t *CerebrasTransformer) TransformRequest(req map[string]interface{}, model string) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制所有字段
	for k, v := range req {
		result[k] = v
	}

	// 移除 reasoning 相关字段（Cerebras 不支持）
	if _, ok := result["reasoning"]; ok {
		delete(result, "reasoning")
	}

	// 移除 thinking 相关字段
	if _, ok := result["thinking"]; ok {
		delete(result, "thinking")
	}

	// Cerebras 特定的参数调整
	if model == "" || strings.Contains(strings.ToLower(model), "cerebras") {
		// 使用默认模型
		result["model"] = "llama-3.1-70b"
	}

	return result
}

// SambanovaTransformer SambaNova 转换器
type SambanovaTransformer struct{}

// NewSambanovaTransformer 创建新的 SambaNova 转换器
func NewSambanovaTransformer() *SambanovaTransformer {
	return &SambanovaTransformer{}
}

// TransformRequest 转换请求
func (t *SambanovaTransformer) TransformRequest(req map[string]interface{}, model string) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制所有字段
	for k, v := range req {
		result[k] = v
	}

	// SambaNova 特定的参数调整
	// 确保模型名称正确
	if model != "" {
		result["model"] = model
	}

	return result
}

// HyperbolicTransformer Hyperbolic 转换器
type HyperbolicTransformer struct{}

// NewHyperbolicTransformer 创建新的 Hyperbolic 转换器
func NewHyperbolicTransformer() *HyperbolicTransformer {
	return &HyperbolicTransformer{}
}

// TransformRequest 转换请求
func (t *HyperbolicTransformer) TransformRequest(req map[string]interface{}, model string) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制所有字段
	for k, v := range req {
		result[k] = v
	}

	// Hyperbolic 特定的参数调整
	if model != "" {
		result["model"] = model
	}

	return result
}

// NovitaTransformer Novita 转换器
type NovitaTransformer struct{}

// NewNovitaTransformer 创建新的 Novita 转换器
func NewNovitaTransformer() *NovitaTransformer {
	return &NovitaTransformer{}
}

// TransformRequest 转换请求
func (t *NovitaTransformer) TransformRequest(req map[string]interface{}, model string) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制所有字段
	for k, v := range req {
		result[k] = v
	}

	// Novita 特定的参数调整
	if model != "" {
		result["model"] = model
	}

	return result
}

// FireworksTransformer Fireworks 转换器
type FireworksTransformer struct{}

// NewFireworksTransformer 创建新的 Fireworks 转换器
func NewFireworksTransformer() *FireworksTransformer {
	return &FireworksTransformer{}
}

// TransformRequest 转换请求
func (t *FireworksTransformer) TransformRequest(req map[string]interface{}, model string) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制所有字段
	for k, v := range req {
		result[k] = v
	}

	// Fireworks 特定的参数调整
	if model != "" {
		// Fireworks 模型名称格式: accounts/fireworks/models/model-name
		if !strings.Contains(model, "/") {
			result["model"] = "accounts/fireworks/models/" + model
		}
	}

	return result
}
