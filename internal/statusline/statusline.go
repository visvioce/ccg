package statusline

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ModuleConfig represents a statusline module configuration
type ModuleConfig struct {
	Type       string         `json:"type"`
	Icon       string         `json:"icon,omitempty"`
	Text       string         `json:"text"`
	Color      string         `json:"color,omitempty"`
	Background string         `json:"background,omitempty"`
	ScriptPath string         `json:"scriptPath,omitempty"`
	Options    map[string]any `json:"options,omitempty"`
}

// ThemeConfig represents a theme configuration
type ThemeConfig struct {
	Modules []ModuleConfig `json:"modules"`
}

// Input represents the statusline input data
type Input struct {
	HookEventName  string `json:"hook_event_name"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Model          struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
	Version     string `json:"version,omitempty"`
	OutputStyle struct {
		Name string `json:"name"`
	} `json:"output_style,omitempty"`
	Cost *struct {
		TotalCostUSD     float64 `json:"total_cost_usd"`
		TotalDurationMs  float64 `json:"total_duration_ms"`
		TotalAPIDuration float64 `json:"total_api_duration_ms"`
		LinesAdded       int     `json:"total_lines_added"`
		LinesRemoved     int     `json:"total_lines_removed"`
	} `json:"cost,omitempty"`
	ContextWindow *struct {
		TotalInputTokens  int `json:"total_input_tokens"`
		TotalOutputTokens int `json:"total_output_tokens"`
		ContextWindowSize int `json:"context_window_size"`
		CurrentUsage      *struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		} `json:"current_usage"`
	} `json:"context_window,omitempty"`
}

var (
	defaultTheme = ThemeConfig{
		Modules: []ModuleConfig{
			{Type: "workDir", Icon: "󰉋", Text: "{{workDirName}}", Color: "bright_blue"},
			{Type: "gitBranch", Icon: "", Text: "{{gitBranch}}", Color: "bright_magenta"},
			{Type: "model", Icon: "󰚩", Text: "{{model}}", Color: "bright_cyan"},
			{Type: "usage", Icon: "↑", Text: "{{inputTokens}}", Color: "bright_green"},
			{Type: "usage", Icon: "↓", Text: "{{outputTokens}}", Color: "bright_yellow"},
		},
	}
	powerlineTheme = ThemeConfig{
		Modules: []ModuleConfig{
			{Type: "workDir", Icon: "󰉋", Text: "{{workDirName}}", Color: "white", Background: "bg_bright_blue"},
			{Type: "gitBranch", Icon: "", Text: "{{gitBranch}}", Color: "white", Background: "bg_bright_magenta"},
			{Type: "model", Icon: "󰚩", Text: "{{model}}", Color: "white", Background: "bg_bright_cyan"},
			{Type: "usage", Icon: "↑", Text: "{{inputTokens}}", Color: "white", Background: "bg_bright_green"},
			{Type: "usage", Icon: "↓", Text: "{{outputTokens}}", Color: "white", Background: "bg_bright_yellow"},
		},
	}
	simpleTheme = ThemeConfig{
		Modules: []ModuleConfig{
			{Type: "workDir", Icon: "", Text: "{{workDirName}}", Color: "bright_blue"},
			{Type: "gitBranch", Icon: "", Text: "{{gitBranch}}", Color: "bright_magenta"},
			{Type: "model", Icon: "", Text: "{{model}}", Color: "bright_cyan"},
			{Type: "usage", Icon: "↑", Text: "{{inputTokens}}", Color: "bright_green"},
			{Type: "usage", Icon: "↓", Text: "{{outputTokens}}", Color: "bright_yellow"},
		},
	}
	fullTheme = ThemeConfig{
		Modules: []ModuleConfig{
			{Type: "workDir", Icon: "󰉋", Text: "{{workDirName}}", Color: "bright_blue"},
			{Type: "gitBranch", Icon: "", Text: "{{gitBranch}}", Color: "bright_magenta"},
			{Type: "model", Icon: "󰚩", Text: "{{model}}", Color: "bright_cyan"},
			{Type: "context", Icon: "🪟", Text: "{{contextPercent}}% / {{contextWindowSize}}", Color: "bright_green"},
			{Type: "speed", Icon: "⚡", Text: "{{tokenSpeed}} t/s {{isStreaming}}", Color: "bright_yellow"},
			{Type: "cost", Icon: "💰", Text: "{{cost}}", Color: "bright_magenta"},
			{Type: "duration", Icon: "⏱️", Text: "{{duration}}", Color: "bright_white"},
			{Type: "lines", Icon: "📝", Text: "+{{linesAdded}}/-{{linesRemoved}}", Color: "bright_cyan"},
		},
	}

	// ANSI escape codes
	reset = "\x1b[0m"

	// 256-color map
	colorMap = map[string]int{
		"black": 0, "red": 1, "green": 2, "yellow": 3, "blue": 4,
		"magenta": 5, "cyan": 6, "white": 7,
		"bright_black": 8, "bright_red": 9, "bright_green": 10, "bright_yellow": 11,
		"bright_blue": 12, "bright_magenta": 13, "bright_cyan": 14, "bright_white": 15,
		"bg_black": 0, "bg_red": 1, "bg_green": 2, "bg_yellow": 3, "bg_blue": 4,
		"bg_magenta": 5, "bg_cyan": 6, "bg_white": 7,
		"bg_bright_black": 8, "bg_bright_red": 9, "bg_bright_green": 10, "bg_bright_yellow": 11,
		"bg_bright_blue": 12, "bg_bright_magenta": 13, "bg_bright_cyan": 14, "bg_bright_white": 15,
	}

	basicColors = [16][3]int{
		{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
		{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
)

func hexToRgb(hex string) (r, g, b int, ok bool) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
	}
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	rv, err1 := strconv.ParseInt(hex[0:2], 16, 32)
	gv, err2 := strconv.ParseInt(hex[2:4], 16, 32)
	bv, err3 := strconv.ParseInt(hex[4:6], 16, 32)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return int(rv), int(gv), int(bv), true
}

func color256ToRgb(index int) (r, g, b int) {
	if index < 0 || index > 255 {
		return 255, 255, 255
	}
	if index < 16 {
		return basicColors[index][0], basicColors[index][1], basicColors[index][2]
	}
	if index < 232 {
		i := index - 16
		rgb := [6]int{0, 95, 135, 175, 215, 255}
		return rgb[i/36], rgb[(i%36)/6], rgb[i%6]
	}
	gray := 8 + (index-232)*10
	return gray, gray, gray
}

func getTrueColorRgb(colorName string) (r, g, b int, ok bool) {
	if idx, exists := colorMap[colorName]; exists {
		r, g, b = color256ToRgb(idx)
		return r, g, b, true
	}
	if strings.HasPrefix(colorName, "bg_#") {
		return hexToRgb(colorName[3:])
	}
	if strings.HasPrefix(colorName, "#") || regexp.MustCompile(`^[0-9a-fA-F]{3,6}$`).MatchString(colorName) {
		return hexToRgb(colorName)
	}
	return 0, 0, 0, false
}

func fgTrueColor(r, g, b int) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

func bgTrueColor(r, g, b int) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r, g, b)
}

func replaceVariables(text string, variables map[string]string) string {
	re := regexp.MustCompile(`\{\{(\w+)\}\}`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		key := match[2 : len(match)-2]
		if val, ok := variables[key]; ok {
			return val
		}
		return ""
	})
}

func formatUsage(input, output int) (string, string) {
	var inStr, outStr string
	if input > 1000 {
		inStr = fmt.Sprintf("%.1fk", float64(input)/1000)
	} else {
		inStr = fmt.Sprintf("%d", input)
	}
	if output > 1000 {
		outStr = fmt.Sprintf("%.1fk", float64(output)/1000)
	} else {
		outStr = fmt.Sprintf("%d", output)
	}
	return inStr, outStr
}

func calculateContextPercent(cw *struct {
	TotalInputTokens  int `json:"total_input_tokens"`
	TotalOutputTokens int `json:"total_output_tokens"`
	ContextWindowSize int `json:"context_window_size"`
	CurrentUsage      *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"current_usage"`
}) int {
	if cw == nil || cw.CurrentUsage == nil || cw.ContextWindowSize == 0 {
		return 0
	}
	current := cw.CurrentUsage.InputTokens + cw.CurrentUsage.CacheCreationInputTokens + cw.CurrentUsage.CacheReadInputTokens
	return int(math.Round(float64(current) / float64(cw.ContextWindowSize) * 100))
}

