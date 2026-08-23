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
	// ArbitraryArgs lets unrecognized commands fall through to the root Run,
	// which forwards them to git - so every git verb works as `yok <verb>`.
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Print(cmd.UsageString())
			return
		}
		executeGitCommand(args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	RootCmd.SetVersionTemplate("Yok CLI v{{.Version}}\n")
	addGitCommands()

	// Register --version without a shorthand so cobra does not claim `-v`,
	// which git commands like `yok remote -v` need.
	RootCmd.Flags().Bool("version", false, "version for yok")

	// Unknown git verbs that also pass flags (e.g. `yok remote -v`) fail root
	// flag parsing before they reach Run; forward those straight to git.
	RootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
			if output, gitErr := git.ExecuteCommand(os.Args[1:]...); gitErr == nil {
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

// addGitCommands adds explicit subcommands for common git commands so they
// appear in help output; everything else falls through to the root command.
func addGitCommands() {
	gitCommands := []string{
		"add", "commit", "push", "pull", "checkout", "branch", "status",
		"log", "fetch", "merge", "rebase", "reset", "tag", "stash",
	}
	for _, gitCmd := range gitCommands {
		command := gitCmd
		RootCmd.AddCommand(&cobra.Command{
			Use:                command,
			Short:              fmt.Sprintf("Execute git %s", command),
			Run:                func(cmd *cobra.Command, args []string) { executeGitCommand(append([]string{command}, args...)) },
			DisableFlagParsing: true,
		})
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
