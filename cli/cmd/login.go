package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/config"
	"github.com/velgardey/yok/cli/internal/types"
	"github.com/velgardey/yok/cli/internal/utils"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your Yok personal access token",
	Run: func(cmd *cobra.Command, args []string) {
		var token string
		prompt := &survey.Password{Message: "Paste your Yok token (yok_...):"}
		opts := utils.GetSurveyOptions()
		if err := survey.AskOne(prompt, &token, opts); err != nil {
			utils.HandleError(err, "Login cancelled")
		}
		token = strings.TrimSpace(token)

		rc := utils.ResolveConfig()
		cfg := types.Config{
			Token:      token,
			APIURL:     rc.APIURL,
			SiteDomain: rc.SiteDomain,
		}

		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(cfg.APIURL, "/")+"/auth/me", nil)
		utils.HandleError(err, "Failed to build validation request")
		utils.WithAuth(req)

		resp, err := utils.CreateHTTPClient().Do(req)
		utils.HandleError(err, "Failed to reach API")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Token rejected (HTTP %d): %s\n", resp.StatusCode, string(body))
			os.Exit(1)
		}

		utils.HandleError(config.SaveConfig(cfg), "Failed to save config")
		fmt.Println("[OK] Logged in.")
	},
}

func init() {
	RootCmd.AddCommand(loginCmd)
}
