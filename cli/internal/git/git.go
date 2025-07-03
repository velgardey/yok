package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/velgardey/yok/cli/internal/utils"
)

// ExecuteCommand runs a git command and returns its output
func ExecuteCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// GetRepoInfo gets repository information from the current directory or prompts user
// DEPRECATED: This function is no longer used. Use API client functions instead.
func GetRepoInfo(useManualEntry bool) (string, string, error) {
	return "", "", fmt.Errorf("GetRepoInfo is deprecated - use API client functions instead")
}

// GetRemoteURL gets the remote URL using git command
func GetRemoteURL() (string, error) {
	// Try to get origin remote first (most common case)
	output, err := ExecuteCommand("remote", "get-url", "origin")
	if err == nil && strings.TrimSpace(output) != "" {
		return strings.TrimSpace(output), nil
	}

	// If origin doesn't exist, try to get any remote
	output, err = ExecuteCommand("remote")
	if err != nil {
		return "", fmt.Errorf("failed to list git remotes: %w", err)
	}

	remotes := strings.Fields(strings.TrimSpace(output))
	if len(remotes) == 0 {
		return "", fmt.Errorf("no git remotes configured")
	}

	// Get URL of the first available remote
	output, err = ExecuteCommand("remote", "get-url", remotes[0])
	if err != nil {
		return "", fmt.Errorf("failed to get URL for remote '%s': %w", remotes[0], err)
	}

	remoteURL := strings.TrimSpace(output)
	if remoteURL == "" {
		return "", fmt.Errorf("remote '%s' has no URL configured", remotes[0])
	}

	return remoteURL, nil
}

// EnsureRepo ensures that the current directory is a git repository
func EnsureRepo() error {
	_, err := os.Stat(".git")
	if os.IsNotExist(err) {
		utils.InfoColor.Print("No Git repository found. Initializing... ")
		_, err := ExecuteCommand("init")
		if err != nil {
			return fmt.Errorf("failed to initialize git repo: %v", err)
		}
		utils.SuccessColor.Println("Done")
	}
	return nil
}

// CheckLocalRemoteSync checks if local changes match remote
func CheckLocalRemoteSync() (bool, error) {
	// First check if we have a remote
	remoteURL, err := GetRemoteURL()
	if err != nil {
		return false, fmt.Errorf("failed to get remote URL: %w", err)
	}
	if remoteURL == "" {
		return false, fmt.Errorf("no remote repository configured")
	}

	// Fetch latest from remote
	if _, err := ExecuteCommand("fetch"); err != nil {
		return false, fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// Check if we have an upstream branch
	if _, err := ExecuteCommand("rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
		return false, fmt.Errorf("no upstream branch configured")
	}

	// Check if we're behind the remote
	behindOutput, err := ExecuteCommand("rev-list", "--count", "HEAD..@{upstream}")
	if err != nil {
		return false, fmt.Errorf("failed to check if behind remote: %w", err)
	}
	if behindCount := strings.TrimSpace(behindOutput); behindCount != "0" {
		return false, fmt.Errorf("your local branch is %s commits behind the remote", behindCount)
	}

	// Check if we're ahead of the remote
	aheadOutput, err := ExecuteCommand("rev-list", "--count", "@{upstream}..HEAD")
	if err != nil {
		return false, fmt.Errorf("failed to check if ahead of remote: %w", err)
	}
	if aheadCount := strings.TrimSpace(aheadOutput); aheadCount != "0" {
		return false, fmt.Errorf("your local branch is %s commits ahead of the remote", aheadCount)
	}

	// Check for uncommitted changes
	if hasUncommittedChanges() {
		return false, fmt.Errorf("you have uncommitted changes")
	}

	return true, nil
}

// hasUncommittedChanges checks if there are any uncommitted changes
func hasUncommittedChanges() bool {
	statusOutput, err := ExecuteCommand("status", "--porcelain")
	if err != nil {
		return false // Assume no changes if we can't check
	}
	return strings.TrimSpace(statusOutput) != ""
}

// HandleUncommittedChanges checks for uncommitted changes and offers to commit and push them
func HandleUncommittedChanges() error {
	if !hasUncommittedChanges() {
		return nil // No changes to handle
	}

	// Show uncommitted changes
	statusOutput, err := ExecuteCommand("status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	fmt.Println("Uncommitted changes detected:")
	fmt.Println(statusOutput)

	// Ask user if they want to commit changes
	if !confirmCommitChanges() {
		return fmt.Errorf("you have uncommitted changes")
	}

	// Get commit message
	commitMessage, err := getCommitMessage()
	if err != nil {
		return err
	}

	// Perform git operations
	return CommitAndPushChanges(commitMessage)
}

// confirmCommitChanges asks user if they want to commit changes
func confirmCommitChanges() bool {
	opts := utils.GetSurveyOptions()

	var commitChanges bool
	prompt := &survey.Confirm{
		Message: "Do you want to commit and push these changes before deploying?",
		Default: true,
	}

	if err := survey.AskOne(prompt, &commitChanges, opts); err != nil {
		return false
	}

	return commitChanges
}

// getCommitMessage prompts user for a commit message
func getCommitMessage() (string, error) {
	opts := utils.GetSurveyOptions()

	var commitMessage string
	prompt := &survey.Input{
		Message: "Enter a commit message:",
	}

	if err := survey.AskOne(prompt, &commitMessage, opts); err != nil {
		return "", fmt.Errorf("error getting commit message: %w", err)
	}

	if strings.TrimSpace(commitMessage) == "" {
		return "", fmt.Errorf("commit message cannot be empty")
	}

	return commitMessage, nil
}

// CommitAndPushChanges performs the git add, commit, and push operations
func CommitAndPushChanges(commitMessage string) error {
	// Git add
	utils.InfoColor.Print("[+] Adding changes... ")
	if _, err := ExecuteCommand("add", "."); err != nil {
		fmt.Println()
		return fmt.Errorf("error adding files: %w", err)
	}
	utils.SuccessColor.Println("Done")

	// Git commit
	utils.InfoColor.Print("[*] Committing changes... ")
	if _, err := ExecuteCommand("commit", "-m", commitMessage); err != nil {
		fmt.Println()
		return fmt.Errorf("error committing changes: %w", err)
	}
	utils.SuccessColor.Println("Done")

	// Git push
	utils.InfoColor.Print("[^] Pushing to remote... ")
	if _, err := ExecuteCommand("push"); err != nil {
		fmt.Println()
		return fmt.Errorf("error pushing changes: %w", err)
	}
	utils.SuccessColor.Println("Done")

	return nil
}
