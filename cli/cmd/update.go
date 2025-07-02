package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

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

	// The selfupdate library uses http.DefaultClient internally
	// so we just need to set the default client

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

// backupCurrentBinary creates a backup copy of the binary
func backupCurrentBinary(execPath string) (string, error) {
	backupPath := execPath + ".backup"
	os.Remove(backupPath) // Remove existing backup if present

	// Copy file
	source, err := os.Open(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to open current binary: %w", err)
	}
	defer source.Close()

	backup, err := os.OpenFile(backupPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer backup.Close()

	if _, err = io.Copy(backup, source); err != nil {
		return "", fmt.Errorf("failed to copy binary to backup: %w", err)
	}

	// Match permissions
	info, err := os.Stat(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	if err := os.Chmod(backupPath, info.Mode()); err != nil {
		return "", fmt.Errorf("failed to set backup permissions: %w", err)
	}

	return backupPath, nil
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

// runUpdateWithElevation executes the update with appropriate elevation (sudo on Unix, UAC on Windows)
func runUpdateWithElevation(execPath string, latest *selfupdate.Release) error {
	if runtime.GOOS == "windows" {
		// For Windows, we need to use a different approach since we can't replace a running binary
		// Create a temporary batch file that will:
		// 1. Wait for our process to exit
		// 2. Download and replace the binary
		// 3. Restart the binary if needed
		tmpDir := os.TempDir()
		updateBatchPath := filepath.Join(tmpDir, "yok_update.bat")

		// Get our process ID
		pid := os.Getpid()

		// Create batch file content
		batchContent := fmt.Sprintf(`@echo off
echo Waiting for Yok CLI to exit (PID: %d)...
timeout /t 1 /nobreak > nul
tasklist | find " %d " > nul
if not errorlevel 1 goto wait

echo Downloading update from %s...
powershell -Command "$webClient = New-Object System.Net.WebClient; $webClient.DownloadFile('%s', '%s.new')"
if errorlevel 1 (
    echo Failed to download update.
    exit /b 1
)

echo Installing update...
move /y "%s.new" "%s"
if errorlevel 1 (
    echo Failed to replace binary. Please try again with administrator privileges.
    exit /b 1
)

echo.
echo ✅ Yok CLI has been updated to %s successfully!
echo Run 'yok version' to verify the update.
`, pid, pid, latest.URL, latest.AssetURL, execPath, execPath, execPath, latest.Version)

		// Write the batch file
		if err := os.WriteFile(updateBatchPath, []byte(batchContent), 0700); err != nil {
			return fmt.Errorf("failed to create update script: %v", err)
		}

		// Start the batch file in a new process and exit our process
		cmd := exec.Command("cmd", "/c", "start", "cmd", "/c", updateBatchPath)
		if err := cmd.Start(); err != nil {
			os.Remove(updateBatchPath)
			return fmt.Errorf("failed to start update process: %v", err)
		}

		// Inform the user
		utils.InfoColor.Println("Update process has started in a new window.")
		fmt.Println("Please wait while the update completes...")
		fmt.Println("This window will close automatically.")

		// Exit after a short delay
		time.Sleep(1 * time.Second)
		os.Exit(0)
		return nil
	} else {
		// For Unix systems (Linux/macOS)
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

echo "✅ Yok CLI has been updated to %s successfully!"
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
}

// runSelfUpdate implements the update logic
func runSelfUpdate(cmd *cobra.Command, force bool, checkOnly bool) error {
	// Check if we're already running with sudo
	sudoMode := isRunningWithSudo()
	sudoFlag, _ := cmd.Flags().GetBool("sudo-mode")

	// If we're on Unix and not running with sudo, and not in sudo mode flag
	if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && !sudoMode && !sudoFlag {
		// Check if we need sudo by trying to write to the binary location
		execPath, err := os.Executable()
		if err == nil {
			execPath, err = filepath.EvalSymlinks(execPath)
			if err == nil {
				if !hasWritePermission(execPath) {
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
	}

	spinner := utils.StartSpinner("Checking for updates...")
	latest, hasUpdate, err := checkForUpdates()
	utils.StopSpinner(spinner)

	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	currentVersion := strings.TrimPrefix(version, "v")

	// Just checking for updates
	if checkOnly {
		if hasUpdate {
			utils.InfoColor.Printf("\nUpdate available: %s (current: %s)\n", latest.Version, currentVersion)
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
	fmt.Printf("Latest version: %s\n", latest.Version)
	fmt.Printf("Release page: %s\n", latest.URL)

	// Confirm update unless forced
	if !force {
		updateConfirm := false
		updatePrompt := &survey.Confirm{
			Message: fmt.Sprintf("Do you want to update from v%s to %s?", currentVersion, latest.Version),
			Default: true,
		}
		if err := survey.AskOne(updatePrompt, &updateConfirm); err != nil {
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

	// Check if we have permission to update the binary
	if !hasWritePermission(targetPath) && !sudoMode {
		// We don't have permission, try to run with elevation
		if sudoFlag {
			// We're already trying to elevate, but still don't have permission
			return fmt.Errorf("failed to get write permission even with elevated privileges")
		}

		utils.InfoColor.Println("This update requires elevated privileges.")

		// Automatically handle elevation
		return runUpdateWithElevation(execPath, latest)
	}

	// Create backup if target exists
	backupPath := ""
	if _, err := os.Stat(targetPath); err == nil {
		backupPath, err = backupCurrentBinary(targetPath)
		if err != nil {
			utils.WarnColor.Printf("Warning: Failed to create backup: %v\n", err)
			if !force {
				continueWithoutBackup := false
				backupPrompt := &survey.Confirm{
					Message: "Failed to create backup. Continue with update anyway?",
					Default: false,
				}
				if err := survey.AskOne(backupPrompt, &continueWithoutBackup); err != nil || !continueWithoutBackup {
					return fmt.Errorf("update cancelled: could not create backup")
				}
			}
		} else {
			utils.InfoColor.Printf("Created backup at: %s\n", backupPath)
		}
	}

	// Special handling for Windows as we can't replace a running binary
	if runtime.GOOS == "windows" {
		return runUpdateWithElevation(execPath, latest)
	}

	// Run update
	utils.InfoColor.Printf("Updating Yok CLI from v%s to %s\n", currentVersion, latest.Version)
	spinner = utils.StartSpinner("Downloading and installing update...")

	// Use selfupdate.UpdateTo to download and replace the binary
	err = selfupdate.UpdateTo(latest.AssetURL, targetPath)
	utils.StopSpinner(spinner)

	if err != nil {
		// Try to restore from backup
		if backupPath != "" {
			utils.WarnColor.Println("Update failed, attempting to restore from backup...")
			if restoreErr := os.Rename(backupPath, targetPath); restoreErr != nil {
				utils.ErrorColor.Printf("Failed to restore backup: %v\n", restoreErr)
				return fmt.Errorf("update failed and backup restoration failed: %v, %v", err, restoreErr)
			}
			utils.InfoColor.Println("Successfully restored from backup")
		}
		return fmt.Errorf("failed to update binary: %v", err)
	}

	// Cleanup
	if backupPath != "" {
		os.Remove(backupPath)
	}

	utils.SuccessColor.Printf("✅ Yok CLI has been updated to %s!\n", latest.Version)

	// Display release notes
	if latest.ReleaseNotes != "" {
		utils.InfoColor.Println("\nRelease notes:")
		fmt.Println(latest.ReleaseNotes)
	}

	// Verify update by running the new binary to check its version
	verifyUpdate := func() bool {
		cmd := exec.Command(targetPath, "--version")
		output, err := cmd.CombinedOutput()
		if err != nil {
			utils.WarnColor.Printf("Failed to verify update: %v\n", err)
			return false
		}

		outputStr := string(output)
		expectedVersionStr := fmt.Sprintf("Yok CLI v%s", latest.Version)
		if !strings.Contains(outputStr, expectedVersionStr) {
			utils.WarnColor.Printf("Version mismatch after update. Expected: %s, Got: %s\n",
				expectedVersionStr, strings.TrimSpace(outputStr))
			return false
		}

		utils.InfoColor.Printf("\nVerified new version: %s", output)
		return true
	}

	// Only verify on Unix systems as Windows might have file locking issues
	if runtime.GOOS != "windows" {
		if !verifyUpdate() {
			utils.WarnColor.Println("Update may not have completed correctly. Please try again or reinstall.")
		}
	} else {
		utils.InfoColor.Println("\nUpdate completed. Please restart your terminal to use the new version.")
	}

	return nil
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
				fmt.Println("3. Try running with elevated privileges (sudo/admin)")
				fmt.Println("4. Check if GitHub API is accessible from your network")

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
