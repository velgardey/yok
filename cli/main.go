package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/go-git/go-git/v5"
	"github.com/gookit/color"
	"github.com/spf13/cobra"
)

// Version will be injected at build time by GoReleaser
var version = "dev"

// Define colors
var (
	infoColor    = color.New(color.FgCyan)
	errorColor   = color.New(color.FgRed, color.Bold)
	warnColor    = color.New(color.FgYellow)
	successColor = color.New(color.FgGreen, color.Bold)
)

// Constants
const (
	apiURL     = "http://api.yok.ninja"
	configFile = ".yok-config.json"
)

// HTTP client with reasonable timeout
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// Types
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GitRepoURL string `json:"gitRepoUrl"`
	Slug       string `json:"slug"`
	Framework  string `json:"framework"`
}

type ProjectResponse struct {
	Status string `json:"status"`
	Data   struct {
		Project Project `json:"project"`
	} `json:"data"`
}

type DeploymentResponse struct {
	Status string `json:"status"`
	Data   struct {
		DeploymentId  string `json:"deploymentId"`
		DeploymentUrl string `json:"deploymentUrl"`
	} `json:"data"`
}

type Config struct {
	ProjectID string `json:"projectId"`
	RepoName  string `json:"repoName"`
}

type ProjectCheckResponse struct {
	Status string `json:"status"`
	Data   struct {
		Exists  bool    `json:"exists"`
		Project Project `json:"project,omitempty"`
	} `json:"data"`
}

// New types for listing deployments and checking status
type Deployment struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DeploymentListResponse struct {
	Status string `json:"status"`
	Data   struct {
		Deployments []Deployment `json:"deployments"`
	} `json:"data"`
}

type DeploymentStatusResponse struct {
	Status string `json:"status"`
	Data   struct {
		Deployment Deployment `json:"deployment"`
	} `json:"data"`
}

// GitHub release information
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Prerelease bool   `json:"prerelease"`
}

// Execute git commands
func executeGitCommand(args ...string) (string, error) {
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

// Get the repository information from the current directory
func getRepoInfo(useManualEntry bool) (string, string, error) {
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
	if err := ensureGitRepo(); err != nil {
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
		infoColor.Print("Adding remote origin... ")
		_, err := executeGitCommand("remote", "add", "origin", remoteURLPrompt)
		if err != nil {
			return "", "", fmt.Errorf("failed to add remote: %s", err)
		}
		successColor.Println("Done")

		remoteURL = remoteURLPrompt
	}

	// Extract the repository name from the remote URL
	parts := strings.Split(remoteURL, "/")
	repoName := parts[len(parts)-1]

	// Remove the .git extension if present
	repoName = strings.TrimSuffix(repoName, ".git")

	return remoteURL, repoName, nil
}

// Detect the framework used in the repository
func detectFramework() string {
	files, _ := filepath.Glob("*")

	for _, file := range files {
		if file == "package.json" {
			data, err := os.ReadFile(file)
			if err == nil {
				content := string(data)

				if strings.Contains(content, "vite") {
					return "VITE"
				} else if strings.Contains(content, "svelte") {
					return "SVELTE"
				} else if strings.Contains(content, "react") {
					return "REACT"
				} else if strings.Contains(content, "vue") {
					return "VUE"
				} else if strings.Contains(content, "angular") {
					return "ANGULAR"
				} else if strings.Contains(content, "next") {
					return "NEXT"
				}
				return "OTHER"
			}
		}
	}

	// Check for static sites
	for _, file := range files {
		if file == "index.html" {
			return "STATIC"
		}
	}
	return "OTHER"
}

// Check if a project with the given name already exists
func findProjectByName(name string) (*Project, error) {
	// URL encode the name to handle spaces and special characters
	escapedName := url.QueryEscape(name)
	resp, err := httpClient.Get(apiURL + "/project/check?name=" + escapedName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle non-success status codes
	if resp.StatusCode != http.StatusOK {
		// If endpoint doesn't exist (404), the API might be an older version
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil // Just return nil to indicate no project found
		}
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var checkResp ProjectCheckResponse
	if err := json.Unmarshal(body, &checkResp); err != nil {
		return nil, err
	}

	// If project exists, return it
	if checkResp.Status == "success" && checkResp.Data.Exists {
		return &checkResp.Data.Project, nil
	}

	return nil, nil
}

// Create or get a project
func getOrCreateProject(name, repoURL, framework string) (*Project, error) {
	// Check if project already exists by name
	existingProject, err := findProjectByName(name)
	if err != nil {
		return nil, fmt.Errorf("error checking for existing project: %v", err)
	}

	if existingProject != nil {
		infoColor.Printf("Project '%s' already exists. Using existing project.\n", name)
		return existingProject, nil
	}

	// Project doesn't exist, create it
	s := startSpinner("Creating project on Yok...")
	defer stopSpinner(s)

	projectData := map[string]string{
		"name":       name,
		"gitRepoUrl": repoURL,
		"framework":  framework,
	}

	jsonData, err := json.Marshal(projectData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL+"/project", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to create project: %s", string(body))
	}

	var projectResp ProjectResponse
	if err := json.Unmarshal(body, &projectResp); err != nil {
		return nil, err
	}

	return &projectResp.Data.Project, nil
}

// startSpinner creates and starts a new spinner with the given message
func startSpinner(message string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[25], 700*time.Millisecond)
	s.Suffix = " " + message
	s.Start()
	return s
}

// stopSpinner safely stops a spinner
func stopSpinner(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}

// Deploy project at api.yok.ninja/deploy
func deployProject(projectID string) (*DeploymentResponse, error) {
	s := startSpinner("Deploying project to Yok...")
	defer stopSpinner(s)

	deployData := map[string]string{
		"projectId": projectID,
	}

	jsonData, err := json.Marshal(deployData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL+"/deploy", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("failed to deploy project: %s", string(body))
	}

	var deploymentResp DeploymentResponse
	if err := json.Unmarshal(body, &deploymentResp); err != nil {
		return nil, err
	}
	return &deploymentResp, nil
}

// Save configuration to a local file
func saveConfig(config Config) error {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, jsonData, 0644)
}

