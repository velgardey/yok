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
	// Status command to check deployment status
	var statusCmd = &cobra.Command{
		Use:   "status [deploymentId]",
		Short: "Check the status of a deployment",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var deploymentId string
			var err error

			// If no deployment ID provided, ask the user to select from recent deployments
			if len(args) == 0 {
				// Load config and ensure project ID exists
				conf := config.GetProjectIDOrExit()

				// Select a deployment
				deploymentId, err = api.SelectDeploymentFromList(conf.ProjectID, nil)
				if err != nil {
					if err.Error() == "no matching deployments found" {
						utils.InfoColor.Println("No deployments found for this project.")
						return
					}
					utils.HandleError(err, "Error selecting deployment")
				}
			} else {
				deploymentId = args[0]
			}

			// Get deployment status
			s := utils.StartSpinner("Fetching deployment status...")

			status, err := api.GetDeploymentStatus(deploymentId)
			utils.StopSpinner(s)

			if err != nil {
				utils.ErrorColor.Printf("Failed to get deployment status: %v\n", err)
				return
			}

			// Print status
			fmt.Println("Deployment Status:")
			fmt.Printf("ID: %s\n", status.ID)

			// Color-coded status
			utils.FormatDeploymentStatus(status.Status)

			fmt.Printf("Created: %s (%s ago)\n",
				status.CreatedAt.Format(time.RFC3339),
				time.Since(status.CreatedAt).Round(time.Second))
			fmt.Printf("Last Updated: %s (%s ago)\n",
				status.UpdatedAt.Format(time.RFC3339),
				time.Since(status.UpdatedAt).Round(time.Second))
		},
	}

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
