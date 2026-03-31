package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/musistudio/ccg/internal/agent"
	"github.com/musistudio/ccg/internal/cache"
	"github.com/musistudio/ccg/internal/config"
	"github.com/musistudio/ccg/internal/logger"
	"github.com/musistudio/ccg/internal/middleware"
	"github.com/musistudio/ccg/internal/plugin"
	"github.com/musistudio/ccg/internal/preset"
	"github.com/musistudio/ccg/internal/provider"
	"github.com/musistudio/ccg/internal/router"
	"github.com/musistudio/ccg/internal/tokenizer"
	"github.com/musistudio/ccg/internal/transformer"
)

type Server struct {
	cfg             *config.Config
	router          *router.Router
	providerService *provider.ProviderService
	transformer     *transformer.TransformerRegistry
	agentManager    *agent.AgentManager
	pluginManager   *plugin.PluginManager
	presetManager   *preset.PresetManager
	logger          *logger.Logger
	engine          *gin.Engine
	addr            string
	authEnabled     bool
	apiKey          string
}

func New() *Server {
	cfg := config.New()

	configPath := config.GetDefaultConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		if err := cfg.Load(configPath); err != nil {
			log.Printf("Warning: failed to load config: %v", err)
		}
	}

	tokenizer.Init()

	registry := transformer.BuildDefaultRegistry()
	providerSvc := provider.New(cfg, registry)
	rtr := router.New(cfg)

	authEnabled := cfg.Get("HOST") != "0.0.0.0" && cfg.Get("APIKEY") != ""

	return &Server{
		cfg:             cfg,
		router:          rtr,
		providerService: providerSvc,
		transformer:     registry,
		agentManager:    agent.GlobalAgentManager,
		pluginManager:   plugin.GlobalPluginManager,
		presetManager:   preset.NewPresetManager(),
		logger:          logger.GetLogger(),
		addr:            getAddr(cfg),
		authEnabled:     authEnabled,
		apiKey:          cfg.Get("APIKEY"),
	}
}

