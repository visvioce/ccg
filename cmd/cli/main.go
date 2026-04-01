package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/musistudio/ccg/internal/config"
	"github.com/musistudio/ccg/internal/preset"
	"github.com/musistudio/ccg/internal/server"
	"github.com/musistudio/ccg/internal/statusline"
	"github.com/musistudio/ccg/pkg/colors"
	"github.com/musistudio/ccg/pkg/shared"
)

const VERSION = "2.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "start":
		startServer()
	case "stop":
		stopServer()
	case "restart":
		stopServer()
		startServer()
	case "status":
		showStatus()
	case "model":
		showModels()
	case "ui":
		startUI()
	case "preset":
		handlePreset()
	case "activate":
		activate()
	case "env":
		showEnv()
	case "code":
		runCode()
	case "statusline":
		runStatusline()
	case "install":
		handleInstall()
	case "-h", "--help", "help":
		printUsage()
	case "-v", "--version", "version":
		showVersion()
	default:
		// Check if it's a preset name
		if handlePresetQuickCall(command) {
			return
		}
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`%s%s%s%s

`, colors.Bold, colors.Colorize(colors.Cyan, "CCG"), colors.Reset, colors.Colorize(colors.Dim, " - Claude Code Router"))
	fmt.Printf("%s%s%s\n\n", colors.Dim, "Usage:", colors.Reset)
	fmt.Printf("  %sccg %s<%scommand%s>\n\n", colors.Bold, colors.Reset, colors.Dim, colors.Reset)
	
	fmt.Printf("%s%s%s\n", colors.Dim, "Commands:", colors.Reset)
	fmt.Printf("  %s%-15s%s Start the CCG server\n", colors.Colorize(colors.Green, "• "), "start", colors.Reset)
	fmt.Printf("  %s%-15s%s Stop the CCG server\n", colors.Colorize(colors.Green, "• "), "stop", colors.Reset)
	fmt.Printf("  %s%-15s%s Restart the CCG server\n", colors.Colorize(colors.Green, "• "), "restart", colors.Reset)
	fmt.Printf("  %s%-15s%s Show server status\n", colors.Colorize(colors.Green, "• "), "status", colors.Reset)
	fmt.Printf("  %s%-15s%s Integrated statusline for prompt\n", colors.Colorize(colors.Green, "• "), "statusline", colors.Reset)
	fmt.Printf("  %s%-15s%s Execute claude command\n", colors.Colorize(colors.Green, "• "), "code", colors.Reset)
	fmt.Printf("  %s%-15s%s Interactive model selection\n", colors.Colorize(colors.Green, "• "), "model", colors.Reset)
	fmt.Printf("  %s%-15s%s Open Web UI\n", colors.Colorize(colors.Green, "• "), "ui", colors.Reset)
	fmt.Printf("  %s%-15s%s Manage presets\n", colors.Colorize(colors.Green, "• "), "preset", colors.Reset)
	fmt.Printf("  %s%-15s%s Install preset from marketplace\n", colors.Colorize(colors.Green, "• "), "install", colors.Reset)
	fmt.Printf("  %s%-15s%s Output environment variables\n", colors.Colorize(colors.Green, "• "), "activate", colors.Reset)
	fmt.Printf("  %s%-15s%s Show environment variables\n", colors.Colorize(colors.Green, "• "), "env", colors.Reset)
	fmt.Printf("  %s%-15s%s Show version\n", colors.Colorize(colors.Green, "• "), "-v, version", colors.Reset)
	fmt.Printf("  %s%-15s%s Execute prompt with preset\n", colors.Colorize(colors.Green, "• "), "<preset>", colors.Reset)
	
	fmt.Printf("\n%s%s%s\n", colors.Dim, "Examples:", colors.Reset)
	fmt.Printf("  %sccg start%s\n", colors.Colorize(colors.Cyan, ""), colors.Reset)
	fmt.Printf("  %sccg status%s\n", colors.Colorize(colors.Cyan, ""), colors.Reset)
	fmt.Printf("  %sccg model%s\n", colors.Colorize(colors.Cyan, ""), colors.Reset)
	fmt.Printf("  %sccg preset list%s\n", colors.Colorize(colors.Cyan, ""), colors.Reset)
	fmt.Printf("  %sccg my-preset %s\"Write a Hello World\"%s  %s# Use preset configuration%s\n", 
		colors.Colorize(colors.Cyan, ""), 
		colors.Colorize(colors.Yellow, ""),
		colors.Reset,
		colors.Dim,
		colors.Reset)
}

