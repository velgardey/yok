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
	Use:     "yok",
	Short:   "Yok CLI - Git Wrapper and Deployment Tool",
	Long:    "Yok CLI is a git wrapper and a deployment tool that allows you to deploy your static web applications directly from your git repository.",
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Customize version template
	RootCmd.SetVersionTemplate("Yok CLI v{{.Version}}\n")

	// Add git command support
	addGitCommands()

	// Set up special handling for unknown commands to pass them to git
	RootCmd.SetFlagErrorFunc(handleUnknownCommand)

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// handleUnknownCommand handles unknown commands by trying to pass them to git
func handleUnknownCommand(cmd *cobra.Command, err error) error {
	// Check if the command is a git command that we don't explicitly handle
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		if output, cmdErr := git.ExecuteCommand(os.Args[1:]...); cmdErr == nil {
			fmt.Print(output)
			os.Exit(0)
		}
	}
	return err
}

func init() {
	// Git commands will be added in Execute() function to avoid initialization issues
}

// addGitCommands adds all common git commands as explicit subcommands
func addGitCommands() {
	// List of common git commands to support
	gitCommands := []string{
		"add", "commit", "push", "pull", "checkout", "branch", "status",
		"log", "fetch", "merge", "rebase", "reset", "tag", "stash",
	}

	// Add each git command as a subcommand
	for _, gitCmd := range gitCommands {
		RootCmd.AddCommand(createGitCommand(gitCmd))
	}

	// Add a fallback command handler for all other git commands
	RootCmd.AddCommand(createGitFallbackCommand())
}

// createGitCommand creates a cobra command for a specific git command
func createGitCommand(gitCmd string) *cobra.Command {
	return &cobra.Command{
		Use:   gitCmd,
		Short: fmt.Sprintf("Execute git %s", gitCmd),
		Run: func(cmd *cobra.Command, args []string) {
			executeGitCommand(append([]string{gitCmd}, args...))
		},
		DisableFlagParsing: true,
	}
}

// createGitFallbackCommand creates a fallback command for other git commands
func createGitFallbackCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "git",
		Short: "Execute any other git command",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			executeGitCommand(args)
		},
		DisableFlagParsing: true,
	}
}

// executeGitCommand executes a git command and handles errors
func executeGitCommand(args []string) {
	output, err := git.ExecuteCommand(args...)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Print(output)
}
