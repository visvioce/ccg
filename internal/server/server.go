package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	"github.com/musistudio/ccg/internal/transformer"
	"github.com/musistudio/ccg/internal/tokenizer"
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
		port = "3000"
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

	engine.GET("/", s.handleRoot)

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
				"id":         p.Name + "," + m,
				"object":     "model",
				"owned_by":   p.Name,
				"provider":   p.Name,
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

	config := map[string]any{
		"providers": providers,
		"router":   routerCfg,
	}

	c.JSON(200, config)
}

func (s *Server) handleUpdateConfig(c *gin.Context) {
	var newConfig map[string]any
	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	data, _ := json.Marshal(newConfig)
	configPath := config.GetDefaultConfigPath()
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save config: " + err.Error()})
		return
	}

	s.cfg.Load(configPath)

	c.JSON(200, gin.H{"success": true, "message": "Configuration saved. Restart to apply changes."})
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
	bodyVal, _ := c.Get("requestBody")
	body, ok := bodyVal.(map[string]any)
	if !ok {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	providerVal, _ := c.Get("provider")
	sessionIdVal, _ := c.Get("sessionId")

	model := body["model"].(string)
	providerName := ""
	if pv, ok := providerVal.(string); ok {
		providerName = pv
	}

	sessionId := ""
	if sv, ok := sessionIdVal.(string); ok {
		sessionId = sv
	}

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

	stream := false
	if bstream, ok := body["stream"].(bool); ok {
		stream = bstream
	}

	body["model"] = model

	if agent.ShouldHandleAgentTools(body) {
		agent.AddAgentToolsToRequest(body)
	}

	transformedBody := s.transformer.TransformRequest(body, providerName, transforms)

	reqBody, _ := json.Marshal(transformedBody)

	startTime := time.Now()

	proxyReq, err := http.NewRequest("POST", providerHost, strings.NewReader(string(reqBody)))
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

	if !stream && resp.StatusCode == 200 {
		var respData map[string]any
		if bodyBytes, err := io.ReadAll(resp.Body); err == nil {
			json.Unmarshal(bodyBytes, &respData)
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
