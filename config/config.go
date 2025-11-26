package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration structure
type Config struct {
	GitLab struct {
		Token         string `yaml:"token"`
		URL           string `yaml:"url"`
		IncludeGroups []int  `yaml:"include_groups,omitempty"`
	} `yaml:"gitlab"`
}

// LoadConfig reads and parses the YAML configuration file
func LoadConfig(filePath string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", filePath)
	}

	// Read file contents
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML configuration: %w", err)
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// Validate checks that all required configuration fields are present
func (c *Config) Validate() error {
	if c.GitLab.Token == "" {
		return fmt.Errorf("gitlab.token is required")
	}
	if c.GitLab.URL == "" {
		return fmt.Errorf("gitlab.url is required")
	}
	return nil
}
