package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/musistudio/ccg/internal/config"
)

const (
	esc        = "\x1b["
	reset      = esc + "0m"
	boldGreen  = esc + "1m" + esc + "32m"
	cyan       = esc + "36m"
	boldCyan   = esc + "1m" + esc + "36m"
	yellow     = esc + "33m"
	boldYellow = esc + "1m" + esc + "33m"
	dimStyle   = esc + "2m"
)

var availableTransformers = []string{
	"anthropic", "deepseek", "gemini", "openrouter", "groq",
	"maxtoken", "tooluse", "reasoning", "sampling",
	"enhancetool", "cleancache", "vertex-gemini", "openai",
	"cerebras", "vercel", "streamoptions", "customparams",
	"maxcompletiontokens", "forcereasoning",
}

// runFullModelSelector runs the full interactive model selector
func runFullModelSelector() {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()
	if err := cfg.Load(configPath); err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	providers := cfg.GetProviders()
	router := cfg.GetRouter()

	displayCurrentConfig(providers, router)

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s═══════════════════════════════════════════════%s\n", boldCyan, reset)
	fmt.Printf("%s           Add/Update Model%s\n", boldCyan, reset)
	fmt.Printf("%s═══════════════════════════════════════════════%s\n\n", boldCyan, reset)

	fmt.Println("  1) Set Default Model")
	fmt.Println("  2) Set Background Model")
	fmt.Println("  3) Set Think Model")
	fmt.Println("  4) Set Long Context Model")
	fmt.Println("  5) Set Web Search Model")
	fmt.Println("  6) Set Image Model")
	fmt.Printf("  7) %s+ Add New Model%s\n", boldGreen, reset)
	fmt.Printf("  8) %s+ Add New Provider%s\n", boldGreen, reset)
	fmt.Printf("  0) Exit%s\n", reset)
	fmt.Print("\nEnter choice (0-8): ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	modelTypes := map[string]string{
		"1": "default", "2": "background", "3": "think",
		"4": "longContext", "5": "webSearch", "6": "image",
	}

	switch choice {
	case "0":
		return
	case "7":
		addNewModelInteractive(cfg, configPath, reader)
	case "8":
		addNewProviderInteractive(cfg, configPath, reader)
	default:
		modelType, ok := modelTypes[choice]
		if !ok {
			fmt.Println("Invalid choice.")
			return
		}
		selectModelForType(cfg, configPath, router, modelType, reader)
	}
}

func displayCurrentConfig(providers []config.Provider, router *config.RouterConfig) {
	fmt.Printf("\n%s═══════════════════════════════════════════════%s\n", boldCyan, reset)
	fmt.Printf("%s           Current Configuration%s\n", boldCyan, reset)
	fmt.Printf("%s═══════════════════════════════════════════════%s\n\n", boldCyan, reset)

	if router != nil {
		printModel("Default Model", router.Default)
		printModel("Background Model", router.Background)
		printModel("Think Model", router.Think)
		printModel("Long Context Model", router.LongContext)
		printModel("Web Search Model", router.WebSearch)
		printModel("Image Model", router.Image)
	}

	fmt.Printf("\n%s═══════════════════════════════════════════════%s\n", boldCyan, reset)
	fmt.Printf("%s           Available Providers%s\n", boldCyan, reset)
	fmt.Printf("%s═══════════════════════════════════════════════%s\n\n", boldCyan, reset)

	for _, p := range providers {
		fmt.Printf("  %s[%s]%s\n", boldCyan, p.Name, reset)
		for _, m := range p.Models {
			fmt.Printf("    %s%s,%s%s\n", cyan, p.Name, m, reset)
		}
		fmt.Println()
	}
}

func printModel(label, value string) {
	if value == "" {
		fmt.Printf("  %s: %sNot configured%s\n", label, dimStyle, reset)
	} else {
		parts := strings.SplitN(value, ",", 2)
		if len(parts) == 2 {
			fmt.Printf("  %s: %s%s%s | %s\n", label, yellow, parts[0], reset, parts[1])
		} else {
			fmt.Printf("  %s: %s\n", label, value)
		}
	}
}

