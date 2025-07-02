package cmd

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/utils"
)

// getLatestRelease retrieves the latest release info from GitHub
func getLatestRelease() (*utils.GitHubRelease, error) {
	// Repository info
	const repoName = "velgardey/yok"
	const apiURLTemplate = "https://api.github.com/repos/%s/releases/latest"
	apiURL := fmt.Sprintf(apiURLTemplate, repoName)

	// Create request with proper headers
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set user agent to avoid GitHub API rate limits
	req.Header.Set("User-Agent", "Yok-CLI-Updater")

	// Create HTTP client
	client := &http.Client{
		Timeout: utils.HttpTimeout,
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %v", err)
	}
	defer resp.Body.Close()

	// Handle HTTP status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Continue processing
	case http.StatusNotFound:
		return nil, fmt.Errorf("no releases found for %s", repoName)
	case http.StatusForbidden:
		return nil, fmt.Errorf("rate limit exceeded, please try again later")
	default:
		return nil, fmt.Errorf("GitHub API returned status code: %d", resp.StatusCode)
	}

	var release utils.GitHubRelease
	if err := utils.DecodeJSON(resp.Body, &release); err != nil {
		return nil, fmt.Errorf("error parsing GitHub response: %v", err)
	}

	// Validate response
	if release.TagName == "" {
		return nil, fmt.Errorf("received empty tag name from GitHub")
	}

	return &release, nil
}

// updateBinary handles platform-specific update process
func updateBinary() error {
	// Repository info
	const repoName = "velgardey/yok"

	// Get architecture
	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
		// These architectures are supported
	default:
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Create command based on OS
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Check if PowerShell is available
		if _, err := exec.LookPath("powershell"); err != nil {
			return fmt.Errorf("PowerShell is required for updates on Windows: %v", err)
		}

		scriptURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/scripts/install.ps1", repoName)
		cmd = exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-Command",
			fmt.Sprintf("Invoke-Expression ((New-Object System.Net.WebClient).DownloadString('%s'))", scriptURL))

	case "darwin", "linux":
		// Check for required tools
		if _, err := exec.LookPath("curl"); err != nil {
			return fmt.Errorf("curl is required for updates: %v", err)
		}

		if _, err := exec.LookPath("bash"); err != nil {
			return fmt.Errorf("bash is required for updates: %v", err)
		}

		scriptURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/scripts/install.sh", repoName)
		cmd = exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s | bash", scriptURL))

	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Connect command's stdout/stderr to our process
	cmd.Stdout = utils.GetStdout()
	cmd.Stderr = utils.GetStderr()

	// Run the command
	return cmd.Run()
}

// runSelfUpdate implements the self-update functionality
func runSelfUpdate() error {
	// Get the latest version from GitHub API
	utils.InfoColor.Print("Checking for updates... ")
	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("error checking for updates: %v", err)
	}
	fmt.Println("Done")

	latestVersion := release.TagName

	// Check if current version is already the latest
	if !utils.CompareVersions(version, latestVersion) {
		utils.SuccessColor.Printf("You're already using the latest version (v%s)\n", version)
		return nil
	}

	// Display update information
	utils.InfoColor.Printf("\nAvailable update:\n")
	fmt.Printf("Current version: v%s\n", version)
	fmt.Printf("Latest version: %s\n", latestVersion)

	// Ask for confirmation before updating
	updateConfirm := false
	updatePrompt := &survey.Confirm{
		Message: fmt.Sprintf("Do you want to update from v%s to %s?", version, latestVersion),
		Default: true,
	}
	if err := survey.AskOne(updatePrompt, &updateConfirm); err != nil {
		return fmt.Errorf("update cancelled: %v", err)
	}

	if !updateConfirm {
		utils.InfoColor.Println("Update cancelled")
		return nil
	}

	// Run update process
	utils.InfoColor.Println("Updating Yok CLI...")

	// Start a spinner for visual feedback
	s := utils.StartSpinner("Downloading and installing update...")
	err = updateBinary()
	utils.StopSpinner(s)

	if err != nil {
		// Show more detailed troubleshooting help
		utils.WarnColor.Println("\nTroubleshooting tips:")
		fmt.Println("1. Check your internet connection")
		fmt.Println("2. Make sure you have permission to write to the installation directory")
		fmt.Println("3. Try running with elevated privileges (sudo/admin)")
		fmt.Printf("4. Try manual installation from: https://github.com/velgardey/yok/releases/tag/%s\n", latestVersion)

		return fmt.Errorf("update failed: %v", err)
	}

	utils.SuccessColor.Printf("✅ Updated to version %s successfully!\n", latestVersion)
	return nil
}

func init() {
	// Add self-update command
	var selfUpdateCmd = &cobra.Command{
		Use:   "self-update",
		Short: "Update Yok CLI to the latest version",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runSelfUpdate(); err != nil {
				utils.ErrorColor.Printf("%v\n", err)
			}
		},
	}

	// Add version command
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Yok CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Yok CLI v%s\n", version)
		},
	}

	// Add commands to root
	RootCmd.AddCommand(selfUpdateCmd, versionCmd)
}
