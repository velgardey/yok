package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/velgardey/yok/cli/internal/types"
	"github.com/velgardey/yok/cli/internal/utils"
)

// SaveConfig saves the configuration to a local file
func SaveConfig(config types.Config) error {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(utils.ConfigFile, jsonData, 0644)
}

// LoadConfig loads configuration from a local file
func LoadConfig() (types.Config, error) {
	var config types.Config
	data, err := os.ReadFile(utils.ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return empty config if file doesn't exist
		}
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
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
		return err
	}

	configFilePath := filepath.Join(cwd, utils.ConfigFile)
	return os.RemoveAll(configFilePath)
}
