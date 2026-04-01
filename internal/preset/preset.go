package preset

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/musistudio/ccg/internal/config"
)

type Preset struct {
	Name        string               `json:"name"`
	Version     string               `json:"version"`
	Description string               `json:"description,omitempty"`
	Author      string               `json:"author,omitempty"`
	Keywords    []string             `json:"keywords,omitempty"`
	Providers   []config.Provider    `json:"providers,omitempty"`
	Router      *config.RouterConfig `json:"router,omitempty"`
	Schema      []InputSchema        `json:"schema,omitempty"`
	Required    []string             `json:"required,omitempty"`
}

type InputSchema struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Label   string `json:"label"`
	Prompt  string `json:"prompt,omitempty"`
	Default string `json:"default,omitempty"`
}

type PresetManager struct {
	presetsDir string
}

func NewPresetManager() *PresetManager {
	return &PresetManager{
		presetsDir: filepath.Join(config.GetConfigDir(), "presets"),
	}
}

func (m *PresetManager) EnsurePresetsDir() error {
	if _, err := os.Stat(m.presetsDir); os.IsNotExist(err) {
		return os.MkdirAll(m.presetsDir, 0755)
	}
	return nil
}

func (m *PresetManager) ListPresets() ([]Preset, error) {
	if err := m.EnsurePresetsDir(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(m.presetsDir)
	if err != nil {
		return nil, err
	}

	var presets []Preset
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(m.presetsDir, entry.Name(), "manifest.json")
		preset, err := m.LoadPresetFromFile(manifestPath)
		if err != nil {
			continue
		}
		presets = append(presets, preset)
	}

	return presets, nil
}

func (m *PresetManager) LoadPresetFromFile(path string) (Preset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Preset{}, err
	}

	var preset Preset
	if err := json.Unmarshal(data, &preset); err != nil {
		return Preset{}, err
	}

	return preset, nil
}

func (m *PresetManager) GetPreset(name string) (*Preset, error) {
	manifestPath := filepath.Join(m.presetsDir, name, "manifest.json")
	preset, err := m.LoadPresetFromFile(manifestPath)
	if err != nil {
		return nil, err
	}
	return &preset, nil
}

func (m *PresetManager) InstallPreset(source string, name string) error {
	if err := m.EnsurePresetsDir(); err != nil {
		return err
	}

	var presetData []byte
	var loadErr error

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		resp, err := http.Get(source)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to download preset: %s", resp.Status)
		}
		presetData, loadErr = io.ReadAll(resp.Body)
	} else if _, err := os.Stat(source); err == nil {
		presetData, loadErr = os.ReadFile(source)
	} else {
		return fmt.Errorf("invalid preset source: %s", source)
	}

	if loadErr != nil {
		return loadErr
	}

	var preset Preset
	if err := json.Unmarshal(presetData, &preset); err != nil {
		return fmt.Errorf("invalid preset format: %w", err)
	}

	presetName := name
	if presetName == "" {
		presetName = preset.Name
	}

	presetDir := filepath.Join(m.presetsDir, presetName)
	if err := os.MkdirAll(presetDir, 0755); err != nil {
		return err
	}

	manifestPath := filepath.Join(presetDir, "manifest.json")
	if err := os.WriteFile(manifestPath, presetData, 0644); err != nil {
		return err
	}

	return nil
}

func (m *PresetManager) ApplyPreset(name string, secrets map[string]string) error {
	preset, err := m.GetPreset(name)
	if err != nil {
		return err
	}

	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	if err := cfg.Load(configPath); err != nil {
		cfg = config.New()
	}

	for i := range preset.Providers {
		for key, value := range secrets {
			preset.Providers[i].APIKey = strings.ReplaceAll(preset.Providers[i].APIKey, "{{"+key+"}}", value)
		}
	}

	cfgData := map[string]any{
		"Providers": preset.Providers,
		"Router":    preset.Router,
	}

	data, _ := json.MarshalIndent(cfgData, "", "  ")
	return os.WriteFile(configPath, data, 0644)
}

func (m *PresetManager) DeletePreset(name string) error {
	presetDir := filepath.Join(m.presetsDir, name)
	if _, err := os.Stat(presetDir); os.IsNotExist(err) {
		return fmt.Errorf("preset not found: %s", name)
	}
	return os.RemoveAll(presetDir)
}

func (m *PresetManager) ExportPreset(name string, outputPath string) error {
	preset, err := m.GetPreset(name)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, data, 0644)
}
