package config

import (
	"encoding/json"
	"os"
)

// SetProviders sets the providers in the config
func (c *Config) SetProviders(providers []Provider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers = providers
	c.data["providers"] = providers
}

// SetRouter sets the router configuration
func (c *Config) SetRouter(router *RouterConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.router = router
	c.data["router"] = router
}

// ToJSON converts the config to JSON bytes
func (c *Config) ToJSON() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Create a copy of the data for serialization
	data := make(map[string]any)
	
	// Copy providers
	if c.providers != nil {
		data["providers"] = c.providers
	}
	
	// Copy router
	if c.router != nil {
		data["router"] = c.router
	}
	
	// Copy other config values
	for k, v := range c.data {
		if k != "providers" && k != "router" {
			data[k] = v
		}
	}
	
	return json.MarshalIndent(data, "", "  ")
}

// WriteConfig writes config data to a file
func WriteConfig(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// Set sets a config value
func (c *Config) Set(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data[key] = value
}
