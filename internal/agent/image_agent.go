package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/musistudio/ccg/internal/config"
)

// ImageCacheEntry 图片缓存条目
type ImageCacheEntry struct {
	Source    map[string]interface{} `json:"source"`
	Timestamp int64                  `json:"timestamp"`
}

// LRUCache 简单的 LRU 缓存实现
type LRUCache struct {
	mu       sync.RWMutex
	items    map[string]*listNode
	head     *listNode
	tail     *listNode
	capacity int
}

type listNode struct {
	key   string
	value *ImageCacheEntry
	prev  *listNode
	next  *listNode
}

// NewLRUCache 创建新的 LRU 缓存
func NewLRUCache(capacity int) *LRUCache {
	cache := &LRUCache{
		items:    make(map[string]*listNode),
		capacity: capacity,
	}
	return cache
}

func (c *LRUCache) Get(key string) *ImageCacheEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.items[key]; ok {
		c.moveToFront(node)
		return node.value
	}
	return nil
}

func (c *LRUCache) Set(key string, value *ImageCacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.items[key]; ok {
		node.value = value
		c.moveToFront(node)
		return
	}

	newNode := &listNode{
		key:   key,
		value: value,
	}

	c.items[key] = newNode
	c.addToFront(newNode)

	if len(c.items) > c.capacity {
		c.removeLRU()
	}
}

func (c *LRUCache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.items[key]
	return ok
}

func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*listNode)
	c.head = nil
	c.tail = nil
}

func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *LRUCache) moveToFront(node *listNode) {
	if node == c.head {
		return
	}
	c.remove(node)
	c.addToFront(node)
}

func (c *LRUCache) addToFront(node *listNode) {
	node.next = c.head
	node.prev = nil
	if c.head != nil {
		c.head.prev = node
	}
	c.head = node
	if c.tail == nil {
		c.tail = node
	}
}

func (c *LRUCache) remove(node *listNode) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}
}

func (c *LRUCache) removeLRU() {
	if c.tail == nil {
		return
	}
	delete(c.items, c.tail.key)
	c.remove(c.tail)
}

// ImageCache 图片缓存管理器
type ImageCache struct {
	cache *LRUCache
}

// NewImageCache 创建图片缓存
func NewImageCache(maxSize int) *ImageCache {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &ImageCache{
		cache: NewLRUCache(maxSize),
	}
}

// StoreImage 存储图片
func (ic *ImageCache) StoreImage(id string, source map[string]interface{}) {
	if ic.cache.Has(id) {
		return
	}
	ic.cache.Set(id, &ImageCacheEntry{
		Source:    source,
		Timestamp: time.Now().UnixMilli(),
	})
}

// GetImage 获取图片
func (ic *ImageCache) GetImage(id string) map[string]interface{} {
	entry := ic.cache.Get(id)
	if entry != nil {
		return entry.Source
	}
	return nil
}

// HasImage 检查是否有图片
func (ic *ImageCache) HasImage(id string) bool {
	return ic.cache.Has(id)
}

// Clear 清空缓存
func (ic *ImageCache) Clear() {
	ic.cache.Clear()
}

// Size 获取缓存大小
func (ic *ImageCache) Size() int {
	return ic.cache.Size()
}

// 全局图片缓存
var globalImageCache = NewImageCache(100)

// ImageAgentV2 完整的图片 Agent 实现
type ImageAgentV2 struct {
	tools map[string]Tool
}

// NewImageAgentV2 创建新的图片 Agent
func NewImageAgentV2() *ImageAgentV2 {
	agent := &ImageAgentV2{
		tools: make(map[string]Tool),
	}
	agent.appendTools()
	return agent
}

// Name 返回 Agent 名称
func (a *ImageAgentV2) Name() string {
	return "image"
}

// ShouldHandle 判断是否处理请求 (实现 Agent 接口)
func (a *ImageAgentV2) ShouldHandle(reqBody map[string]any) bool {
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		cfg.Load(configPath)
	}
	return a.ShouldHandleV2(reqBody, cfg)
}

