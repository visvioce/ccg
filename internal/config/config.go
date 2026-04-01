package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Config struct {
	mu         sync.RWMutex
	providers  []Provider
	router     *RouterConfig
	data       map[string]any
	configPath string
	onChange   func() // Callback when config changes externally
}

// SetOnChange sets a callback to be called when config changes
func (c *Config) SetOnChange(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onChange = fn
}

// Reload reloads the config from disk
func (c *Config) Reload() error {
	return c.Load(c.configPath)
}

type Provider struct {
	Name          string         `json:"name"`
	Host          string         `json:"api_base_url"`
	APIKey        string         `json:"api_key"`
	Models        []string       `json:"models"`
	Transform     map[string]any `json:"transformer"`
	TransformList []string       `json:"transform"`
}

type RouterConfig struct {
	Default              string              `json:"default"`
	Background           string              `json:"background"`
	Think                string              `json:"think"`
	LongContext          string              `json:"longContext"`
	LongContextThreshold int                 `json:"longContextThreshold"`
	WebSearch            string              `json:"webSearch"`
	Image                string              `json:"image"`
	Scenarios            map[string]string   `json:"scenarios"`
	Fallback             map[string][]string `json:"Fallback"`
}

type AppConfig struct {
	Providers    []Provider          `json:"providers"`
	ProvidersAlt []Provider          `json:"Providers"` // CCR兼容
	Fallback     map[string][]string `json:"fallback"`
	FallbackAlt  map[string][]string `json:"Fallback"` // CCR兼容
	Router       *RouterConfig       `json:"Router"`
}

func New() *Config {
	return &Config{
		data: map[string]any{
			"PORT": "3456",
		},
	}
}

// stripJSON5 preprocesses JSON5 content to make it valid JSON
func stripJSON5(data []byte) []byte {
	s := string(data)

	// Remove single-line comments (// ...)
	s = regexp.MustCompile(`(?m)//[^\n]*`).ReplaceAllString(s, "")

	// Remove multi-line comments (/* ... */)
	s = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(s, "")

	// Remove trailing commas before } or ]
	s = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(s, "$1")

	return []byte(s)
}

func (c *Config) Load(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.configPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Preprocess JSON5 (strip comments and trailing commas)
	processedData := stripJSON5(data)

	// Merge with existing data (preserve defaults like PORT)
	newData := make(map[string]any)
	if err := json.Unmarshal(processedData, &newData); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	// Apply environment variable interpolation
	newData = interpolateEnvVars(newData).(map[string]any)
	// Merge new data into existing data
	for k, v := range newData {
		c.data[k] = v
	}

	var cfg AppConfig
	if err := json.Unmarshal(processedData, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// 兼容CCR格式: 如果providers为空，使用Providers
	if len(cfg.Providers) == 0 && len(cfg.ProvidersAlt) > 0 {
		cfg.Providers = cfg.ProvidersAlt
	}

	// 兼容CCR格式: 如果fallback为空，使用Fallback
	if cfg.Fallback == nil && cfg.FallbackAlt != nil {
		cfg.Fallback = cfg.FallbackAlt
	}

	// 将 fallback 配置同步到 Router 中，保持一致
	if cfg.Router != nil && cfg.Fallback != nil {
		cfg.Router.Fallback = cfg.Fallback
	}

	for i := range cfg.Providers {
		provider := &cfg.Providers[i]
		if provider.Host == "" {
			provider.Host = provider.APIKey
		}
		if v, ok := c.data[provider.Name]; ok {
			if pm, ok := v.(map[string]any); ok {
				if url, ok := pm["api_base_url"].(string); ok && url != "" {
					provider.Host = url
				}
				if key, ok := pm["api_key"].(string); ok && key != "" {
					provider.APIKey = key
				}
			}
		}
	}

	c.providers = cfg.Providers
	c.router = cfg.Router

	return nil
}

// interpolateEnvVars replaces $VAR and ${VAR} patterns with environment variable values
func interpolateEnvVars(obj any) any {
	switch v := obj.(type) {
	case string:
		return interpolateString(v)
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			result[key] = interpolateEnvVars(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = interpolateEnvVars(val)
		}
		return result
	default:
		return obj
	}
}

// interpolateString replaces $VAR_NAME and ${VAR_NAME} with env var values
func interpolateString(s string) string {
	return os.ExpandEnv(s)
}

func (c *Config) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if v, ok := c.data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		// 兼容数字类型（如PORT: 3456）
		if n, ok := v.(float64); ok {
			return fmt.Sprintf("%.0f", n)
		}
		if n, ok := v.(int); ok {
			return fmt.Sprintf("%d", n)
		}
	}
	return ""
}

func (c *Config) GetInt(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if v, ok := c.data[key]; ok {
		if n, ok := v.(int); ok {
			return n
		}
	}
	return 0
}

func (c *Config) GetProviders() []Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.providers
}

func (c *Config) GetRouter() *RouterConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.router
}

func (c *Config) GetFallback() map[string][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.router != nil {
		return c.router.Fallback
	}
	return nil
}