func startServer() {
	fmt.Printf("%sStarting CCG server...\n", colors.Colorize(colors.Cyan, "→ "))

	// Check if already running
	if isRunning() {
		fmt.Println(colors.Colorize(colors.Yellow, "⚠ CCG server is already running"))
		return
	}

	// Check if daemon mode
	if len(os.Args) > 2 && os.Args[2] == "--daemon" {
		// Run in background using setsid to create new session
		cmd := exec.Command("setsid", os.Args[0], "start")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			fmt.Printf("%sFailed to start daemon: %v\n", colors.Colorize(colors.Red, "✗ "), err)
			os.Exit(1)
		}
		// Don't save PID here - let the child process save its own PID
		// Just wait a moment for child to start
		time.Sleep(500 * time.Millisecond)
		// Read PID from file (child should have created it)
		if pidData, err := os.ReadFile(shared.PIDFile); err == nil {
			fmt.Printf("%sCCG server started in background (PID: %s)\n", colors.Colorize(colors.Green, "✓ "), strings.TrimSpace(string(pidData)))
		} else {
			fmt.Println(colors.Colorize(colors.Green, "✓ CCG server started in background"))
		}
		return
	}

	// Run in foreground
	// Save PID to file first
	if err := os.WriteFile(shared.PIDFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		fmt.Printf("%sWarning: failed to write PID file: %v\n", colors.Colorize(colors.Yellow, "⚠ "), err)
	}
	fmt.Println(colors.Colorize(colors.Green, "✓ CCG server started"))
	srv := server.New()
	if err := srv.Start(); err != nil {
		fmt.Printf("%sServer error: %v\n", colors.Colorize(colors.Red, "✗ "), err)
		os.Exit(1)
	}
}

func startUI() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()
	if err := cfg.Load(configPath); err == nil {
		// Config loaded successfully
	}

	port := cfg.Get("PORT")
	if port == "" {
		port = "3456"
	}
	host := cfg.Get("HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	// Start server in background if not running
	if !isRunning() {
		log.Println("Starting CCG server...")
		startServerInBackground()
		time.Sleep(2 * time.Second)
	}

	// Open browser
	url := fmt.Sprintf("http://%s:%s", host, port)
	var cmd *exec.Cmd
	switch {
	case os.Getenv("WSL_DISTRO_NAME") != "":
		cmd = exec.Command("wslview", url)
	case os.Getenv("OS") == "Windows_NT":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open browser: %v", err)
		fmt.Printf("Please open %s in your browser\n", url)
	} else {
		fmt.Printf("Opening Web UI at %s...\n", url)
	}
}

func stopServer() {
	fmt.Printf("%sStopping CCG server...\n", colors.Colorize(colors.Cyan, "→ "))

	// Check reference count
	if data, err := os.ReadFile(refCountFile); err == nil {
		count := 0
		fmt.Sscanf(string(data), "%d", &count)
		if count > 0 {
			fmt.Printf("%sCannot stop server: %d active code sessions running\n", colors.Colorize(colors.Yellow, "⚠ "), count)
			return
		}
	}

	// Read PID from file
	pidData, err := os.ReadFile(shared.PIDFile)
	if err != nil {
		fmt.Println(colors.Colorize(colors.Yellow, "⚠ CCG server is not running (no PID file)"))
		return
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		fmt.Println(colors.Colorize(colors.Yellow, "⚠ Invalid PID file"))
		os.Remove(shared.PIDFile)
		return
	}

	// Kill the process
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println(colors.Colorize(colors.Yellow, "⚠ CCG server is not running"))
		os.Remove(shared.PIDFile)
		return
	}

	if err := process.Kill(); err != nil {
		fmt.Printf("%sFailed to stop server: %v\n", colors.Colorize(colors.Red, "✗ "), err)
	} else {
		fmt.Println(colors.Colorize(colors.Green, "✓ CCG server stopped successfully"))
	}

	// Clean up PID file
	os.Remove(shared.PIDFile)
}