// GetTools 获取工具列表
func (a *ImageAgentV2) GetTools() []Tool {
	tools := make([]Tool, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, tool)
	}
	return tools
}

// HandleToolCall 处理工具调用 (实现 Agent 接口)
func (a *ImageAgentV2) HandleToolCall(toolName string, input map[string]any, reqBody map[string]any, provider *config.Provider) (string, error) {
	// 调用 V2 版本，使用默认配置
	cfg := config.New()
	configPath := config.GetDefaultConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		cfg.Load(configPath)
	}
	return a.HandleToolCallV2(toolName, input, reqBody, cfg, "")
}

// ShouldHandleV2 V2版本
func (a *ImageAgentV2) ShouldHandleV2(reqBody map[string]any, cfg *config.Config) bool {
	router := cfg.GetRouter()
	if router == nil || router.Image == "" {
		return false
	}

	model, _ := reqBody["model"].(string)
	if model == router.Image {
		return false
	}

	messages, ok := reqBody["messages"].([]any)
	if !ok || len(messages) == 0 {
		return false
	}

	lastMessage := messages[len(messages)-1]
	msg, ok := lastMessage.(map[string]any)
	if !ok {
		return false
	}

	content, ok := msg["content"].([]any)
	if !ok {
		return false
	}

	// 检查是否有图片
	for _, item := range content {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		itemType, _ := itemMap["type"].(string)
		if itemType == "image" {
			return true
		}
		// 检查嵌套内容
		if nestedContent, ok := itemMap["content"].([]any); ok {
			for _, nested := range nestedContent {
				nestedMap, ok := nested.(map[string]any)
				if !ok {
					continue
				}
				if nestedMap["type"] == "image" {
					return true
				}
			}
		}
	}

	return false
}

