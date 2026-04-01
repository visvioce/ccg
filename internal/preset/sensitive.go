package preset

import (
	"os"
	"regexp"
	"strings"
)

// Sensitive field patterns
var sensitivePatterns = []string{
	"api_key", "apikey", "apiKey", "APIKEY",
	"api_secret", "apisecret", "apiSecret",
	"secret", "SECRET",
	"token", "TOKEN", "auth_token",
	"password", "PASSWORD", "passwd",
	"private_key", "privateKey",
	"access_key", "accessKey",
}

// Environment variable placeholder regex
var envVarRegex = regexp.MustCompile(`^\$\{?[A-Z_][A-Z0-9_]*\}?$`)

// isSensitiveField checks if a field name is sensitive
func isSensitiveField(fieldName string) bool {
	lower := strings.ToLower(fieldName)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// isEnvPlaceholder checks if a value is already an environment variable placeholder
func isEnvPlaceholder(value string) bool {
	return envVarRegex.MatchString(strings.TrimSpace(value))
}

// GenerateEnvVarName generates an environment variable name from entity and field
func GenerateEnvVarName(entityName, fieldName string) string {
	prefix := strings.ToUpper(regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(entityName, "_"))
	field := strings.ToUpper(regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(fieldName, "_"))
	if prefix == field {
		return prefix
	}
	return prefix + "_" + field
}

// SanitizeConfig recursively sanitizes sensitive fields in a config object
func SanitizeConfig(config map[string]any) (map[string]any, int) {
	count := 0
	result := sanitizeObject(config, "", &count)
	if m, ok := result.(map[string]any); ok {
		return m, count
	}
	return config, 0
}

func sanitizeObject(obj any, path string, count *int) any {
	switch v := obj.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			currentPath := path
			if currentPath != "" {
				currentPath += "."
			}
			currentPath += key

			if isSensitiveField(key) {
				if strVal, ok := val.(string); ok {
					if isEnvPlaceholder(strVal) {
						result[key] = strVal
					} else {
						// Extract entity name from path
						entityName := "CONFIG"
						envVar := GenerateEnvVarName(entityName, key)
						result[key] = "${" + envVar + "}"
						*count++
					}
				} else {
					result[key] = val
				}
			} else {
				result[key] = sanitizeObject(val, currentPath, count)
			}
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = sanitizeObject(val, path+"["+string(rune('0'+i))+"]", count)
		}
		return result
	default:
		return obj
	}
}

// FillSensitiveInputs replaces environment variable placeholders with actual values
func FillSensitiveInputs(config map[string]any, inputs map[string]string) map[string]any {
	result := make(map[string]any)
	fillObject(config, "", inputs, result)
	return result
}

func fillObject(obj map[string]any, path string, inputs map[string]string, result map[string]any) {
	for key, val := range obj {
		currentPath := path
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += key

		if strVal, ok := val.(string); ok && isEnvPlaceholder(strVal) {
			if input, ok := inputs[currentPath]; ok {
				result[key] = input
			} else {
				// Try to resolve from environment
				varName := extractEnvVarName(strVal)
				if varName != "" {
					if envVal := os.Getenv(varName); envVal != "" {
						result[key] = envVal
					} else {
						result[key] = strVal
					}
				} else {
					result[key] = strVal
				}
			}
		} else if subMap, ok := val.(map[string]any); ok {
			subResult := make(map[string]any)
			fillObject(subMap, currentPath, inputs, subResult)
			result[key] = subResult
		} else {
			result[key] = val
		}
	}
}

func extractEnvVarName(value string) string {
	value = strings.TrimSpace(value)
	// Match ${VAR_NAME}
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		return value[2 : len(value)-1]
	}
	// Match $VAR_NAME
	if strings.HasPrefix(value, "$") && !strings.HasPrefix(value, "${") {
		return value[1:]
	}
	return ""
}
