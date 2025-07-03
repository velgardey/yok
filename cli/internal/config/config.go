package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/velgardey/yok/cli/internal/types"
	"github.com/velgardey/yok/cli/internal/utils"
)

// SaveConfig saves the configuration to a local file
func SaveConfig(config types.Config) error {
	// Validate configuration before saving
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(utils.ConfigFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfig loads configuration from a local file
func LoadConfig() (types.Config, error) {
	var config types.Config

	data, err := os.ReadFile(utils.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return empty config if file doesn't exist
		}
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// GetProjectIDOrExit loads the config and exits if no project ID is found
func GetProjectIDOrExit() types.Config {
	config, err := LoadConfig()
	utils.HandleError(err, "Error loading configuration")

	if config.ProjectID == "" {
		utils.ErrorColor.Println("No project configured. Run 'yok create' or 'yok deploy' first.")
		os.Exit(1)
	}

	return config
}

// RemoveConfig deletes the configuration file
func RemoveConfig() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	configFilePath := filepath.Join(cwd, utils.ConfigFile)
	if err := os.RemoveAll(configFilePath); err != nil {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	return nil
}

// ValidateConfig validates the configuration data
func ValidateConfig(config types.Config) error {
	if strings.TrimSpace(config.ProjectID) == "" {
		return fmt.Errorf("project ID cannot be empty")
	}

	if strings.TrimSpace(config.RepoName) == "" {
		return fmt.Errorf("repository name cannot be empty")
	}

	return nil
}

// GetConfigPath returns the full path to the configuration file
func GetConfigPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	return filepath.Join(cwd, utils.ConfigFile), nil
}

// ConfigExists checks if a configuration file exists
func ConfigExists() bool {
	configPath, err := GetConfigPath()
	if err != nil {
		return false
	}

	_, err = os.Stat(configPath)
	return err == nil
}
