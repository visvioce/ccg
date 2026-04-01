package preset

import (
	"fmt"
	"strings"
)

// MergeStrategy represents how to handle config conflicts
type MergeStrategy string

const (
	MergeStrategyAsk       MergeStrategy = "ask"
	MergeStrategyOverwrite MergeStrategy = "overwrite"
	MergeStrategyMerge     MergeStrategy = "merge"
	MergeStrategySkip      MergeStrategy = "skip"
)

// MergeCallbacks provides interactive callbacks for merge conflicts
type MergeCallbacks struct {
	OnRouterConflict      func(key string, existingValue, newValue any) (bool, error)
	OnTransformerConflict func(transformerPath string) (string, error) // "keep", "overwrite", "skip"
	OnConfigConflict      func(key string) (bool, error)
}

// MergeConfig merges a preset config into a base config using the specified strategy
func MergeConfig(baseConfig, presetConfig map[string]any, strategy MergeStrategy, callbacks *MergeCallbacks) (map[string]any, error) {
	result := make(map[string]any)
	for k, v := range baseConfig {
		result[k] = v
	}

	// Merge Providers
	if providers, ok := presetConfig["Providers"].([]any); ok && len(providers) > 0 {
		result["Providers"] = mergeProviders(result["Providers"], providers)
	}
	if providers, ok := presetConfig["providers"].([]any); ok && len(providers) > 0 {
		result["providers"] = mergeProviders(result["providers"], providers)
	}

	// Merge Router
	if router, ok := presetConfig["Router"].(map[string]any); ok {
		existingRouter := make(map[string]any)
		if er, ok := result["Router"].(map[string]any); ok {
			existingRouter = er
		}
		merged, err := mergeRouter(existingRouter, router, strategy, callbacks)
		if err != nil {
			return nil, err
		}
		result["Router"] = merged
	}

	// Merge transformers
	if transformers, ok := presetConfig["transformers"].([]any); ok && len(transformers) > 0 {
		existing := make([]any, 0)
		if et, ok := result["transformers"].([]any); ok {
			existing = et
		}
		result["transformers"] = mergeTransformers(existing, transformers, strategy, callbacks)
	}

	// Merge other top-level config
	excludeKeys := map[string]bool{
		"Providers": true, "providers": true,
		"Router": true, "transformers": true,
	}
	for key, value := range presetConfig {
		if excludeKeys[key] {
			continue
		}
		if value == nil {
			continue
		}
		existingValue, hasExisting := result[key]
		if !hasExisting || existingValue == nil {
			result[key] = value
		} else {
			if strategy == MergeStrategyOverwrite {
				result[key] = value
			} else if strategy == MergeStrategyAsk && callbacks != nil && callbacks.OnConfigConflict != nil {
				overwrite, err := callbacks.OnConfigConflict(key)
				if err != nil {
					return nil, err
				}
				if overwrite {
					result[key] = value
				}
			}
			// skip/merge: keep existing
		}
	}

	return result, nil
}

func mergeProviders(existing, incoming any) []any {
	existingSlice, _ := existing.([]any)
	result := make([]any, len(existingSlice))
	copy(result, existingSlice)

	incomingSlice, _ := incoming.([]any)
	existingNames := make(map[string]int)
	for i, p := range result {
		if pMap, ok := p.(map[string]any); ok {
			if name, ok := pMap["name"].(string); ok {
				existingNames[strings.ToLower(name)] = i
			}
		}
	}

	for _, p := range incomingSlice {
		if pMap, ok := p.(map[string]any); ok {
			if name, ok := pMap["name"].(string); ok {
				if idx, exists := existingNames[strings.ToLower(name)]; exists {
					result[idx] = p // Overwrite
				} else {
					result = append(result, p) // Add
				}
			}
		}
	}

	return result
}

func mergeRouter(existing, incoming map[string]any, strategy MergeStrategy, callbacks *MergeCallbacks) (map[string]any, error) {
	result := make(map[string]any)
	for k, v := range existing {
		result[k] = v
	}

	for key, value := range incoming {
		if value == nil {
			continue
		}
		existingValue, hasExisting := result[key]
		if !hasExisting || existingValue == nil {
			result[key] = value
		} else {
			if strategy == MergeStrategyAsk && callbacks != nil && callbacks.OnRouterConflict != nil {
				overwrite, err := callbacks.OnRouterConflict(key, existingValue, value)
				if err != nil {
					return nil, err
				}
				if overwrite {
					result[key] = value
				}
			} else if strategy == MergeStrategyOverwrite {
				result[key] = value
			}
			// skip/merge: keep existing
		}
	}

	return result, nil
}

func mergeTransformers(existing, incoming []any, strategy MergeStrategy, callbacks *MergeCallbacks) []any {
	if len(existing) == 0 {
		return incoming
	}
	if len(incoming) == 0 {
		return existing
	}

	result := make([]any, len(existing))
	copy(result, existing)

	existingPaths := make(map[string]int)
	for i, t := range result {
		if tMap, ok := t.(map[string]any); ok {
			if path, ok := tMap["path"].(string); ok && path != "" {
				existingPaths[path] = i
			}
		}
	}

	for _, t := range incoming {
		if tMap, ok := t.(map[string]any); ok {
			path, hasPath := tMap["path"].(string)
			if !hasPath || path == "" {
				result = append(result, t)
				continue
			}
			if idx, exists := existingPaths[path]; exists {
				if strategy == MergeStrategyOverwrite {
					result[idx] = t
				} else if strategy == MergeStrategyAsk && callbacks != nil && callbacks.OnTransformerConflict != nil {
					action, err := callbacks.OnTransformerConflict(path)
					if err == nil && action == "overwrite" {
						result[idx] = t
					}
				}
			} else {
				result = append(result, t)
			}
		}
	}

	return result
}

// ValidatePreset validates a preset configuration
func ValidatePreset(preset map[string]any) (errors []string, warnings []string) {
	// Check metadata
	name, _ := preset["name"].(string)
	if name == "" {
		errors = append(errors, "Missing preset name")
	}

	// Check Providers
	if providers, ok := preset["Providers"].([]any); ok {
		for i, p := range providers {
			if pMap, ok := p.(map[string]any); ok {
				if pMap["name"] == nil || pMap["name"] == "" {
					errors = append(errors, fmt.Sprintf("Provider %d missing name", i))
				}
				if pMap["api_base_url"] == nil || pMap["api_base_url"] == "" {
					errors = append(errors, fmt.Sprintf("Provider %d missing api_base_url", i))
				}
				models, _ := pMap["models"].([]any)
				if len(models) == 0 {
					warnings = append(warnings, fmt.Sprintf("Provider %d has no models", i))
				}
			}
		}
	}

	return errors, warnings
}
