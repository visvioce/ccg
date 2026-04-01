package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/musistudio/ccg/internal/config"
)

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type Agent interface {
	Name() string
	ShouldHandle(reqBody map[string]any) bool
	ShouldHandleV2(reqBody map[string]any, cfg *config.Config) bool
	GetTools() []Tool
	HandleToolCall(toolName string, input map[string]any, reqBody map[string]any, provider *config.Provider) (string, error)
	HandleToolCallV2(toolName string, input map[string]any, reqBody map[string]any, cfg *config.Config, sessionId string) (string, error)
	ReqHandler(reqBody map[string]any, sessionId string)
}

type AgentManager struct {
	mu      sync.RWMutex
	agents  map[string]Agent
	enabled bool
}

func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents:  make(map[string]Agent),
		enabled: true,
	}
}

func (m *AgentManager) RegisterAgent(agent Agent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[agent.Name()] = agent
}

func (m *AgentManager) GetAgent(name string) Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.agents[name]
}

func (m *AgentManager) GetAllTools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tools []Tool
	for _, agent := range m.agents {
		tools = append(tools, agent.GetTools()...)
	}
	return tools
}

func (m *AgentManager) ShouldHandle(reqBody map[string]any) bool {
	if !m.enabled {
		return false
	}

	for _, agent := range m.agents {
		if agent.ShouldHandle(reqBody) {
			return true
		}
	}
	return false
}

func (m *AgentManager) HandleToolCall(toolName string, input map[string]any, reqBody map[string]any, provider *config.Provider) (string, error) {
	toolName = strings.TrimPrefix(toolName, "agent_")

	for _, agent := range m.agents {
		for _, tool := range agent.GetTools() {
			if tool.Name == toolName {
				return agent.HandleToolCall(toolName, input, reqBody, provider)
			}
		}
	}

	return "", fmt.Errorf("tool not found: %s", toolName)
}

type ImageAgent struct{}

func NewImageAgent() *ImageAgent {
	return &ImageAgent{}
}

func (a *ImageAgent) Name() string {
	return "image"
}

func (a *ImageAgent) ShouldHandle(reqBody map[string]any) bool {
	tools, ok := reqBody["tools"].([]any)
	if !ok {
		return false
	}

	for _, t := range tools {
		if tool, ok := t.(map[string]any); ok {
			if name, ok := tool["name"].(string); ok {
				if strings.Contains(strings.ToLower(name), "image") || strings.Contains(strings.ToLower(name), "vision") {
					return true
				}
			}
		}
	}

	if messages, ok := reqBody["messages"].([]any); ok {
		for _, m := range messages {
			if msg, ok := m.(map[string]any); ok {
				if content, ok := msg["content"].([]any); ok {
					for _, c := range content {
						if c, ok := c.(map[string]any); ok {
							if c["type"] == "image" || c["image"] != nil {
								return true
							}
						}
					}
				}
			}
		}
	}

	return false
}

func (a *ImageAgent) GetTools() []Tool {
	return []Tool{
		{
			Name:        "image_generation",
			Description: "Generate images from text descriptions",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prompt": map[string]any{
						"type":        "string",
						"description": "Text description of the image to generate",
					},
					"model": map[string]any{
						"type":        "string",
						"description": "Model to use for image generation",
					},
					"n": map[string]any{
						"type":        "integer",
						"description": "Number of images to generate",
					},
					"size": map[string]any{
						"type":        "string",
						"description": "Size of the image (e.g., 1024x1024)",
					},
				},
				"required": []string{"prompt"},
			},
		},
	}
}

// ShouldHandleV2 新版本，支持 Config
func (a *ImageAgent) ShouldHandleV2(reqBody map[string]any, cfg *config.Config) bool {
	return a.ShouldHandle(reqBody)
}

// HandleToolCallV2 新版本，支持 Config 和 sessionId
func (a *ImageAgent) HandleToolCallV2(toolName string, input map[string]any, reqBody map[string]any, cfg *config.Config, sessionId string) (string, error) {
	return a.HandleToolCall(toolName, input, reqBody, &cfg.GetProviders()[0])
}

