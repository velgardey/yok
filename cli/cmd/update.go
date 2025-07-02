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

	"github.com/AlecAivazis/survey/v2"
	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/utils"
)

// checkForUpdates checks for newer version on GitHub
func checkForUpdates() (*selfupdate.Release, bool, error) {
	http.DefaultClient = utils.CreateHTTPClient()

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

// suggestElevatedUpdate provides guidance for updating with elevated permissions
func suggestElevatedUpdate() {
	utils.WarnColor.Println("The update requires elevated permissions.")
	fmt.Println("\nPlease try updating with elevated privileges:")

	if runtime.GOOS == "windows" {
		fmt.Println("Right-click on Command Prompt/PowerShell and select 'Run as administrator', then run:")
		fmt.Println("    yok self-update --force")
	} else {
		fmt.Println("Run the command with sudo:")
		fmt.Println("    sudo yok self-update --force")
	}
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

// runSudoSelfUpdate executes the update with sudo
func runSudoSelfUpdate(force bool) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	args := []string{"self-update"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, "--sudo-mode") // Prevent infinite loops

	cmd := exec.Command("sudo", append([]string{execPath}, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	utils.InfoColor.Println("This operation requires elevated privileges.")
	fmt.Println("You may be prompted for your password.")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo execution failed: %v", err)
	}

	return nil
}

// runSelfUpdate implements the update logic
func runSelfUpdate(cmd *cobra.Command, force bool, checkOnly bool) error {
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

	// Check permissions
	if !hasWritePermission(targetPath) {
		sudoMode, _ := cmd.Flags().GetBool("sudo-mode")
		if isRunningWithSudo() || sudoMode {
			// Already elevated, continue
		} else {
			// Need elevation
			if runtime.GOOS == "windows" {
				suggestElevatedUpdate()
				return fmt.Errorf("insufficient permissions to update binary")
			} else {
				return runSudoSelfUpdate(force)
			}
		}
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

	// Verify update
	if runtime.GOOS != "windows" {
		cmd := exec.Command(targetPath, "--version")
		output, err := cmd.CombinedOutput()
		if err == nil {
			utils.InfoColor.Printf("\nVerified new version: %s", output)
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
				fmt.Println("3. Try running with elevated privileges (sudo/admin)")
				fmt.Println("4. Check if GitHub API is accessible from your network")

				if strings.Contains(err.Error(), "permission") ||
					strings.Contains(err.Error(), "insufficient") {
					fmt.Println("\nSince this appears to be a permissions issue, try:")
					if runtime.GOOS == "windows" {
						fmt.Println("    Run Command Prompt as Administrator, then run: yok self-update --force")
					} else {
						fmt.Println("    sudo yok self-update --force")
					}
				}

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