func formatCost(costUSD float64) string {
	if costUSD < 0.01 {
		return fmt.Sprintf("%.2f¢", costUSD*100)
	}
	return fmt.Sprintf("$%.2f", costUSD)
}

func formatDuration(ms float64) string {
	if math.IsNaN(ms) || ms <= 0 {
		return ""
	}
	if ms < 1000 {
		return fmt.Sprintf("%.0fms", ms)
	}
	if ms < 60000 {
		return fmt.Sprintf("%.1fs", ms/1000)
	}
	minutes := int(ms / 60000)
	seconds := int(math.Mod(ms, 60000) / 1000)
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

func getTokenSpeed(sessionID string) (tokensPerSecond int, timeToFirstToken float64) {
	tmpDir := filepath.Join(os.TempDir(), "claude-code-router")
	statsFile := filepath.Join(tmpDir, fmt.Sprintf("session-%s.json", sessionID))
	data, err := os.ReadFile(statsFile)
	if err != nil {
		return 0, 0
	}
	var stats struct {
		TokensPerSecond int     `json:"tokensPerSecond"`
		TimeToFirst     float64 `json:"timeToFirstToken"`
		Timestamp       int64   `json:"timestamp"`
	}
	if err := json.Unmarshal(data, &stats); err != nil {
		return 0, 0
	}
	if time.Since(time.Unix(stats.Timestamp/1000, 0)) > 3*time.Second {
		return 0, stats.TimeToFirst
	}
	return stats.TokensPerSecond, stats.TimeToFirst
}

func getGitBranch(workDir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func canDisplayNerdFonts() bool {
	if os.Getenv("USE_SIMPLE_ICONS") == "true" {
		return false
	}
	termProgram := os.Getenv("TERM_PROGRAM")
	for _, t := range []string{"iTerm.app", "vscode", "Hyper", "kitty", "alacritty"} {
		if termProgram == t {
			return true
		}
	}
	colorTerm := os.Getenv("COLORTERM")
	if strings.Contains(colorTerm, "truecolor") || strings.Contains(colorTerm, "24bit") {
		return true
	}
	return os.Getenv("USE_SIMPLE_ICONS") != "true"
}

func shouldUseSimpleTheme() bool {
	return os.Getenv("USE_SIMPLE_ICONS") == "true" || os.Getenv("TERM") == "dumb"
}

func getThemeConfig() (ThemeConfig, string) {
	configPath := filepath.Join(os.Getenv("HOME"), ".claude-code-router", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ThemeConfig{}, "default"
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ThemeConfig{}, "default"
	}
	sl, ok := cfg["StatusLine"].(map[string]any)
	if !ok {
		return ThemeConfig{}, "default"
	}
	style := "default"
	if s, ok := sl["currentStyle"].(string); ok {
		style = s
	}
	themeRaw, ok := sl[style].(map[string]any)
	if !ok {
		return ThemeConfig{}, style
	}
	modulesRaw, ok := themeRaw["modules"].([]any)
	if !ok {
		return ThemeConfig{}, style
	}
	var modules []ModuleConfig
	for _, m := range modulesRaw {
		if mMap, ok := m.(map[string]any); ok {
			mod := ModuleConfig{}
			if v, ok := mMap["type"].(string); ok {
				mod.Type = v
			}
			if v, ok := mMap["icon"].(string); ok {
				mod.Icon = v
			}
			if v, ok := mMap["text"].(string); ok {
				mod.Text = v
			}
			if v, ok := mMap["color"].(string); ok {
				mod.Color = v
			}
			if v, ok := mMap["background"].(string); ok {
				mod.Background = v
			}
			if v, ok := mMap["scriptPath"].(string); ok {
				mod.ScriptPath = v
			}
			modules = append(modules, mod)
		}
	}
	return ThemeConfig{Modules: modules}, style
}

// RenderStatusLine renders the statusline from input JSON
func RenderStatusLine(inputJSON string) string {
	var input Input
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return ""
	}

	useSimple := shouldUseSimpleTheme()
	canNerd := canDisplayNerdFonts()

	var defaultT ThemeConfig
	if useSimple || !canNerd {
		defaultT = simpleTheme
	} else {
		defaultT = defaultTheme
	}

	projectTheme, style := getThemeConfig()
	theme := defaultT
	if len(projectTheme.Modules) > 0 {
		theme = projectTheme
	}

	workDir := input.Workspace.CurrentDir
	gitBranch := getGitBranch(workDir)
	workDirName := filepath.Base(workDir)

	model := input.Model.DisplayName
	inputTokens := 0
	outputTokens := 0

	// Read transcript for last assistant message
	if input.TranscriptPath != "" {
		if data, err := os.ReadFile(input.TranscriptPath); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			for i := len(lines) - 1; i >= 0; i-- {
				var msg struct {
					Type    string `json:"type"`
					Message struct {
						Model string `json:"model"`
						Usage struct {
							InputTokens  int `json:"input_tokens"`
							OutputTokens int `json:"output_tokens"`
						} `json:"usage"`
					} `json:"message"`
				}
				if err := json.Unmarshal([]byte(lines[i]), &msg); err == nil && msg.Type == "assistant" {
					if msg.Message.Model != "" {
						model = msg.Message.Model
					}
					inputTokens = msg.Message.Usage.InputTokens
					outputTokens = msg.Message.Usage.OutputTokens
					break
				}
			}
		}
	}

	formattedInput, formattedOutput := formatUsage(inputTokens, outputTokens)
	tokensPerSec, ttft := getTokenSpeed(input.SessionID)
	isStreaming := tokensPerSec > 0
	tokenSpeedStr := ""
	if tokensPerSec > 0 {
		tokenSpeedStr = fmt.Sprintf("%d", tokensPerSec)
	}
	streamingIndicator := ""
	if isStreaming {
		streamingIndicator = "streaming"
	}
	ttftStr := ""
	if ttft > 0 {
		ttftStr = formatDuration(ttft)
	}
	ctxPercent := calculateContextPercent(input.ContextWindow)
	ctxWindowSize := ""
	if input.ContextWindow != nil && input.ContextWindow.ContextWindowSize > 0 {
		if input.ContextWindow.ContextWindowSize > 1000 {
			ctxWindowSize = fmt.Sprintf("%.0fk", float64(input.ContextWindow.ContextWindowSize)/1000)
		} else {
			ctxWindowSize = fmt.Sprintf("%d", input.ContextWindow.ContextWindowSize)
		}
	}
	totalInput := ""
	totalOutput := ""
	if input.ContextWindow != nil {
		if input.ContextWindow.TotalInputTokens > 1000 {
			totalInput = fmt.Sprintf("%.1fk", float64(input.ContextWindow.TotalInputTokens)/1000)
		} else {
			totalInput = fmt.Sprintf("%d", input.ContextWindow.TotalInputTokens)
		}
		if input.ContextWindow.TotalOutputTokens > 1000 {
			totalOutput = fmt.Sprintf("%.1fk", float64(input.ContextWindow.TotalOutputTokens)/1000)
		} else {
			totalOutput = fmt.Sprintf("%d", input.ContextWindow.TotalOutputTokens)
		}
	}

	costStr := ""
	durationStr := ""
	linesAdded := 0
	linesRemoved := 0
	if input.Cost != nil {
		if input.Cost.TotalCostUSD > 0 {
			costStr = formatCost(input.Cost.TotalCostUSD)
		}
		if input.Cost.TotalDurationMs > 0 {
			durationStr = formatDuration(input.Cost.TotalDurationMs)
		}
		linesAdded = input.Cost.LinesAdded
		linesRemoved = input.Cost.LinesRemoved
	}

	variables := map[string]string{
		"workDirName":        workDirName,
		"gitBranch":          gitBranch,
		"model":              model,
		"inputTokens":        formattedInput,
		"outputTokens":       formattedOutput,
		"tokenSpeed":         tokenSpeedStr,
		"isStreaming":        streamingIndicator,
		"timeToFirstToken":   ttftStr,
		"contextPercent":     fmt.Sprintf("%d", ctxPercent),
		"streamingIndicator": streamingIndicator,
		"contextWindowSize":  ctxWindowSize,
		"totalInputTokens":   totalInput,
		"totalOutputTokens":  totalOutput,
		"cost":               costStr,
		"duration":           durationStr,
		"linesAdded":         fmt.Sprintf("%d", linesAdded),
		"linesRemoved":       fmt.Sprintf("%d", linesRemoved),
		"netLines":           fmt.Sprintf("%d", linesAdded-linesRemoved),
		"version":            input.Version,
		"sessionId":          "",
	}
	if len(input.SessionID) >= 8 {
		variables["sessionId"] = input.SessionID[:8]
	}

	if style == "powerline" {
		return renderPowerline(theme, variables)
	}
	return renderDefault(theme, variables)
}

