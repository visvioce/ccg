package transformer

import (
	"encoding/json"
	"strings"
)

// ParseToolArguments parses tool call arguments with fallback strategies
// First tries standard JSON, then lenient parsing, finally returns safe fallback
func ParseToolArguments(argsString string) string {
	// Handle empty or null input
	if argsString == "" || strings.TrimSpace(argsString) == "" || argsString == "{}" {
		return "{}"
	}

	// First attempt: Standard JSON parsing
	var result map[string]any
	if err := json.Unmarshal([]byte(argsString), &result); err == nil {
		return argsString
	}

	// Second attempt: Lenient JSON parsing (handle common issues)
	repaired := repairJSON(argsString)
	if repaired != "" {
		return repaired
	}

	// All parsing attempts failed - return safe fallback
	return "{}"
}

// repairJSON attempts to repair common JSON issues
func repairJSON(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// Ensure it starts with { and ends with }
	if !strings.HasPrefix(input, "{") {
		input = "{" + input
	}
	if !strings.HasSuffix(input, "}") {
		input = input + "}"
	}

	// Try to parse after basic fixes
	var result map[string]any
	if err := json.Unmarshal([]byte(input), &result); err == nil {
		bytes, _ := json.Marshal(result)
		return string(bytes)
	}

	// Handle trailing commas
	input = strings.ReplaceAll(input, ",}", "}")
	input = strings.ReplaceAll(input, ",]", "]")

	if err := json.Unmarshal([]byte(input), &result); err == nil {
		bytes, _ := json.Marshal(result)
		return string(bytes)
	}

	// Handle single quotes instead of double quotes
	input = strings.ReplaceAll(input, "'", "\"")

	if err := json.Unmarshal([]byte(input), &result); err == nil {
		bytes, _ := json.Marshal(result)
		return string(bytes)
	}

	// Handle unquoted keys
	input = fixUnquotedKeys(input)

	if err := json.Unmarshal([]byte(input), &result); err == nil {
		bytes, _ := json.Marshal(result)
		return string(bytes)
	}

	return ""
}

// fixUnquotedKeys attempts to quote unquoted object keys
func fixUnquotedKeys(input string) string {
	// Simple pattern: look for {key: or ,key: patterns
	result := input

	// Common patterns to fix
	patterns := []struct {
		find    string
		replace string
	}{
		{"{ ", "{"},
		{" }", "}"},
		{"[ ", "["},
		{" ]", "]"},
		{": ", ":"},
		{" :", ":"},
		{", ", ","},
		{" ,", ","},
	}

	for _, p := range patterns {
		result = strings.ReplaceAll(result, p.find, p.replace)
	}

	return result
}

// ParseToolArgumentsSafe safely parses tool arguments with a default fallback
func ParseToolArgumentsSafe(argsString string, defaultValue string) string {
	if defaultValue == "" {
		defaultValue = "{}"
	}
	result := ParseToolArguments(argsString)
	if result == "{}" && argsString != "" && argsString != "{}" {
		return defaultValue
	}
	return result
}