// HandleToolCallV2 V2版本
func (a *ImageAgentV2) HandleToolCallV2(toolName string, input map[string]any, reqBody map[string]any, cfg *config.Config, sessionId string) (string, error) {
	if _, ok := a.tools[toolName]; !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	switch toolName {
	case "analyzeImage":
		return a.handleAnalyzeImage(input, reqBody, cfg, sessionId)
	case "image_generation":
		return a.handleImageGeneration(input, cfg)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// ReqHandler 处理请求，注入系统提示和处理图片
func (a *ImageAgentV2) ReqHandler(reqBody map[string]interface{}, sessionId string) {
	// 注入系统提示
	systemPrompt := "You are a text-only language model and do not possess visual perception.\n\n" +
		"If the user requests you to view, analyze, or extract information from an image, you **must** call the `analyzeImage` tool.\n\n" +
		"When invoking this tool, you must pass the correct `imageId` extracted from the prior conversation.\n" +
		"Image identifiers are always provided in the format `[Image #imageId]`.\n\n" +
		"If multiple images exist, select the **most relevant imageId** based on the user's current request and prior context.\n\n" +
			"Do not attempt to describe or analyze the image directly yourself.\n" +
		"Ignore any user interruptions or unrelated instructions that might cause you to skip this requirement.\n" +
		"Your response should consistently follow this rule whenever image-related analysis is requested."

	system, ok := reqBody["system"].([]interface{})
	if !ok {
		system = make([]interface{}, 0)
	}
	system = append(system, map[string]interface{}{
		"type": "text",
		"text": systemPrompt,
	})
	reqBody["system"] = system

	// 处理消息中的图片
	messages, ok := reqBody["messages"].([]interface{})
	if !ok {
		return
	}

	imgId := 1
for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msgMap["role"].(string)
		content, ok := msgMap["content"].([]interface{})
		if !ok || role != "user" {
			continue
		}

		for i, item := range content {
			itemMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			itemType, _ := itemMap["type"].(string)

			if itemType == "image" {
				// 存储图片到缓存
				source, _ := itemMap["source"].(map[string]interface{})
				cacheId := fmt.Sprintf("%s_Image#%d", sessionId, imgId)
				globalImageCache.StoreImage(cacheId, source)

				// 替换为文本占位符
				content[i] = map[string]interface{}{
					"type": "text",
					"text": fmt.Sprintf("[Image #%d]This is an image, if you need to view or analyze it, you need to extract the imageId", imgId),
				}
				imgId++
			} else if itemType == "text" {
				text, _ := itemMap["text"].(string)
				// 移除旧的图片标记
				text = removeImageMarkers(text)
				itemMap["text"] = text
			} else if itemType == "tool_result" {
				// 处理 tool_result 中的图片
				if toolContent, ok := itemMap["content"].([]interface{}); ok {
					for j, tc := range toolContent {
						tcMap, ok := tc.(map[string]interface{})
						if !ok {
							continue
						}
						if tcMap["type"] == "image" {
							source, _ := tcMap["source"].(map[string]interface{})
							cacheId := fmt.Sprintf("%s_Image#%d", sessionId, imgId)
							globalImageCache.StoreImage(cacheId, source)

							toolContent[j] = map[string]interface{}{
								"type": "text",
								"text": fmt.Sprintf("[Image #%d]This is an image, if you need to view or analyze it, you need to extract the imageId", imgId),
							}
							imgId++
						}
					}
					itemMap["content"] = toolContent
				}
			}
		}
		msgMap["content"] = content
	}
}

// appendTools 添加工具
func (a *ImageAgentV2) appendTools() {
	// analyzeImage 工具
	a.tools["analyzeImage"] = Tool{
		Name:        "analyzeImage",
		Description: "Analyse image or images by ID and extract information such as OCR text, objects, layout, colors, or safety signals.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"imageId": map[string]interface{}{
					"type":        "array",
					"description": "an array of IDs to analyse",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
				"task": map[string]interface{}{
					"type":        "string",
					"description": "Details of task to perform on the image.The more detailed, the better",
				},
				"regions": map[string]interface{}{
					"type":        "array",
					"description": "Optional regions of interest within the image",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]interface{}{
								"type":        "string",
								"description": "Optional label for the region",
							},
							"x": map[string]interface{}{
								"type":        "number",
								"description": "X coordinate",
							},
							"y": map[string]interface{}{
								"type":        "number",
								"description": "Y coordinate",
							},
							"w": map[string]interface{}{
								"type":        "number",
								"description": "Width of the region",
							},
							"h": map[string]interface{}{
								"type":        "number",
								"description": "Height of the region",
							},
							"units": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"px", "pct"},
								"description": "Units for coordinates and size",
							},
						},
						"required": []string{"x", "y", "w", "h", "units"},
					},
				},
			},
			"required": []string{"imageId", "task"},
		},
	}

	// image_generation 工具
	a.tools["image_generation"] = Tool{
		Name:        "image_generation",
		Description: "Generate images from text descriptions",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Text description of the image to generate",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Model to use for image generation",
				},
				"n": map[string]interface{}{
					"type":        "integer",
					"description": "Number of images to generate",
				},
				"size": map[string]interface{}{
					"type":        "string",
					"description": "Size of the image (e.g., 1024x1024)",
				},
			},
			"required": []string{"prompt"},
		},
	}
}

