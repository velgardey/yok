package types

import (
	"time"
)

// Project represents a project entity
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	GitRepoURL string `json:"gitRepoUrl"`
	Slug       string `json:"slug"`
	Framework  string `json:"framework"`
}

// ProjectResponse wraps a project response from the API
type ProjectResponse struct {
	Status string `json:"status"`
	Data   struct {
		Project Project `json:"project"`
	} `json:"data"`
}

// DeploymentResponse wraps a deployment response from the API
type DeploymentResponse struct {
	Status string `json:"status"`
	Data   struct {
		DeploymentId  string `json:"deploymentId"`
		DeploymentUrl string `json:"deploymentUrl"`
	} `json:"data"`
}

// Config stores local configuration
type Config struct {
	ProjectID string `json:"projectId"`
	RepoName  string `json:"repoName"`
}

// ProjectCheckResponse wraps a project check response
type ProjectCheckResponse struct {
	Status string `json:"status"`
	Data   struct {
		Exists  bool    `json:"exists"`
		Project Project `json:"project,omitempty"`
	} `json:"data"`
}

// Deployment represents a deployment entity
type Deployment struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// DeploymentListResponse wraps a deployment list response
type DeploymentListResponse struct {
	Status string `json:"status"`
	Data   struct {
		Deployments []Deployment `json:"deployments"`
	} `json:"data"`
}

// DeploymentStatusResponse wraps a deployment status response
type DeploymentStatusResponse struct {
	Status string `json:"status"`
	Data   struct {
		Deployment Deployment `json:"deployment"`
	} `json:"data"`
}

// GitHubRelease represents GitHub release information
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Prerelease bool   `json:"prerelease"`
}