func showStatus() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(colors.Colorize(colors.Yellow, "CCG is not configured. Run 'ccg start' to start with default settings."))
		return
	}

	if err := cfg.Load(configPath); err != nil {
		fmt.Printf("%sError loading config: %v\n", colors.Colorize(colors.Red, "✗ "), err)
		return
	}

	fmt.Printf("%sCCG Status:\n", colors.Colorize(colors.BoldCyan, "→ "))
	fmt.Printf("  %s%s\n", colors.Colorize(colors.Dim, "Config: "), configPath)
	fmt.Printf("  %s", colors.Colorize(colors.Dim, "Server: "))

	if isRunning() {
		fmt.Println(colors.Colorize(colors.Green, "Running ✓"))
	} else {
		fmt.Println(colors.Colorize(colors.Red, "Stopped ✗"))
	}

	providers := cfg.GetProviders()
	fmt.Printf("  %s%d configured\n", colors.Colorize(colors.Dim, "Providers: "), len(providers))

	for _, p := range providers {
		fmt.Printf("    %s%s %s(%d models)\n", 
			colors.Colorize(colors.Green, "•"),
			colors.Colorize(colors.Bold, p.Name),
			colors.Colorize(colors.Dim, ""),
			len(p.Models))
	}

	router := cfg.GetRouter()
	if router != nil {
		fmt.Printf("  %s\n", colors.Colorize(colors.Dim, "Router:"))
		fmt.Printf("    %s%s\n", colors.Colorize(colors.Dim, "default: "), router.Default)
		if router.Background != "" {
			fmt.Printf("    %s%s\n", colors.Colorize(colors.Dim, "background: "), router.Background)
		}
		if router.Think != "" {
			fmt.Printf("    %s%s\n", colors.Colorize(colors.Dim, "think: "), router.Think)
		}
	}
}

func showModels() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	if err := cfg.Load(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	providers := cfg.GetProviders()

	// Non-interactive mode: just list models
	if cfg.IsNonInteractiveMode() || (len(os.Args) > 2 && os.Args[2] == "--list") {
		fmt.Println("Available models:")
		for _, p := range providers {
			fmt.Printf("[%s]\n", p.Name)
			for _, m := range p.Models {
				fmt.Printf("  %s,%s\n", p.Name, m)
			}
		}
		return
	}

	runFullModelSelector()
}