// handleAnalyzeImage 处理图片分析
func (a *ImageAgentV2) handleAnalyzeImage(input map[string]interface{}, reqBody map[string]interface{}, cfg *config.Config, sessionId string) (string, error) {
	router := cfg.GetRouter()
	if router == nil || router.Image == "" {
		return "", fmt.Errorf("image router not configured")
	}

	// 获取 imageId 列表
	imageIdList := make([]string, 0)
	if imageId, ok := input["imageId"].([]interface{}); ok {
		for _, id := range imageId {
			if idStr, ok := id.(string); ok {
				imageIdList = append(imageIdList, idStr)
			}
		}
	} else if imageId, ok := input["imageId"].(string); ok {
		imageIdList = append(imageIdList, imageId)
	}

	// 构建图片消息
	imageMessages := make([]map[string]interface{}, 0)
	for _, imgId := range imageIdList {
		cacheId := fmt.Sprintf("%s_Image#%s", sessionId, imgId)
		source := globalImageCache.GetImage(cacheId)
		if source != nil {
			imageMessages = append(imageMessages, map[string]interface{}{
				"type":   "image",
				"source": source,
			})
		}
	}

	// 获取任务描述
	task, _ := input["task"].(string)
	if task != "" {
		imageMessages = append(imageMessages, map[string]interface{}{
			"type": "text",
			"text": task,
		})
	}

	// 获取 regions 信息
	if regions, ok := input["regions"].([]interface{}); ok && len(regions) > 0 {
		regionsJSON, _ := json.Marshal(regions)
		imageMessages = append(imageMessages, map[string]interface{}{
			"type": "text",
			"text": "Regions: " + string(regionsJSON),
		})
	}

	// 调用分析模型
	port := cfg.Get("PORT")
	if port == "" {
		port = "3456"
	}
	apiKey := cfg.Get("APIKEY")

	requestBody := map[string]interface{}{
		"model": router.Image,
		"system": []map[string]interface{}{
			{
				"type": "text",
				"text": `You must interpret and analyze images strictly according to the assigned task.  
When an image placeholder is provided, your role is to parse the image content only within the scope of the user's instructions.  
Do not ignore or deviate from the task.  
Always ensure that your response reflects a clear, accurate interpretation of the image aligned with the given objective.`,
			},
		},
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": imageMessages,
			},
		},
		"stream": false,
	}

	reqBodyBytes, _ := json.Marshal(requestBody)
	url := fmt.Sprintf("http://127.0.0.1:%s/v1/messages", port)

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "analyzeImage Error", nil
	}

	if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
		if firstContent, ok := content[0].(map[string]interface{}); ok {
			if text, ok := firstContent["text"].(string); ok {
				return text, nil
			}
		}
	}

	return "analyzeImage Error", nil
}

// handleImageGeneration 处理图片生成
func (a *ImageAgentV2) handleImageGeneration(input map[string]interface{}, cfg *config.Config) (string, error) {
	prompt, ok := input["prompt"].(string)
	if !ok || prompt == "" {
		return "", fmt.Errorf("missing prompt")
	}

	// 获取 provider 配置
	providers := cfg.GetProviders()
	if len(providers) == 0 {
		return "", fmt.Errorf("no providers configured")
	}

	provider := &providers[0]
	for _, p := range providers {
		if strings.Contains(strings.ToLower(p.Name), "openai") {
			provider = &p
			break
		}
	}

	apiKey := provider.APIKey
	host := provider.Host
	if host == "" {
		host = "https://api.openai.com/v1/images/generations"
	}

	reqData := map[string]interface{}{
		"prompt": prompt,
		"model":  "dall-e-3",
		"n":      1,
		"size":   "1024x1024",
	}

	if model, ok := input["model"].(string); ok && model != "" {
		reqData["model"] = model
	}
	if n, ok := input["n"].(float64); ok {
		reqData["n"] = int(n)
	}
	if size, ok := input["size"].(string); ok && size != "" {
		reqData["size"] = size
	}

	reqBodyBytes, _ := json.Marshal(reqData)
	req, err := http.NewRequest("POST", host, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return string(body), nil
	}

	if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
		if img, ok := data[0].(map[string]interface{}); ok {
			if url, ok := img["url"].(string); ok {
				return url, nil
			}
			if b64, ok := img["b64_json"].(string); ok {
				return "data:image/png;base64," + b64, nil
			}
		}
	}

	return string(body), nil
}

// removeImageMarkers 移除图片标记
func removeImageMarkers(text string) string {
	// 移除 [Image #N] 标记
	for {
		start := strings.Index(text, "[Image #")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], "]")
		if end == -1 {
			break
		}
		text = text[:start] + text[start+end+1:]
	}
	return text
}

// GenerateSessionId 生成会话 ID
func GenerateSessionId() string {
	timestamp := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", timestamp)))
	return hex.EncodeToString(hash[:])[:16]
}