func getAddr(cfg *config.Config) string {
	port := cfg.Get("PORT")
	if port == "" {
		port = "3456"
	}
	host := cfg.Get("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	return host + ":" + port
}

func (s *Server) Setup() *gin.Engine {
	if s.engine != nil {
		return s.engine
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.Logger())

	engine.Use(middleware.CORSMiddleware())
	engine.Use(middleware.ModelParseMiddleware())

	if s.authEnabled {
		engine.Use(middleware.AuthMiddleware(s.cfg.Get("HOST"), s.apiKey))
	}

	engine.POST("/v1/messages", s.handleV1Messages)
	engine.POST("/v1/chat/completions", s.handleV1ChatCompletions)
	engine.POST("/v1/messages/count_tokens", s.handleCountTokens)
	engine.GET("/health", s.handleHealth)
	engine.GET("/v1/models", s.handleModels)

	// Root path will be handled by Web UI if available, otherwise handleRoot
	// engine.GET("/", s.handleRoot) // Moved to after Web UI setup

	api := engine.Group("/api")
	{
		api.GET("/config", s.handleGetConfig)
		api.POST("/config", s.handleUpdateConfig)
		api.GET("/providers", s.handleGetProviders)
		api.POST("/providers", s.handleAddProvider)
		api.PUT("/providers/:index", s.handleUpdateProvider)
		api.DELETE("/providers/:index", s.handleDeleteProvider)

		api.GET("/transformers", s.handleGetTransformers)

		api.GET("/plugins", s.handleGetPlugins)
		api.POST("/plugins/:name/enable", s.handleEnablePlugin)
		api.POST("/plugins/:name/disable", s.handleDisablePlugin)

		api.GET("/agents", s.handleGetAgents)
		api.GET("/agents/tools", s.handleGetAgentTools)

		api.GET("/token-stats", s.handleGetTokenStats)
		api.GET("/global-token-stats", s.handleGetGlobalTokenStats)
	}

	engine.GET("/presets", s.handleListPresets)
	engine.GET("/presets/:name", s.handleGetPreset)
	engine.POST("/presets/install", s.handleInstallPreset)
	engine.POST("/presets/:name/apply", s.handleApplyPreset)
	engine.DELETE("/presets/:name", s.handleDeletePreset)

	engine.POST("/restart", s.handleRestart)

	engine.GET("/logs/files", s.handleLogFiles)
	engine.GET("/logs", s.handleGetLogs)
	engine.DELETE("/logs", s.handleClearLogs)

	// Web UI compatible endpoints (without /api prefix)
	engine.GET("/config", s.handleGetConfig)
	engine.POST("/config", s.handleUpdateConfig)

	// Update endpoints for Web UI
	engine.GET("/update/check", s.handleCheckUpdate)
	engine.POST("/api/update/perform", s.handlePerformUpdate)

	// Serve Web UI static files
	webUIDir := os.Getenv("CCG_WEB_UI_DIR")
	if webUIDir == "" {
		// Look for web UI in common locations
		possiblePaths := []string{
			"./webui/dist",
			"../webui/dist",
			"/usr/share/ccg/webui",
			filepath.Join(config.GetConfigDir(), "webui"),
		}
		for _, path := range possiblePaths {
			if _, err := os.Stat(path); err == nil {
				webUIDir = path
				break
			}
		}
	}

	if webUIDir != "" {
		log.Printf("Serving Web UI from: %s", webUIDir)
		// Serve static files
		engine.Static("/assets", filepath.Join(webUIDir, "assets"))
		engine.StaticFile("/favicon.ico", filepath.Join(webUIDir, "favicon.ico"))

		// Root handler to serve Web UI
		engine.GET("/", func(c *gin.Context) {
			c.File(filepath.Join(webUIDir, "index.html"))
		})

		// SPA fallback - serve index.html for all non-API routes
		engine.NoRoute(func(c *gin.Context) {
			// Don't interfere with API routes
			if strings.HasPrefix(c.Request.URL.Path, "/api/") ||
				strings.HasPrefix(c.Request.URL.Path, "/v1/") ||
				c.Request.URL.Path == "/config" ||
				c.Request.URL.Path == "/restart" ||
				c.Request.URL.Path == "/update/check" ||
				c.Request.URL.Path == "/logs" ||
				c.Request.URL.Path == "/logs/files" ||
				c.Request.URL.Path == "/presets" ||
				c.Request.URL.Path == "/health" {
				c.JSON(404, gin.H{"error": "Not found"})
				return
			}

			// Serve index.html for SPA routes
			indexPath := filepath.Join(webUIDir, "index.html")
			c.File(indexPath)
		})
	} else {
		// No Web UI available, use default root handler
		engine.GET("/", s.handleRoot)
	}

	s.engine = engine
	return engine
}

func (s *Server) handleRoot(c *gin.Context) {
	c.JSON(200, gin.H{"message": "LLMs API", "version": "2.0.0"})
}

func (s *Server) handleCountTokens(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	reqBody := toRequestBody(body)
	count := tokenizer.CountRequestTokens(reqBody)

	c.JSON(200, gin.H{"tokens": count})
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "version": "2.0.0"})
}

func (s *Server) handleModels(c *gin.Context) {
	providers := s.cfg.GetProviders()
	models := make([]map[string]any, 0)

	for _, p := range providers {
		for _, m := range p.Models {
			models = append(models, map[string]any{
				"id":       p.Name + "," + m,
				"object":   "model",
				"owned_by": p.Name,
				"provider": p.Name,
			})
		}
	}

	c.JSON(200, gin.H{
		"object": "list",
		"data":   models,
	})
}

