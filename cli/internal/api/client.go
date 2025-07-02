package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/velgardey/yok/cli/internal/types"
	"github.com/velgardey/yok/cli/internal/utils"
)

const (
	// ApiURL is the base URL for the Yok API
	ApiURL = "http://api.yok.ninja"
)

// HTTP client with reasonable timeout
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// FindProjectByName checks if a project with the given name already exists
func FindProjectByName(name string) (*types.Project, error) {
	// URL encode the name to handle spaces and special characters
	escapedName := url.QueryEscape(name)
	resp, err := httpClient.Get(ApiURL + "/project/check?name=" + escapedName)
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

	var checkResp types.ProjectCheckResponse
	if err := json.Unmarshal(body, &checkResp); err != nil {
		return nil, err
	}

	// If project exists, return it
	if checkResp.Status == "success" && checkResp.Data.Exists {
		return &checkResp.Data.Project, nil
	}

	return nil, nil
}

// GetOrCreateProject creates or gets a project
func GetOrCreateProject(name, repoURL, framework string) (*types.Project, error) {
	// Check if project already exists by name
	existingProject, err := FindProjectByName(name)
	if err != nil {
		return nil, fmt.Errorf("error checking for existing project: %v", err)
	}

	if existingProject != nil {
		utils.InfoColor.Printf("Project '%s' already exists. Using existing project.\n", name)
		return existingProject, nil
	}

	// Project doesn't exist, create it
	s := utils.StartSpinner("Creating project on Yok...")
	defer utils.StopSpinner(s)

	projectData := map[string]string{
		"name":       name,
		"gitRepoUrl": repoURL,
		"framework":  framework,
	}

	jsonData, err := json.Marshal(projectData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", ApiURL+"/project", bytes.NewBuffer(jsonData))
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

	var projectResp types.ProjectResponse
	if err := json.Unmarshal(body, &projectResp); err != nil {
		return nil, err
	}

	return &projectResp.Data.Project, nil
}

// DeployProject deploys a project to Yok
func DeployProject(projectID string) (*types.DeploymentResponse, error) {
	s := utils.StartSpinner("Deploying project to Yok...")
	defer utils.StopSpinner(s)

	deployData := map[string]string{
		"projectId": projectID,
	}

	jsonData, err := json.Marshal(deployData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", ApiURL+"/deploy", bytes.NewBuffer(jsonData))
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

	var deploymentResp types.DeploymentResponse
	if err := json.Unmarshal(body, &deploymentResp); err != nil {
		return nil, err
	}
	return &deploymentResp, nil
}

// GetDeploymentStatus gets the status of a deployment
func GetDeploymentStatus(deploymentID string) (*types.Deployment, error) {
	resp, err := httpClient.Get(ApiURL + "/deployment/" + deploymentID)
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

	var statusResp types.DeploymentStatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, err
	}

	return &statusResp.Data.Deployment, nil
}

// ListDeployments lists deployments for a project
func ListDeployments(projectID string) ([]types.Deployment, error) {
	resp, err := httpClient.Get(ApiURL + "/project/" + projectID + "/deployments")
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

	var listResp types.DeploymentListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, err
	}

	return listResp.Data.Deployments, nil
}

// CancelDeployment cancels a deployment
func CancelDeployment(deploymentID string) error {
	cancelData := map[string]string{
		"deploymentId": deploymentID,
	}

	jsonData, err := json.Marshal(cancelData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", ApiURL+"/deployment/"+deploymentID+"/cancel", bytes.NewBuffer(jsonData))
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

// GetProject gets a project by ID
func GetProject(projectID string) (*types.Project, error) {
	// Try to get the project directly by ID first
	resp, err := httpClient.Get(ApiURL + "/project/" + projectID)
	if err != nil {
		return nil, err
	}

	// If the endpoint doesn't exist or returns an error, try the deployments list endpoint as a fallback
	if resp.StatusCode != http.StatusOK {
		// If the /project/:id endpoint is not available, we'll try a workaround
		// by listing deployments and looking up the project from there
		resp.Body.Close()

		// Get the deployments for this project
		deploymentsResp, err := httpClient.Get(ApiURL + "/project/" + projectID + "/deployments")
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

		var listResp types.DeploymentListResponse
		if err := json.Unmarshal(body, &listResp); err != nil {
			return nil, err
		}

		if len(listResp.Data.Deployments) > 0 {
			// We have a deployment, but we still don't have the project slug
			// Return a project with just the ID filled in
			return &types.Project{
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

	var projectResp types.ProjectResponse
	if err := json.Unmarshal(body, &projectResp); err != nil {
		return nil, err
	}

	return &projectResp.Data.Project, nil
}

// FollowDeploymentStatus follows the status of a deployment until completion or failure
func FollowDeploymentStatus(deploymentID string, deploymentURL string, projectID string) {
	// Create spinner with specific configuration to prevent clearing previous lines
	s := utils.StartSpinner("Waiting for deployment to complete...")

	for {
		time.Sleep(3 * time.Second) // Check every 3 seconds

		status, err := GetDeploymentStatus(deploymentID)
		if err != nil {
			utils.StopSpinner(s)
			utils.WarnColor.Printf("\nFailed to get deployment status: %v\n", err)
			break
		}

		if status.Status == "COMPLETED" {
			utils.StopSpinner(s)
			utils.SuccessColor.Printf("\n✅ Deployment completed successfully!\n")

			// Try to get the project slug for a nicer URL
			project, err := GetProject(projectID)
			if err == nil && project.Slug != "" {
				utils.InfoColor.Printf("ℹ️ Your site is available at:\n")
				fmt.Printf("- https://%s.yok.ninja\n", project.Slug)
				fmt.Printf("- %s\n", deploymentURL)
			} else {
				// If we couldn't get the project or it doesn't have a slug, just show the deployment URL
				utils.InfoColor.Printf("ℹ️ Your site is now available at: %s\n", deploymentURL)
			}
			break
		} else if status.Status == "FAILED" {
			utils.StopSpinner(s)
			utils.ErrorColor.Printf("\n❌ Deployment failed\n")
			break
		}
		// Continue waiting for other status values
	}
}

// SelectDeploymentFromList prompts the user to select a deployment from a list
// filter can be used to filter deployments by status (e.g. only in-progress deployments)
// if filter is nil, all deployments are shown
func SelectDeploymentFromList(projectID string, filter func(types.Deployment) bool) (string, error) {
	// Get recent deployments
	deployments, err := ListDeployments(projectID)
	if err != nil {
		return "", fmt.Errorf("error fetching deployments: %v", err)
	}

	// Filter deployments if a filter is provided
	filteredDeployments := []types.Deployment{}
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

// DetectFramework detects the framework used in the repository
func DetectFramework() string {
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

// PromptForProjectCreationDetails asks the user for a project name, checks if it exists, and
// gets Git repo info. Returns project details and a flag indicating if the user is using an existing project.
func PromptForProjectCreationDetails() (string, string, string, *types.Project, bool, error) {
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
	existingProject, err := FindProjectByName(projectName)
	if err != nil {
		utils.WarnColor.Printf("Warning: Could not check if project exists: %v\n", err)
		// Continue anyway, the creation step will fail if there's a duplicate
	} else if existingProject != nil {
		utils.InfoColor.Printf("Project with name '%s' already exists!\n", projectName)

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

	// Return but leave getting repository info to the caller
	return projectName, "", "", nil, false, nil
}
