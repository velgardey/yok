package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/git"
)

var version = "dev" // Will be injected at build time by GoReleaser

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "yok",
	Short: "Yok CLI - Git Wrapper and Deployment Tool",
	Long:  "Yok CLI is a git wrapper and a deployment tool that allows you to deploy your static web applications directly from your git repository.",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Set up special handling for unknown commands to pass them to git
	RootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		// Check if the command is a git command that we don't explicitly handle
		if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
			output, cmdErr := git.ExecuteCommand(os.Args[1:]...)
			if cmdErr == nil {
				fmt.Print(output)
				os.Exit(0)
			}
		}
		return err
	})

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Add git commands as passthrough
	addGitCommands()
}

// Add all common git commands as explicit subcommands
func addGitCommands() {
	// List of common git commands
	gitCommands := []string{
		"add", "commit", "push", "pull", "checkout", "branch", "status",
		"log", "fetch", "merge", "rebase", "reset", "tag", "stash",
	}

	for _, gitCmd := range gitCommands {
		cmd := &cobra.Command{
			Use:   gitCmd,
			Short: fmt.Sprintf("Execute git %s", gitCmd),
			Run: func(gitCmd string) func(cmd *cobra.Command, args []string) {
				return func(cmd *cobra.Command, args []string) {
					allArgs := append([]string{gitCmd}, args...)
					output, err := git.ExecuteCommand(allArgs...)
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					fmt.Print(output)
				}
			}(gitCmd),
			DisableFlagParsing: true,
		}
		RootCmd.AddCommand(cmd)
	}

	// Add a fallback command handler for all other git commands
	var fallbackCmd = &cobra.Command{
		Use:   "git",
		Short: "Execute any other git command",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			output, err := git.ExecuteCommand(args...)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			fmt.Print(output)
		},
		DisableFlagParsing: true,
	}
	RootCmd.AddCommand(fallbackCmd)
}