// Load configuration from a local file
func loadConfig() (Config, error) {
	var config Config
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return empty config if file doesn't exist
		}
		return config, err
	}
	err = json.Unmarshal(data, &config)
	return config, err
}

// handleError prints error messages and exits with non-zero code if err is not nil
func handleError(err error, message string) {
	if err != nil {
		errorColor.Printf("%s: %v\n", message, err)
		os.Exit(1)
	}
}

// Get status of a deployment
func getDeploymentStatus(deploymentID string) (*Deployment, error) {
	resp, err := httpClient.Get(apiURL + "/deployment/" + deploymentID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var statusResp DeploymentStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, err
	}

	return &statusResp.Data.Deployment, nil
}

// List deployments for a project
func listDeployments(projectID string) ([]Deployment, error) {
	resp, err := httpClient.Get(apiURL + "/project/" + projectID + "/deployments")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var listResp DeploymentListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, err
	}

	return listResp.Data.Deployments, nil
}

// Cancel a deployment
func cancelDeployment(deploymentID string) error {
	cancelData := map[string]string{
		"deploymentId": deploymentID,
	}

	jsonData, err := json.Marshal(cancelData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", apiURL+"/deployment/"+deploymentID+"/cancel", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to cancel deployment: %s", string(body))
	}

	return nil
}

// Check if local changes match remote
func checkLocalRemoteSync() (bool, error) {
	// Fetch latest from remote
	_, err := executeGitCommand("fetch")
	if err != nil {
		return false, fmt.Errorf("failed to fetch from remote: %v", err)
	}

	// Check if we're behind the remote
	behindOutput, err := executeGitCommand("rev-list", "--count", "HEAD..@{upstream}")
	if err != nil {
		return false, fmt.Errorf("failed to check if behind remote: %v", err)
	}
	behindCount := strings.TrimSpace(behindOutput)
	if behindCount != "0" {
		return false, fmt.Errorf("your local branch is %s commits behind the remote", behindCount)
	}

	// Check if we're ahead of the remote
	aheadOutput, err := executeGitCommand("rev-list", "--count", "@{upstream}..HEAD")
	if err != nil {
		return false, fmt.Errorf("failed to check if ahead of remote: %v", err)
	}
	aheadCount := strings.TrimSpace(aheadOutput)
	if aheadCount != "0" {
		return false, fmt.Errorf("your local branch is %s commits ahead of the remote", aheadCount)
	}

	// Check for uncommitted changes
	statusOutput, err := executeGitCommand("status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check for uncommitted changes: %v", err)
	}
	if statusOutput != "" {
		return false, fmt.Errorf("you have uncommitted changes")
	}

	return true, nil
}

