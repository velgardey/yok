package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/utils"
)

// checkForUpdates checks for newer version on GitHub
func checkForUpdates() (*selfupdate.Release, bool, error) {
	// Create and set HTTP client with reasonable timeout
	httpClient := utils.CreateHTTPClient()
	http.DefaultClient = httpClient

	latest, found, err := selfupdate.DetectLatest("velgardey/yok")
	if err != nil {
		return nil, false, fmt.Errorf("error checking for updates: %w", err)
	}

	if !found {
		return nil, false, fmt.Errorf("no release found for velgardey/yok")
	}

	currentVersion := strings.TrimPrefix(version, "v")
	v, err := semver.Parse(currentVersion)
	if err != nil {
		// Handle dev version
		if currentVersion == "dev" || currentVersion == "development" {
			return latest, true, nil // Always update dev versions
		}
		return nil, false, fmt.Errorf("failed to parse current version: %w", err)
	}

	return latest, latest.Version.GT(v), nil
}

// getCurrentVersion returns the current version without the 'v' prefix
func getCurrentVersion() string {
	return strings.TrimPrefix(version, "v")
}

// getLatestVersionNoAPI makes an HTTP request to GitHub releases page
// and extracts the latest version from the redirect URL
func getLatestVersionNoAPI() (string, error) {
	client := utils.CreateHTTPClient()

	// Disable following redirects so we can capture the redirect URL
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get("https://github.com/velgardey/yok/releases/latest")
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Extract version from the Location header
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no redirect location found")
	}

	// Parse version from URL
	parts := strings.Split(location, "/")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid redirect URL format")
	}

	version := parts[len(parts)-1]
	if !strings.HasPrefix(version, "v") {
		return "", fmt.Errorf("invalid version format: %s", version)
	}

	return strings.TrimPrefix(version, "v"), nil
}

// detectInstallLocation returns the appropriate directory for binary installation
func detectInstallLocation() (string, error) {
	// Get default locations by platform
	var defaultLocations []string
	switch runtime.GOOS {
	case "windows":
		defaultLocations = []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "yok"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "yok"),
		}
	default: // darwin, linux
		defaultLocations = []string{
			"/usr/local/bin",
			"/opt/homebrew/bin", // For macOS with Homebrew
			"/usr/bin",
			"/bin",
			filepath.Join(os.Getenv("HOME"), ".local", "bin"),
		}
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get current executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	execDir := filepath.Dir(execPath)

	// If current executable is in standard location, use that
	for _, dir := range defaultLocations {
		if execDir == dir {
			return dir, nil
		}
	}

	// Try standard locations first
	if runtime.GOOS == "windows" {
		dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "yok")
		if isLocationWritable(dir) {
			return dir, nil
		}
	} else if isLocationWritable("/usr/local/bin") {
		return "/usr/local/bin", nil
	}

	// Try current directory
	if isLocationWritable(execDir) {
		return execDir, nil
	}

	// Try all other default locations
	for _, dir := range defaultLocations {
		if isLocationWritable(dir) {
			return dir, nil
		}
	}

	return "", fmt.Errorf("no writable installation location found")
}

// isLocationWritable checks if a directory is writable
func isLocationWritable(dir string) bool {
	// Ensure directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return false
		}
	}

	// Check write permissions
	testFile := filepath.Join(dir, ".write_test")
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	os.Remove(testFile)
	return true
}

// hasWritePermission checks if the binary can be updated without elevation
func hasWritePermission(execPath string) bool {
	file, err := os.OpenFile(execPath, os.O_WRONLY, 0)
	if err == nil {
		file.Close()
		return true
	}
	return false
}

// isRunningWithSudo checks if process has elevated privileges
func isRunningWithSudo() bool {
	if runtime.GOOS != "windows" {
		cmd := exec.Command("id", "-u")
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "0" {
			return true
		}
	}
	return false
}

