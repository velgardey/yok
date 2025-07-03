package cmd

import (
	"fmt"

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
		Run:   runDeploy,
	}

	// Ship command - combines git commit, push, and deploy
	var shipCmd = &cobra.Command{
		Use:   "ship",
		Short: "Commit, push, and deploy your project to the web using Yok",
		Run:   runShip,
	}

	// Add commands to root
	RootCmd.AddCommand(deployCmd, shipCmd)
}

// runDeploy handles the deploy command logic
func runDeploy(cmd *cobra.Command, args []string) {
	// Get project configuration
	config, err := EnsureProjectID()
	utils.HandleError(err, "Error setting up project")

	// Check repository sync status
	if err := checkRepositorySync(); err != nil {
		utils.WarnColor.Printf("Warning: %v\n", err)
		if !confirmContinueDeployment() {
			utils.ErrorColor.Println("Deployment cancelled")
			return
		}
	}

	// Deploy the project
	deployment, err := api.DeployProject(config.ProjectID)
	utils.HandleError(err, "Error deploying project")

	utils.SuccessColor.Printf("[OK] Deployment triggered: %s\n", deployment.Data.DeploymentId)

	// Follow deployment status
	api.FollowDeploymentStatus(deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
}

// checkRepositorySync checks if the local repository is in sync with remote
func checkRepositorySync() error {
	utils.InfoColor.Print("Checking local/remote sync... ")

	_, err := git.CheckLocalRemoteSync()
	if err != nil {
		utils.SuccessColor.Println()

		// Try to handle uncommitted changes
		if handleErr := git.HandleUncommittedChanges(); handleErr != nil {
			return handleErr
		}

		return err
	}

	utils.SuccessColor.Println("Done")
	return nil
}

// confirmContinueDeployment asks user if they want to continue with deployment
func confirmContinueDeployment() bool {
	opts := utils.GetSurveyOptions()

	var continueDeploy bool
	prompt := &survey.Confirm{
		Message: "Do you want to continue with deployment anyway?",
		Default: false,
	}

	if err := survey.AskOne(prompt, &continueDeploy, opts); err != nil {
		return false
	}

	return continueDeploy
}

// runShip handles the ship command logic (commit, push, and deploy)
func runShip(cmd *cobra.Command, args []string) {
	// Get commit message
	commitMessage, err := getShipCommitMessage()
	if err != nil {
		utils.ErrorColor.Printf("Error: %v\n", err)
		return
	}

	// Perform git operations using the centralized function
	if err := git.CommitAndPushChanges(commitMessage); err != nil {
		utils.HandleError(err, "Git operations failed")
	}

	// Get project configuration and deploy
	config, err := EnsureProjectID()
	utils.HandleError(err, "Error setting up project")

	// Deploy the project
	deployment, err := api.DeployProject(config.ProjectID)
	utils.HandleError(err, "Error deploying project")

	utils.SuccessColor.Printf("[OK] Deployment triggered: %s\n", deployment.Data.DeploymentId)

	// Follow deployment status
	api.FollowDeploymentStatus(deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
}

// getShipCommitMessage prompts user for commit message
func getShipCommitMessage() (string, error) {
	opts := utils.GetSurveyOptions()

	var commitMessage string
	prompt := &survey.Input{
		Message: "Enter a commit message:",
	}

	if err := survey.AskOne(prompt, &commitMessage, opts); err != nil {
		return "", err
	}

	if commitMessage == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	return commitMessage, nil
}