func activate() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	if err := cfg.Load(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	port := cfg.Get("PORT")
	if port == "" {
		port = "3456"
	}
	host := cfg.Get("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	apiKey := cfg.Get("APIKEY")
	proxyURL := cfg.Get("PROXY_URL")
	timeout := cfg.Get("API_TIMEOUT_MS")

	baseURL := fmt.Sprintf("http://%s:%s", host, port)

	fmt.Printf("export ANTHROPIC_BASE_URL=%s/v1/messages\n", baseURL)
	fmt.Printf("export ANTHROPIC_AUTH_TOKEN=%s\n", apiKey)
	fmt.Printf("export ANTHROPIC_API_KEY=%s\n", apiKey)
	fmt.Printf("export CLAUDE_CODE_USE_CCR=true\n")

	if proxyURL != "" {
		fmt.Printf("export NO_PROXY=%s\n", proxyURL)
		fmt.Printf("export HTTP_PROXY=%s\n", proxyURL)
		fmt.Printf("export HTTPS_PROXY=%s\n", proxyURL)
	}
	if timeout != "" {
		fmt.Printf("export API_TIMEOUT_MS=%s\n", timeout)
	}

	fmt.Printf("export DISABLE_TELEMETRY=1\n")
	fmt.Printf("export DISABLE_COST_WARNINGS=1\n")

	fmt.Println("")
	fmt.Println("# Add to your shell profile to use CCG:")
	fmt.Println("# source <(ccg activate)")
}

func showEnv() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	if err := cfg.Load(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	providers := cfg.GetProviders()

	fmt.Println("Environment variables:")
	fmt.Printf("  CCG_HOST=%s\n", cfg.Get("HOST"))
	fmt.Printf("  CCG_PORT=%s\n", cfg.Get("PORT"))

	for _, p := range providers {
		envName := toUpper(p.Name) + "_API_KEY"
		fmt.Printf("  %s=%s\n", envName, p.APIKey)
	}
}

func runCode() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()
	if err := cfg.Load(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Ensure server is running
	if !isRunning() {
		log.Println("Starting CCG server...")
		startServerInBackground()
		time.Sleep(2 * time.Second)
	}

	// Build environment variables for Claude Code
	port := cfg.Get("PORT")
	if port == "" {
		port = "3456"
	}
	host := cfg.Get("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	apiKey := cfg.Get("APIKEY")

	baseURL := fmt.Sprintf("http://%s:%s", host, port)

	// Set environment variables for Claude Code
	env := os.Environ()
	env = append(env, fmt.Sprintf("ANTHROPIC_BASE_URL=%s/v1/messages", baseURL))
	env = append(env, fmt.Sprintf("ANTHROPIC_AUTH_TOKEN=%s", apiKey))
	env = append(env, fmt.Sprintf("ANTHROPIC_API_KEY=%s", apiKey))
	env = append(env, "CLAUDE_CODE_USE_CCR=true")

	// Non-interactive mode support
	if cfg.IsNonInteractiveMode() {
		env = append(env, "CI=true")
		env = append(env, "FORCE_COLOR=0")
		env = append(env, "NODE_NO_READLINE=1")
		env = append(env, "TERM=dumb")
	}

	// Create settings file for Claude Code
	settingsPath := filepath.Join(os.TempDir(), fmt.Sprintf("ccg-settings-%d.json", os.Getpid()))
	settings := map[string]any{
		"apiKeyHelper":           nil,
		"customApiKeyResponses":  []any{},
		"hasCompletedOnboarding": true,
		"mcpServers":             map[string]any{},
		"projects":               map[string]any{},
		"permissions": map[string]any{
			"allow": []any{
				"Bash(find:*)",
				"Bash(ls:*)",
				"Bash(tree:*)",
				"Bash(cat:*)",
				"Bash(head:*)",
				"Bash(tail:*)",
				"Bash(wc:*)",
				"Bash(grep:*)",
				"Bash(awk:*)",
				"Bash(sort:*)",
				"Bash(uniq:*)",
				"Bash(diff:*)",
				"Bash(realpath:*)",
				"Bash(file:*)",
				"Bash(stat:*)",
				"Bash(md5sum:*)",
				"Bash(sha256sum:*)",
				"Bash(echo:*)",
				"Bash(pwd:*)",
				"Bash(which:*)",
				"Bash(date:*)",
			},
			"deny": []any{},
		},
	}
	settingsData, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, settingsData, 0644)
	defer os.Remove(settingsPath)

	env = append(env, fmt.Sprintf("CLAUDE_CODE_SETTINGS=%s", settingsPath))

	// Find claude executable
	claudePath := cfg.Get("CLAUDE_PATH")
	if claudePath == "" {
		var err error
		claudePath, err = exec.LookPath("claude")
		if err != nil {
			fmt.Println("Error: 'claude' command not found. Please install Claude Code CLI.")
			fmt.Println("  npm install -g @anthropic-ai/claude-code")
			os.Exit(1)
		}
	}

	// Build command arguments
	args := []string{}
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	// Launch Claude Code
	cmd := exec.Command(claudePath, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Increment reference count
	incrementRefCount()
	defer decrementRefCount()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatalf("Failed to run claude: %v", err)
	}
}

const refCountFile = "/tmp/ccg-reference-count.txt"

func incrementRefCount() {
	data, _ := os.ReadFile(refCountFile)
	count := 0
	fmt.Sscanf(string(data), "%d", &count)
	count++
	os.WriteFile(refCountFile, []byte(fmt.Sprintf("%d", count)), 0644)
}

func decrementRefCount() {
	data, _ := os.ReadFile(refCountFile)
	count := 0
	fmt.Sscanf(string(data), "%d", &count)
	count--
	if count <= 0 {
		os.Remove(refCountFile)
	} else {
		os.WriteFile(refCountFile, []byte(fmt.Sprintf("%d", count)), 0644)
	}
}

func getDefaultHost(providerName string) string {
	hosts := map[string]string{
		"openai":     "https://api.openai.com/v1/chat/completions",
		"anthropic":  "https://api.anthropic.com/v1/messages",
		"deepseek":   "https://api.deepseek.com/v1/chat/completions",
		"google":     "https://generativelanguage.googleapis.com/v1beta/models",
		"groq":       "https://api.groq.com/openai/v1/chat/completions",
		"openrouter": "https://openrouter.ai/api/v1/chat/completions",
		"cerebras":   "https://api.cerebras.ai/v1/chat/completions",
	}

	lower := strings.ToLower(providerName)
	if host, ok := hosts[lower]; ok {
		return host
	}
	return "https://api.openai.com/v1/chat/completions"
}

func runStatusline() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		output := statusline.RenderStatusLine(line)
		if output != "" {
			fmt.Println(output)
		}
	}
}