func renderDefault(theme ThemeConfig, variables map[string]string) string {
	var parts []string
	for _, mod := range theme.Modules {
		text := replaceVariables(mod.Text, variables)
		if text == "" {
			continue
		}
		displayText := mod.Text
		if mod.Icon != "" {
			displayText = mod.Icon + " " + text
		} else {
			displayText = text
		}
		colorCode := ""
		if mod.Color != "" {
			if r, g, b, ok := getTrueColorRgb(mod.Color); ok {
				colorCode = fgTrueColor(r, g, b)
			}
		}
		bgCode := ""
		if mod.Background != "" {
			if r, g, b, ok := getTrueColorRgb(mod.Background); ok {
				bgCode = bgTrueColor(r, g, b)
			}
		}
		parts = append(parts, bgCode+colorCode+displayText+reset)
	}
	return strings.Join(parts, " ")
}

const sepRight = "\uE0B0"

func renderPowerline(theme ThemeConfig, variables map[string]string) string {
	var segments []string
	for i, mod := range theme.Modules {
		text := replaceVariables(mod.Text, variables)
		if text == "" {
			continue
		}
		displayText := mod.Text
		if mod.Icon != "" {
			displayText = mod.Icon + " " + text
		} else {
			displayText = text
		}

		bgName := mod.Background
		if bgName == "" {
			bgName = "bg_bright_blue"
		}
		bgR, bgG, bgB, bgOk := getTrueColorRgb(bgName)
		if !bgOk {
			bgR, bgG, bgB = 33, 150, 243
		}

		fgR, fgG, fgB := 255, 255, 255
		if mod.Color != "" {
			if r, g, b, ok := getTrueColorRgb(mod.Color); ok {
				fgR, fgG, fgB = r, g, b
			}
		}

		body := bgTrueColor(bgR, bgG, bgB) + fgTrueColor(fgR, fgG, fgB) + " " + displayText + " " + reset

		var nextBgName string
		if i < len(theme.Modules)-1 {
			nextBgName = theme.Modules[i+1].Background
		}

		if nextBgName != "" {
			nextR, nextG, nextB, nextOk := getTrueColorRgb(nextBgName)
			if !nextOk {
				nextR, nextG, nextB = 0, 0, 0
			}
			sep := fgTrueColor(bgR, bgG, bgB) + bgTrueColor(nextR, nextG, nextB) + sepRight + reset
			segments = append(segments, body+sep)
		} else {
			sep := fgTrueColor(bgR, bgG, bgB) + bgTrueColor(0, 0, 0) + sepRight + reset
			segments = append(segments, body+sep)
		}
	}
	return strings.Join(segments, "")
}
