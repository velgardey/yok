package cmd

import (
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/config"
	"github.com/velgardey/yok/cli/internal/utils"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear your stored Yok credentials",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		utils.HandleError(err, "Failed to load config")

		cfg.Token = ""
		utils.HandleError(config.SaveConfig(cfg), "Failed to save config")
		utils.SuccessColor.Println("[OK] Logged out.")
	},
}

func init() {
	RootCmd.AddCommand(logoutCmd)
}
