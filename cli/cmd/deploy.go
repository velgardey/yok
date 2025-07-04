package cmd

import (
	"fmt"
	"os"
	"os/signal"

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

	// Add flags to the deploy command
	deployCmd.Flags().BoolP("logs", "l", false, "Follow deployment logs")
	deployCmd.Flags().BoolP("no-sync-check", "n", false, "Skip repository sync check")

	// Ship command - combines git commit, push, and deploy
	var shipCmd = &cobra.Command{
		Use:   "ship",
		Short: "Commit, push, and deploy your project to the web using Yok",
		Run:   runShip,
	}

	// Add flags to the ship command
	shipCmd.Flags().BoolP("logs", "l", false, "Follow deployment logs")

	// Add commands to root
	RootCmd.AddCommand(deployCmd, shipCmd)
}

// runDeploy handles the deploy command logic
func runDeploy(cmd *cobra.Command, args []string) {
	// Get flags
	followLogs, _ := cmd.Flags().GetBool("logs")
	skipSyncCheck, _ := cmd.Flags().GetBool("no-sync-check")

	// Get project configuration
	config, err := EnsureProjectID()
	utils.HandleError(err, "Error setting up project")

	// Check repository sync status
	if !skipSyncCheck {
		if err := checkRepositorySync(); err != nil {
			utils.WarnColor.Printf("Warning: %v\n", err)
			if !confirmContinueDeployment() {
				utils.ErrorColor.Println("Deployment cancelled")
				return
			}
		}
	}

	// Deploy the project
	deployment, err := api.DeployProject(config.ProjectID)
	utils.HandleError(err, "Error deploying project")

	utils.SuccessColor.Printf("[OK] Deployment triggered: %s\n", deployment.Data.DeploymentId)

	// Ask if user wants to follow logs if not explicitly specified
	if !cmd.Flags().Changed("logs") {
		utils.InfoColor.Println("Would you like to follow deployment logs?")
		followLogs = confirmFollowLogs()
	}

	// Handle deployment follow-up based on flags
	handleDeploymentFollowUp(followLogs, deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
}

// runShip handles the ship command logic (commit, push, and deploy)
func runShip(cmd *cobra.Command, args []string) {
	// Get flags
	followLogs, _ := cmd.Flags().GetBool("logs")

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

	// Ask if user wants to follow logs if not explicitly specified
	if !cmd.Flags().Changed("logs") {
		utils.InfoColor.Println("Would you like to follow deployment logs?")
		followLogs = confirmFollowLogs()
	}

	// Handle deployment follow-up based on flags
	handleDeploymentFollowUp(followLogs, deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
}

// handleDeploymentFollowUp handles the post-deployment logic (following logs or status)
func handleDeploymentFollowUp(followLogs bool, deploymentID string, deploymentURL string, projectID string) {
	if followLogs {
		// Follow logs
		utils.InfoColor.Println("Following deployment logs (Press Ctrl+C to stop)...")

		// Create a channel for stopping log streaming
		stopChan := make(chan bool)

		// Set up a signal handler for Ctrl+C
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)

		// Start a goroutine to handle Ctrl+C
		go func() {
			<-signalChan
			stopChan <- true
		}()

		// Stream logs and get completion status
		deploymentSucceeded := api.StreamDeploymentLogs(deploymentID, stopChan)

		// Show URLs and exit with appropriate code based on completion status
		if deploymentSucceeded {
			showDeploymentUrls(projectID, deploymentID, deploymentURL)
			os.Exit(0)
		} else {
			// Check if deployment actually failed or was just interrupted
			status, err := api.GetDeploymentStatus(deploymentID)
			if err == nil && status.Status == "FAILED" {
				utils.ErrorColor.Println("Deployment failed. Check the logs above for detailed error messages.")
				os.Exit(1)
			}
		}
	} else {
		// Just follow deployment status
		api.FollowDeploymentStatus(deploymentID, deploymentURL, projectID)

		// Check final status to determine exit code
		finalStatus, err := api.GetDeploymentStatus(deploymentID)
		if err == nil && finalStatus.Status == "FAILED" {
			os.Exit(1)
		}
	}
}

// showDeploymentUrls displays the URLs where the deployed site is available
func showDeploymentUrls(projectID string, deploymentID string, deploymentURL string) {
	utils.InfoColor.Printf("[i] Your site is available at:\n")

	// Try to get the project slug for a nicer URL
	project, err := api.GetProject(projectID)
	if err == nil && project.Slug != "" {
		fmt.Printf("- https://%s.yok.ninja\n", project.Slug)
	}

	// Always try to show a deployment-specific URL
	if deploymentURL != "" {
		fmt.Printf("- %s\n", deploymentURL)
	} else {
		// If we don't have the deploymentURL, fetch it from the API
		deployment, err := api.GetDeploymentStatus(deploymentID)
		if err == nil && deployment.DeploymentUrl != "" {
			fmt.Printf("- %s\n", deployment.DeploymentUrl)
		} else {
			// Construct the URL if we couldn't get it from the API
			fmt.Printf("- https://%s.yok.ninja\n", deploymentID)
		}
	}
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

// confirmFollowLogs asks user if they want to follow deployment logs
func confirmFollowLogs() bool {
	opts := utils.GetSurveyOptions()

	var followLogs bool
	prompt := &survey.Confirm{
		Message: "Do you want to follow deployment logs?",
		Default: true,
	}

	if err := survey.AskOne(prompt, &followLogs, opts); err != nil {
		return false
	}

	return followLogs
}