// ReqHandler 处理请求，注入系统提示
func (a *ImageAgent) ReqHandler(reqBody map[string]any, sessionId string) {
	// 基础版本不做处理，完整功能在 ImageAgentV2 中
}

func (a *ImageAgent) HandleToolCall(toolName string, input map[string]any, reqBody map[string]any, provider *config.Provider) (string, error) {
	prompt, ok := input["prompt"].(string)
	if !ok {
		return "", fmt.Errorf("missing prompt")
	}

	apiKey := provider.APIKey
	host := provider.Host
	if host == "" {
		host = "https://api.openai.com/v1/images/generations"
	}

	reqData := map[string]any{
		"prompt": prompt,
		"model":  "dall-e-3",
		"n":      1,
		"size":   "1024x1024",
	}

	if model, ok := input["model"].(string); ok {
		reqData["model"] = model
	}
	if n, ok := input["n"].(float64); ok {
		reqData["n"] = int(n)
	}
	if size, ok := input["size"].(string); ok {
		reqData["size"] = size
	}

	reqBodyBytes, _ := json.Marshal(reqData)
	req, err := http.NewRequest("POST", host, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]any
	json.Unmarshal(body, &result)

	if data, ok := result["data"].([]any); ok && len(data) > 0 {
		if img, ok := data[0].(map[string]any); ok {
			if url, ok := img["url"].(string); ok {
				return url, nil
			}
			if b64, ok := img["b64_json"].(string); ok {
				return "data:image/png;base64," + b64, nil
			}
		}
	}

	return string(body), nil
}

var GlobalAgentManager = NewAgentManager()

func init() {
	GlobalAgentManager.RegisterAgent(NewImageAgentV2())
}

func HandleAgentToolCall(toolName string, input map[string]any, reqBody map[string]any, cfg *config.Config) (string, error) {
	providers := cfg.GetProviders()
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers configured")
	}

	provider := &providers[0]
	return GlobalAgentManager.HandleToolCall(toolName, input, reqBody, provider)
}

func GetAgentTools() []Tool {
	return GlobalAgentManager.GetAllTools()
}

func ShouldHandleAgentTools(reqBody map[string]any) bool {
	return GlobalAgentManager.ShouldHandle(reqBody)
}

func AddAgentToolsToRequest(reqBody map[string]any) {
	tools := GetAgentTools()
	if len(tools) > 0 {
		existingTools, _ := reqBody["tools"].([]any)
		reqBody["tools"] = append(existingTools, tools)
	}
}

// ShouldHandleAgentToolsV2 新版本，支持 Config
func ShouldHandleAgentToolsV2(reqBody map[string]any, cfg *config.Config) bool {
	for _, agent := range GlobalAgentManager.agents {
		if agent.ShouldHandleV2(reqBody, cfg) {
			return true
		}
	}
	return false
}

// HandleAgentToolCallV2 新版本，支持 Config 和 sessionId
func HandleAgentToolCallV2(toolName string, input map[string]any, reqBody map[string]any, cfg *config.Config, sessionId string) (string, error) {
	providers := cfg.GetProviders()
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers configured")
	}

	toolName = strings.TrimPrefix(toolName, "agent_")

	for _, agent := range GlobalAgentManager.agents {
		for _, tool := range agent.GetTools() {
			if tool.Name == toolName {
				return agent.HandleToolCallV2(toolName, input, reqBody, cfg, sessionId)
			}
		}
	}

	return "", fmt.Errorf("tool not found: %s", toolName)
}

// ProcessAgentRequest 处理 Agent 请求（注入系统提示等）
func ProcessAgentRequest(reqBody map[string]any, sessionId string) {
	for _, agent := range GlobalAgentManager.agents {
		agent.ReqHandler(reqBody, sessionId)
	}
}

// AddAgentToolsToRequestV2 新版本
func AddAgentToolsToRequestV2(reqBody map[string]any, cfg *config.Config) {
	tools := GetAgentTools()
	if len(tools) > 0 {
		existingTools, _ := reqBody["tools"].([]any)
		reqBody["tools"] = append(existingTools, tools)
	}
}
