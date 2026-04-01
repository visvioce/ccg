package preset

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/musistudio/ccg/internal/config"
)

type Preset struct {
	Name           string               `json:"name"`
	Version        string               `json:"version"`
	Description    string               `json:"description,omitempty"`
	Author         string               `json:"author,omitempty"`
	Homepage       string               `json:"homepage,omitempty"`
	Repository     string               `json:"repository,omitempty"`
	License        string               `json:"license,omitempty"`
	Keywords       []string             `json:"keywords,omitempty"`
	CCRVersion     string               `json:"ccrVersion,omitempty"`
	Source         string               `json:"source,omitempty"`
	SourceType     string               `json:"sourceType,omitempty"`
	Checksum       string               `json:"checksum,omitempty"`
	Providers      []config.Provider    `json:"providers,omitempty"`
	Router         *config.RouterConfig `json:"router,omitempty"`
	Schema         []InputSchema        `json:"schema,omitempty"`
	Required       []string             `json:"required,omitempty"`
	Template       map[string]any       `json:"template,omitempty"`
	ConfigMappings []ConfigMapping      `json:"configMappings,omitempty"`
	UserValues     map[string]string    `json:"userValues,omitempty"`
}

type ConfigMapping struct {
	Target string           `json:"target"`
	Value  any              `json:"value"`
	When   []map[string]any `json:"when,omitempty"`
}

// InputType represents the type of input field
type InputType string

const (
	InputTypePassword    InputType = "password"
	InputTypeInput       InputType = "input"
	InputTypeSelect      InputType = "select"
	InputTypeMultiselect InputType = "multiselect"
	InputTypeConfirm     InputType = "confirm"
	InputTypeEditor      InputType = "editor"
	InputTypeNumber      InputType = "number"
)

// InputOption represents a selectable option
type InputOption struct {
	Label       string `json:"label"`
	Value       any    `json:"value"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// DynamicOptions represents a dynamic option source
type DynamicOptions struct {
	Type          string        `json:"type"` // static, providers, models, custom
	Options       []InputOption `json:"options,omitempty"`
	ProviderField string        `json:"providerField,omitempty"`
	Source        string        `json:"source,omitempty"`
}

// Condition represents a conditional expression
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator,omitempty"` // eq, ne, in, nin, gt, lt, gte, lte, exists
	Value    any    `json:"value,omitempty"`
}

// InputSchema represents a required input field (matches CCR's RequiredInput)
type InputSchema struct {
	ID           string    `json:"id"`
	Type         InputType `json:"type,omitempty"`
	Label        string    `json:"label,omitempty"`
	Prompt       string    `json:"prompt,omitempty"`
	Placeholder  string    `json:"placeholder,omitempty"`
	Options      any       `json:"options,omitempty"` // []InputOption or DynamicOptions
	When         any       `json:"when,omitempty"`    // Condition or []Condition
	DefaultValue any       `json:"defaultValue,omitempty"`
	Required     *bool     `json:"required,omitempty"`
	Validator    any       `json:"validator,omitempty"` // string (regex) or ignored in Go
	Min          *float64  `json:"min,omitempty"`
	Max          *float64  `json:"max,omitempty"`
	Rows         *int      `json:"rows,omitempty"`
	DependsOn    []string  `json:"dependsOn,omitempty"`
}

