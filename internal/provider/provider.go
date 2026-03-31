package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/musistudio/ccg/internal/cache"
	"github.com/musistudio/ccg/internal/config"
	"github.com/musistudio/ccg/internal/transformer"
)

type ProviderService struct {
	cfg        *config.Config
	registry   *transformer.TransformerRegistry
	httpClient *http.Client
}

func New(cfg *config.Config, registry *transformer.TransformerRegistry) *ProviderService {
	return &ProviderService{
		cfg:        cfg,
		registry:   registry,
		httpClient: &http.Client{},
	}
}

func (s *ProviderService) ForwardRequest(providerName, model string, reqBody []byte, sessionId string, stream bool) (*http.Response, error) {
	provider := s.cfg.GetProvider(providerName)
	if provider == nil {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	providerHost := provider.Host
	if providerHost == "" {
		providerHost = s.GetDefaultHost(providerName)
	}

	transforms := s.cfg.GetProviderTransform(providerName, model)

	var reqData map[string]any
	if err := json.Unmarshal(reqBody, &reqData); err != nil {
		return nil, fmt.Errorf("failed to parse request body: %w", err)
	}

	reqData["model"] = model
	transformedBody := s.registry.TransformRequest(reqData, providerName, transforms)

	processedBody, _ := json.Marshal(transformedBody)

	url := providerHost
	if !strings.HasSuffix(url, "/v1/messages") && !strings.HasSuffix(url, "/v1/chat/completions") {
		url = strings.TrimRight(url, "/")
		url += "/v1/messages"
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(processedBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if !stream && resp.StatusCode == http.StatusOK {
		var respBody map[string]any
		if body, err := io.ReadAll(resp.Body); err == nil {
			json.Unmarshal(body, &respBody)
			if usage, ok := respBody["usage"].(map[string]any); ok {
				inputTokens := 0
				outputTokens := 0
				if it, ok := usage["input_tokens"].(float64); ok {
					inputTokens = int(it)
				}
				if ot, ok := usage["output_tokens"].(float64); ok {
					outputTokens = int(ot)
				}
				if sessionId != "" {
					cache.SessionUsage.Put(sessionId, cache.Usage{
						InputTokens:  inputTokens,
						OutputTokens: outputTokens,
					})
				}
			}
		}
	}

	return resp, nil
}

func (s *ProviderService) GetDefaultHost(providerName string) string {
	hosts := map[string]string{
		"openai":     "https://api.openai.com/v1/chat/completions",
		"anthropic":  "https://api.anthropic.com/v1/messages",
		"deepseek":   "https://api.deepseek.com/v1/chat/completions",
		"google":     "https://generativelanguage.googleapis.com/v1beta/models",
		"groq":       "https://api.groq.com/openai/v1/chat/completions",
		"openrouter": "https://openrouter.ai/api/v1/chat/completions",
		"cerebras":   "https://api.cerebras.ai/v1/chat/completions",
		"anyscale":   "https://api.endpoints.anyscale.com/v1/chat/completions",
		"azure":      "https://{resource}.openai.azure.com/openai/deployments/{deployment}/chat/completions",
		"vertex":     "https://{region}-aiplatform.googleapis.com/v1/{project}/locations/{region}/publishers/anthropic/models/{model}:predict",
		"iflow":      "https://apis.iflow.cn/v1/chat/completions",
		"modelscope": "https://api-inference.modelscope.cn/v1/chat/completions",
		"NIM":        "https://integrate.api.nvidia.com/v1/chat/completions",
		"shusheng":   "https://chat.intern-ai.org.cn/api/v1/chat/completions",
		"gitcode":    "https://api-ai.gitcode.com/v1/chat/completions",
	}

	lower := strings.ToLower(providerName)
	if host, ok := hosts[lower]; ok {
		return host
	}
	return "https://api.openai.com/v1/chat/completions"
}

func (s *ProviderService) GetDefaultTransforms(providerName string) []string {
	transforms := map[string][]string{
		"anthropic":  {"anthropic->openai"},
		"deepseek":   {},
		"google":     {},
		"groq":       {},
		"openrouter": {},
		"cerebras":   {},
	}

	lower := strings.ToLower(providerName)
	if t, ok := transforms[lower]; ok {
		return t
	}
	return nil
}
