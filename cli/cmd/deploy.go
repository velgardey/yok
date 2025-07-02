package cmd

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/api"
	"github.com/velgardey/yok/cli/internal/git"
	"github.com/velgardey/yok/cli/internal/utils"
)

func init() {
	// Deploy command
	var deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy your project to the web using Yok",
		Run: func(cmd *cobra.Command, args []string) {
			// Get project ID
			config, err := EnsureProjectID()
			utils.HandleError(err, "Error setting up project")

			// Check if local branch is in sync with remote
			utils.InfoColor.Print("Checking local/remote sync... ")
			_, err = git.CheckLocalRemoteSync()
			if err != nil {
				utils.SuccessColor.Println()
				utils.WarnColor.Printf("Warning: %v\n", err)

				// Check for uncommitted changes and handle them
				err = git.HandleUncommittedChanges()
				if err != nil {
					utils.WarnColor.Printf("Warning: %v\n", err)

					// Ask user if they want to continue anyway
					continueDeploy := false
					syncPrompt := &survey.Confirm{
						Message: "Do you want to continue with deployment anyway?",
						Default: false,
					}
					survey.AskOne(syncPrompt, &continueDeploy)

					if !continueDeploy {
						utils.ErrorColor.Println("Deployment cancelled")
						return
					}
				}
			} else {
				utils.SuccessColor.Println("Done")
			}

			// Deploy project with stored ID
			deployment, err := api.DeployProject(config.ProjectID)
			utils.HandleError(err, "Error deploying project")

			utils.SuccessColor.Printf("✅ Deployment triggered: %s\n", deployment.Data.DeploymentId)

			// Automatically follow the deployment status
			api.FollowDeploymentStatus(deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
		},
	}

	// Ship command - combines git commit, push, and deploy
	var shipCmd = &cobra.Command{
		Use:   "ship",
		Short: "Commit, push, and deploy your project to the web using Yok",
		Run: func(cmd *cobra.Command, args []string) {
			// Get commit message
			var commitMessage string
			msgPrompt := &survey.Input{
				Message: "Enter a commit message:",
			}
			err := survey.AskOne(msgPrompt, &commitMessage)
			utils.HandleError(err, "Error getting commit message")

			if commitMessage == "" {
				utils.ErrorColor.Println("Error: Commit message cannot be empty")
				return
			}

			// Git add
			utils.InfoColor.Print("📝 Adding changes... ")
			_, err = git.ExecuteCommand("add", ".")
			if err != nil {
				utils.SuccessColor.Println()
				utils.HandleError(err, "Error adding files")
			}
			utils.SuccessColor.Println("Done")

			// Git commit
			utils.InfoColor.Print("💾 Committing changes... ")
			_, err = git.ExecuteCommand("commit", "-m", commitMessage)
			if err != nil {
				utils.SuccessColor.Println()
				utils.HandleError(err, "Error committing changes")
			}
			utils.SuccessColor.Println("Done")

			// Git push
			utils.InfoColor.Print("🚀 Pushing to remote... ")
			_, err = git.ExecuteCommand("push")
			if err != nil {
				utils.SuccessColor.Println()
				utils.HandleError(err, "Error pushing changes")
			}
			utils.SuccessColor.Println("Done")

			// Get project ID and deploy
			config, err := EnsureProjectID()
			utils.HandleError(err, "Error setting up project")

			// Deploy project with stored ID
			deployment, err := api.DeployProject(config.ProjectID)
			utils.HandleError(err, "Error deploying project")

			utils.SuccessColor.Printf("✅ Deployment triggered: %s\n", deployment.Data.DeploymentId)

			// Automatically follow the deployment status
			api.FollowDeploymentStatus(deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
		},
	}

	// Add commands to root
	RootCmd.AddCommand(deployCmd, shipCmd)
}