func selectModelForType(cfg *config.Config, configPath string, router *config.RouterConfig, modelType string, reader *bufio.Reader) {
	providers := cfg.GetProviders()

	fmt.Printf("\n%sSelect a model for '%s':%s\n\n", boldYellow, modelType, reset)

	var modelList []string
	for _, p := range providers {
		for _, m := range p.Models {
			modelList = append(modelList, p.Name+","+m)
		}
	}

	for i, m := range modelList {
		fmt.Printf("  %d) %s%s%s\n", i+1, cyan, m, reset)
	}
	fmt.Print("\nEnter model number: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)
	idx := 0
	fmt.Sscanf(choice, "%d", &idx)
	if idx < 1 || idx > len(modelList) {
		fmt.Println("Invalid choice.")
		return
	}

	selectedModel := modelList[idx-1]

	if router == nil {
		router = &config.RouterConfig{}
	}
	switch modelType {
	case "default":
		router.Default = selectedModel
	case "background":
		router.Background = selectedModel
	case "think":
		router.Think = selectedModel
	case "longContext":
		router.LongContext = selectedModel
	case "webSearch":
		router.WebSearch = selectedModel
	case "image":
		router.Image = selectedModel
	}

	saveConfig(cfg, configPath, providers, router)
	fmt.Printf("%s✓ %s model set to: %s%s\n\n", boldGreen, modelType, selectedModel, reset)
}

func addNewModelInteractive(cfg *config.Config, configPath string, reader *bufio.Reader) {
	providers := cfg.GetProviders()

	fmt.Printf("\n%sAdd New Model%s\n\n", boldCyan, reset)

	// Select provider
	fmt.Println("Select provider:")
	for i, p := range providers {
		fmt.Printf("  %d) %s\n", i+1, p.Name)
	}
	fmt.Printf("  %d) %s+ Add New Provider%s\n", len(providers)+1, boldGreen, reset)
	fmt.Print("\nEnter choice: ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)
	idx := 0
	fmt.Sscanf(choice, "%d", &idx)

	if idx == len(providers)+1 {
		addNewProviderInteractive(cfg, configPath, reader)
		return
	}
	if idx < 1 || idx > len(providers) {
		fmt.Println("Invalid choice.")
		return
	}

	provider := &providers[idx-1]

	fmt.Print("\nEnter model name: ")
	modelName, _ := reader.ReadString('\n')
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		fmt.Println("Model name cannot be empty.")
		return
	}

	// Check if model already exists
	for _, m := range provider.Models {
		if m == modelName {
			fmt.Println("Model already exists in this provider.")
			return
		}
	}

	provider.Models = append(provider.Models, modelName)

	// Ask about transformer
	fmt.Print("\nAdd transformer configuration? (y/N): ")
	tfAnswer, _ := reader.ReadString('\n')
	tfAnswer = strings.TrimSpace(strings.ToLower(tfAnswer))
	if tfAnswer == "y" || tfAnswer == "yes" {
		configureTransformersInteractive(reader, provider)
	}

	saveConfig(cfg, configPath, providers, cfg.GetRouter())
	fmt.Printf("%s✓ Model '%s' added to provider '%s'%s\n\n", boldGreen, modelName, provider.Name, reset)

	// Ask to set in router
	fmt.Print("Set this model in router configuration? (y/N): ")
	setAnswer, _ := reader.ReadString('\n')
	setAnswer = strings.TrimSpace(strings.ToLower(setAnswer))
	if setAnswer == "y" || setAnswer == "yes" {
		fmt.Println("\nSelect model type:")
		fmt.Println("  1) default  2) background  3) think")
		fmt.Println("  4) longContext  5) webSearch  6) image")
		fmt.Print("Enter choice: ")
		typeChoice, _ := reader.ReadString('\n')
		typeChoice = strings.TrimSpace(typeChoice)
		modelTypeMap := map[string]string{"1": "default", "2": "background", "3": "think", "4": "longContext", "5": "webSearch", "6": "image"}
		if mt, ok := modelTypeMap[typeChoice]; ok {
			router := cfg.GetRouter()
			if router == nil {
				router = &config.RouterConfig{}
			}
			fullModel := provider.Name + "," + modelName
			switch mt {
			case "default":
				router.Default = fullModel
			case "background":
				router.Background = fullModel
			case "think":
				router.Think = fullModel
			case "longContext":
				router.LongContext = fullModel
			case "webSearch":
				router.WebSearch = fullModel
			case "image":
				router.Image = fullModel
			}
			saveConfig(cfg, configPath, providers, router)
			fmt.Printf("%s✓ %s set to: %s%s\n\n", boldGreen, mt, fullModel, reset)
		}
	}
}

func addNewProviderInteractive(cfg *config.Config, configPath string, reader *bufio.Reader) {
	providers := cfg.GetProviders()

	fmt.Printf("\n%sAdding New Provider%s\n\n", boldCyan, reset)

	fmt.Print("Provider name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		fmt.Println("Provider name cannot be empty.")
		return
	}
	for _, p := range providers {
		if p.Name == name {
			fmt.Println("Provider already exists.")
			return
		}
	}

	fmt.Print("API base URL: ")
	host, _ := reader.ReadString('\n')
	host = strings.TrimSpace(host)
	if host == "" {
		fmt.Println("API base URL cannot be empty.")
		return
	}

	fmt.Print("API key: ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	fmt.Print("Model names (comma-separated): ")
	modelsStr, _ := reader.ReadString('\n')
	modelsStr = strings.TrimSpace(modelsStr)
	if modelsStr == "" {
		fmt.Println("At least one model name is required.")
		return
	}
	var models []string
	for _, m := range strings.Split(modelsStr, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			models = append(models, m)
		}
	}

	newProvider := config.Provider{
		Name:   name,
		Host:   host,
		APIKey: apiKey,
		Models: models,
	}

	// Ask about transformer
	fmt.Print("\nAdd transformer configuration? (y/N): ")
	tfAnswer, _ := reader.ReadString('\n')
	tfAnswer = strings.TrimSpace(strings.ToLower(tfAnswer))
	if tfAnswer == "y" || tfAnswer == "yes" {
		configureTransformersInteractive(reader, &newProvider)
	}

	providers = append(providers, newProvider)

	fmt.Printf("%s\n✓ Provider '%s' added successfully%s\n\n", boldGreen, name, reset)

	// Ask to set model
	fmt.Print("Set one of these models in router configuration? (y/N): ")
	setAnswer, _ := reader.ReadString('\n')
	setAnswer = strings.TrimSpace(strings.ToLower(setAnswer))
	if setAnswer == "y" || setAnswer == "yes" {
		selectedModel := models[0]
		if len(models) > 1 {
			fmt.Println("\nSelect model:")
			for i, m := range models {
				fmt.Printf("  %d) %s\n", i+1, m)
			}
			fmt.Print("Enter choice: ")
			mChoice, _ := reader.ReadString('\n')
			mChoice = strings.TrimSpace(mChoice)
			mIdx := 0
			fmt.Sscanf(mChoice, "%d", &mIdx)
			if mIdx >= 1 && mIdx <= len(models) {
				selectedModel = models[mIdx-1]
			}
		}

		fmt.Println("\nSelect model type:")
		fmt.Println("  1) default  2) background  3) think")
		fmt.Println("  4) longContext  5) webSearch  6) image")
		fmt.Print("Enter choice: ")
		typeChoice, _ := reader.ReadString('\n')
		typeChoice = strings.TrimSpace(typeChoice)
		modelTypeMap := map[string]string{"1": "default", "2": "background", "3": "think", "4": "longContext", "5": "webSearch", "6": "image"}
		router := cfg.GetRouter()
		if router == nil {
			router = &config.RouterConfig{}
		}
		if mt, ok := modelTypeMap[typeChoice]; ok {
			fullModel := name + "," + selectedModel
			switch mt {
			case "default":
				router.Default = fullModel
			case "background":
				router.Background = fullModel
			case "think":
				router.Think = fullModel
			case "longContext":
				router.LongContext = fullModel
			case "webSearch":
				router.WebSearch = fullModel
			case "image":
				router.Image = fullModel
			}
		}
		saveConfig(cfg, configPath, providers, router)
	} else {
		saveConfig(cfg, configPath, providers, cfg.GetRouter())
	}
}

func configureTransformersInteractive(reader *bufio.Reader, provider *config.Provider) {
	fmt.Printf("\n%sAvailable transformers:%s\n", boldYellow, reset)
	for i, t := range availableTransformers {
		fmt.Printf("  %d) %s\n", i+1, t)
	}

	if provider.Transform == nil {
		provider.Transform = make(map[string]any)
	}

	for {
		fmt.Print("\nSelect transformer number (0 to finish): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
		idx := 0
		fmt.Sscanf(choice, "%d", &idx)
		if idx == 0 {
			break
		}
		if idx < 1 || idx > len(availableTransformers) {
			fmt.Println("Invalid choice.")
			continue
		}

		transformer := availableTransformers[idx-1]

		if transformer == "maxtoken" {
			fmt.Print("Max tokens (default 30000): ")
			maxTokens, _ := reader.ReadString('\n')
			maxTokens = strings.TrimSpace(maxTokens)
			if maxTokens == "" {
				maxTokens = "30000"
			}
			provider.Transform[transformer] = map[string]any{"max_tokens": maxTokens}
		} else if transformer == "openrouter" {
			fmt.Print("Provider routing (e.g., moonshotai/fp8, or empty to skip): ")
			provInput, _ := reader.ReadString('\n')
			provInput = strings.TrimSpace(provInput)
			if provInput != "" {
				provider.Transform[transformer] = map[string]any{"provider": map[string]any{"only": []string{provInput}}}
			} else {
				provider.Transform[transformer] = true
			}
		} else {
			// Check if already in use list
			if use, ok := provider.Transform["use"].([]any); ok {
				found := false
				for _, u := range use {
					if s, ok := u.(string); ok && s == transformer {
						found = true
						break
					}
				}
				if !found {
					provider.Transform["use"] = append(use, transformer)
				}
			} else {
				provider.Transform["use"] = []string{transformer}
			}
		}

		fmt.Printf("%s✓ Transformer '%s' added%s\n", boldGreen, transformer, reset)
	}
}

func saveConfig(cfg *config.Config, configPath string, providers []config.Provider, router *config.RouterConfig) {
	configData := map[string]any{
		"Providers": providers,
		"Router":    router,
	}
	if apiKey := cfg.Get("APIKEY"); apiKey != "" {
		configData["APIKEY"] = apiKey
	}
	if host := cfg.Get("HOST"); host != "" {
		configData["HOST"] = host
	}
	if port := cfg.Get("PORT"); port != "" {
		configData["PORT"] = port
	}
	if logLevel := cfg.Get("LOG_LEVEL"); logLevel != "" {
		configData["LOG_LEVEL"] = logLevel
	}
	if log := cfg.Get("LOG"); log != "" {
		configData["LOG"] = log == "true"
	}
	if timeout := cfg.Get("API_TIMEOUT_MS"); timeout != "" {
		configData["API_TIMEOUT_MS"] = timeout
	}
	if proxy := cfg.Get("PROXY_URL"); proxy != "" {
		configData["PROXY_URL"] = proxy
	}

	data, _ := json.MarshalIndent(configData, "", "  ")
	os.WriteFile(configPath, data, 0644)
}