func handleInstall() {
	if len(os.Args) < 3 {
		fmt.Printf("%sUsage: ccg install <preset-name>\n", colors.Colorize(colors.Yellow, "⚠ "))
		os.Exit(1)
	}

	name := os.Args[2]
	pm := preset.NewPresetManager()

	fmt.Printf("%sInstalling preset '%s' from marketplace...\n", colors.Colorize(colors.Cyan, "→ "), name)
	
	marketURL := fmt.Sprintf("https://raw.githubusercontent.com/musistudio/ccg-presets/main/%s/manifest.json", name)
	if err := pm.InstallPreset(marketURL, name); err != nil {
		fmt.Printf("%sError installing preset: %v\n", colors.Colorize(colors.Red, "✗ "), err)
		os.Exit(1)
	}

	fmt.Printf("%sPreset '%s' installed successfully\n", colors.Colorize(colors.Green, "✓ "), colors.Colorize(colors.Bold, name))
}

func showVersion() {
	fmt.Printf("%s%s%s %sversion %s%s\n", 
		colors.Bold, 
		colors.Colorize(colors.Cyan, "CCG"), 
		colors.Reset,
		colors.Dim,
		VERSION,
		colors.Reset)
}

func isRunning() bool {
	// Check if PID file exists
	pidData, err := os.ReadFile(shared.PIDFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return false
	}

	// Check if process exists (signal 0 doesn't kill, just checks)
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, signal 0 checks if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func toUpper(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		result[i] = c
	}
	return string(result)
}

func handlePreset() {
	if len(os.Args) < 3 {
		printPresetUsage()
		os.Exit(1)
	}

	subcommand := os.Args[2]

	pm := preset.NewPresetManager()

	switch subcommand {
	case "list":
		presets, err := pm.ListPresets()
		if err != nil {
			fmt.Printf("%sError listing presets: %v\n", colors.Colorize(colors.Red, "✗ "), err)
			return
		}

		if len(presets) == 0 {
			fmt.Println(colors.Colorize(colors.Yellow, "No presets installed."))
			return
		}

		fmt.Printf("%sInstalled presets:\n", colors.Colorize(colors.BoldCyan, "→ "))
		for _, p := range presets {
			fmt.Printf("  %s%s %s(v%s)%s\n", 
				colors.Colorize(colors.Green, "• "),
				colors.Colorize(colors.Bold, p.Name),
				colors.Dim,
				p.Version,
				colors.Reset)
			if p.Description != "" {
				fmt.Printf("    %s%s%s\n", colors.Dim, p.Description, colors.Reset)
			}
		}

	case "info":
		if len(os.Args) < 4 {
			fmt.Println("Usage: ccg preset info <name>")
			return
		}
		name := os.Args[3]
		p, err := pm.GetPreset(name)
		if err != nil {
			fmt.Printf("Preset not found: %v\n", err)
			return
		}
		data, _ := json.MarshalIndent(p, "", "  ")
		fmt.Println(string(data))

	case "install":
		if len(os.Args) < 4 {
			fmt.Println("Usage: ccg preset install <source> [name]")
			return
		}
		source := os.Args[3]
		name := ""
		if len(os.Args) > 4 {
			name = os.Args[4]
		}
		if err := pm.InstallPreset(source, name); err != nil {
			fmt.Printf("Error installing preset: %v\n", err)
			return
		}
		fmt.Println("Preset installed successfully.")

	case "export":
		if len(os.Args) < 4 {
			fmt.Println("Usage: ccg preset export <name> [output] [--description <desc>] [--author <author>] [--tags <tag1,tag2>] [--include-sensitive]")
			return
		}
		name := os.Args[3]
		output := name + ".json"
		description := ""
		author := ""
		tags := ""
		includeSensitive := false

		// Parse optional arguments
		for i := 4; i < len(os.Args); i++ {
			switch os.Args[i] {
			case "--description":
				if i+1 < len(os.Args) {
					description = os.Args[i+1]
					i++
				}
			case "--author":
				if i+1 < len(os.Args) {
					author = os.Args[i+1]
					i++
				}
			case "--tags":
				if i+1 < len(os.Args) {
					tags = os.Args[i+1]
					i++
				}
			case "--include-sensitive":
				includeSensitive = true
			default:
				if !strings.HasPrefix(os.Args[i], "--") {
					output = os.Args[i]
				}
			}
		}

		if err := pm.ExportPresetWithOptions(name, output, description, author, tags, includeSensitive); err != nil {
			fmt.Printf("Error exporting preset: %v\n", err)
			return
		}
		fmt.Printf("Preset exported to: %s\n", output)

	case "delete":
		if len(os.Args) < 4 {
			fmt.Println("Usage: ccg preset delete <name>")
			return
		}
		name := os.Args[3]
		if err := pm.DeletePreset(name); err != nil {
			fmt.Printf("Error deleting preset: %v\n", err)
			return
		}
		fmt.Println("Preset deleted successfully.")

	default:
		printPresetUsage()
	}
}

