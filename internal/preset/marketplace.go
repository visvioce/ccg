package preset

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const marketURL = "https://pub-0dc3e1677e894f07bbea11b17a29e032.r2.dev/presets.json"

// MarketPreset represents a preset entry in the marketplace
type MarketPreset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Author      string   `json:"author,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	Stars       int      `json:"stars,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	URL         string   `json:"url,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	Checksum    string   `json:"checksum,omitempty"`
}

// GetMarketPresets fetches preset market data from remote
func GetMarketPresets() ([]MarketPreset, error) {
	resp, err := http.Get(marketURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch market: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("market returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var presets []MarketPreset
	if err := json.Unmarshal(body, &presets); err != nil {
		return nil, fmt.Errorf("failed to parse market data: %w", err)
	}

	return presets, nil
}

// FindMarketPresetByName finds a preset in the market by name
func FindMarketPresetByName(name string) (*MarketPreset, error) {
	presets, err := GetMarketPresets()
	if err != nil {
		return nil, err
	}

	// Exact match by ID
	for _, p := range presets {
		if p.ID == name {
			return &p, nil
		}
	}

	// Exact match by name
	for _, p := range presets {
		if p.Name == name {
			return &p, nil
		}
	}

	// Case-insensitive match
	lowerName := toLower(name)
	for _, p := range presets {
		if toLower(p.Name) == lowerName || toLower(p.ID) == lowerName {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("preset not found in marketplace: %s", name)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}
