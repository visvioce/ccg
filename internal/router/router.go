package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/musistudio/ccg/internal/cache"
	"github.com/musistudio/ccg/internal/config"
	"github.com/musistudio/ccg/internal/tokenizer"
	"github.com/robertkrimen/otto"
)

const (
	ClaudeProjectsDir = ".claude/projects"
)

type ScenarioType string

const (
	ScenarioDefault      ScenarioType = "default"
	ScenarioBackground   ScenarioType = "background"
	ScenarioThink        ScenarioType = "think"
	ScenarioLongContext ScenarioType = "longContext"
	ScenarioWebSearch   ScenarioType = "webSearch"
	ScenarioImage       ScenarioType = "image"
)

type RouterResult struct {
	Model    string
	Scenario ScenarioType
}

type Router struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Router {
	return &Router{cfg: cfg}
}

func (r *Router) Route(reqBody *tokenizer.RequestBody, sessionId string) *RouterResult {
	providers := r.cfg.GetProviders()
	router := r.cfg.GetRouter()

	if strings.Contains(reqBody.Model, ",") {
		parts := strings.SplitN(reqBody.Model, ",", 2)
		providerName := parts[0]
		modelName := parts[1]

		for i := range providers {
			if strings.EqualFold(providers[i].Name, providerName) {
				for _, m := range providers[i].Models {
					if strings.EqualFold(m, modelName) {
						return &RouterResult{
							Model:    reqBody.Model,
							Scenario: ScenarioDefault,
						}
					}
				}
			}
		}
		return &RouterResult{
			Model:    reqBody.Model,
			Scenario: ScenarioDefault,
		}
	}

	customRouterPath := r.cfg.Get("CUSTOM_ROUTER_PATH")
	if customRouterPath != "" {
		if model := r.loadCustomRouter(customRouterPath, reqBody, sessionId); model != "" {
			return &RouterResult{
				Model:    model,
				Scenario: ScenarioDefault,
			}
		}
	}

	projectRouter := r.loadProjectRouter(sessionId)
	if projectRouter != nil {
		router = projectRouter
	}

	var lastUsage *cache.Usage
	if sessionId != "" {
		if usageVal, ok := cache.SessionUsage.Get(sessionId); ok {
			if usage, ok := usageVal.(cache.Usage); ok {
				lastUsage = &usage
			}
		}
	}

	longContextThreshold := 60000
	if router != nil && router.LongContextThreshold > 0 {
		longContextThreshold = router.LongContextThreshold
	}

	tokenCount := tokenizer.CountRequestTokens(reqBody)

	if lastUsage != nil && lastUsage.InputTokens > longContextThreshold && tokenCount > 20000 {
		if router != nil && router.LongContext != "" {
			return &RouterResult{
				Model:    router.LongContext,
				Scenario: ScenarioLongContext,
			}
		}
	}

	if tokenCount > longContextThreshold {
		if router != nil && router.LongContext != "" {
			return &RouterResult{
				Model:    router.LongContext,
				Scenario: ScenarioLongContext,
			}
		}
	}

	if r.hasSubagentModel(reqBody) {
		if model := r.extractSubagentModel(reqBody); model != "" {
			return &RouterResult{
				Model:    model,
				Scenario: ScenarioDefault,
			}
		}
	}

	if router != nil && router.Default != "" {
		if strings.Contains(reqBody.Model, "claude") && strings.Contains(reqBody.Model, "haiku") {
			if router.Background != "" {
				return &RouterResult{
					Model:    router.Background,
					Scenario: ScenarioBackground,
				}
			}
		}
	}

	if r.hasWebSearch(reqBody) && router != nil && router.WebSearch != "" {
		return &RouterResult{
			Model:    router.WebSearch,
			Scenario: ScenarioWebSearch,
		}
	}

	if r.hasThinking(reqBody) && router != nil && router.Think != "" {
		return &RouterResult{
			Model:    router.Think,
			Scenario: ScenarioThink,
		}
	}

	if r.hasImage(reqBody) && router != nil && router.Image != "" {
		return &RouterResult{
			Model:    router.Image,
			Scenario: ScenarioImage,
		}
	}

	defaultModel := router.Default
	if router == nil {
		defaultModel = reqBody.Model
	}

	return &RouterResult{
		Model:    defaultModel,
		Scenario: ScenarioDefault,
	}
}

func (r *Router) hasSubagentModel(body *tokenizer.RequestBody) bool {
	if body.System == nil {
		return false
	}

	sysStr, ok := body.System.(string)
	if !ok {
		return false
	}

	return strings.Contains(sysStr, "<CCG-SUBAGENT-MODEL>")
}