func printPresetUsage() {
	fmt.Println(`Usage: ccg preset <command>
 
 Commands:
   list               List all installed presets
   info <name>        Show preset details
   install <source>   Install preset from file, URL, or name
   export <name>      Export preset to file
   delete <name>     Delete a preset`)
}

// handlePresetQuickCall handles quick preset invocation: ccg <preset-name> "prompt"
func handlePresetQuickCall(presetName string) bool {
	pm := preset.NewPresetManager()

	// Check if preset exists
	p, err := pm.GetPreset(presetName)
	if err != nil {
		return false
	}

	// Get the prompt from remaining arguments
	if len(os.Args) < 3 {
		fmt.Printf("Usage: ccg %s <prompt>\n", presetName)
		fmt.Printf("Preset '%s' found. Use it with a prompt.\n", p.Name)
		return true
	}

	prompt := strings.Join(os.Args[2:], " ")

	// Ensure server is running
	if !isRunning() {
		log.Println("Starting CCG server...")
		startServerInBackground()
		time.Sleep(2 * time.Second)
	}

	// Load preset configuration and execute
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	// Apply preset configuration
	secrets := make(map[string]string)
	// Could load secrets from environment or prompt
	for _, schema := range p.Schema {
		if envVal := os.Getenv(schema.ID); envVal != "" {
			secrets[schema.ID] = envVal
		}
	}

	// Apply preset to config
	if err := pm.ApplyPreset(presetName, secrets); err != nil {
		fmt.Printf("Error applying preset: %v\n", err)
		return true
	}

	// Execute code command with preset
	cfg.Load(configPath)
	providers := cfg.GetProviders()
	if len(providers) == 0 {
		fmt.Println("Error: No providers configured in preset")
		return true
	}

	provider := providers[0]
	host := provider.Host
	if host == "" {
		host = getDefaultHost(provider.Name)
	}

	// Extract model from router or first model
	router := cfg.GetRouter()
	model := ""
	if router != nil && router.Default != "" {
		model = router.Default
	} else if len(provider.Models) > 0 {
		model = provider.Models[0]
	}

	// Execute the prompt
	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", host, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return true
	}
	defer resp.Body.Close()

	result, _ := io.ReadAll(resp.Body)

	// Parse and display response
	var respData map[string]any
	if err := json.Unmarshal(result, &respData); err == nil {
		if content, ok := respData["content"].([]any); ok && len(content) > 0 {
			if text, ok := content[0].(map[string]any)["text"].(string); ok {
				fmt.Println(text)
				return true
			}
		}
		if choices, ok := respData["choices"].([]any); ok && len(choices) > 0 {
			if message, ok := choices[0].(map[string]any)["message"].(map[string]any); ok {
				if content, ok := message["content"].(string); ok {
					fmt.Println(content)
					return true
				}
			}
		}
	}

	fmt.Println(string(result))
	return true
}

func startServerInBackground() {
	cmd := exec.Command(os.Args[0], "start", "--daemon")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Start()
}
