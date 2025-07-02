package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/api"
	"github.com/velgardey/yok/cli/internal/config"
	"github.com/velgardey/yok/cli/internal/git"
	"github.com/velgardey/yok/cli/internal/types"
	"github.com/velgardey/yok/cli/internal/utils"
)

// EnsureProjectID loads config and ensures a project ID exists, creating a project if needed
func EnsureProjectID() (types.Config, error) {
	// Load config to check if we have a stored project ID
	conf, err := config.LoadConfig()
	if err != nil {
		return conf, fmt.Errorf("error loading configuration: %v", err)
	}

	// If no stored project ID, we need to create/find one
	if conf.ProjectID == "" {
		projectName, repoURL, framework, existingProject, usingExisting, err := api.PromptForProjectCreationDetails()
		if err != nil {
			return conf, err
		}

		if usingExisting {
			// Use existing project
			utils.SuccessColor.Printf("✅ Using existing project: %s\n", existingProject.Name)

			// Save project ID for future use
			conf.ProjectID = existingProject.ID
			conf.RepoName = existingProject.Name
			if err := config.SaveConfig(conf); err != nil {
				utils.WarnColor.Printf("Warning: Could not save project ID: %v\n", err)
			}

			return conf, nil
		}

		// Get repository info based on user's choice (manual entry or auto-detect)
		useManualEntry := false // Default to auto-detect
		var tempRepoName string
		repoURL, tempRepoName, err = git.GetRepoInfo(useManualEntry)
		if err != nil {
			return conf, fmt.Errorf("error getting repository info: %v", err)
		}
		_ = tempRepoName // Avoid unused variable warning

		framework = api.DetectFramework()

		// Create or get existing project (double-check since another user might have created it)
		project, err := api.GetOrCreateProject(projectName, repoURL, framework)
		if err != nil {
			return conf, fmt.Errorf("error creating project: %v", err)
		}

		utils.SuccessColor.Printf("✅ Using project: %s\n", project.Name)

		// Save project ID for future use
		conf.ProjectID = project.ID
		conf.RepoName = project.Name
		if err := config.SaveConfig(conf); err != nil {
			utils.WarnColor.Printf("Warning: Could not save project ID: %v\n", err)
		}
	} else {
		utils.InfoColor.Printf("Using stored project ID for: %s\n", conf.RepoName)
	}

	return conf, nil
}

func init() {
	// Create command for creating a new project
	var createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new project on Yok",
		Run: func(cmd *cobra.Command, args []string) {
			projectName, repoURL, framework, existingProject, usingExisting, err := api.PromptForProjectCreationDetails()
			utils.HandleError(err, "Error getting project details")

			if usingExisting {
				// Display project info and save the project ID
				utils.SuccessColor.Printf("✅ Using existing project\n")

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
				conf := types.Config{
					ProjectID: existingProject.ID,
					RepoName:  existingProject.Name,
				}
				err = config.SaveConfig(conf)
				if err != nil {
					utils.WarnColor.Printf("Warning: Could not save project ID: %v\n", err)
				} else {
					utils.SuccessColor.Println("\n✅ Project ID saved for future deployments")
				}
				return
			}

			// Get repository information
			useManualEntry := true // Set to true for manual entry, user can choose based on prompt
			var tempRepoName string
			repoURL, tempRepoName, err = git.GetRepoInfo(useManualEntry)
			utils.HandleError(err, "Error getting repository info")
			_ = tempRepoName // Avoid unused variable warning

			framework = api.DetectFramework()

			// Create or get existing project
			project, err := api.GetOrCreateProject(projectName, repoURL, framework)
			utils.HandleError(err, "Error creating project")

			utils.SuccessColor.Printf("✅ Project created/updated successfully\n")

			// Display comprehensive project info
			fmt.Println("\nProject Information:")
			fmt.Printf("ID: %s\n", project.ID)
			fmt.Printf("Name: %s\n", project.Name)
			fmt.Printf("Framework: %s\n", project.Framework)
			fmt.Printf("Slug: %s\n", project.Slug)
			fmt.Printf("Git URL: %s\n", project.GitRepoURL)
			if project.Slug != "" {
				fmt.Printf("Project URL: https://%s.yok.ninja\n", project.Slug)
			}

			// Save project ID
			conf := types.Config{
				ProjectID: project.ID,
				RepoName:  project.Name,
			}
			err = config.SaveConfig(conf)
			if err != nil {
				utils.WarnColor.Printf("Warning: Could not save project ID: %v\n", err)
			} else {
				utils.SuccessColor.Println("\n✅ Project ID saved for future deployments")
			}
		},
	}

	// Reset config command
	var resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Reset stored project ID",
		Run: func(cmd *cobra.Command, args []string) {
			err := config.RemoveConfig()
			if err != nil {
				utils.HandleError(err, "Error removing config file")
			} else {
				utils.SuccessColor.Println("✅ Project configuration reset successfully")
			}
		},
	}

	// Reset config command (alias)
	var resetConfigCmd = &cobra.Command{
		Use:   "reset-config",
		Short: "Reset stored project ID configuration",
		Run: func(cmd *cobra.Command, args []string) {
			err := config.RemoveConfig()
			if err != nil {
				utils.HandleError(err, "Error removing config file")
			} else {
				utils.SuccessColor.Println("✅ Project configuration reset successfully")
			}
		},
	}

	// Add commands to root
	RootCmd.AddCommand(createCmd, resetCmd, resetConfigCmd)
}
