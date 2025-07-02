package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of Yok CLI",
	Long:  `Display the current version of Yok CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Yok CLI v%s\n", version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