// runUnixUpdate handles the update process for Unix-based systems (Linux/macOS)
func runUnixUpdate(execPath string, latest *selfupdate.Release) error {
	utils.InfoColor.Println("This operation requires elevated privileges.")
	fmt.Println("You will be prompted for your password.")

	// Check if we can run sudo without a password prompt first
	sudoTestCmd := exec.Command("sudo", "-n", "true")
	sudoTestCmd.Stderr = nil
	sudoNoPasswd := (sudoTestCmd.Run() == nil)

	// Get the absolute path to the current binary
	execPath, err := filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Create a temporary script for the update
	tmpDir := os.TempDir()
	scriptPath := filepath.Join(tmpDir, "yok_update.sh")

	// Write the update script
	script := fmt.Sprintf(`#!/bin/bash
set -e

echo "Downloading update from %s..."
curl -L -o "%s.new" "%s"
chmod +x "%s.new"

echo "Installing update..."
mv "%s.new" "%s"

echo "[OK] Yok CLI has been updated to %s successfully!"
echo "Run 'yok version' to verify the update."
`, latest.URL, execPath, latest.AssetURL, execPath, execPath, execPath, latest.Version)

	if err := os.WriteFile(scriptPath, []byte(script), 0700); err != nil {
		return fmt.Errorf("failed to create update script: %v", err)
	}
	defer os.Remove(scriptPath)

	// If sudo requires password, inform the user they need to use sudo directly
	if !sudoNoPasswd {
		utils.InfoColor.Println("Your system requires a password for sudo operations.")
		utils.InfoColor.Println("Please run the following command to update:")
		fmt.Printf("\n    sudo yok self-update --force\n\n")
		return fmt.Errorf("please run the update with sudo")
	}

	// Run the script with sudo
	cmd := exec.Command("sudo", "bash", scriptPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createWindowsUpdateScript generates a PowerShell script for updating the Windows binary
func createWindowsUpdateScript(targetPath, version string) (string, error) {
	// Create a temp file for the update script
	tmpDir := os.TempDir()
	scriptPath := filepath.Join(tmpDir, "yok_update.ps1")

	// Format the download URL based on version
	downloadUrl := fmt.Sprintf("https://github.com/velgardey/yok/releases/download/v%s/yok_%s_windows_amd64.zip",
		version, version)

	// Create backup path
	backupPath := targetPath + ".backup"

	// Build the script content without complex formatting
	scriptContent := []string{
		"# Yok CLI Self-Update Script",
		"$ErrorActionPreference = \"Stop\"",
		"$ProgressPreference = \"SilentlyContinue\"  # Makes downloads faster",
		"",
		"# Function to handle errors",
		"function Handle-Error {",
		"    param(",
		"        [Parameter(Mandatory=$true)][string]$ErrorMessage,",
		"        [Parameter(Mandatory=$false)][object]$ErrorDetail = $null",
		"    )",
		"    ",
		"    Write-Host \"`n====== ERROR ======\" -ForegroundColor Red",
		"    Write-Host $ErrorMessage -ForegroundColor Red",
		"    ",
		"    if ($ErrorDetail) {",
		"        Write-Host \"`nError details:\" -ForegroundColor Red",
		"        Write-Host $ErrorDetail.Exception.Message -ForegroundColor Red",
		"    }",
		"    ",
		"    # Restore from backup if available",
		fmt.Sprintf("    if (Test-Path \"%s\") {", backupPath),
		"        Write-Host \"Restoring from backup...\" -ForegroundColor Yellow",
		"        try {",
		fmt.Sprintf("            Copy-Item -Path \"%s\" -Destination \"%s\" -Force", backupPath, targetPath),
		"            Write-Host \"Restored successfully.\" -ForegroundColor Green",
		"        } catch {",
		"            Write-Host \"Failed to restore from backup: $_\" -ForegroundColor Red",
		"        }",
		"    }",
		"    ",
		"    # Cleanup ",
		"    if (Test-Path \"$env:TEMP\\yok_update\") {",
		"        Remove-Item -Path \"$env:TEMP\\yok_update\" -Recurse -Force -ErrorAction SilentlyContinue",
		"    }",
		"    ",
		"    # Self-delete after delay - give time to read error",
		"    Start-Sleep -Seconds 5",
		"    Remove-Item -Path $PSCommandPath -Force -ErrorAction SilentlyContinue",
		"    exit 1",
		"}",
		"",
		"try {",
		"    # Wait for the main process to exit",
		"    Start-Sleep -Seconds 2",
		"    ",
		fmt.Sprintf("    Write-Host \"Updating Yok CLI to v%s...\" -ForegroundColor Cyan", version),
		"    ",
		"    # Create temp directory for update",
		"    $updateDir = \"$env:TEMP\\yok_update\"",
		"    if (Test-Path $updateDir) {",
		"        Remove-Item -Path $updateDir -Recurse -Force",
		"    }",
		"    New-Item -ItemType Directory -Path $updateDir -Force | Out-Null",
		"    ",
		"    # Download the update",
		"    $zipPath = \"$updateDir\\yok.zip\"",
		fmt.Sprintf("    Write-Host \"Downloading update from %s...\" -ForegroundColor Cyan", downloadUrl),
		"    try {",
		fmt.Sprintf("        Invoke-WebRequest -Uri \"%s\" -OutFile $zipPath", downloadUrl),
		"    } catch {",
		"        Handle-Error \"Failed to download the update package\" $_",
		"    }",
		"    ",
		"    # Create backup of current executable",
		"    Write-Host \"Creating backup...\" -ForegroundColor Cyan",
		"    try {",
		fmt.Sprintf("        Copy-Item -Path \"%s\" -Destination \"%s\" -Force", targetPath, backupPath),
		"    } catch {",
		"        Handle-Error \"Failed to create backup\" $_",
		"    }",
		"    ",
		"    # Extract and replace the binary",
		"    Write-Host \"Installing update...\" -ForegroundColor Cyan",
		"    try {",
		"        Expand-Archive -Path $zipPath -DestinationPath $updateDir -Force",
		fmt.Sprintf("        Copy-Item -Path \"$updateDir\\yok.exe\" -Destination \"%s\" -Force", targetPath),
		"    } catch {",
		"        Handle-Error \"Failed to install the update\" $_",
		"    }",
		"    ",
		"    # Cleanup",
		"    Write-Host \"Cleaning up...\" -ForegroundColor Cyan",
		"    Remove-Item -Path $updateDir -Recurse -Force -ErrorAction SilentlyContinue",
		"    ",
		fmt.Sprintf("    Write-Host \"`n[OK] Yok CLI has been updated to v%s successfully!\" -ForegroundColor Green", version),
		"    Write-Host \"Run 'yok version' to verify the update.\" -ForegroundColor Cyan",
		"    ",
		"    # Self-delete after a delay",
		"    Start-Sleep -Seconds 1",
		"    Remove-Item -Path $PSCommandPath -Force -ErrorAction SilentlyContinue",
		"} catch {",
		"    Handle-Error \"An unexpected error occurred during update\" $_",
		"}",
	}

	// Join the script lines with newlines
	script := strings.Join(scriptContent, "\n")

	// Write the script to disk
	err := os.WriteFile(scriptPath, []byte(script), 0700)
	if err != nil {
		return "", fmt.Errorf("failed to create update script: %v", err)
	}

	return scriptPath, nil
}

// runWindowsUpdate handles the update process for Windows
func runWindowsUpdate(execPath string, version string) error {
	// Create the PowerShell script
	scriptPath, err := createWindowsUpdateScript(execPath, version)
	if err != nil {
		return err
	}

	utils.InfoColor.Println("Starting update process...")
	utils.InfoColor.Println("The CLI will exit and a new process will complete the update.")

	// Launch PowerShell script as a separate process
	cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start (not Run) to avoid waiting for completion
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update process: %v", err)
	}

	// Exit immediately after starting the update process
	fmt.Println("Update in progress... please wait.")
	os.Exit(0)
	return nil // This is never reached
}

// runSelfUpdate implements the update logic
func runSelfUpdate(cmd *cobra.Command, force bool, checkOnly bool) error {
	// Check for sudo/admin mode
	sudoMode := isRunningWithSudo()
	sudoFlag, _ := cmd.Flags().GetBool("sudo-mode")

	// For Unix systems, if we're not running with sudo and don't have the sudo flag set,
	// check if we need sudo by trying to write to the binary location
	if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && !sudoMode && !sudoFlag {
		execPath, err := os.Executable()
		if err == nil {
			execPath, err = filepath.EvalSymlinks(execPath)
			if err == nil && !hasWritePermission(execPath) {
				utils.InfoColor.Println("This operation requires elevated privileges.")
				forceFlag := ""
				if force {
					forceFlag = " --force"
				}
				fmt.Println("Please run: sudo yok self-update" + forceFlag)
				return fmt.Errorf("please run with sudo")
			}
		}
	}

	// For Windows, we'll use a different approach without requiring the GitHub API
	var currentVersion, latestVersionStr string
	var hasUpdate bool
	var latest *selfupdate.Release
	var err error

	currentVersion = getCurrentVersion()

	// Check for updates
	spinner := utils.StartSpinner("Checking for updates...")

	if runtime.GOOS == "windows" {
		// Use non-API method for Windows
		latestVersionStr, err = getLatestVersionNoAPI()
		if err != nil {
			utils.StopSpinner(spinner)
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		// Parse versions for comparison
		currentSemver, err := semver.Parse(currentVersion)
		if err != nil {
			if currentVersion == "dev" || currentVersion == "development" {
				hasUpdate = true // Always update dev versions
			} else {
				utils.StopSpinner(spinner)
				return fmt.Errorf("failed to parse current version: %w", err)
			}
		} else {
			latestSemver, err := semver.Parse(latestVersionStr)
			if err != nil {
				utils.StopSpinner(spinner)
				return fmt.Errorf("failed to parse latest version: %w", err)
			}
			hasUpdate = latestSemver.GT(currentSemver)
		}
	} else {
		// Use the existing API-based method for non-Windows
		latest, hasUpdate, err = checkForUpdates()
		if err != nil {
			utils.StopSpinner(spinner)
			return fmt.Errorf("failed to check for updates: %w", err)
		}
		latestVersionStr = latest.Version.String()
	}

	utils.StopSpinner(spinner)

	// Just checking for updates
	if checkOnly {
		if hasUpdate {
			utils.InfoColor.Printf("\nUpdate available: v%s (current: %s)\n", latestVersionStr, currentVersion)
			fmt.Printf("Run 'yok self-update' to update to the latest version\n")
		} else {
			utils.SuccessColor.Printf("You're already using the latest version (v%s)\n", currentVersion)
		}
		return nil
	}

	// No update available
	if !hasUpdate && !force {
		utils.SuccessColor.Printf("You're already using the latest version (v%s)\n", currentVersion)
		return nil
	}

	// Display update information
	utils.InfoColor.Printf("\nAvailable update:\n")
	fmt.Printf("Current version: v%s\n", currentVersion)
	fmt.Printf("Latest version: v%s\n", latestVersionStr)

	// Show release page URL based on platform
	if runtime.GOOS == "windows" {
		fmt.Printf("Release page: https://github.com/velgardey/yok/releases/tag/v%s\n", latestVersionStr)
	} else if latest != nil {
		fmt.Printf("Release page: %s\n", latest.URL)
	}

	// Confirm update unless forced
	if !force {
		updateConfirm := false
		updatePrompt := &survey.Confirm{
			Message: fmt.Sprintf("Do you want to update from v%s to v%s?", currentVersion, latestVersionStr),
			Default: true,
		}
		opts := utils.GetSurveyOptions()
		if err := survey.AskOne(updatePrompt, &updateConfirm, opts); err != nil {
			return fmt.Errorf("update cancelled: %v", err)
		}

		if !updateConfirm {
			utils.InfoColor.Println("Update cancelled")
			return nil
		}
	}

	// Get executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Resolve symlinks to get the actual binary path
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks: %v", err)
	}

	// Handle special executable names (testing builds)
	execDir := filepath.Dir(execPath)
	targetName := "yok"
	if runtime.GOOS == "windows" {
		targetName += ".exe"
	}

	// Use standard installation paths for test builds
	installDir := execDir
	if strings.HasSuffix(filepath.Base(execPath), ".new") || strings.HasSuffix(filepath.Base(execPath), ".test") {
		var err error
		installDir, err = detectInstallLocation()
		if err != nil {
			return fmt.Errorf("failed to detect installation location: %v", err)
		}
	}

	targetPath := filepath.Join(installDir, targetName)

	// Handle platform-specific update
	if runtime.GOOS == "windows" {
		return runWindowsUpdate(targetPath, latestVersionStr)
	} else if latest != nil {
		return runUnixUpdate(targetPath, latest)
	}

	return fmt.Errorf("update not implemented for this platform")
}

// Set up the update command
var updateCmd *cobra.Command

func init() {
	var (
		force     bool
		checkOnly bool
		sudoMode  bool
	)

	updateCmd = &cobra.Command{
		Use:     "self-update",
		Short:   "Update Yok CLI to the latest version",
		Long:    `Update Yok CLI to the latest version from GitHub releases.`,
		Aliases: []string{"update"},
		Run: func(cmd *cobra.Command, args []string) {
			if err := runSelfUpdate(cmd, force, checkOnly); err != nil {
				utils.ErrorColor.Printf("Update failed: %v\n", err)

				utils.WarnColor.Println("\nTroubleshooting tips:")
				fmt.Println("1. Check your internet connection")
				fmt.Println("2. Make sure you have permission to write to the installation directory")

				// Platform-specific troubleshooting tips
				if runtime.GOOS == "windows" {
					fmt.Println("3. Try running with administrator privileges")
					fmt.Println("4. Ensure PowerShell execution policy allows running scripts")
				} else {
					fmt.Println("3. Try running with elevated privileges (sudo/admin)")
				}

				fmt.Println("4. Check if GitHub is accessible from your network")

				os.Exit(1)
			}
		},
	}

	updateCmd.Flags().BoolVarP(&force, "force", "f", false, "Force update without confirmation")
	updateCmd.Flags().BoolVarP(&checkOnly, "check", "c", false, "Only check for updates without installing")
	updateCmd.Flags().BoolVar(&sudoMode, "sudo-mode", false, "Internal flag to prevent sudo loop")
	updateCmd.Flags().MarkHidden("sudo-mode")

	RootCmd.AddCommand(updateCmd)
}
