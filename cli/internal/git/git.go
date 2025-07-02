package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/go-git/go-git/v5"
	"github.com/gookit/color"
)

// InfoColor and SuccessColor for console output
var (
	InfoColor    = color.New(color.FgCyan)
	SuccessColor = color.New(color.FgGreen, color.Bold)
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
func GetRepoInfo(useManualEntry bool) (string, string, error) {
	// If manual entry is requested, prompt for the Git repo URL
	if useManualEntry {
		var repoURL string
		prompt := &survey.Input{
			Message: "Enter your Git repository URL:",
			Help:    "This should be the HTTPS or SSH URL to your Git repository",
		}
		if err := survey.AskOne(prompt, &repoURL); err != nil {
			return "", "", fmt.Errorf("failed to get repo URL: %s", err)
		}

		if repoURL == "" {
			return "", "", fmt.Errorf("repository URL cannot be empty")
		}

		// Extract the repository name from the URL
		parts := strings.Split(repoURL, "/")
		repoName := parts[len(parts)-1]
		// Remove the .git extension if present
		repoName = strings.TrimSuffix(repoName, ".git")

		return repoURL, repoName, nil
	}

	// Auto-detection flow
	// Make sure we have a git repository
	if err := EnsureRepo(); err != nil {
		return "", "", err
	}

	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", "", fmt.Errorf("error opening git repository: %v", err)
	}

	// Get the remote URL
	remotes, err := repo.Remotes()
	if err != nil {
		return "", "", fmt.Errorf("failed to get remotes: %s", err)
	}

	// Try to find the origin remote
	var remoteURL string
	for _, remote := range remotes {
		if remote.Config().Name == "origin" && len(remote.Config().URLs) > 0 {
			remoteURL = remote.Config().URLs[0]
			break
		}
	}

	// If no remote URL found, try to get the default remote
	if remoteURL == "" && len(remotes) > 0 {
		remoteURL = remotes[0].Config().URLs[0]
	}

	if remoteURL == "" {
		// No remote found, prompt the user for a remote URL
		var remoteURLPrompt string
		prompt := &survey.Input{
			Message: "No remote found. Please enter a git remote URL:",
		}
		if err := survey.AskOne(prompt, &remoteURLPrompt); err != nil {
			return "", "", fmt.Errorf("failed to get remote URL: %s", err)
		}

		if remoteURLPrompt == "" {
			return "", "", fmt.Errorf("remote URL cannot be empty")
		}

		// Add the remote URL quietly in the background
		InfoColor.Print("Adding remote origin... ")
		_, err := ExecuteCommand("remote", "add", "origin", remoteURLPrompt)
		if err != nil {
			return "", "", fmt.Errorf("failed to add remote: %s", err)
		}
		SuccessColor.Println("Done")

		remoteURL = remoteURLPrompt
	}

	// Extract the repository name from the remote URL
	parts := strings.Split(remoteURL, "/")
	repoName := parts[len(parts)-1]

	// Remove the .git extension if present
	repoName = strings.TrimSuffix(repoName, ".git")

	return remoteURL, repoName, nil
}

// EnsureRepo ensures that the current directory is a git repository
func EnsureRepo() error {
	_, err := os.Stat(".git")
	if os.IsNotExist(err) {
		InfoColor.Print("No Git repository found. Initializing... ")
		_, err := ExecuteCommand("init")
		if err != nil {
			return fmt.Errorf("failed to initialize git repo: %v", err)
		}
		SuccessColor.Println("Done")
	}
	return nil
}

// CheckLocalRemoteSync checks if local changes match remote
func CheckLocalRemoteSync() (bool, error) {
	// Fetch latest from remote
	_, err := ExecuteCommand("fetch")
	if err != nil {
		return false, fmt.Errorf("failed to fetch from remote: %v", err)
	}

	// Check if we're behind the remote
	behindOutput, err := ExecuteCommand("rev-list", "--count", "HEAD..@{upstream}")
	if err != nil {
		return false, fmt.Errorf("failed to check if behind remote: %v", err)
	}
	behindCount := strings.TrimSpace(behindOutput)
	if behindCount != "0" {
		return false, fmt.Errorf("your local branch is %s commits behind the remote", behindCount)
	}

	// Check if we're ahead of the remote
	aheadOutput, err := ExecuteCommand("rev-list", "--count", "@{upstream}..HEAD")
	if err != nil {
		return false, fmt.Errorf("failed to check if ahead of remote: %v", err)
	}
	aheadCount := strings.TrimSpace(aheadOutput)
	if aheadCount != "0" {
		return false, fmt.Errorf("your local branch is %s commits ahead of the remote", aheadCount)
	}

	// Check for uncommitted changes
	statusOutput, err := ExecuteCommand("status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}
	if statusOutput != "" {
		return false, fmt.Errorf("you have uncommitted changes")
	}

	return true, nil
}

// HandleUncommittedChanges checks for uncommitted changes and offers to commit and push them
func HandleUncommittedChanges() error {
	// Check for uncommitted changes
	statusOutput, err := ExecuteCommand("status", "--porcelain")
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}

	if statusOutput == "" {
		// No changes to commit
		return nil
	}

	// We have uncommitted changes, ask user if they want to commit them
	fmt.Println("Uncommitted changes detected:")
	fmt.Println(statusOutput)

	commitChanges := false
	commitPrompt := &survey.Confirm{
		Message: "Do you want to commit and push these changes before deploying?",
		Default: true,
	}
	survey.AskOne(commitPrompt, &commitChanges)

	if !commitChanges {
		return fmt.Errorf("you have uncommitted changes")
	}

	// Get commit message
	var commitMessage string
	msgPrompt := &survey.Input{
		Message: "Enter a commit message:",
	}
	err = survey.AskOne(msgPrompt, &commitMessage)
	if err != nil {
		return fmt.Errorf("error getting commit message: %v", err)
	}

	if commitMessage == "" {
		return fmt.Errorf("commit message cannot be empty")
	}

	// Git add
	InfoColor.Print("📝 Adding changes... ")
	_, err = ExecuteCommand("add", ".")
	if err != nil {
		fmt.Println()
		return fmt.Errorf("error adding files: %v", err)
	}
	SuccessColor.Println("Done")

	// Git commit
	InfoColor.Print("💾 Committing changes... ")
	_, err = ExecuteCommand("commit", "-m", commitMessage)
	if err != nil {
		fmt.Println()
		return fmt.Errorf("error committing changes: %v", err)
	}
	SuccessColor.Println("Done")

	// Git push
	InfoColor.Print("🚀 Pushing to remote... ")
	_, err = ExecuteCommand("push")
	if err != nil {
		fmt.Println()
		return fmt.Errorf("error pushing changes: %v", err)
	}
	SuccessColor.Println("Done")

	return nil
}
