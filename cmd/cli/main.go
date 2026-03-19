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
	"strings"

	"github.com/musistudio/ccg/internal/config"
	"github.com/musistudio/ccg/internal/preset"
	"github.com/musistudio/ccg/internal/server"
	"github.com/musistudio/ccg/internal/tui"
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
	case "model", "models":
		showModels()
	case "ui":
		fmt.Println("Web UI is not implemented in CCG.")
		fmt.Println("Use 'ccg tui' for terminal UI instead.")
		os.Exit(1)
	case "tui":
		runTUI()
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
	case "-v", "--version", "version":
		showVersion()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`CCG - Claude Code Router v` + VERSION + `

Usage: ccg <command>

Commands:
  start         Start the CCG server
  stop          Stop the CCG server
  restart       Restart the CCG server
  status        Show server status
  statusline    Integrated statusline for prompt
  code          Execute claude command
  model         Interactive model selection
  ui            Open Web UI (not implemented)
  tui           Open Terminal UI
  preset        Manage presets
  install       Install preset from marketplace
  activate      Output environment variables
  env           Show environment variables
  -v, version   Show version

Examples:
  ccg start
  ccg status
  ccg model
  ccg tui
  ccg preset list`)
}

func startServer() {
	log.Println("Starting CCG server...")
	srv := server.New()
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func stopServer() {
	log.Println("Stopping CCG server...")
	exec.Command("pkill", "-f", "ccg-server").Run()
}

func showStatus() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("CCG is not configured. Run 'ccg start' to start with default settings.")
		return
	}

	if err := cfg.Load(configPath); err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Printf("CCG Status:\n")
	fmt.Printf("  Config: %s\n", configPath)
	fmt.Printf("  Server: ")

	if isRunning() {
		fmt.Println("Running")
	} else {
		fmt.Println("Stopped")
	}

	providers := cfg.GetProviders()
	fmt.Printf("  Providers: %d configured\n", len(providers))

	for _, p := range providers {
		apiKeyDisplay := p.APIKey
		if len(apiKeyDisplay) > 8 {
			apiKeyDisplay = apiKeyDisplay[:8] + "..."
		}
		fmt.Printf("    - %s (%d models)\n", p.Name, len(p.Models))
	}

	router := cfg.GetRouter()
	if router != nil {
		fmt.Printf("  Router:\n")
		fmt.Printf("    default: %s\n", router.Default)
		if router.Background != "" {
			fmt.Printf("    background: %s\n", router.Background)
		}
		if router.Think != "" {
			fmt.Printf("    think: %s\n", router.Think)
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
	fmt.Println("Available models:")
	fmt.Println("")

	for _, p := range providers {
		fmt.Printf("[%s]\n", p.Name)
		for _, m := range p.Models {
			fmt.Printf("  %s,%s\n", p.Name, m)
		}
		fmt.Println("")
	}
}

func activate() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()

	if err := cfg.Load(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	providers := cfg.GetProviders()

	fmt.Printf("export CCG_HOST=%s\n", cfg.Get("HOST"))
	fmt.Printf("export CCG_PORT=%s\n", cfg.Get("PORT"))

	for _, p := range providers {
		envName := toUpper(p.Name) + "_API_KEY"
		fmt.Printf("export %s=%s\n", envName, p.APIKey)
	}

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
	if len(os.Args) < 3 {
		fmt.Println("Usage: ccg code <prompt>")
		os.Exit(1)
	}

	prompt := strings.Join(os.Args[2:], " ")

	cfg := config.New()
	configPath := config.GetDefaultConfigPath()
	if err := cfg.Load(configPath); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	providers := cfg.GetProviders()
	if len(providers) == 0 {
		fmt.Println("Error: No providers configured")
		os.Exit(1)
	}

	provider := providers[0]
	host := provider.Host
	if host == "" {
		host = getDefaultHost(provider.Name)
	}

	reqBody := map[string]any{
		"model": provider.Models[0],
		"messages": []map[string]any{
			{"role": "user", "content": prompt},
		},
		"stream": false,
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", host+"/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	result, _ := io.ReadAll(resp.Body)
	fmt.Println(string(result))
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

		var input map[string]any
		if err := json.Unmarshal([]byte(line), &input); err != nil {
			continue
		}

		showStatus()
	}
}

func handleInstall() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ccg install <preset-name>")
		os.Exit(1)
	}

	name := os.Args[2]
	pm := preset.NewPresetManager()

	marketURL := fmt.Sprintf("https://raw.githubusercontent.com/musistudio/ccg-presets/main/%s/manifest.json", name)
	if err := pm.InstallPreset(marketURL, name); err != nil {
		fmt.Printf("Error installing preset: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Preset '%s' installed successfully\n", name)
}

func showVersion() {
	fmt.Printf("CCG version %s\n", VERSION)
}

func runTUI() {
	if err := tui.Run(); err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func isRunning() bool {
	cmd := exec.Command("pgrep", "-f", "ccg-server")
	return cmd.Run() == nil
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
			fmt.Printf("Error listing presets: %v\n", err)
			return
		}

		if len(presets) == 0 {
			fmt.Println("No presets installed.")
			return
		}

		fmt.Println("Installed presets:")
		for _, p := range presets {
			fmt.Printf("  - %s (v%s)\n", p.Name, p.Version)
			if p.Description != "" {
				fmt.Printf("    %s\n", p.Description)
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
			fmt.Println("Usage: ccg preset export <name> [output]")
			return
		}
		name := os.Args[3]
		output := ""
		if len(os.Args) > 4 {
			output = os.Args[4]
		} else {
			output = name + ".json"
		}
		if err := pm.ExportPreset(name, output); err != nil {
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