// formatDeploymentStatus prints a deployment status with appropriate coloring
func formatDeploymentStatus(status string) {
	switch status {
	case "COMPLETED":
		successColor.Printf("Status: %s\n", status)
	case "FAILED":
		errorColor.Printf("Status: %s\n", status)
	case "PENDING", "QUEUED", "IN_PROGRESS":
		infoColor.Printf("Status: %s\n", status)
	default:
		fmt.Printf("Status: %s\n", status)
	}
}

// formatTableRow prints a row in the deployments table with colored status
func formatTableRow(id string, status string, createdAt time.Time) {
	shortID := id
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	fmt.Printf("%-10s ", shortID)
	switch status {
	case "COMPLETED":
		successColor.Printf("%-12s ", status)
	case "FAILED":
		errorColor.Printf("%-12s ", status)
	case "PENDING", "QUEUED", "IN_PROGRESS":
		infoColor.Printf("%-12s ", status)
	default:
		fmt.Printf("%-12s ", status)
	}
	fmt.Printf("%-20s\n", createdAt.Format("Jan 02 15:04:05"))
}

// selectDeploymentFromList prompts the user to select a deployment from a list
// filter can be used to filter deployments by status (e.g. only in-progress deployments)
// if filter is nil, all deployments are shown
func selectDeploymentFromList(projectID string, filter func(Deployment) bool) (string, error) {
	// Get recent deployments
	deployments, err := listDeployments(projectID)
	if err != nil {
		return "", fmt.Errorf("error fetching deployments: %v", err)
	}

	// Filter deployments if a filter is provided
	filteredDeployments := []Deployment{}
	if filter != nil {
		for _, d := range deployments {
			if filter(d) {
				filteredDeployments = append(filteredDeployments, d)
			}
		}
	} else {
		filteredDeployments = deployments
	}

	if len(filteredDeployments) == 0 {
		return "", fmt.Errorf("no matching deployments found")
	}

	// Create options for selection
	options := make([]string, len(filteredDeployments))
	for i, d := range filteredDeployments {
		timeAgo := time.Since(d.CreatedAt).Round(time.Second)
		options[i] = fmt.Sprintf("%s (%s) - %s - %s ago",
			d.ID[:8], d.Status, d.CreatedAt.Format("Jan 02 15:04"), timeAgo)
	}

	var selected int
	prompt := &survey.Select{
		Message: "Select a deployment:",
		Options: options,
	}
	survey.AskOne(prompt, &selected)

	return filteredDeployments[selected].ID, nil
}

// getProjectIDOrExit loads the config and exits if no project ID is found
func getProjectIDOrExit() Config {
	config, err := loadConfig()
	handleError(err, "Error loading configuration")

	if config.ProjectID == "" {
		errorColor.Println("No project configured. Run 'yok create' or 'yok deploy' first.")
		os.Exit(1)
	}

	return config
}

// promptForProjectCreationDetails asks the user for a project name, checks if it exists, and
// gets Git repo info. Returns project details and a flag indicating if the user is using an existing project.
func promptForProjectCreationDetails() (string, string, string, *Project, bool, error) {
	// Get project name
	var projectName string
	prompt := &survey.Input{
		Message: "Enter a name for your project:",
	}
	if err := survey.AskOne(prompt, &projectName); err != nil {
		return "", "", "", nil, false, fmt.Errorf("error getting project name: %v", err)
	}

	if projectName == "" {
		return "", "", "", nil, false, fmt.Errorf("project name cannot be empty")
	}

	// Check if a project with this name already exists
	existingProject, err := findProjectByName(projectName)
	if err != nil {
		warnColor.Printf("Warning: Could not check if project exists: %v\n", err)
		// Continue anyway, the creation step will fail if there's a duplicate
	} else if existingProject != nil {
		infoColor.Printf("Project with name '%s' already exists!\n", projectName)

		// Ask if user wants to use the existing project
		useExisting := false
		confirmPrompt := &survey.Confirm{
			Message: "Do you want to use this existing project?",
			Default: true,
		}
		survey.AskOne(confirmPrompt, &useExisting)

		if useExisting {
			// User wants to use the existing project
			return projectName, existingProject.GitRepoURL, existingProject.Framework, existingProject, true, nil
		}
		// User chose not to use existing project, ask for a different name
		return "", "", "", nil, false, fmt.Errorf("a project with this name already exists, please choose a different name")
	}

	// Ask whether to auto-detect git repo or enter manually
	repoOptions := []string{
		"Auto-detect Git repository from current directory",
		"Manually enter Git repository URL",
	}
	repoOptionIndex := 0
	repoPrompt := &survey.Select{
		Message: "How would you like to specify the Git repository?",
		Options: repoOptions,
		Default: 0,
	}
	if err := survey.AskOne(repoPrompt, &repoOptionIndex); err != nil {
		return "", "", "", nil, false, fmt.Errorf("error getting repository option: %v", err)
	}

	// Get repository info based on user's choice
	useManualEntry := repoOptionIndex == 1
	repoURL, _, err := getRepoInfo(useManualEntry)
	if err != nil {
		return "", "", "", nil, false, fmt.Errorf("error getting repository info: %v", err)
	}

	framework := detectFramework()

	return projectName, repoURL, framework, nil, false, nil
}

