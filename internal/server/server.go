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
	presetConfigs   map[string]map[string]any // preset name -> config
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

	// Enable auth middleware when:
	// 1. APIKEY is set (always require auth), OR
	// 2. HOST is 0.0.0.0 (public access, need auth check)
	authEnabled := cfg.Get("APIKEY") != "" || cfg.Get("HOST") == "0.0.0.0"

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
		presetConfigs:   make(map[string]map[string]any),
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

	// Auth applied per-route group, not globally (CCR behavior: / and /health are public)

	// Core endpoints (matching @musistudio/llms)
	engine.GET("/", s.handleRoot)
	engine.GET("/health", s.handleHealth)

	// Main endpoint with namespace routing support
	engine.POST("/v1/messages", s.handleV1Messages)
	engine.POST("/v1/messages/count_tokens", s.handleCountTokens)

	// Auth middleware for protected routes
	if s.authEnabled {
		authMw := middleware.AuthMiddleware(s.cfg.Get("HOST"), s.apiKey, s.cfg.Get("PORT"))
		engine.Use(authMw)
	}

	api := engine.Group("/api")
	{
		api.GET("/config", s.handleGetConfig)
		api.POST("/config", s.handleUpdateConfig)
		api.GET("/transformers", s.handleGetTransformers)

		// Provider CRUD endpoints
		api.GET("/providers", s.handleGetProviders)
		api.POST("/providers", s.handleAddProvider)
		api.GET("/providers/:id", s.handleGetProvider)
		api.PUT("/providers/:id", s.handleUpdateProvider)
		api.DELETE("/providers/:id", s.handleDeleteProvider)

		api.GET("/presets", s.handleListPresets)
		api.GET("/presets/:name", s.handleGetPreset)
		api.POST("/presets/:name/apply", s.handleApplyPreset)
		api.DELETE("/presets/:name", s.handleDeletePreset)
		api.GET("/presets/market", s.handleGetMarketPresets)
		api.POST("/presets/install/github", s.handleInstallFromGithub)

		api.GET("/logs/files", s.handleLogFiles)
		api.GET("/logs", s.handleGetLogs)
		api.DELETE("/logs", s.handleClearLogs)

		api.POST("/update/perform", s.handlePerformUpdate)
		api.GET("/update/check", s.handleCheckUpdate)

		api.POST("/restart", s.handleRestart)
	}

	// Static files for Web UI
	engine.Static("/ui", "./web/dist")

	// Preset namespace routes
	engine.POST("/preset/:presetName/v1/messages", s.handlePresetV1Messages)
	engine.POST("/preset/:presetName/v1/messages/count_tokens", s.handlePresetCountTokens)

	s.loadPresetConfigs()
	s.registerPluginsFromConfig()
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
	c.JSON(200, gin.H{"status": "ok", "timestamp": time.Now().Unix()})
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
		"Providers":      uiProviders,
		"Router":         uiRouter,
		"transformers":   []map[string]any{},
		"LOG":            s.cfg.Get("LOG") == "true",
		"LOG_LEVEL":      s.cfg.Get("LOG_LEVEL"),
		"CLAUDE_PATH":    s.cfg.Get("CLAUDE_PATH"),
		"HOST":           s.cfg.Get("HOST"),
		"PORT":           s.cfg.GetInt("PORT"),
		"APIKEY":         s.cfg.Get("APIKEY"),
		"API_TIMEOUT_MS": s.cfg.Get("API_TIMEOUT_MS"),
		"PROXY_URL":      s.cfg.Get("PROXY_URL"),
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

func (s *Server) handleGetProvider(c *gin.Context) {
	id := c.Param("id")
	index := 0
	fmt.Sscanf(id, "%d", &index)

	providers := s.cfg.GetProviders()
	if index < 0 || index >= len(providers) {
		c.JSON(404, gin.H{"error": "Provider not found"})
		return
	}

	c.JSON(200, providers[index])
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

	id := c.Param("id")
	index := 0
	fmt.Sscanf(id, "%d", &index)

	providers := s.cfg.GetProviders()
	if index < 0 || index >= len(providers) {
		c.JSON(404, gin.H{"error": "Provider not found"})
		return
	}

	providers[index] = provider
	s.saveProviders(providers)

	c.JSON(200, gin.H{"success": true})
}

