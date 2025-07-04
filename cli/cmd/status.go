package cmd

import (
	"fmt"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/api"
	"github.com/velgardey/yok/cli/internal/config"
	"github.com/velgardey/yok/cli/internal/types"
	"github.com/velgardey/yok/cli/internal/utils"
)

func init() {
	// Status command
	var statusCmd = &cobra.Command{
		Use:   "status [deployment_id]",
		Short: "Check the status of your Yok deployments",
		Long:  "Check the status of your current or a specific deployment",
		Args:  cobra.MaximumNArgs(1),
		Run:   runStatus,
	}

	// Add flags to status command
	statusCmd.Flags().BoolP("all", "a", false, "Show all deployments, not just recent ones")
	statusCmd.Flags().BoolP("logs", "l", false, "Show logs for the selected deployment")

	// List command to list all deployments
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all deployments for your project",
		Run: func(cmd *cobra.Command, args []string) {
			// Get project ID and ensure it exists
			conf := config.GetProjectIDOrExit()

			// Get deployments
			s := utils.StartSpinner("Fetching deployments...")

			deployments, err := api.ListDeployments(conf.ProjectID)
			utils.StopSpinner(s)

			if err != nil {
				utils.ErrorColor.Printf("Failed to list deployments: %v\n", err)
				return
			}

			if len(deployments) == 0 {
				utils.InfoColor.Println("No deployments found for this project.")
				return
			}

			// Print deployments table
			fmt.Println("\nDeployments for", conf.RepoName)
			fmt.Println("------------------------------------------------------------------------------")
			fmt.Printf("%-36s %-12s %-20s\n", "ID", "STATUS", "CREATED")
			fmt.Println("------------------------------------------------------------------------------")

			for _, d := range deployments {
				utils.FormatTableRow(d.ID, d.Status, d.CreatedAt)
			}
		},
	}

	// Cancel command to cancel a deployment
	var cancelCmd = &cobra.Command{
		Use:   "cancel [deploymentId]",
		Short: "Cancel a running deployment",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var deploymentId string

			// If no deployment ID provided, ask the user to select from recent in-progress deployments
			if len(args) == 0 {
				// Load config and ensure project ID exists
				conf := config.GetProjectIDOrExit()

				// Select a deployment that is in progress
				var err error
				deploymentId, err = api.SelectDeploymentFromList(conf.ProjectID, func(d types.Deployment) bool {
					return d.Status == "PENDING" || d.Status == "QUEUED" || d.Status == "IN_PROGRESS"
				})
				if err != nil {
					if err.Error() == "no matching deployments found" {
						utils.InfoColor.Println("No in-progress deployments found to cancel.")
						return
					}
					utils.HandleError(err, "Error selecting deployment")
				}
			} else {
				deploymentId = args[0]
			}

			// Confirm cancellation
			confirm := false
			cancelPrompt := &survey.Confirm{
				Message: fmt.Sprintf("Are you sure you want to cancel deployment %s?", deploymentId),
				Default: false,
			}
			opts := utils.GetSurveyOptions()
			survey.AskOne(cancelPrompt, &confirm, opts)

			if !confirm {
				utils.InfoColor.Println("Cancellation aborted.")
				return
			}

			// Cancel deployment
			s := utils.StartSpinner("Cancelling deployment...")

			err := api.CancelDeployment(deploymentId)
			utils.StopSpinner(s)

			if err != nil {
				utils.ErrorColor.Printf("Failed to cancel deployment: %v\n", err)
				return
			}

			utils.SuccessColor.Println("[OK] Deployment cancelled successfully")
		},
	}

	// Add commands to root
	RootCmd.AddCommand(statusCmd, listCmd, cancelCmd)
}

// runStatus handles the status command logic
func runStatus(cmd *cobra.Command, args []string) {
	// Get flags
	showAll, _ := cmd.Flags().GetBool("all")
	showLogs, _ := cmd.Flags().GetBool("logs")

	// Get project configuration
	config, err := EnsureProjectID()
	utils.HandleError(err, "Error setting up project")

	var deploymentID string

	// If deployment ID is provided directly, use it
	if len(args) > 0 {
		deploymentID = args[0]
	} else {
		// Show recent deployments for selection
		var filter func(types.Deployment) bool
		if !showAll {
			// Only show recent deployments (last 24 hours) if not showing all
			filter = func(d types.Deployment) bool {
				return time.Since(d.CreatedAt) < 24*time.Hour
			}
		} else {
			filter = nil
		}

		// Let user select a deployment
		deploymentID, err = api.SelectDeploymentFromList(config.ProjectID, filter)
		if err != nil {
			utils.ErrorColor.Printf("Error selecting deployment: %v\n", err)
			return
		}
	}

	// Get deployment details
	deployment, err := api.GetDeploymentStatus(deploymentID)
	utils.HandleError(err, "Error fetching deployment details")

	// Get project details (if possible)
	project, err := api.GetProject(config.ProjectID)
	if err != nil {
		// If we can't get project details, just continue with what we have
		utils.WarnColor.Printf("Warning: Could not fetch project details: %v\n", err)
	}

	// Display deployment status information
	fmt.Println()
	utils.InfoColor.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	utils.InfoColor.Printf("Deployment ID:    %s\n", deployment.ID)
	utils.InfoColor.Printf("Project:          %s\n", project.Name)

	// Show status with appropriate color
	utils.InfoColor.Printf("Status:           ")
	switch deployment.Status {
	case "COMPLETED":
		utils.SuccessColor.Println(deployment.Status)
	case "FAILED":
		utils.ErrorColor.Println(deployment.Status)
	case "BUILDING", "UPLOADING", "PENDING":
		utils.WarnColor.Println(deployment.Status)
	default:
		fmt.Println(deployment.Status)
	}

	utils.InfoColor.Printf("Created:          %s\n", deployment.CreatedAt.Format("Jan 02, 2006 15:04:05"))

	if deployment.CompletedAt != nil {
		utils.InfoColor.Printf("Completed:        %s\n", deployment.CompletedAt.Format("Jan 02, 2006 15:04:05"))
		duration := deployment.CompletedAt.Sub(deployment.CreatedAt)
		utils.InfoColor.Printf("Duration:         %s\n", duration.Round(time.Second))
	}

	if deployment.Status == "COMPLETED" && project.Slug != "" {
		utils.InfoColor.Printf("Public URL:       https://%s.yok.ninja\n", project.Slug)
	}

	if deployment.DeploymentUrl != "" {
		utils.InfoColor.Printf("Deployment URL:   %s\n", deployment.DeploymentUrl)
	}
	utils.InfoColor.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Show logs if requested
	if showLogs {
		utils.InfoColor.Println("Showing deployment logs:")
		fmt.Println()

		// Fetch logs
		logs, err := api.GetDeploymentLogs(deploymentID, "")
		utils.HandleError(err, "Error fetching logs")

		// Create log renderer
		logRenderer := utils.NewLogRenderer()

		// Display logs
		for _, logEntry := range logs.Data.Logs {
			logRenderer.RenderLogEntry(logEntry)
		}
	}
}