// ensureProjectID loads config and ensures a project ID exists, creating a project if needed
func ensureProjectID() (Config, error) {
	// Load config to check if we have a stored project ID
	config, err := loadConfig()
	if err != nil {
		return config, fmt.Errorf("error loading configuration: %v", err)
	}

	// If no stored project ID, we need to create/find one
	if config.ProjectID == "" {
		projectName, repoURL, framework, existingProject, usingExisting, err := promptForProjectCreationDetails()

		if err != nil {
			return config, err
		}

		if usingExisting {
			// Use existing project
			successColor.Printf("✅ Using existing project: %s\n", existingProject.Name)

			// Save project ID for future use
			config.ProjectID = existingProject.ID
			config.RepoName = existingProject.Name
			if err := saveConfig(config); err != nil {
				warnColor.Printf("Warning: Could not save project ID: %v\n", err)
			}

			return config, nil
		}

		// Create or get existing project (double-check since another user might have created it)
		project, err := getOrCreateProject(projectName, repoURL, framework)
		if err != nil {
			return config, fmt.Errorf("error creating project: %v", err)
		}

		successColor.Printf("✅ Using project: %s\n", project.Name)

		// Save project ID for future use
		config.ProjectID = project.ID
		config.RepoName = project.Name
		if err := saveConfig(config); err != nil {
			warnColor.Printf("Warning: Could not save project ID: %v\n", err)
		}
	} else {
		infoColor.Printf("Using stored project ID for: %s\n", config.RepoName)
	}

	return config, nil
}

// New function to get a project by ID
func getProject(projectID string) (*Project, error) {
	// Try to get the project directly by ID first
	resp, err := httpClient.Get(apiURL + "/project/" + projectID)
	if err != nil {
		return nil, err
	}

	// If the endpoint doesn't exist or returns an error, try the deployments list endpoint as a fallback
	if resp.StatusCode != http.StatusOK {
		// If the /project/:id endpoint is not available, we'll try a workaround
		// by listing deployments and looking up the project from there
		resp.Body.Close()

		// Get the deployments for this project
		deploymentsResp, err := httpClient.Get(apiURL + "/project/" + projectID + "/deployments")
		if err != nil {
			return nil, err
		}
		defer deploymentsResp.Body.Close()

		if deploymentsResp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get project or deployments, API returned status code: %d", deploymentsResp.StatusCode)
		}

		body, err := io.ReadAll(deploymentsResp.Body)
		if err != nil {
			return nil, err
		}

		var listResp DeploymentListResponse
		if err := json.Unmarshal(body, &listResp); err != nil {
			return nil, err
		}

		if len(listResp.Data.Deployments) > 0 {
			// We have a deployment, but we still don't have the project slug
			// Return a project with just the ID filled in
			return &Project{
				ID: projectID,
				// Other fields will be empty
			}, nil
		}

		return nil, fmt.Errorf("no deployments found for project")
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var projectResp ProjectResponse
	if err := json.Unmarshal(body, &projectResp); err != nil {
		return nil, err
	}

	return &projectResp.Data.Project, nil
}

// followDeploymentStatus follows the status of a deployment until completion or failure
func followDeploymentStatus(deploymentID string, deploymentURL string, projectID string) {
	// Create spinner with specific configuration to prevent clearing previous lines
	s := spinner.New(spinner.CharSets[25], 700*time.Millisecond)
	s.Suffix = " Waiting for deployment to complete..."
	s.Writer = os.Stdout
	s.FinalMSG = "" // No final message - we'll print our own
	s.Start()

	for {
		time.Sleep(3 * time.Second) // Check every 3 seconds

		status, err := getDeploymentStatus(deploymentID)
		if err != nil {
			s.Stop()
			warnColor.Printf("\nFailed to get deployment status: %v\n", err)
			break
		}

		if status.Status == "COMPLETED" {
			s.Stop()
			successColor.Printf("\n✅ Deployment completed successfully!\n")

			// Try to get the project slug for a nicer URL
			project, err := getProject(projectID)
			if err == nil && project.Slug != "" {
				infoColor.Printf("ℹ️ Your site is available at:\n")
				fmt.Printf("- https://%s.yok.ninja\n", project.Slug)
				fmt.Printf("- %s\n", deploymentURL)
			} else {
				// If we couldn't get the project or it doesn't have a slug, just show the deployment URL
				infoColor.Printf("ℹ️ Your site is now available at: %s\n", deploymentURL)
			}
			break
		} else if status.Status == "FAILED" {
			s.Stop()
			errorColor.Printf("\n❌ Deployment failed\n")
			break
		}
		// Continue waiting for other status values
	}
}