func (r *Router) extractSubagentModel(body *tokenizer.RequestBody) string {
	re := regexp.MustCompile(`<CCG-SUBAGENT-MODEL>(.*?)</CCG-SUBAGENT-MODEL>`)

	var sysStr string
	switch v := body.System.(type) {
	case string:
		sysStr = v
	}

	matches := re.FindStringSubmatch(sysStr)
	if len(matches) > 1 {
		body.System = strings.ReplaceAll(sysStr, matches[0], "")
		return matches[1]
	}
	return ""
}

func (r *Router) hasWebSearch(body *tokenizer.RequestBody) bool {
	for _, tool := range body.Tools {
		if strings.HasPrefix(tool.Type, "web_search") {
			return true
		}
	}
	return false
}

func (r *Router) hasImage(body *tokenizer.RequestBody) bool {
	for _, msg := range body.Messages {
		if content, ok := msg.Content.([]any); ok {
			for _, item := range content {
				if itemMap, ok := item.(map[string]any); ok {
					if itemType, ok := itemMap["type"].(string); ok && itemType == "image" {
						return true
					}
				}
			}
		}
	}
	return false
}

func (r *Router) hasThinking(body *tokenizer.RequestBody) bool {
	return body.Thinking != nil
}

func SearchProjectBySession(sessionId string) string {
	if resultVal, ok := cache.SessionProject.Get(sessionId); ok {
		if result, ok := resultVal.(string); ok {
			return result
		}
	}

	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}

	projectsDir := filepath.Join(home, ClaudeProjectsDir)
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		cache.SessionProject.Put(sessionId, "")
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionFile := filepath.Join(projectsDir, entry.Name(), sessionId+".jsonl")
		if _, err := os.Stat(sessionFile); err == nil {
			cache.SessionProject.Put(sessionId, entry.Name())
			return entry.Name()
		}
	}

	cache.SessionProject.Put(sessionId, "")
	return ""
}

func (r *Router) loadCustomRouter(path string, reqBody *tokenizer.RequestBody, sessionId string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	code := string(data)

	// 创建 JavaScript 虚拟机
	vm := otto.New()

	// 设置上下文对象
	ctx := map[string]interface{}{
		"model":     reqBody.Model,
		"tokenCount": tokenizer.CountRequestTokens(reqBody),
		"sessionId": sessionId,
	}

	// 将上下文转换为 JSON 以便传递给 JavaScript
	ctxJSON, _ := json.Marshal(ctx)

	// 构建完整的 JavaScript 代码
	script := `
		var context = ` + string(ctxJSON) + `;
		var reqBody = context;
		var sessionId = context.sessionId;

		// 提供一个简单的日志函数
		function log(msg) {
			console.log(msg);
		}

		// 执行用户代码
		` + code + `

		// 返回结果
		if (typeof route === 'function') {
			route(context);
		}

		// 返回选中的模型
		context.model || "";
	`

	// 执行 JavaScript
	result, err := vm.Run(script)
	if err != nil {
		// JavaScript 执行错误，记录日志但不中断
		return ""
	}

	// 获取返回值
	if result.IsString() {
		model, _ := result.ToString()
		return model
	}

	// 检查 context 对象是否被修改
	if val, err := vm.Get("context"); err == nil {
		if obj := val.Object(); obj != nil {
			if modelVal, err := obj.Get("model"); err == nil && modelVal.IsString() {
				model, _ := modelVal.ToString()
				return model
			}
		}
	}

	return ""
}

func (r *Router) loadProjectRouter(sessionId string) *config.RouterConfig {
	if sessionId == "" {
		return nil
	}

	project := SearchProjectBySession(sessionId)
	if project == "" {
		return nil
	}

	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}

	sessionConfigPath := filepath.Join(home, ".claude", "projects", project, sessionId+".json")
	if data, err := os.ReadFile(sessionConfigPath); err == nil {
		var cfg map[string]any
		if json.Unmarshal(data, &cfg) == nil {
			if router, ok := cfg["Router"].(map[string]any); ok {
				return &config.RouterConfig{
					Default: getString(router, "default"),
					Background: getString(router, "background"),
					Think: getString(router, "think"),
					LongContext: getString(router, "longContext"),
					LongContextThreshold: getInt(router, "longContextThreshold"),
					WebSearch: getString(router, "webSearch"),
				}
			}
		}
	}

	projectConfigPath := filepath.Join(home, ".claude", "projects", project, "config.json")
	if data, err := os.ReadFile(projectConfigPath); err == nil {
		var cfg map[string]any
		if json.Unmarshal(data, &cfg) == nil {
			if router, ok := cfg["Router"].(map[string]any); ok {
				return &config.RouterConfig{
					Default: getString(router, "default"),
					Background: getString(router, "background"),
					Think: getString(router, "think"),
					LongContext: getString(router, "longContext"),
					LongContextThreshold: getInt(router, "longContextThreshold"),
					WebSearch: getString(router, "webSearch"),
				}
			}
		}
	}

	return nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