// evaluateCondition evaluates a single condition against values
func evaluateCondition(cond Condition, values map[string]any) bool {
	actualValue := values[cond.Field]

	switch cond.Operator {
	case "", "eq":
		return fmt.Sprintf("%v", actualValue) == fmt.Sprintf("%v", cond.Value)
	case "ne":
		return fmt.Sprintf("%v", actualValue) != fmt.Sprintf("%v", cond.Value)
	case "exists":
		return actualValue != nil
	case "in":
		if arr, ok := cond.Value.([]any); ok {
			for _, v := range arr {
				if fmt.Sprintf("%v", actualValue) == fmt.Sprintf("%v", v) {
					return true
				}
			}
		}
		return false
	case "nin":
		if arr, ok := cond.Value.([]any); ok {
			for _, v := range arr {
				if fmt.Sprintf("%v", actualValue) == fmt.Sprintf("%v", v) {
					return false
				}
			}
		}
		return true
	case "gt":
		return toFloat(actualValue) > toFloat(cond.Value)
	case "lt":
		return toFloat(actualValue) < toFloat(cond.Value)
	case "gte":
		return toFloat(actualValue) >= toFloat(cond.Value)
	case "lte":
		return toFloat(actualValue) <= toFloat(cond.Value)
	default:
		return fmt.Sprintf("%v", actualValue) == fmt.Sprintf("%v", cond.Value)
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

// evaluateConditions evaluates conditions (supports single or array with AND logic)
func evaluateConditions(conditions any, values map[string]any) bool {
	if conditions == nil {
		return true
	}

	switch c := conditions.(type) {
	case Condition:
		return evaluateCondition(c, values)
	case []Condition:
		for _, cond := range c {
			if !evaluateCondition(cond, values) {
				return false
			}
		}
		return true
	case map[string]any:
		cond := Condition{}
		if f, ok := c["field"].(string); ok {
			cond.Field = f
		}
		if o, ok := c["operator"].(string); ok {
			cond.Operator = o
		}
		cond.Value = c["value"]
		return evaluateCondition(cond, values)
	case []any:
		for _, item := range c {
			if m, ok := item.(map[string]any); ok {
				cond := Condition{}
				if f, ok := m["field"].(string); ok {
					cond.Field = f
				}
				if o, ok := m["operator"].(string); ok {
					cond.Operator = o
				}
				cond.Value = m["value"]
				if !evaluateCondition(cond, values) {
					return false
				}
			}
		}
		return true
	default:
		return true
	}
}

// shouldShowField determines if a field should be shown based on conditions
func shouldShowField(field InputSchema, values map[string]any) bool {
	if field.When == nil {
		return true
	}
	return evaluateConditions(field.When, values)
}

// getDefaultValue returns the default value for a field
func getDefaultValue(field InputSchema) any {
	if field.DefaultValue != nil {
		return field.DefaultValue
	}
	switch field.Type {
	case InputTypeConfirm:
		return false
	case InputTypeMultiselect:
		return []any{}
	case InputTypeNumber:
		return 0
	default:
		return ""
	}
}

// validateInput validates a user input value
func validateInput(field InputSchema, value any) (bool, string) {
	// Check required
	if field.Required == nil || *field.Required {
		if value == nil || value == "" {
			return false, fmt.Sprintf("%s is required", field.LabelOrID())
		}
	}
	if value == nil && (field.Required != nil && !*field.Required) {
		return true, ""
	}
	// Type check
	switch field.Type {
	case InputTypeNumber:
		if toFloat(value) == 0 && fmt.Sprintf("%v", value) != "0" {
			return false, fmt.Sprintf("%s must be a number", field.LabelOrID())
		}
		if field.Min != nil && toFloat(value) < *field.Min {
			return false, fmt.Sprintf("%s must be at least %v", field.LabelOrID(), *field.Min)
		}
		if field.Max != nil && toFloat(value) > *field.Max {
			return false, fmt.Sprintf("%s must be at most %v", field.LabelOrID(), *field.Max)
		}
	}
	return true, ""
}

// LabelOrID returns the label or falls back to ID
func (f InputSchema) LabelOrID() string {
	if f.Label != "" {
		return f.Label
	}
	return f.ID
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

// ExportPresetWithOptions exports a preset with metadata options
func (m *PresetManager) ExportPresetWithOptions(name, outputPath, description, author, tags string, includeSensitive bool) error {
	preset, err := m.GetPreset(name)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return err
	}

	var configMap map[string]any
	if err := json.Unmarshal(data, &configMap); err == nil {
		// Add metadata
		if description != "" {
			configMap["description"] = description
		}
		if author != "" {
			configMap["author"] = author
		}
		if tags != "" {
			configMap["keywords"] = strings.Split(tags, ",")
		}

		// Sanitize unless --include-sensitive
		if !includeSensitive {
			sanitized, count := SanitizeConfig(configMap)
			if count > 0 {
				log.Printf("Sanitized %d sensitive fields", count)
			}
			configMap = sanitized
		}

		data, _ = json.MarshalIndent(configMap, "", "  ")
	}

	return os.WriteFile(outputPath, data, 0644)
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

	// Parse as generic map for sanitization
	var configMap map[string]any
	if err := json.Unmarshal(data, &configMap); err == nil {
		sanitized, count := SanitizeConfig(configMap)
		if count > 0 {
			log.Printf("Sanitized %d sensitive fields in preset export", count)
		}
		data, _ = json.MarshalIndent(sanitized, "", "  ")
	}

	return os.WriteFile(outputPath, data, 0644)
}

// LoadConfigFromManifest loads the full config from a preset's manifest
func (m *PresetManager) LoadConfigFromManifest(presetDir string) (map[string]any, error) {
	return m.loadConfigFromManifest(presetDir)
}

// replaceTemplateVariables replaces #{variable} placeholders with values
func replaceTemplateVariables(template any, values map[string]string) any {
	switch v := template.(type) {
	case string:
		result := v
		for key, val := range values {
			result = strings.ReplaceAll(result, "#{"+key+"}", val)
		}
		return result
	case map[string]any:
		result := make(map[string]any)
		for k, val := range v {
			result[k] = replaceTemplateVariables(val, values)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = replaceTemplateVariables(val, values)
		}
		return result
	default:
		return template
	}
}

// applyConfigMappings applies config mappings to set values at specific paths
func applyConfigMappings(config map[string]any, mappings []ConfigMapping, values map[string]string) map[string]any {
	result := make(map[string]any)
	for k, v := range config {
		result[k] = v
	}

	for _, mapping := range mappings {
		var value any
		if strVal, ok := mapping.Value.(string); ok && strings.HasPrefix(strVal, "#{") && strings.HasSuffix(strVal, "}") {
			varName := strVal[2 : len(strVal)-1]
			value = values[varName]
		} else {
			value = mapping.Value
		}
		setValueByPath(result, mapping.Target, value)
	}

	return result
}

// setValueByPath sets a value in a nested map by dot-separated path
func setValueByPath(obj map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := obj
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
		} else {
			if next, ok := current[part].(map[string]any); ok {
				current = next
			} else {
				next := make(map[string]any)
				current[part] = next
				current = next
			}
		}
	}
}