func (s *Server) handleGetConfig(c *gin.Context) {
	providers := s.cfg.GetProviders()
	routerCfg := s.cfg.GetRouter()

	// Convert providers to Web UI format
	uiProviders := make([]map[string]any, len(providers))
	for i, p := range providers {
		uiProviders[i] = map[string]any{
			"name":         p.Name,
			"api_base_url": p.Host,
			"api_key":      p.APIKey,
			"models":       p.Models,
		}
	}

	// Convert router to Web UI format
	uiRouter := map[string]any{}
	if routerCfg != nil {
		uiRouter = map[string]any{
			"default":              routerCfg.Default,
			"background":           routerCfg.Background,
			"think":                routerCfg.Think,
			"longContext":          routerCfg.LongContext,
			"longContextThreshold": routerCfg.LongContextThreshold,
			"webSearch":            routerCfg.WebSearch,
			"image":                routerCfg.Image,
		}
	}

	// Build full config in Web UI expected format
	config := map[string]any{
		"Providers": uiProviders,
		"Router":    uiRouter,
		"transformers": []map[string]any{},
		"LOG":       s.cfg.Get("LOG") == "true",
		"LOG_LEVEL": s.cfg.Get("LOG_LEVEL"),
		"CLAUDE_PATH": s.cfg.Get("CLAUDE_PATH"),
		"HOST":        s.cfg.Get("HOST"),
		"PORT":        s.cfg.GetInt("PORT"),
		"APIKEY":      s.cfg.Get("APIKEY"),
		"API_TIMEOUT_MS": s.cfg.Get("API_TIMEOUT_MS"),
		"PROXY_URL":   s.cfg.Get("PROXY_URL"),
	}

	c.JSON(200, config)
}

func (s *Server) handleUpdateConfig(c *gin.Context) {
	var newConfig map[string]any
	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Handle both Web UI format (Providers with capital P) and internal format
	providersData, ok := newConfig["Providers"].([]any)
	if !ok {
		providersData, _ = newConfig["providers"].([]any)
	}

	// Convert providers from Web UI format to internal format
	var providers []config.Provider
	for _, p := range providersData {
		if pMap, ok := p.(map[string]any); ok {
			provider := config.Provider{
				Name:   getString(pMap, "name"),
				Host:   getString(pMap, "api_base_url"),
				APIKey: getString(pMap, "api_key"),
			}
			if models, ok := pMap["models"].([]any); ok {
				for _, m := range models {
					if modelStr, ok := m.(string); ok {
						provider.Models = append(provider.Models, modelStr)
					}
				}
			}
			providers = append(providers, provider)
		}
	}

	// Handle router config
	routerData, ok := newConfig["Router"].(map[string]any)
	if !ok {
		routerData, _ = newConfig["router"].(map[string]any)
	}

	routerCfg := &config.RouterConfig{}
	if routerData != nil {
		routerCfg.Default = getString(routerData, "default")
		routerCfg.Background = getString(routerData, "background")
		routerCfg.Think = getString(routerData, "think")
		routerCfg.LongContext = getString(routerData, "longContext")
		routerCfg.WebSearch = getString(routerData, "webSearch")
		routerCfg.Image = getString(routerData, "image")
		if threshold, ok := routerData["longContextThreshold"].(float64); ok {
			routerCfg.LongContextThreshold = int(threshold)
		}
	}

	// Build config data to save
	configToSave := map[string]any{
		"providers": providers,
		"router":    routerCfg,
	}

	// Add other settings if present
	for _, key := range []string{"LOG", "LOG_LEVEL", "CLAUDE_PATH", "HOST", "PORT", "APIKEY", "API_TIMEOUT_MS", "PROXY_URL"} {
		if val, ok := newConfig[key]; ok {
			configToSave[key] = val
		}
	}

	data, _ := json.MarshalIndent(configToSave, "", "  ")
	configPath := config.GetDefaultConfigPath()
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save config: " + err.Error()})
		return
	}

	s.cfg.Load(configPath)

	c.JSON(200, gin.H{"success": true, "message": "Configuration saved successfully"})
}

// getString safely gets a string value from a map
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (s *Server) handleGetProviders(c *gin.Context) {
	providers := s.cfg.GetProviders()
	c.JSON(200, providers)
}

