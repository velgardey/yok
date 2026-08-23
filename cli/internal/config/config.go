package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/velgardey/yok/cli/internal/types"
)

// configFile is the path of the local configuration file, relative to the
// working directory. It is a var so tests can point it at a temp location.
var configFile = ".yok-config.json"

// SaveConfig saves the configuration to a local file
func SaveConfig(config types.Config) error {
	jsonData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Tighten permissions on pre-existing files created with older versions.
	if err := os.Chmod(configFile, 0600); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

// LoadConfig loads configuration from a local file
func LoadConfig() (types.Config, error) {
	var config types.Config

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return empty config if file doesn't exist
		}
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	config.ProjectID = strings.TrimSpace(config.ProjectID)
	config.RepoName = strings.TrimSpace(config.RepoName)
	config.APIURL = strings.TrimSpace(config.APIURL)
	config.Token = strings.TrimSpace(config.Token)
	config.SiteDomain = strings.TrimSpace(config.SiteDomain)

	return config, nil
}

// RemoveConfig deletes the configuration file
func RemoveConfig() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	configFilePath := filepath.Join(cwd, configFile)
	if err := os.RemoveAll(configFilePath); err != nil {
		return fmt.Errorf("failed to remove config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the full path to the configuration file
func GetConfigPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	return filepath.Join(cwd, configFile), nil
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