func (c *Config) GetProvider(name string) *Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lowercaseName := strings.ToLower(name)
	for i := range c.providers {
		if strings.ToLower(c.providers[i].Name) == lowercaseName {
			return &c.providers[i]
		}
	}
	return nil
}

// IsNonInteractiveMode returns true if non-interactive mode is enabled
func (c *Config) IsNonInteractiveMode() bool {
	val := c.Get("NON_INTERACTIVE_MODE")
	return val == "true" || val == "1"
}

func (c *Config) GetProviderTransform(providerName, model string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, p := range c.providers {
		if strings.ToLower(p.Name) == strings.ToLower(providerName) {
			if p.Transform != nil {
				if modelTransform, ok := p.Transform[model].(string); ok {
					return []string{modelTransform}
				}
				if use, ok := p.Transform["use"].([]any); ok {
					var transforms []string
					for _, t := range use {
						if tstr, ok := t.(string); ok {
							transforms = append(transforms, tstr)
						} else if tarr, ok := t.([]any); ok {
							for _, titem := range tarr {
								if tstr, ok := titem.(string); ok {
									transforms = append(transforms, tstr)
								}
							}
						}
					}
					return transforms
				}
			}
			return p.TransformList
		}
	}
	return nil
}

func GetConfigDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return filepath.Join(home, ".claude-code-router")
}

func GetDefaultConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

// GetPluginsDir returns the plugins directory
func GetPluginsDir() string {
	return filepath.Join(GetConfigDir(), "plugins")
}

// GetPresetsDir returns the presets directory
func GetPresetsDir() string {
	return filepath.Join(GetConfigDir(), "presets")
}

// PluginConfig represents a plugin configuration entry
type PluginConfig struct {
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Options map[string]any `json:"options,omitempty"`
}

// GetPluginsFromConfig returns plugins configured in the config file
func (c *Config) GetPluginsFromConfig() []PluginConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var plugins []PluginConfig

	// Check "plugins" key
	if p, ok := c.data["plugins"].([]any); ok {
		for _, item := range p {
			if pMap, ok := item.(map[string]any); ok {
				pc := PluginConfig{}
				if name, ok := pMap["name"].(string); ok {
					pc.Name = name
				}
				if enabled, ok := pMap["enabled"].(bool); ok {
					pc.Enabled = enabled
				}
				if opts, ok := pMap["options"].(map[string]any); ok {
					pc.Options = opts
				}
				plugins = append(plugins, pc)
			}
		}
	}

	// Also check "Plugins" key
	if p, ok := c.data["Plugins"].([]any); ok {
		for _, item := range p {
			if pMap, ok := item.(map[string]any); ok {
				pc := PluginConfig{}
				if name, ok := pMap["name"].(string); ok {
					pc.Name = name
				}
				if enabled, ok := pMap["enabled"].(bool); ok {
					pc.Enabled = enabled
				}
				if opts, ok := pMap["options"].(map[string]any); ok {
					pc.Options = opts
				}
				plugins = append(plugins, pc)
			}
		}
	}

	return plugins
}

// GetRaw returns the raw config value for a key
func (c *Config) GetRaw(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// GetPIDFile returns the PID file path
func GetPIDFile() string {
	return filepath.Join(GetConfigDir(), ".claude-code-router.pid")
}

// Validate checks the config for required fields
func (c *Config) Validate() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var errors []string

	providers := c.providers
	if len(providers) > 0 {
		// When providers are configured, HOST and APIKEY should be set
		host, _ := c.data["HOST"].(string)
		apiKey, _ := c.data["APIKEY"].(string)
		if host == "" {
			errors = append(errors, "HOST is required when providers are configured")
		}
		if apiKey == "" {
			errors = append(errors, "APIKEY is required when providers are configured")
		}
	}

	for i, p := range providers {
		if p.Name == "" {
			errors = append(errors, fmt.Sprintf("Provider %d missing name", i))
		}
		if len(p.Models) == 0 {
			errors = append(errors, fmt.Sprintf("Provider '%s' has no models", p.Name))
		}
	}

	return errors
}

func EnsureConfigDir() error {
	dir := GetConfigDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

// BackupConfigFile creates a timestamped backup of the config file, keeping only the 3 most recent backups
func BackupConfigFile() (string, error) {
	configPath := GetDefaultConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil // No config file to backup
	}

	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupPath := configPath + "." + timestamp + ".bak"

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %w", err)
	}

	// Clean up old backups, keeping only the 3 most recent
	cleanupOldBackups(configPath)

	return backupPath, nil
}

// cleanupOldBackups removes old backup files, keeping only the 3 most recent
func cleanupOldBackups(configPath string) {
	configDir := filepath.Dir(configPath)
	configFileName := filepath.Base(configPath)

	entries, err := os.ReadDir(configDir)
	if err != nil {
		return
	}

	var backupFiles []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), configFileName) && strings.HasSuffix(entry.Name(), ".bak") {
			backupFiles = append(backupFiles, entry.Name())
		}
	}

	// Sort descending (newest first)
	sort.Sort(sort.Reverse(sort.StringSlice(backupFiles)))

	// Delete all but the 3 most recent
	for i := 3; i < len(backupFiles); i++ {
		os.Remove(filepath.Join(configDir, backupFiles[i]))
	}
}