// InstallFromGitHub installs a preset from a GitHub repository
func (m *PresetManager) InstallFromGitHub(repoURL, name string) error {
	if err := m.EnsurePresetsDir(); err != nil {
		return err
	}

	// Parse GitHub repo URL
	// Format: owner/repo or https://github.com/owner/repo
	ownerRepo := repoURL
	if strings.Contains(repoURL, "github.com") {
		parts := strings.Split(repoURL, "github.com")
		if len(parts) > 1 {
			ownerRepo = strings.TrimPrefix(parts[1], "/")
			ownerRepo = strings.TrimSuffix(ownerRepo, ".git")
		}
	}

	// Download ZIP
	zipURL := fmt.Sprintf("https://github.com/%s/archive/refs/heads/main.zip", ownerRepo)
	resp, err := http.Get(zipURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		// Try master branch
		zipURL = fmt.Sprintf("https://github.com/%s/archive/refs/heads/master.zip", ownerRepo)
		resp, err = http.Get(zipURL)
		if err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to download: status %d", resp.StatusCode)
		}
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Save to temp file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("ccg-preset-%d.zip", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, zipData, 0644); err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// Extract ZIP
	presetDir := filepath.Join(m.presetsDir, name)
	if err := extractZip(tmpFile, presetDir); err != nil {
		return fmt.Errorf("failed to extract preset: %w", err)
	}

	return nil
}

// InstallFromZip installs a preset from a local ZIP file
func (m *PresetManager) InstallFromZip(zipPath, name string) error {
	if err := m.EnsurePresetsDir(); err != nil {
		return err
	}

	presetDir := filepath.Join(m.presetsDir, name)
	return extractZip(zipPath, presetDir)
}