func (s *Server) handleAddProvider(c *gin.Context) {
	var provider config.Provider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	providers := s.cfg.GetProviders()
	providers = append(providers, provider)
	s.saveProviders(providers)

	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleUpdateProvider(c *gin.Context) {
	var provider config.Provider
	if err := c.ShouldBindJSON(&provider); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	index := 0
	fmt.Sscanf(c.Param("index"), "%d", &index)

	providers := s.cfg.GetProviders()
	if index < 0 || index >= len(providers) {
		c.JSON(400, gin.H{"error": "Invalid index"})
		return
	}

	providers[index] = provider
	s.saveProviders(providers)

	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleDeleteProvider(c *gin.Context) {
	index := 0
	fmt.Sscanf(c.Param("index"), "%d", &index)

	providers := s.cfg.GetProviders()
	if index < 0 || index >= len(providers) {
		c.JSON(400, gin.H{"error": "Invalid index"})
		return
	}

	providers = append(providers[:index], providers[index+1:]...)
	s.saveProviders(providers)

	c.JSON(200, gin.H{"success": true})
}

func (s *Server) saveProviders(providers []config.Provider) {
	data := map[string]any{
		"providers": providers,
		"router":    s.cfg.GetRouter(),
	}

	configPath := config.GetDefaultConfigPath()
	dataBytes, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(configPath, dataBytes, 0644)

	s.cfg.Load(configPath)
}

func (s *Server) handleGetTransformers(c *gin.Context) {
	transformers := []map[string]any{
		{"name": "anthropic", "endpoint": nil},
		{"name": "anthropic->openai", "endpoint": nil},
		{"name": "openai->anthropic", "endpoint": nil},
		{"name": "deepseek", "endpoint": nil},
		{"name": "gemini", "endpoint": nil},
		{"name": "google", "endpoint": nil},
		{"name": "groq", "endpoint": nil},
		{"name": "openrouter", "endpoint": nil},
		{"name": "cerebras", "endpoint": nil},
		{"name": "maxtoken", "endpoint": nil},
		{"name": "maxcompletiontokens", "endpoint": nil},
		{"name": "forcereasoning", "endpoint": nil},
		{"name": "sampling", "endpoint": nil},
		{"name": "streamoptions", "endpoint": nil},
		{"name": "cleancache", "endpoint": nil},
		{"name": "enhancetool", "endpoint": nil},
		{"name": "customparams", "endpoint": nil},
		{"name": "vercel", "endpoint": nil},
		{"name": "reasoning", "endpoint": nil},
		{"name": "tooluse", "endpoint": nil},
		{"name": "rewritesystemprompt", "endpoint": nil},
	}

	c.JSON(200, gin.H{"transformers": transformers})
}

func (s *Server) handleGetPlugins(c *gin.Context) {
	plugins := s.pluginManager.GetAllPlugins()
	result := make([]map[string]any, 0, len(plugins))

	for _, p := range plugins {
		result = append(result, map[string]any{
			"name":    p.Metadata.Name,
			"enabled": p.Metadata.Enabled,
			"options": p.Metadata.Options,
		})
	}

	c.JSON(200, result)
}

func (s *Server) handleEnablePlugin(c *gin.Context) {
	name := c.Param("name")
	if err := s.pluginManager.EnablePlugin(name); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleDisablePlugin(c *gin.Context) {
	name := c.Param("name")
	if err := s.pluginManager.DisablePlugin(name); err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleGetAgents(c *gin.Context) {
	agents := []map[string]any{
		{"name": "image", "enabled": true},
	}

	c.JSON(200, agents)
}

func (s *Server) handleGetAgentTools(c *gin.Context) {
	tools := agent.GetAgentTools()
	c.JSON(200, tools)
}

func (s *Server) handleGetTokenStats(c *gin.Context) {
	provider := c.Query("provider")
	model := c.Query("model")

	stats := plugin.GetTokenSpeedStats(provider, model)
	c.JSON(200, stats)
}

func (s *Server) handleGetGlobalTokenStats(c *gin.Context) {
	stats := plugin.GetGlobalTokenSpeedStats()
	c.JSON(200, stats)
}

func (s *Server) handleListPresets(c *gin.Context) {
	presets, err := s.presetManager.ListPresets()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"presets": presets})
}

func (s *Server) handleGetPreset(c *gin.Context) {
	name := c.Param("name")
	preset, err := s.presetManager.GetPreset(name)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, preset)
}

func (s *Server) handleInstallPreset(c *gin.Context) {
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := s.presetManager.InstallPreset(req.URL, req.Name); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleApplyPreset(c *gin.Context) {
	name := c.Param("name")

	var req struct {
		Secrets map[string]string `json:"secrets"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if err := s.presetManager.ApplyPreset(name, req.Secrets); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleDeletePreset(c *gin.Context) {
	name := c.Param("name")
	if err := s.presetManager.DeletePreset(name); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleRestart(c *gin.Context) {
	c.JSON(200, gin.H{"success": true, "message": "Service restart triggered"})
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}

func (s *Server) handleLogFiles(c *gin.Context) {
	files := s.logger.GetLogFiles()
	c.JSON(200, files)
}

func (s *Server) handleGetLogs(c *gin.Context) {
	filePath := c.Query("file")
	if filePath != "" {
		content, err := s.logger.GetLogContent(filePath)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, content)
		return
	}

	entries := s.logger.GetEntries("", 100)
	c.JSON(200, entries)
}

func (s *Server) handleClearLogs(c *gin.Context) {
	if err := s.logger.ClearLogs(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleV1Messages(c *gin.Context) {
	// 1. 获取请求体
	bodyVal, _ := c.Get("requestBody")
	body, ok := bodyVal.(map[string]any)
	if !ok {
		body = make(map[string]any)
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}
	}

	// 2. 获取provider和sessionId
	providerVal, _ := c.Get("provider")
	sessionIdVal, _ := c.Get("sessionId")

	model := ""
	if m, ok := body["model"].(string); ok {
		model = m
	}
	providerName := ""
	if pv, ok := providerVal.(string); ok {
		providerName = pv
	}
	sessionId := ""
	if sv, ok := sessionIdVal.(string); ok {
		sessionId = sv
	}

	// 3. 路由选择provider和model
	providerHost := ""
	providerAPIKey := ""
	transforms := []string{}

	if providerName == "" {
		providers := s.cfg.GetProviders()
		routerResult := s.router.Route(toRequestBody(body), sessionId)
		model = routerResult.Model

		for _, p := range providers {
			for _, m := range p.Models {
				if m == model || model == p.Name+","+m {
					providerName = p.Name
					providerHost = p.Host
					providerAPIKey = p.APIKey
					transforms = s.cfg.GetProviderTransform(p.Name, model)
					break
				}
			}
			if providerName != "" {
				break
			}
		}
	}

	if providerName == "" {
		providerName = "openai"
	}

	if p := s.cfg.GetProvider(providerName); p != nil {
		providerHost = p.Host
		providerAPIKey = p.APIKey
		transforms = s.cfg.GetProviderTransform(providerName, model)
	}

	if providerHost == "" {
		providerHost = s.providerService.GetDefaultHost(providerName)
	}

	// 4. 检查是否流式响应
	stream := false
	if bstream, ok := body["stream"].(bool); ok {
		stream = bstream
	}

	// 5. 更新model - 提取实际的model名称（去掉provider前缀）
	actualModel := model
	if strings.Contains(model, ",") {
		parts := strings.SplitN(model, ",", 2)
		if len(parts) == 2 {
			actualModel = parts[1]
		}
	}
	body["model"] = actualModel

	// 6. 处理agent工具
	if agent.ShouldHandleAgentTools(body) {
		agent.AddAgentToolsToRequest(body)
	}

	// 7. 转换请求格式（Anthropic -> OpenAI）
	transformedBody := s.transformer.TransformRequest(body, providerName, transforms)
	reqBody, _ := json.Marshal(transformedBody)

	// Debug: 打印转换后的请求
	log.Printf("Debug: providerName=%s, providerHost=%s", providerName, providerHost)
	log.Printf("Debug: transformedBody=%s", string(reqBody))

	// 8. 发送请求到provider
	startTime := time.Now()
	proxyReq, err := http.NewRequest("POST", providerHost, bytes.NewReader(reqBody))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	if providerAPIKey != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+providerAPIKey)
	}
	if stream {
		proxyReq.Header.Set("Accept", "text/event-stream")
	}

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		c.JSON(502, gin.H{"error": "Failed to proxy request: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// 9. 复制响应头
	for k, v := range resp.Header {
		if k != "Content-Length" && k != "Transfer-Encoding" {
			c.Header(k, v[0])
		}
	}

	// 10. 处理响应
	if stream {
		// 流式响应：转换格式后转发
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(resp.StatusCode)
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(500, gin.H{"error": "Streaming not supported"})
			return
		}

		// 使用流式转换器
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), 1024*1024) // 增加缓冲区大小
		
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
					fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
					flusher.Flush()
					continue
				}
				
				// 转换 chunk 格式
				converted := transformer.ProcessStreamChunk([]byte(data), providerName)
				fmt.Fprintf(c.Writer, "data: %s\n\n", converted)
				flusher.Flush()
			}
		}
	} else {
		// 非流式响应：读取并转换
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("Debug: respBody length=%d", len(respBody))
		if len(respBody) > 0 {
			log.Printf("Debug: respBody=%s", string(respBody[:min(500, len(respBody))]))
		}

		// 处理使用统计
		if resp.StatusCode == 200 {
			var respData map[string]any
			if err := json.Unmarshal(respBody, &respData); err == nil {
				if usage, ok := respData["usage"].(map[string]any); ok {
					inputTokens := 0
					outputTokens := 0
					if it, ok := usage["input_tokens"].(float64); ok {
						inputTokens = int(it)
					}
					if ot, ok := usage["output_tokens"].(float64); ok {
						outputTokens = int(ot)
					}
					plugin.RecordTokenSpeed(providerName, model, inputTokens, outputTokens, duration)
					if sessionId != "" {
						cache.SessionUsage.Put(sessionId, cache.Usage{
							InputTokens:  inputTokens,
							OutputTokens: outputTokens,
						})
					}
				}
			}
		}

		// 11. 处理 Agent 工具调用（非流式响应）
		if resp.StatusCode == 200 {
			var respData map[string]any
			if err := json.Unmarshal(respBody, &respData); err == nil {
				// 检查是否有工具调用
				if choices, ok := respData["choices"].([]any); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]any); ok {
						if message, ok := choice["message"].(map[string]any); ok {
							if toolCalls, ok := message["tool_calls"].([]any); ok && len(toolCalls) > 0 {
								// 处理 Agent 工具调用
								for _, tc := range toolCalls {
									if tcMap, ok := tc.(map[string]any); ok {
										if function, ok := tcMap["function"].(map[string]any); ok {
											toolName, _ := function["name"].(string)
											if toolName != "" {
												var input map[string]any
												if args, ok := function["arguments"].(string); ok {
													json.Unmarshal([]byte(args), &input)
												}
																							// 调用 Agent 处理工具
																							result, err := agent.HandleAgentToolCallV2(toolName, input, body, s.cfg, sessionId)
																							if err == nil && result != "" {
																								// 将工具结果添加到响应中
																								message["content"] = result
																								delete(message, "tool_calls")
																								// 重新序列化响应
																								respBody, _ = json.Marshal(respData)
																							}
																						}
																					}
																				}
																			}
																		}
																	}
																}
															}
														}
													}
		// 12. 转换响应格式（OpenAI -> Anthropic）
		transformedResp := s.transformer.TransformResponse(respBody, providerName, transforms)
		log.Printf("Debug: transformedResp length=%d", len(transformedResp))
		c.Data(resp.StatusCode, "application/json", transformedResp)
	}
}

func (s *Server) handleV1ChatCompletions(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	model, _ := body["model"].(string)
	providerName := ""
	providerHost := ""
	providerAPIKey := ""
	transforms := []string{}

	sessionId := ""

	providers := s.cfg.GetProviders()
	routerResult := s.router.Route(toRequestBody(body), sessionId)
	model = routerResult.Model

	for _, p := range providers {
		for _, m := range p.Models {
			if m == model || model == p.Name+","+m {
				providerName = p.Name
				providerHost = p.Host
				providerAPIKey = p.APIKey
				transforms = s.cfg.GetProviderTransform(p.Name, model)
				break
			}
		}
		if providerName != "" {
			break
		}
	}

	if providerName == "" {
		providerName = "openai"
	}

	if p := s.cfg.GetProvider(providerName); p != nil {
		providerHost = p.Host
		providerAPIKey = p.APIKey
		transforms = s.cfg.GetProviderTransform(providerName, model)
	}

	if providerHost == "" {
		providerHost = s.providerService.GetDefaultHost(providerName)
	}

	stream := false
	if bstream, ok := body["stream"].(bool); ok {
		stream = bstream
	}

	transformedBody := s.transformer.TransformRequest(body, providerName, transforms)

	reqBody, _ := json.Marshal(transformedBody)

	url := providerHost
	if !strings.HasSuffix(url, "/v1/chat/completions") {
		url = strings.TrimRight(url, "/")
		url += "/v1/chat/completions"
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(reqBody)))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if providerAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+providerAPIKey)
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	resp, err := http.DefaultClient.Transport.RoundTrip(req)
	if err != nil {
		c.JSON(502, gin.H{"error": "Failed to proxy request: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		c.Header(k, v[0])
	}

	if stream {
		c.Stream(func(w io.Writer) bool {
			_, err := io.Copy(w, resp.Body)
			return err == nil
		})
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		transformedResp := s.transformer.TransformResponse(respBody, providerName, transforms)
		c.Data(resp.StatusCode, "application/json", transformedResp)
	}
}

func toRequestBody(m map[string]any) *tokenizer.RequestBody {
	body := &tokenizer.RequestBody{
		Model: "",
	}

	if v, ok := m["model"].(string); ok {
		body.Model = v
	}

	if v, ok := m["messages"].([]any); ok {
		body.Messages = make([]tokenizer.Message, len(v))
		for i, msg := range v {
			if msgMap, ok := msg.(map[string]any); ok {
				tokenizerMsg := tokenizer.Message{}
				if role, ok := msgMap["role"].(string); ok {
					tokenizerMsg.Role = role
				}
				if content, ok := msgMap["content"]; ok {
					tokenizerMsg.Content = content
				}
				body.Messages[i] = tokenizerMsg
			}
		}
	}

	if v, ok := m["system"]; ok {
		body.System = v
	}

	if v, ok := m["tools"].([]any); ok {
		body.Tools = make([]tokenizer.Tool, len(v))
		for i, tool := range v {
			if toolMap, ok := tool.(map[string]any); ok {
				t := tokenizer.Tool{}
				if name, ok := toolMap["name"].(string); ok {
					t.Name = name
				}
				if desc, ok := toolMap["description"].(string); ok {
					t.Description = desc
				}
				if schema, ok := toolMap["input_schema"]; ok {
					t.InputSchema = schema
				}
				body.Tools[i] = t
			}
		}
	}

	if v, ok := m["thinking"]; ok {
		body.Thinking = v
	}

	return body
}

func (s *Server) Start() error {
	engine := s.Setup()

	go func() {
		log.Printf("Starting CCG server on %s", s.addr)
		if err := engine.Run(s.addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	return nil
}

// handleCheckUpdate checks for updates (Web UI compatible)
func (s *Server) handleCheckUpdate(c *gin.Context) {
	// For now, return no updates available
	// This can be extended to check GitHub releases
	c.JSON(200, gin.H{
		"hasUpdate": false,
	})
}

// handlePerformUpdate performs update (Web UI compatible)
func (s *Server) handlePerformUpdate(c *gin.Context) {
	// For now, return success but don't actually update
	// This can be extended to perform actual updates
	c.JSON(200, gin.H{
		"success": true,
		"message": "CCG is up to date",
	})
}
