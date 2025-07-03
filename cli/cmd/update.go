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

// hasAdminPrivileges checks if the current process has administrator privileges on Windows
func hasAdminPrivileges() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// Try writing to a protected location (Program Files)
	testFile := filepath.Join(os.Getenv("PROGRAMFILES"), ".yok_admin_test")
	file, err := os.OpenFile(testFile, os.O_RDWR|os.O_CREATE, 0666)
	if err == nil {
		file.Close()
		os.Remove(testFile)
		return true
	}

	return false
}

// runWindowsUpdate implements the Spawned Process Pattern for Windows updates
func runWindowsUpdate(execPath string, latest *selfupdate.Release) error {
	// Create a temporary updater script
	tmpDir := os.TempDir()
	updateBatchPath := filepath.Join(tmpDir, "yok_update.bat")

	// Get our process ID and directory info
	pid := os.Getpid()
	execDir := filepath.Dir(execPath)

	// Create the updater script following the Spawned Process Pattern
	batchContent := fmt.Sprintf(`@echo off
title Yok CLI Update Process
color 0A
echo Yok CLI Update Process
echo ====================
echo.

rem Step 1: Wait for main process to terminate
echo Waiting for Yok CLI to exit (PID: %d)...
:wait_loop
timeout /t 1 /nobreak > nul
tasklist | find " %d " > nul 2>&1
if not errorlevel 1 goto wait_loop
echo Main process exited, continuing with update.
echo.

rem Step 2: Download the update if not already downloaded
if not exist "%s.new" (
    echo Downloading update from %s...
    powershell -Command "$ProgressPreference = 'SilentlyContinue'; try { Invoke-WebRequest -Uri '%s' -OutFile '%s.new' -UseBasicParsing } catch { exit 1 }"
    if errorlevel 1 (
        echo Failed to download update.
        echo Please check your internet connection and try again.
        pause
        exit /b 1
    )
    echo Download completed successfully.
)

rem Step 3: Replace the executable file
echo.
echo Installing update...
echo Target location: %s

rem Ensure target directory exists
if not exist "%s" mkdir "%s"

rem Make multiple attempts to replace the file
set MAX_ATTEMPTS=3
set ATTEMPT=1

:replace_loop
echo Attempt %%ATTEMPT%% of %%MAX_ATTEMPTS%%...

rem Try direct move first
if exist "%s" (
    rem Create backup
    copy /y "%s" "%s.backup" > nul 2>&1
)

move /y "%s.new" "%s" > nul 2>&1
if not errorlevel 1 (
    echo File replaced successfully.
    goto success
)

rem If move fails, try copy and delete
copy /y "%s.new" "%s" > nul 2>&1
if not errorlevel 1 (
    del "%s.new" > nul 2>&1
    echo File replaced with copy method.
    goto success
)

rem Try PowerShell force copy as another method
powershell -Command "Copy-Item -Path '%s.new' -Destination '%s' -Force"
if not errorlevel 1 (
    if exist "%s.new" del "%s.new" > nul 2>&1
    echo File replaced with PowerShell method.
    goto success
)

rem If still not successful, increment attempt and try again
set /a ATTEMPT=ATTEMPT+1
if %%ATTEMPT%% leq %%MAX_ATTEMPTS%% (
    echo Replacement attempt failed, retrying after a delay...
    timeout /t 2 > nul
    goto replace_loop
)

rem All attempts failed
echo.
echo Failed to replace the executable after %%MAX_ATTEMPTS%% attempts.
echo The update file is still available at: %s.new
echo.
echo Possible causes:
echo - The file is still in use by another process
echo - You don't have sufficient permissions
echo.
pause
exit /b 1

:success
echo.
echo ✅ Yok CLI has been updated to %s successfully!
echo.

rem Step 4: Launch the new version (optional)
set LAUNCH_APP=n
set /p LAUNCH_APP="Would you like to launch Yok CLI now? (y/n): "
if /i "%%LAUNCH_APP%%" == "y" (
    echo Starting Yok CLI...
    start "" "%s" version
)

echo.
echo Update completed! You can now use 'yok' to run the updated version.
echo.
pause
`, pid, pid, execPath, latest.URL, latest.AssetURL, execPath, execPath, execDir, execDir, execPath, execPath, execPath, execPath, execPath, execPath, execPath, execPath, execPath, execPath, execPath, execPath, latest.Version, execPath)

	// Write the batch file
	if err := os.WriteFile(updateBatchPath, []byte(batchContent), 0700); err != nil {
		return fmt.Errorf("failed to create update script: %v", err)
	}

	// Request elevated privileges if needed and start the updater process
	var cmd *exec.Cmd
	if !hasAdminPrivileges() {
		utils.InfoColor.Println("Requesting administrator privileges for update...")
		cmd = exec.Command("powershell", "-Command",
			fmt.Sprintf("Start-Process cmd -ArgumentList '/k', '\"%s\"' -Verb RunAs", updateBatchPath))
	} else {
		cmd = exec.Command("cmd", "/c", "start", "cmd", "/k", updateBatchPath)
	}

	if err := cmd.Start(); err != nil {
		os.Remove(updateBatchPath)
		return fmt.Errorf("failed to start update process: %v", err)
	}

	// Inform the user
	utils.InfoColor.Println("Update process started in a new window.")
	fmt.Println("The update will complete after this window closes.")
	fmt.Println("Please wait for the update process to finish.")

	// Exit the main application to allow the updater to work
	time.Sleep(1 * time.Second)
	os.Exit(0)

	return nil // This line never executes but is needed for compilation
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

	// Check for updates
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

	// Special platform-specific handling
	if runtime.GOOS == "windows" {
		// For Windows, use the Spawned Process Pattern
		return runWindowsUpdate(targetPath, latest)
	} else {
		// For Unix systems, check if we need elevation
		if !hasWritePermission(targetPath) && !sudoMode {
			if sudoFlag {
				return fmt.Errorf("failed to get write permission even with elevated privileges")
			}
			return runUnixUpdate(targetPath, latest)
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

		// Verify update on Unix systems
		cmd := exec.Command(targetPath, "--version")
		output, err := cmd.CombinedOutput()
		if err == nil {
			utils.InfoColor.Printf("\nVerified new version: %s", output)
		} else {
			utils.WarnColor.Println("Update may not have completed correctly. Please try again or reinstall.")
		}
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

				// Platform-specific troubleshooting tips
				if runtime.GOOS == "windows" {
					fmt.Println("3. Try running as administrator (right-click Command Prompt/PowerShell and select 'Run as administrator')")
					fmt.Println("4. Make sure no other processes are using the Yok CLI executable")
					fmt.Println("5. Check if your antivirus is blocking the update process")
				} else {
					fmt.Println("3. Try running with elevated privileges (sudo/admin)")
				}

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