// extractZip extracts a ZIP file to a target directory, handling GitHub archive structure
func extractZip(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Detect root directory
	rootDir := ""
	if len(r.File) > 0 {
		parts := strings.SplitN(r.File[0].Name, "/", 2)
		if len(parts) > 1 {
			rootDir = parts[0] + "/"
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	for _, f := range r.File {
		name := f.Name
		// Strip root directory
		if rootDir != "" && strings.HasPrefix(name, rootDir) {
			name = strings.TrimPrefix(name, rootDir)
		}
		if name == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, name)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(targetDir)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(targetPath, 0755)
			continue
		}

		// Create parent directory
		os.MkdirAll(filepath.Dir(targetPath), 0755)

		// Extract file
		rc, err := f.Open()
		if err != nil {
			return err
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}

		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

// processStatusLineConfig resolves relative scriptPath in StatusLine config
func processStatusLineConfig(statusLineConfig any, presetDir string) any {
	slMap, ok := statusLineConfig.(map[string]any)
	if !ok {
		return statusLineConfig
	}

	for key, val := range slMap {
		if theme, ok := val.(map[string]any); ok {
			if modules, ok := theme["modules"].([]any); ok {
				for i, mod := range modules {
					if mMap, ok := mod.(map[string]any); ok {
						if scriptPath, ok := mMap["scriptPath"].(string); ok && scriptPath != "" && !filepath.IsAbs(scriptPath) {
							mMap["scriptPath"] = filepath.Join(presetDir, scriptPath)
							modules[i] = mMap
						}
					}
				}
				theme["modules"] = modules
				slMap[key] = theme
			}
		}
	}

	return slMap
}

// processTransformersConfig resolves relative path in transformers config
func processTransformersConfig(transformers any, presetDir string) any {
	tArr, ok := transformers.([]any)
	if !ok {
		return transformers
	}

	for i, t := range tArr {
		if tMap, ok := t.(map[string]any); ok {
			if path, ok := tMap["path"].(string); ok && path != "" && !filepath.IsAbs(path) {
				tMap["path"] = filepath.Join(presetDir, path)
				tArr[i] = tMap
			}
		}
	}

	return tArr
}

// loadConfigFromManifest loads config from manifest, applying userValues and template
func (m *PresetManager) loadConfigFromManifest(presetDir string) (map[string]any, error) {
	manifestPath := filepath.Join(presetDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	// Extract userValues
	userValues := make(map[string]string)
	if uv, ok := manifest["userValues"].(map[string]any); ok {
		for k, v := range uv {
			if s, ok := v.(string); ok {
				userValues[k] = s
			}
		}
	}

	var config map[string]any

	// If template exists, apply it with userValues
	if template, ok := manifest["template"].(map[string]any); ok {
		result := replaceTemplateVariables(template, userValues)
		if cfg, ok := result.(map[string]any); ok {
			config = cfg
			// Apply configMappings
			if mappingsRaw, ok := manifest["configMappings"].([]any); ok {
				var mappings []ConfigMapping
				for _, m := range mappingsRaw {
					if mMap, ok := m.(map[string]any); ok {
						cm := ConfigMapping{}
						if t, ok := mMap["target"].(string); ok {
							cm.Target = t
						}
						cm.Value = mMap["value"]
						mappings = append(mappings, cm)
					}
				}
				config = applyConfigMappings(config, mappings, userValues)
			}
		}
	}

	if config == nil {
		config = make(map[string]any)
		metadataFields := map[string]bool{
			"name": true, "version": true, "description": true, "author": true,
			"homepage": true, "repository": true, "license": true,
			"keywords": true, "ccrVersion": true, "source": true, "sourceType": true, "checksum": true,
			"schema": true, "required": true,
			"template": true, "configMappings": true, "userValues": true,
		}
		for k, v := range manifest {
			if !metadataFields[k] {
				config[k] = replaceTemplateVariables(v, userValues)
			}
		}
	}

	// Process StatusLine config (resolve relative scriptPath)
	if sl, ok := config["StatusLine"]; ok {
		config["StatusLine"] = processStatusLineConfig(sl, presetDir)
	}

	// Process transformers config (resolve relative path)
	if tf, ok := config["transformers"]; ok {
		config["transformers"] = processTransformersConfig(tf, presetDir)
	}

	return config, nil
}
