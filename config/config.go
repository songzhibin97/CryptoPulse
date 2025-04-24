package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds application configuration
type Config struct {
	AIEndpoint  string `yaml:"ai_endpoint"`
	ExtEndpoint string `yaml:"ext_endpoint"`
	Port        string `yaml:"port"`
	ProxyURL    string `yaml:"proxy_url"`
	WSProxyURL  string `yaml:"ws_proxy_url"` // New field for WebSocket proxy
}

// LoadConfig reads configuration from config.yaml
func LoadConfig() (Config, error) {
	var cfg Config
	data, err := os.ReadFile("config/config.yaml")
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
