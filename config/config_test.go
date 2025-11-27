package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// **Feature: mr-conflict-checker, Property 1: Configuration Parsing Completeness**
// **Validates: Requirements 1.1, 3.2**
func TestProperty_ConfigurationParsingCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("valid YAML configuration should parse successfully", prop.ForAll(
		func(token, url string) bool {
			// Create a temporary YAML file with valid configuration
			yamlContent := "gitlab:\n  token: " + token + "\n  url: " + url + "\n"

			// Create temporary file
			tmpFile, err := os.CreateTemp("", "config_test_*.yaml")
			if err != nil {
				return false
			}
			defer os.Remove(tmpFile.Name())

			// Write YAML content
			if _, err := tmpFile.WriteString(yamlContent); err != nil {
				tmpFile.Close()
				return false
			}
			tmpFile.Close()

			// Test parsing
			config, err := LoadConfig(tmpFile.Name())
			if err != nil {
				return false
			}

			// Verify parsed values match input
			return config.GitLab.Token == token && config.GitLab.URL == url
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // Non-empty token
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }), // Non-empty URL
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// **Feature: mr-conflict-checker, Property 4: Error Resilience**
// **Validates: Requirements 1.4, 1.5, 3.4, 3.5**
func TestProperty_ErrorResilience(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("system should handle configuration errors gracefully", prop.ForAll(
		func(errorType int) bool {
			switch errorType % 4 {
			case 0: // Test file not found error
				_, err := LoadConfig("/nonexistent/path/config.yaml")
				return err != nil &&
					(containsString(err.Error(), "configuration file not found") ||
						containsString(err.Error(), "no such file or directory"))

			case 1: // Test invalid YAML syntax error
				tmpFile, err := os.CreateTemp("", "invalid_config_*.yaml")
				if err != nil {
					return false
				}
				defer os.Remove(tmpFile.Name())

				// Write invalid YAML
				invalidYAML := "gitlab:\n  token: test\n  url: [unclosed"
				if _, err := tmpFile.WriteString(invalidYAML); err != nil {
					tmpFile.Close()
					return false
				}
				tmpFile.Close()

				_, err = LoadConfig(tmpFile.Name())
				return err != nil && containsString(err.Error(), "failed to parse YAML configuration")

			case 2: // Test missing required token field
				tmpFile, err := os.CreateTemp("", "missing_token_*.yaml")
				if err != nil {
					return false
				}
				defer os.Remove(tmpFile.Name())

				yamlContent := "gitlab:\n  url: https://gitlab.com\n"
				if _, err := tmpFile.WriteString(yamlContent); err != nil {
					tmpFile.Close()
					return false
				}
				tmpFile.Close()

				_, err = LoadConfig(tmpFile.Name())
				return err != nil && containsString(err.Error(), "gitlab.token is required")

			case 3: // Test missing required URL field
				tmpFile, err := os.CreateTemp("", "missing_url_*.yaml")
				if err != nil {
					return false
				}
				defer os.Remove(tmpFile.Name())

				yamlContent := "gitlab:\n  token: test-token\n"
				if _, err := tmpFile.WriteString(yamlContent); err != nil {
					tmpFile.Close()
					return false
				}
				tmpFile.Close()

				_, err = LoadConfig(tmpFile.Name())
				return err != nil && containsString(err.Error(), "gitlab.url is required")

			default:
				return false
			}
		},
		gen.IntRange(0, 100), // Generate different error scenarios
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Helper function to check if a string contains a substring (case-insensitive)
func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr))))
}

// Simple substring search helper
func findSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Unit test for basic functionality
func TestLoadConfig_ValidFile(t *testing.T) {
	// Create a temporary valid config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `gitlab:
  token: test-token-123
  url: https://gitlab.example.com
`

	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Test loading
	config, err := LoadConfig(configFile)
	require.NoError(t, err)
	assert.Equal(t, "test-token-123", config.GitLab.Token)
	assert.Equal(t, "https://gitlab.example.com", config.GitLab.URL)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration file not found")
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Invalid YAML content
	invalidYAML := `gitlab:
  token: test-token
  url: https://gitlab.example.com
  invalid: [unclosed bracket
`

	err := os.WriteFile(configFile, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML configuration")
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				GitLab: struct {
					Token         string `yaml:"token"`
					URL           string `yaml:"url"`
					IncludeGroups []int  `yaml:"include_groups,omitempty"`
				}{
					Token: "valid-token",
					URL:   "https://gitlab.com",
				},
			},
			wantErr: false,
		},
		{
			name: "missing token",
			config: Config{
				GitLab: struct {
					Token         string `yaml:"token"`
					URL           string `yaml:"url"`
					IncludeGroups []int  `yaml:"include_groups,omitempty"`
				}{
					Token: "",
					URL:   "https://gitlab.com",
				},
			},
			wantErr: true,
			errMsg:  "gitlab.token is required",
		},
		{
			name: "missing URL",
			config: Config{
				GitLab: struct {
					Token         string `yaml:"token"`
					URL           string `yaml:"url"`
					IncludeGroups []int  `yaml:"include_groups,omitempty"`
				}{
					Token: "valid-token",
					URL:   "",
				},
			},
			wantErr: true,
			errMsg:  "gitlab.url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