func (s *Server) handleDeleteProvider(c *gin.Context) {
	id := c.Param("id")
	index := 0
	fmt.Sscanf(id, "%d", &index)

	providers := s.cfg.GetProviders()
	if index < 0 || index >= len(providers) {
		c.JSON(404, gin.H{"error": "Provider not found"})
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
		{"name": "openai-responses", "endpoint": nil},
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

	presetDir := filepath.Join(config.GetPresetsDir(), name)

	// Read existing manifest
	manifestPath := filepath.Join(presetDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		c.JSON(404, gin.H{"error": "Preset not found"})
		return
	}

	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		c.JSON(500, gin.H{"error": "Invalid preset manifest"})
		return
	}

	// Save userValues
	if req.Secrets != nil && len(req.Secrets) > 0 {
		manifest["userValues"] = req.Secrets
	}

	// Write back manifest
	updatedData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, updatedData, 0644); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save preset"})
		return
	}

	// Reload preset configs
	s.loadPresetConfigs()

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

func (s *Server) handleGetMarketPresets(c *gin.Context) {
	presets, err := preset.GetMarketPresets()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"presets": presets})
}

func (s *Server) handlePresetUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 50<<20)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to read uploaded file: " + err.Error()})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		c.JSON(400, gin.H{"error": "Only ZIP files are supported"})
		return
	}

	presetName := c.PostForm("name")
	if presetName == "" {
		presetName = strings.TrimSuffix(header.Filename, ".zip")
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("ccg-upload-%d.zip", time.Now().UnixNano()))
	dst, err := os.Create(tmpFile)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to save uploaded file"})
		return
	}
	defer os.Remove(tmpFile)

	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		c.JSON(500, gin.H{"error": "Failed to save uploaded file"})
		return
	}
	dst.Close()

	if err := s.presetManager.InstallFromZip(tmpFile, presetName); err != nil {
		c.JSON(500, gin.H{"error": "Failed to install preset: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "presetName": presetName})
}

func (s *Server) handleInstallFromGithub(c *gin.Context) {
	var req struct {
		PresetName string `json:"presetName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.PresetName == "" {
		c.JSON(400, gin.H{"error": "Preset name is required"})
		return
	}

	// Find in marketplace
	marketPreset, err := preset.FindMarketPresetByName(req.PresetName)
	if err != nil {
		c.JSON(404, gin.H{"error": "Preset not found in marketplace: " + req.PresetName})
		return
	}

	if marketPreset.Repo == "" {
		c.JSON(400, gin.H{"error": "Preset has no repository information"})
		return
	}

	// Install from GitHub
	if err := s.presetManager.InstallFromGitHub(marketPreset.Repo, marketPreset.Name); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "presetName": marketPreset.Name})
}

func (s *Server) loadPresetConfigs() {
	presets, err := s.presetManager.ListPresets()
	if err != nil {
		log.Printf("Warning: failed to load preset configs: %v", err)
		return
	}

	for _, p := range presets {
		presetDir := filepath.Join(config.GetPresetsDir(), p.Name)
		cfg, err := s.presetManager.LoadConfigFromManifest(presetDir)
		if err != nil {
			log.Printf("Warning: failed to load preset config for %s: %v", p.Name, err)
			continue
		}
		s.presetConfigs[p.Name] = cfg
		log.Printf("Loaded preset namespace: /preset/%s", p.Name)
	}
}

func (s *Server) handlePresetV1Messages(c *gin.Context) {
	presetName := c.Param("presetName")

	presetCfg, ok := s.presetConfigs[presetName]
	if !ok {
		presetDir := filepath.Join(config.GetPresetsDir(), presetName)
		cfg, err := s.presetManager.LoadConfigFromManifest(presetDir)
		if err != nil {
			c.JSON(404, gin.H{"error": "Preset not found: " + presetName})
			return
		}
		s.presetConfigs[presetName] = cfg
		presetCfg = cfg
	}

	providers := s.getProvidersFromConfig(presetCfg)
	routerCfg := s.getRouterFromConfig(presetCfg)

	bodyVal, _ := c.Get("requestBody")
	body, ok := bodyVal.(map[string]any)
	if !ok {
		body = make(map[string]any)
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}
	}

	model := ""
	if m, ok := body["model"].(string); ok {
		model = m
	}

	if model == "" && routerCfg != nil && routerCfg.Default != "" {
		model = routerCfg.Default
	}

	var providerName, providerHost, providerAPIKey string
	var transforms []string

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

	if providerHost == "" {
		providerHost = s.providerService.GetDefaultHost(providerName)
	}

	stream := false
	if bstream, ok := body["stream"].(bool); ok {
		stream = bstream
	}

	bodyCopy := make(map[string]any)
	for k, v := range body {
		bodyCopy[k] = v
	}
	actualModel := s.getActualModel(model)
	bodyCopy["model"] = actualModel

	transformedBody := s.transformer.TransformRequest(bodyCopy, providerName, transforms)
	reqBody, _ := json.Marshal(transformedBody)

	proxyReq, err := http.NewRequest("POST", providerHost, bytes.NewReader(reqBody))
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}

	proxyReq.Header.Set("Content-Type", "application/json")
	if providerAPIKey != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+providerAPIKey)
	}
	if stream {
		proxyReq.Header.Set("Accept", "text/event-stream")
	}

	resp, err := s.providerService.GetHTTPClient().Do(proxyReq)
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(resp.StatusCode)

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.JSON(500, gin.H{"error": "Streaming not supported"})
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintf(c.Writer, "%s\n", line)
			flusher.Flush()
		}
	} else {
		respBody, _ := io.ReadAll(resp.Body)
		transformedResp := s.transformer.TransformResponse(respBody, providerName, transforms)
		c.Data(resp.StatusCode, "application/json", transformedResp)
	}
}

func (s *Server) handlePresetCountTokens(c *gin.Context) {
	s.handleCountTokens(c)
}

func (s *Server) getProvidersFromConfig(cfg map[string]any) []config.Provider {
	var providers []config.Provider
	if pData, ok := cfg["Providers"].([]any); ok {
		for _, p := range pData {
			if pMap, ok := p.(map[string]any); ok {
				provider := config.Provider{
					Name:   getString(pMap, "name"),
					Host:   getString(pMap, "api_base_url"),
					APIKey: getString(pMap, "api_key"),
				}
				if models, ok := pMap["models"].([]any); ok {
					for _, m := range models {
						if mStr, ok := m.(string); ok {
							provider.Models = append(provider.Models, mStr)
						}
					}
				}
				providers = append(providers, provider)
			}
		}
	}
	if len(providers) == 0 {
		if pData, ok := cfg["providers"].([]any); ok {
			for _, p := range pData {
				if pMap, ok := p.(map[string]any); ok {
					provider := config.Provider{
						Name:   getString(pMap, "name"),
						Host:   getString(pMap, "api_base_url"),
						APIKey: getString(pMap, "api_key"),
					}
					if models, ok := pMap["models"].([]any); ok {
						for _, m := range models {
							if mStr, ok := m.(string); ok {
								provider.Models = append(provider.Models, mStr)
							}
						}
					}
					providers = append(providers, provider)
				}
			}
		}
	}
	return providers
}

func (s *Server) getRouterFromConfig(cfg map[string]any) *config.RouterConfig {
	if rData, ok := cfg["Router"].(map[string]any); ok {
		return &config.RouterConfig{
			Default:     getString(rData, "default"),
			Background:  getString(rData, "background"),
			Think:       getString(rData, "think"),
			LongContext: getString(rData, "longContext"),
			WebSearch:   getString(rData, "webSearch"),
			Image:       getString(rData, "image"),
		}
	}
	return nil
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

type modelConfig struct {
	model          string
	providerName   string
	providerHost   string
	providerAPIKey string
	transforms     []string
}

func (s *Server) getModelConfig(model string, providers []config.Provider) *modelConfig {
	for _, p := range providers {
		for _, m := range p.Models {
			if m == model || model == p.Name+","+m {
				cfg := &modelConfig{
					model:          model,
					providerName:   p.Name,
					providerHost:   p.Host,
					providerAPIKey: p.APIKey,
					transforms:     s.cfg.GetProviderTransform(p.Name, model),
				}
				if cfg.providerHost == "" {
					cfg.providerHost = s.providerService.GetDefaultHost(cfg.providerName)
				}
				return cfg
			}
		}
	}
	return nil
}

func (s *Server) getActualModel(model string) string {
	if strings.Contains(model, ",") {
		parts := strings.SplitN(model, ",", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return model
}

func (s *Server) registerPluginsFromConfig() {
	pluginsData := s.cfg.GetPluginsFromConfig()
	if len(pluginsData) == 0 {
		return
	}
	for _, p := range pluginsData {
		s.registerPluginByName(p.Name, p.Enabled, p.Options)
	}
}

func (s *Server) registerPluginByName(name string, enabled bool, options map[string]any) {
	switch name {
	case "token-speed":
		s.pluginManager.EnablePlugin("token-speed")
		if !enabled {
			s.pluginManager.DisablePlugin("token-speed")
		}
	default:
		log.Printf("Warning: unknown plugin: %s", name)
	}
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
	providers := s.cfg.GetProviders()
	if providerName == "" {
		routerResult := s.router.Route(toRequestBody(body), sessionId)
		model = routerResult.Model
	}

	// 4. 构建模型列表（主要模型 + 备用模型）
	var modelList []string
	modelList = append(modelList, model)
	if fallbackCfg := s.cfg.GetFallback(); fallbackCfg != nil {
		if fallbackModels, ok := fallbackCfg[model]; ok {
			modelList = append(modelList, fallbackModels...)
		}
	}

	log.Printf("Fallback: Starting request with model list: %v", modelList)

	var lastErr error

	// 5. 按顺序尝试所有模型
	for idx, tryModel := range modelList {
		log.Printf("Fallback: Trying model %d/%d: %s", idx+1, len(modelList), tryModel)

		var cfg *modelConfig
		if idx == 0 && providerName != "" {
			if p := s.cfg.GetProvider(providerName); p != nil {
				cfg = &modelConfig{
					model:          tryModel,
					providerName:   p.Name,
					providerHost:   p.Host,
					providerAPIKey: p.APIKey,
					transforms:     s.cfg.GetProviderTransform(providerName, tryModel),
				}
				if cfg.providerHost == "" {
					cfg.providerHost = s.providerService.GetDefaultHost(cfg.providerName)
				}
			}
		}
		if cfg == nil {
			cfg = s.getModelConfig(tryModel, providers)
		}
		if cfg == nil {
			log.Printf("Fallback: Failed to find config for model %s", tryModel)
			lastErr = fmt.Errorf("model not found: %s", tryModel)
			continue
		}

		if cfg.providerName == "" {
			cfg.providerName = "openai"
		}

		if p := s.cfg.GetProvider(cfg.providerName); p != nil {
			cfg.providerHost = p.Host
			cfg.providerAPIKey = p.APIKey
			cfg.transforms = s.cfg.GetProviderTransform(cfg.providerName, cfg.model)
		}

		if cfg.providerHost == "" {
			cfg.providerHost = s.providerService.GetDefaultHost(cfg.providerName)
		}

		// 6. 检查是否流式响应
		stream := false
		if bstream, ok := body["stream"].(bool); ok {
			stream = bstream
		}

		// 7. 处理请求
		bodyCopy := make(map[string]any)
		for k, v := range body {
			bodyCopy[k] = v
		}
		actualModel := s.getActualModel(cfg.model)
		bodyCopy["model"] = actualModel

		if agent.ShouldHandleAgentTools(bodyCopy) {
			agent.AddAgentToolsToRequest(bodyCopy)
		}

		transformedBody := s.transformer.TransformRequest(bodyCopy, cfg.providerName, cfg.transforms)
		reqBody, _ := json.Marshal(transformedBody)

		log.Printf("Fallback: providerName=%s, providerHost=%s, model=%s", cfg.providerName, cfg.providerHost, actualModel)

		startTime := time.Now()
		proxyReq, err := http.NewRequest("POST", cfg.providerHost, bytes.NewReader(reqBody))
		if err != nil {
			log.Printf("Fallback: Failed to create request for model %s: %v", tryModel, err)
			lastErr = err
			continue
		}

		proxyReq.Header.Set("Content-Type", "application/json")
		if cfg.providerAPIKey != "" {
			proxyReq.Header.Set("Authorization", "Bearer "+cfg.providerAPIKey)
		}
		if stream {
			proxyReq.Header.Set("Accept", "text/event-stream")
		}

		resp, err := s.providerService.GetHTTPClient().Do(proxyReq)
		if err != nil {
			log.Printf("Fallback: Request failed for model %s: %v", tryModel, err)
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("Fallback: Request failed for model %s with status %d", tryModel, resp.StatusCode)
			lastErr = fmt.Errorf("HTTP error %d", resp.StatusCode)
			continue
		}

		log.Printf("Fallback: Success using model %s", tryModel)
		duration := time.Since(startTime)

		for k, v := range resp.Header {
			if k != "Content-Length" && k != "Transfer-Encoding" {
				c.Header(k, v[0])
			}
		}

		if stream {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Status(resp.StatusCode)
			flusher, ok := c.Writer.(http.Flusher)
			if !ok {
				c.JSON(500, gin.H{"error": "Streaming not supported"})
				return
			}

			scanner := bufio.NewScanner(resp.Body)
			scanner.Buffer(make([]byte, 4096), 1024*1024)

			// Agent tool call interception state
			var currentToolName string
			var currentToolArgs string
			var currentToolIndex int = -1
			var currentToolId string
			var toolMessages []map[string]any
			var assistantMessages []map[string]any
			hasAgents := agent.ShouldHandleAgentTools(bodyCopy)

			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == "" {
					fmt.Fprintf(c.Writer, "\n")
					flusher.Flush()
					continue
				}

				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")

					if data == "[DONE]" {
						// If we collected tool calls, make recursive request
						if hasAgents && len(toolMessages) > 0 {
							// Add tool use and result messages
							bodyCopy["messages"] = append(bodyCopy["messages"].([]map[string]any),
								map[string]any{
									"role":    "assistant",
									"content": assistantMessages,
								},
								map[string]any{
									"role":    "user",
									"content": toolMessages,
								},
							)

							// Make recursive request
							recursiveBody, _ := json.Marshal(bodyCopy)
							recursiveReq, _ := http.NewRequest("POST", "http://"+s.addr+"/v1/messages", bytes.NewReader(recursiveBody))
							recursiveReq.Header.Set("Content-Type", "application/json")
							if s.apiKey != "" {
								recursiveReq.Header.Set("x-api-key", s.apiKey)
							}

							recursiveResp, err := http.DefaultClient.Do(recursiveReq)
							if err == nil {
								defer recursiveResp.Body.Close()
								// Stream recursive response
								recursiveScanner := bufio.NewScanner(recursiveResp.Body)
								recursiveScanner.Buffer(make([]byte, 4096), 1024*1024)
								for recursiveScanner.Scan() {
									recLine := recursiveScanner.Text()
									fmt.Fprintf(c.Writer, "%s\n", recLine)
									flusher.Flush()
								}
							}
						}

						fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
						flusher.Flush()
						continue
					}

					// Parse SSE event for agent tool interception
					if hasAgents {
						var chunkData map[string]any
						if err := json.Unmarshal([]byte(data), &chunkData); err == nil {
							// Detect content_block_start with tool name
							if eventType, ok := chunkData["type"].(string); ok && eventType == "content_block_start" {
								if cb, ok := chunkData["content_block"].(map[string]any); ok {
									if toolName, ok := cb["name"].(string); ok {
										if agent.HasAgentTool(toolName) {
											currentToolName = toolName
											currentToolIndex = int(chunkData["index"].(float64))
											if id, ok := cb["id"].(string); ok {
												currentToolId = id
											}
											currentToolArgs = ""
											continue // Don't forward this event
										}
									}
								}
							}

							// Collect tool arguments
							if currentToolIndex >= 0 {
								if deltaType, ok := chunkData["type"].(string); ok && deltaType == "content_block_delta" {
									if idx, ok := chunkData["index"].(float64); ok && int(idx) == currentToolIndex {
										if delta, ok := chunkData["delta"].(map[string]any); ok {
											if partialJSON, ok := delta["partial_json"].(string); ok {
												currentToolArgs += partialJSON
												continue // Don't forward this event
											}
										}
									}
								}
							}

							// Detect content_block_stop for tool call completion
							if currentToolIndex >= 0 {
								if eventType, ok := chunkData["type"].(string); ok && eventType == "content_block_stop" {
									if idx, ok := chunkData["index"].(float64); ok && int(idx) == currentToolIndex {
										// Execute the tool
										var input map[string]any
										json.Unmarshal([]byte(currentToolArgs), &input)

										result, err := agent.HandleAgentToolCallV2(currentToolName, input, bodyCopy, s.cfg, sessionId)
										if err == nil && result != "" {
											assistantMessages = append(assistantMessages, map[string]any{
												"type":  "tool_use",
												"id":    currentToolId,
												"name":  currentToolName,
												"input": input,
											})
											toolMessages = append(toolMessages, map[string]any{
												"tool_use_id": currentToolId,
												"type":        "tool_result",
												"content":     result,
											})
										}

										// Reset state
										currentToolName = ""
										currentToolArgs = ""
										currentToolIndex = -1
										currentToolId = ""
										continue // Don't forward this event
									}
								}
							}
						}
					}

					// Session usage tracking
					if sessionId != "" {
						var chunkData map[string]any
						if err := json.Unmarshal([]byte(data), &chunkData); err == nil {
							if usage, ok := chunkData["usage"].(map[string]any); ok {
								inputTokens := 0
								outputTokens := 0
								if it, ok := usage["prompt_tokens"].(float64); ok {
									inputTokens = int(it)
								} else if it, ok := usage["input_tokens"].(float64); ok {
									inputTokens = int(it)
								}
								if ot, ok := usage["completion_tokens"].(float64); ok {
									outputTokens = int(ot)
								} else if ot, ok := usage["output_tokens"].(float64); ok {
									outputTokens = int(ot)
								}
								cache.SessionUsage.Put(sessionId, cache.Usage{
									InputTokens:  inputTokens,
									OutputTokens: outputTokens,
								})
							}
						}
					}

					// Forward the event
					converted := transformer.ProcessStreamChunk([]byte(data), cfg.providerName)
					fmt.Fprintf(c.Writer, "data: %s\n\n", converted)
					flusher.Flush()
				} else {
					fmt.Fprintf(c.Writer, "%s\n", line)
					flusher.Flush()
				}
			}
		} else {
			respBody, _ := io.ReadAll(resp.Body)
			log.Printf("Fallback: respBody length=%d", len(respBody))
			if len(respBody) > 0 {
				log.Printf("Fallback: respBody=%s", string(respBody[:min(500, len(respBody))]))
			}

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
						plugin.RecordTokenSpeed(cfg.providerName, cfg.model, inputTokens, outputTokens, duration)
						if sessionId != "" {
							cache.SessionUsage.Put(sessionId, cache.Usage{
								InputTokens:  inputTokens,
								OutputTokens: outputTokens,
							})
						}
					}
				}
			}

			if resp.StatusCode == 200 {
				var respData map[string]any
				if err := json.Unmarshal(respBody, &respData); err == nil {
					if choices, ok := respData["choices"].([]any); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]any); ok {
							if message, ok := choice["message"].(map[string]any); ok {
								if toolCalls, ok := message["tool_calls"].([]any); ok && len(toolCalls) > 0 {
									for _, tc := range toolCalls {
										if tcMap, ok := tc.(map[string]any); ok {
											if function, ok := tcMap["function"].(map[string]any); ok {
												toolName, _ := function["name"].(string)
												if toolName != "" {
													var input map[string]any
													if args, ok := function["arguments"].(string); ok {
														json.Unmarshal([]byte(args), &input)
													}
													result, err := agent.HandleAgentToolCallV2(toolName, input, bodyCopy, s.cfg, sessionId)
													if err == nil && result != "" {
														message["content"] = result
														delete(message, "tool_calls")
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
			transformedResp := s.transformer.TransformResponse(respBody, cfg.providerName, cfg.transforms)
			log.Printf("Fallback: transformedResp length=%d", len(transformedResp))
			c.Data(resp.StatusCode, "application/json", transformedResp)
		}
		return
	}

	log.Printf("Fallback: All models failed, returning last error")
	if lastErr != nil {
		c.JSON(502, gin.H{"error": "All fallback models failed: " + lastErr.Error()})
	} else {
		c.JSON(502, gin.H{"error": "All fallback models failed"})
	}
}

func (s *Server) handleV1ChatCompletions(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	model, _ := body["model"].(string)
	sessionId := ""

	providers := s.cfg.GetProviders()
	routerResult := s.router.Route(toRequestBody(body), sessionId)
	model = routerResult.Model

	var modelList []string
	modelList = append(modelList, model)
	if fallbackCfg := s.cfg.GetFallback(); fallbackCfg != nil {
		if fallbackModels, ok := fallbackCfg[model]; ok {
			modelList = append(modelList, fallbackModels...)
		}
	}

	log.Printf("Fallback: Starting chat completions request with model list: %v", modelList)

	var lastErr error

	for idx, tryModel := range modelList {
		log.Printf("Fallback: Trying chat completions model %d/%d: %s", idx+1, len(modelList), tryModel)

		providerName := ""
		providerHost := ""
		providerAPIKey := ""
		transforms := []string{}

		for _, p := range providers {
			for _, m := range p.Models {
				if m == tryModel || tryModel == p.Name+","+m {
					providerName = p.Name
					providerHost = p.Host
					providerAPIKey = p.APIKey
					transforms = s.cfg.GetProviderTransform(p.Name, tryModel)
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
			transforms = s.cfg.GetProviderTransform(providerName, tryModel)
		}

		if providerHost == "" {
			providerHost = s.providerService.GetDefaultHost(providerName)
		}

		stream := false
		if bstream, ok := body["stream"].(bool); ok {
			stream = bstream
		}

		bodyCopy := make(map[string]any)
		for k, v := range body {
			bodyCopy[k] = v
		}
		actualModel := s.getActualModel(tryModel)
		bodyCopy["model"] = actualModel

		transformedBody := s.transformer.TransformRequest(bodyCopy, providerName, transforms)
		reqBody, _ := json.Marshal(transformedBody)

		url := providerHost
		if !strings.HasSuffix(url, "/v1/chat/completions") {
			url = strings.TrimRight(url, "/")
			url += "/v1/chat/completions"
		}

		req, err := http.NewRequest("POST", url, strings.NewReader(string(reqBody)))
		if err != nil {
			log.Printf("Fallback: Failed to create chat completions request for model %s: %v", tryModel, err)
			lastErr = err
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if providerAPIKey != "" {
			req.Header.Set("Authorization", "Bearer "+providerAPIKey)
		}
		if stream {
			req.Header.Set("Accept", "text/event-stream")
		}

		resp, err := s.providerService.GetHTTPClient().Transport.RoundTrip(req)
		if err != nil {
			log.Printf("Fallback: Chat completions request failed for model %s: %v", tryModel, err)
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("Fallback: Chat completions request failed for model %s with status %d", tryModel, resp.StatusCode)
			lastErr = fmt.Errorf("HTTP error %d", resp.StatusCode)
			continue
		}

		log.Printf("Fallback: Success using chat completions model %s", tryModel)

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
		return
	}

	log.Printf("Fallback: All chat completions models failed, returning last error")
	if lastErr != nil {
		c.JSON(502, gin.H{"error": "All fallback models failed: " + lastErr.Error()})
	} else {
		c.JSON(502, gin.H{"error": "All fallback models failed"})
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
