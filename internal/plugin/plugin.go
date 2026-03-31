package plugin

import (
	"fmt"
	"sync"
	"time"
)

type Plugin interface {
	Name() string
	Register(app interface{}) error
	Enabled() bool
}

type PluginMetadata struct {
	Name     string         `json:"name"`
	Enabled  bool           `json:"enabled"`
	Options  map[string]any `json:"options,omitempty"`
	LoadedAt time.Time     `json:"loaded_at"`
}

type CCGPlugin struct {
	Metadata PluginMetadata
	Instance Plugin
}

type PluginManager struct {
	mu            sync.RWMutex
	plugins       map[string]*CCGPlugin
	requestCounts map[string]int
	statsMu       sync.RWMutex
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins:       make(map[string]*CCGPlugin),
		requestCounts: make(map[string]int),
	}
}

func (m *PluginManager) RegisterPlugin(plugin Plugin, options map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metadata := PluginMetadata{
		Name:     plugin.Name(),
		Enabled:  true,
		Options:  options,
		LoadedAt: time.Now(),
	}

	m.plugins[plugin.Name()] = &CCGPlugin{
		Metadata: metadata,
		Instance: plugin,
	}
}

func (m *PluginManager) GetPlugin(name string) *CCGPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plugins[name]
}

func (m *PluginManager) GetAllPlugins() []*CCGPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*CCGPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

func (m *PluginManager) EnablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.plugins[name]; ok {
		p.Metadata.Enabled = true
		return nil
	}
	return fmt.Errorf("plugin not found: %s", name)
}

func (m *PluginManager) DisablePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.plugins[name]; ok {
		p.Metadata.Enabled = false
		return nil
	}
	return fmt.Errorf("plugin not found: %s", name)
}

func (m *PluginManager) RemovePlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.plugins[name]; ok {
		delete(m.plugins, name)
		return nil
	}
	return fmt.Errorf("plugin not found: %s", name)
}

func (m *PluginManager) IncrementRequestCount(pluginName string) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	m.requestCounts[pluginName]++
}

func (m *PluginManager) GetStats() map[string]any {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	stats := make(map[string]any)
	for name, count := range m.requestCounts {
		stats[name] = map[string]any{
			"request_count": count,
		}
	}
	return stats
}

type TokenSpeedPlugin struct{}

func NewTokenSpeedPlugin() *TokenSpeedPlugin {
	return &TokenSpeedPlugin{}
}

func (p *TokenSpeedPlugin) Name() string {
	return "token-speed"
}

func (p *TokenSpeedPlugin) Register(app interface{}) error {
	return nil
}

func (p *TokenSpeedPlugin) Enabled() bool {
	return true
}

func CalculateTokenSpeed(inputTokens, outputTokens int, duration time.Duration) float64 {
	if duration.Seconds() == 0 {
		return 0
	}
	return float64(outputTokens) / duration.Seconds()
}

type TokenSpeedStats struct {
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	DurationMs    int64     `json:"duration_ms"`
	TokenSpeed    float64   `json:"token_speed"`
	Timestamp     time.Time `json:"timestamp"`
}

var (
	tokenSpeedStats    = make([]TokenSpeedStats, 0)
	tokenSpeedStatsMu  sync.RWMutex
	maxStatsRecords    = 1000
)

func RecordTokenSpeed(provider, model string, inputTokens, outputTokens int, duration time.Duration) {
	tokenSpeedStatsMu.Lock()
	defer tokenSpeedStatsMu.Unlock()

	speed := CalculateTokenSpeed(inputTokens, outputTokens, duration)

	stats := TokenSpeedStats{
		Provider:     provider,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		DurationMs:   duration.Milliseconds(),
		TokenSpeed:   speed,
		Timestamp:    time.Now(),
	}

	tokenSpeedStats = append(tokenSpeedStats, stats)

	if len(tokenSpeedStats) > maxStatsRecords {
		tokenSpeedStats = tokenSpeedStats[len(tokenSpeedStats)-maxStatsRecords:]
	}
}

func GetTokenSpeedStats(provider, model string) []TokenSpeedStats {
	tokenSpeedStatsMu.RLock()
	defer tokenSpeedStatsMu.RUnlock()

	var result []TokenSpeedStats
	for _, stats := range tokenSpeedStats {
		if (provider == "" || stats.Provider == provider) && (model == "" || stats.Model == model) {
			result = append(result, stats)
		}
	}
	return result
}

func GetGlobalTokenSpeedStats() map[string]any {
	tokenSpeedStatsMu.RLock()
	defer tokenSpeedStatsMu.RUnlock()

	if len(tokenSpeedStats) == 0 {
		return map[string]any{
			"total_requests": 0,
			"avg_token_speed": 0,
		}
	}

	var totalInput, totalOutput int
	var totalSpeed float64

	for _, stats := range tokenSpeedStats {
		totalInput += stats.InputTokens
		totalOutput += stats.OutputTokens
		totalSpeed += stats.TokenSpeed
	}

	avgSpeed := totalSpeed / float64(len(tokenSpeedStats))

	return map[string]any{
		"total_requests":     len(tokenSpeedStats),
		"total_input_tokens":  totalInput,
		"total_output_tokens": totalOutput,
		"avg_token_speed":    avgSpeed,
	}
}

func ClearTokenStats() {
	tokenSpeedStatsMu.Lock()
	defer tokenSpeedStatsMu.Unlock()
	tokenSpeedStats = make([]TokenSpeedStats, 0)
}

var GlobalPluginManager = NewPluginManager()

func init() {
	GlobalPluginManager.RegisterPlugin(NewTokenSpeedPlugin(), nil)
}

func GetPluginStats() map[string]any {
	return GlobalPluginManager.GetStats()
}
