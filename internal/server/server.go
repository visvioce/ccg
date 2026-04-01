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
	engine.Use(middleware.CORSMiddleware())
	engine.Use(middleware.ModelParseMiddleware())

	// Routes
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
		api.POST("/providers", s.handleCreateProvider)
		api.GET("/providers/:name", s.handleGetProvider)
		api.PUT("/providers/:name", s.handleUpdateProvider)
		api.DELETE("/providers/:name", s.handleDeleteProvider)

		// Plugin endpoints
		api.GET("/plugins", s.handleGetPlugins)
		api.POST("/plugins/:name/enable", s.handleEnablePlugin)
		api.POST("/plugins/:name/disable", s.handleDisablePlugin)

		// Preset endpoints
		api.GET("/presets", s.handleGetPresets)
		api.GET("/presets/:name", s.handleGetPreset)
		api.POST("/presets", s.handleCreatePreset)
		api.POST("/presets/install", s.handleInstallPreset)
		api.POST("/presets/install/github", s.handleInstallGitHubPreset)
		api.POST("/presets/apply", s.handleApplyPreset)
		api.DELETE("/presets/:name", s.handleDeletePreset)
		api.GET("/presets/:name/export", s.handleExportPreset)

		// Token speed endpoint
		api.GET("/token-speed", s.handleGetTokenSpeed)
	}

	// Preset namespace routing - must be after /api routes
	engine.GET("/preset/:presetName/v1/messages", s.handlePresetMessages)

	// OpenAI compatible endpoints
	engine.POST("/v1/chat/completions", s.handleChatCompletions)
	engine.GET("/v1/models", s.handleGetModels)

	// Transformer endpoints
	engine.GET("/api/transformers/:name", s.handleGetTransformer)

	// Agent endpoints
	engine.POST("/api/agents/:name/execute", s.handleExecuteAgent)

	// Custom router endpoints
	engine.Any("/router/*path", s.handleRouterRequest)

	// Static files for UI
	engine.Static("/ui", "./web/dist")
	engine.StaticFile("/ui/index.html", "./web/dist/index.html")

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

// ... rest of the server.go file would continue here
// For brevity, I'm including just the key changes