// Check and initialize Git repo if needed
func ensureGitRepo() error {
	_, err := os.Stat(".git")
	if os.IsNotExist(err) {
		infoColor.Print("No Git repository found. Initializing... ")
		_, err := executeGitCommand("init")
		if err != nil {
			return fmt.Errorf("failed to initialize git repo: %v", err)
		}
		successColor.Println("Done")
	}
	return nil
}

// Check for uncommitted changes and offer to commit and push them
func handleUncommittedChanges() error {
	// Check for uncommitted changes
	statusOutput, err := executeGitCommand("status", "--porcelain")
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
	infoColor.Print("📝 Adding changes... ")
	_, err = executeGitCommand("add", ".")
	if err != nil {
		fmt.Println()
		return fmt.Errorf("error adding files: %v", err)
	}
	successColor.Println("Done")

	// Git commit
	infoColor.Print("💾 Committing changes... ")
	_, err = executeGitCommand("commit", "-m", commitMessage)
	if err != nil {
		fmt.Println()
		return fmt.Errorf("error committing changes: %v", err)
	}
	successColor.Println("Done")

	// Git push
	infoColor.Print("🚀 Pushing to remote... ")
	_, err = executeGitCommand("push")
	if err != nil {
		fmt.Println()
		return fmt.Errorf("error pushing changes: %v", err)
	}
	successColor.Println("Done")

	return nil
}

// Get the latest release info from GitHub
func getLatestRelease() (*GitHubRelease, error) {
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

	// Send the request
	resp, err := httpClient.Do(req)
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

	// Read and parse response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("error parsing GitHub response: %v", err)
	}

	// Validate response
	if release.TagName == "" {
		return nil, fmt.Errorf("received empty tag name from GitHub")
	}

	return &release, nil
}

// Compare version strings to check if latest is newer than current
// Returns true if latest version is newer than current version
func compareVersions(current, latest string) bool {
	// Strip 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Special case handling
	switch {
	case current == "dev" || current == "development":
		return true // Always update development versions
	case latest == "":
		return false // Can't update to empty version
	case current == "":
		return true // Empty current version should update
	}

	// Parse versions into components
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	// Compare each version component
	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}

	for i := 0; i < maxLen; i++ {
		// If we run out of parts in one version, that version is older
		if i >= len(currentParts) {
			return true // Latest has more parts, so it's newer
		}
		if i >= len(latestParts) {
			return false // Current has more parts, so it's newer
		}

		// Try to compare as integers
		currentNum, currentErr := strconv.Atoi(currentParts[i])
		latestNum, latestErr := strconv.Atoi(latestParts[i])

		if currentErr == nil && latestErr == nil {
			// Both are numeric, compare as numbers
			if latestNum > currentNum {
				return true
			}
			if latestNum < currentNum {
				return false
			}
			// Equal components, continue to next component
		} else {
			// At least one is non-numeric, compare as strings
			if currentParts[i] != latestParts[i] {
				return latestParts[i] > currentParts[i]
			}
			// Equal components, continue to next component
		}
	}

	// All components equal
	return false
}

// Platform-specific update logic
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	return cmd.Run()
}

// Add self-update command
var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update Yok CLI to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSelfUpdate(); err != nil {
			errorColor.Printf("%v\n", err)
			os.Exit(1)
		}
	},
}

