package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	Default              string            `json:"default"`
	Background           string            `json:"background"`
	Think                string            `json:"think"`
	LongContext          string            `json:"longContext"`
	LongContextThreshold int               `json:"longContextThreshold"`
	WebSearch            string            `json:"webSearch"`
	Image                string            `json:"image"`
	Scenarios            map[string]string `json:"scenarios"`
}

type AppConfig struct {
	Providers    []Provider    `json:"providers"`
	ProvidersAlt []Provider    `json:"Providers"` // CCR兼容
	Router       *RouterConfig `json:"Router"`
}

func New() *Config {
	return &Config{
		data: map[string]any{
			"PORT": "3456",
		},
	}
}

func (c *Config) Load(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.configPath = path

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Merge with existing data (preserve defaults like PORT)
	newData := make(map[string]any)
	if err := json.Unmarshal(data, &newData); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	// Merge new data into existing data
	for k, v := range newData {
		c.data[k] = v
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// 兼容CCR格式: 如果providers为空，使用Providers
	if len(cfg.Providers) == 0 && len(cfg.ProvidersAlt) > 0 {
		cfg.Providers = cfg.ProvidersAlt
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
	return filepath.Join(home, ".ccg")
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

// GetPIDFile returns the PID file path
func GetPIDFile() string {
	return filepath.Join(GetConfigDir(), ".claude-code-router.pid")
}

func EnsureConfigDir() error {
	dir := GetConfigDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}
