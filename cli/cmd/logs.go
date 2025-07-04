package cmd

import (
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/api"
	"github.com/velgardey/yok/cli/internal/types"
	"github.com/velgardey/yok/cli/internal/utils"
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs [deploymentID]",
	Short: "View logs for deployments",
	Long: `View and follow logs for your deployments.
	
Examples:
  yok logs                    # Interactive selection of deployment to view
  yok logs abc123             # View logs for deployment with ID abc123
  yok logs -f                 # Follow logs for interactively selected deployment
  yok logs abc123 -f          # Follow logs for deployment with ID abc123
  yok logs -t                 # View logs without timestamps
  yok logs -c                 # View logs without colors
  yok logs -r                 # View raw logs (no formatting)`,
	Run: runLogs,
}

func init() {
	RootCmd.AddCommand(logsCmd)

	// Add flags
	logsCmd.Flags().BoolP("follow", "f", false, "Follow logs (stream new logs as they arrive)")
	logsCmd.Flags().BoolP("no-timestamps", "t", false, "Hide timestamps")
	logsCmd.Flags().BoolP("no-color", "c", false, "Disable colored output")
	logsCmd.Flags().BoolP("raw", "r", false, "Display raw logs without formatting")
	logsCmd.Flags().BoolP("wait", "w", false, "Wait for completion (automatically exit when deployment completes)")
}

// runLogs handles the logs command logic
func runLogs(cmd *cobra.Command, args []string) {
	// Get flags
	follow, _ := cmd.Flags().GetBool("follow")
	noTimestamps, _ := cmd.Flags().GetBool("no-timestamps")
	noColor, _ := cmd.Flags().GetBool("no-color")
	rawOutput, _ := cmd.Flags().GetBool("raw")

	// Get project configuration
	config, err := EnsureProjectID()
	utils.HandleError(err, "Error setting up project")

	var deploymentID string

	// If deployment ID is provided directly, use it
	if len(args) > 0 {
		deploymentID = args[0]
	} else {
		// Otherwise, get a list of deployments and prompt user to select one
		filter := func(d types.Deployment) bool { return true } // No filter - show all deployments
		deploymentID, err = api.SelectDeploymentFromList(config.ProjectID, filter)
		utils.HandleError(err, "Error selecting deployment")
	}

	// Get deployment details
	deployment, err := api.GetDeploymentStatus(deploymentID)
	utils.HandleError(err, "Error fetching deployment details")

	// Display deployment information
	utils.InfoColor.Printf("Viewing logs for deployment: %s\n", deploymentID)

	// Show status with appropriate color
	utils.InfoColor.Printf("Status: ")
	switch deployment.Status {
	case "COMPLETED":
		utils.SuccessColor.Println(deployment.Status)
	case "FAILED":
		utils.ErrorColor.Println(deployment.Status)
	case "BUILDING", "UPLOADING", "PENDING":
		utils.WarnColor.Println(deployment.Status)
	default:
		utils.InfoColor.Println(deployment.Status)
	}

	utils.InfoColor.Printf("Created: %s\n", deployment.CreatedAt.Format("Jan 02, 2006 15:04:05"))

	// Configure log renderer
	logRenderer := utils.NewLogRenderer().
		WithTimestamps(!noTimestamps).
		WithColors(!noColor).
		WithRawOutput(rawOutput)

	// Set log renderer for streaming
	api.SetLogRenderer(logRenderer)

	// For completed deployments, we may not want to follow logs
	if follow && (deployment.Status != "COMPLETED" || cmd.Flags().Changed("follow")) {
		utils.InfoColor.Println("Following logs (Press Ctrl+C to stop)...")

		// Create stop channel for log streaming
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
			showDeploymentUrls(config.ProjectID, deploymentID, deployment.DeploymentUrl)
			os.Exit(0)
		} else {
			// Check if deployment actually failed or was just interrupted
			status, err := api.GetDeploymentStatus(deploymentID)
			if err == nil && status.Status == "FAILED" {
				utils.ErrorColor.Println("Deployment failed. Check the logs above for detailed error messages.")
				os.Exit(1)
			}
		}

		return
	}

	// For non-follow mode, just fetch and display logs once
	logs, err := api.GetDeploymentLogs(deploymentID, "")
	utils.HandleError(err, "Error fetching logs")

	for _, logEntry := range logs.Data.Logs {
		logRenderer.RenderLogEntry(logEntry)
	}

	// Show completion message based on deployment status
	switch deployment.Status {
	case "COMPLETED":
		utils.SuccessColor.Println("\nDeployment completed successfully.")
		showDeploymentUrls(config.ProjectID, deploymentID, deployment.DeploymentUrl)
		os.Exit(0)
	case "FAILED":
		utils.ErrorColor.Println("\nDeployment failed. Check the logs above for detailed error messages.")
		os.Exit(1)
	}
}