// Separate function for update logic to improve testability and readability
func runSelfUpdate() error {
	// Get the latest version from GitHub API
	infoColor.Print("Checking for updates... ")
	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("error checking for updates: %v", err)
	}
	fmt.Println("Done")

	latestVersion := release.TagName

	// Check if current version is already the latest
	if !compareVersions(version, latestVersion) {
		successColor.Printf("You're already using the latest version (v%s)\n", version)
		return nil
	}

	// Display update information
	infoColor.Printf("\nAvailable update:\n")
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
		infoColor.Println("Update cancelled")
		return nil
	}

	// Run update process
	infoColor.Println("Updating Yok CLI...")

	// Start a spinner for visual feedback
	s := startSpinner("Downloading and installing update...")
	err = updateBinary()
	stopSpinner(s)

	if err != nil {
		// Show more detailed troubleshooting help
		warnColor.Println("\nTroubleshooting tips:")
		fmt.Println("1. Check your internet connection")
		fmt.Println("2. Make sure you have permission to write to the installation directory")
		fmt.Println("3. Try running with elevated privileges (sudo/admin)")
		fmt.Printf("4. Try manual installation from: https://github.com/velgardey/yok/releases/tag/%s\n", latestVersion)

		return fmt.Errorf("update failed: %v", err)
	}

	successColor.Printf("✅ Updated to version %s successfully!\n", latestVersion)
	return nil
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "yok",
		Short: "Yok CLI - Git Wrapper and Deployment Tool",
		Long:  "Yok CLI is a git wrapper and a deployment tool that allows you to deploy your static web applications directly from your git repository.",
	}

	var deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy your project to the web using Yok",
		Run: func(cmd *cobra.Command, args []string) {
			// Get project ID
			config, err := ensureProjectID()
			handleError(err, "Error setting up project")

			// Check if local branch is in sync with remote
			infoColor.Print("Checking local/remote sync... ")
			_, err = checkLocalRemoteSync()
			if err != nil {
				fmt.Println()
				warnColor.Printf("Warning: %v\n", err)

				// Check for uncommitted changes and handle them
				err = handleUncommittedChanges()
				if err != nil {
					warnColor.Printf("Warning: %v\n", err)

					// Ask user if they want to continue anyway
					continueDeploy := false
					syncPrompt := &survey.Confirm{
						Message: "Do you want to continue with deployment anyway?",
						Default: false,
					}
					survey.AskOne(syncPrompt, &continueDeploy)

					if !continueDeploy {
						errorColor.Println("Deployment cancelled")
						os.Exit(1)
					}
				}
			} else {
				successColor.Println("Done")
			}

			// Deploy project with stored ID
			deployment, err := deployProject(config.ProjectID)
			handleError(err, "Error deploying project")

			successColor.Printf("✅ Deployment triggered: %s\n", deployment.Data.DeploymentId)

			// Automatically follow the deployment status
			followDeploymentStatus(deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
		},
	}

	var shipCmd = &cobra.Command{
		Use:   "ship",
		Short: "Commit, push, and deploy your project to the web using Yok",
		Run: func(cmd *cobra.Command, args []string) {
			// Get commit message
			var commitMessage string
			msgPrompt := &survey.Input{
				Message: "Enter a commit message:",
			}
			err := survey.AskOne(msgPrompt, &commitMessage)
			handleError(err, "Error getting commit message")

			if commitMessage == "" {
				errorColor.Println("Error: Commit message cannot be empty")
				os.Exit(1)
			}

			// Git add
			infoColor.Print("📝 Adding changes... ")
			_, err = executeGitCommand("add", ".")
			if err != nil {
				fmt.Println()
				handleError(err, "Error adding files")
			}
			successColor.Println("Done")

			// Git commit
			infoColor.Print("💾 Committing changes... ")
			_, err = executeGitCommand("commit", "-m", commitMessage)
			if err != nil {
				fmt.Println()
				handleError(err, "Error committing changes")
			}
			successColor.Println("Done")

			// Git push
			infoColor.Print("🚀 Pushing to remote... ")
			_, err = executeGitCommand("push")
			if err != nil {
				fmt.Println()
				handleError(err, "Error pushing changes")
			}
			successColor.Println("Done")

			// Get project ID and deploy
			config, err := ensureProjectID()
			handleError(err, "Error setting up project")

			// Deploy project with stored ID
			deployment, err := deployProject(config.ProjectID)
			handleError(err, "Error deploying project")

			successColor.Printf("✅ Deployment triggered: %s\n", deployment.Data.DeploymentId)

			// Automatically follow the deployment status
			followDeploymentStatus(deployment.Data.DeploymentId, deployment.Data.DeploymentUrl, config.ProjectID)
		},
	}

	// Add new commands for deployment management
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
				config := getProjectIDOrExit()

				// Select a deployment
				deploymentId, err = selectDeploymentFromList(config.ProjectID, nil)
				if err != nil {
					if err.Error() == "no matching deployments found" {
						infoColor.Println("No deployments found for this project.")
						os.Exit(0)
					}
					handleError(err, "Error selecting deployment")
				}
			} else {
				deploymentId = args[0]
			}

			// Get deployment status
			s := startSpinner("Fetching deployment status...")

			status, err := getDeploymentStatus(deploymentId)
			stopSpinner(s)

			if err != nil {
				errorColor.Printf("Failed to get deployment status: %v\n", err)
				os.Exit(1)
			}

			// Print status
			fmt.Println("Deployment Status:")
			fmt.Printf("ID: %s\n", status.ID)

			// Color-coded status
			formatDeploymentStatus(status.Status)

			fmt.Printf("Created: %s (%s ago)\n",
				status.CreatedAt.Format(time.RFC3339),
				time.Since(status.CreatedAt).Round(time.Second))
			fmt.Printf("Last Updated: %s (%s ago)\n",
				status.UpdatedAt.Format(time.RFC3339),
				time.Since(status.UpdatedAt).Round(time.Second))
		},
	}

	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all deployments for your project",
		Run: func(cmd *cobra.Command, args []string) {
			// Get project ID and ensure it exists
			config := getProjectIDOrExit()

			// Get deployments
			s := startSpinner("Fetching deployments...")

			deployments, err := listDeployments(config.ProjectID)
			stopSpinner(s)

			if err != nil {
				errorColor.Printf("Failed to list deployments: %v\n", err)
				os.Exit(1)
			}

			if len(deployments) == 0 {
				infoColor.Println("No deployments found for this project.")
				return
			}

			// Print deployments table
			fmt.Println("\nDeployments for", config.RepoName)
			fmt.Println("---------------------------------------")
			fmt.Printf("%-10s %-12s %-20s\n", "ID", "STATUS", "CREATED")
			fmt.Println("---------------------------------------")

			for _, d := range deployments {
				formatTableRow(d.ID, d.Status, d.CreatedAt)
			}
		},
	}

	var cancelCmd = &cobra.Command{
		Use:   "cancel [deploymentId]",
		Short: "Cancel a running deployment",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var deploymentId string

			// If no deployment ID provided, ask the user to select from recent in-progress deployments
			if len(args) == 0 {
				// Load config and ensure project ID exists
				config := getProjectIDOrExit()

				// Select a deployment that is in progress
				var err error
				deploymentId, err = selectDeploymentFromList(config.ProjectID, func(d Deployment) bool {
					return d.Status == "PENDING" || d.Status == "QUEUED" || d.Status == "IN_PROGRESS"
				})
				if err != nil {
					if err.Error() == "no matching deployments found" {
						infoColor.Println("No in-progress deployments found to cancel.")
						os.Exit(0)
					}
					handleError(err, "Error selecting deployment")
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
			survey.AskOne(cancelPrompt, &confirm)

			if !confirm {
				infoColor.Println("Cancellation aborted.")
				return
			}

			// Cancel deployment
			s := startSpinner("Cancelling deployment...")

			err := cancelDeployment(deploymentId)
			stopSpinner(s)

			if err != nil {
				errorColor.Printf("Failed to cancel deployment: %v\n", err)
				os.Exit(1)
			}

			successColor.Println("✅ Deployment cancelled successfully")
		},
	}

	var createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new project on Yok",
		Run: func(cmd *cobra.Command, args []string) {
			projectName, repoURL, framework, existingProject, usingExisting, err := promptForProjectCreationDetails()
			if err != nil {
				errorColor.Printf("Error: %v\n", err)
				os.Exit(1)
			}

			if usingExisting {
				// Display project info and save the project ID
				successColor.Printf("✅ Using existing project\n")

				// Display comprehensive project info
				fmt.Println("\nProject Information:")
				fmt.Printf("ID: %s\n", existingProject.ID)
				fmt.Printf("Name: %s\n", existingProject.Name)
				fmt.Printf("Framework: %s\n", existingProject.Framework)
				fmt.Printf("Slug: %s\n", existingProject.Slug)
				fmt.Printf("Git URL: %s\n", existingProject.GitRepoURL)
				if existingProject.Slug != "" {
					fmt.Printf("Project URL: https://%s.yok.ninja\n", existingProject.Slug)
				}

				// Save project ID
				config := Config{
					ProjectID: existingProject.ID,
					RepoName:  existingProject.Name,
				}
				err = saveConfig(config)
				if err != nil {
					warnColor.Printf("Warning: Could not save project ID: %v\n", err)
				} else {
					successColor.Println("\n✅ Project ID saved for future deployments")
				}
				return
			}

			// Create or get existing project
			project, err := getOrCreateProject(projectName, repoURL, framework)
			handleError(err, "Error creating project")

			successColor.Printf("✅ Project created/updated successfully\n")

			// Display comprehensive project info
			fmt.Println("\nProject Information:")
			fmt.Printf("ID: %s\n", project.ID)
			fmt.Printf("Name: %s\n", project.Name)
			fmt.Printf("Framework: %s\n", project.Framework)
			fmt.Printf("Slug: %s\n", project.Slug)
			fmt.Printf("Git URL: %s\n", project.GitRepoURL)
			fmt.Printf("Project URL: https://%s.yok.ninja\n", project.Slug)

			// Save project ID
			config := Config{
				ProjectID: project.ID,
				RepoName:  project.Name,
			}
			err = saveConfig(config)
			if err != nil {
				warnColor.Printf("Warning: Could not save project ID: %v\n", err)
			} else {
				successColor.Println("\n✅ Project ID saved for future deployments")
			}
		},
	}

	// Define resetConfigCmd separately from git reset command
	var resetConfigCmd = &cobra.Command{
		Use:   "reset-config",
		Short: "Reset stored project ID configuration",
		Run: func(cmd *cobra.Command, args []string) {
			cwd, err := os.Getwd()
			if err != nil {
				handleError(err, "Error getting current directory")
				return
			}

			configFilePath := filepath.Join(cwd, configFile)
			fmt.Printf("Attempting to remove config file at: %s\n", configFilePath)

			// Force remove the file
			err = os.RemoveAll(configFilePath)
			if err != nil {
				handleError(err, "Error removing config file")
			} else {
				successColor.Println("✅ Project configuration reset successfully")
			}
		},
	}

	// Original resetCmd for git reset integration
	var resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Reset stored project ID",
		Run: func(cmd *cobra.Command, args []string) {
			// Explicitly check if file exists first
			cwd, err := os.Getwd()
			if err != nil {
				handleError(err, "Error getting current directory")
			}

			configFilePath := filepath.Join(cwd, configFile)

			// Check if file exists
			_, err = os.Stat(configFilePath)
			if err == nil {
				// File exists, remove it
				err = os.RemoveAll(configFilePath)
				handleError(err, "Error removing config file")
				successColor.Println("✅ Project configuration reset successfully")
			} else if os.IsNotExist(err) {
				// File doesn't exist
				infoColor.Println("No project configuration found")
			} else {
				// Some other error
				handleError(err, "Error checking config file")
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

	// List of common git commands
	gitCommands := []string{
		"add", "commit", "push", "pull", "checkout", "branch", "status",
		"log", "fetch", "merge", "rebase", "reset", "tag", "stash",
	}

	// Add all common git commands as explicit subcommands
	for _, gitCmd := range gitCommands {
		cmd := &cobra.Command{
			Use:   gitCmd,
			Short: fmt.Sprintf("Execute git %s", gitCmd),
			Run: func(gitCmd string) func(cmd *cobra.Command, args []string) {
				return func(cmd *cobra.Command, args []string) {
					allArgs := append([]string{gitCmd}, args...)
					output, err := executeGitCommand(allArgs...)
					if err != nil {
						errorColor.Printf("Error: %v\n", err)
						os.Exit(1)
					}
					fmt.Print(output)
				}
			}(gitCmd),
			DisableFlagParsing: true,
		}
		rootCmd.AddCommand(cmd)
	}

	// Add a fallback command handler for all other git commands
	var fallbackCmd = &cobra.Command{
		Use:   "git",
		Short: "Execute any other git command",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			output, err := executeGitCommand(args...)
			if err != nil {
				errorColor.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Print(output)
		},
		DisableFlagParsing: true,
	}

	// Add commands to root
	rootCmd.AddCommand(deployCmd, shipCmd, createCmd, resetCmd, resetConfigCmd, fallbackCmd,
		statusCmd, listCmd, cancelCmd, selfUpdateCmd, versionCmd)

	// Set up special handling for unknown commands to pass them to git
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		// Check if the command is a git command that we don't explicitly handle
		if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
			output, cmdErr := executeGitCommand(os.Args[1:]...)
			if cmdErr == nil {
				fmt.Print(output)
				os.Exit(0)
			}
		}
		return err
	})

	if err := rootCmd.Execute(); err != nil {
		errorColor.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
