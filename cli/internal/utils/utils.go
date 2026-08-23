package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/gookit/color"
	"github.com/velgardey/yok/cli/internal/config"
	"github.com/velgardey/yok/cli/internal/types"
)

// ANSI colors for terminal output
var (
	// Main colors
	InfoColor    = color.New(color.FgCyan)
	ErrorColor   = color.New(color.FgRed, color.Bold)
	WarnColor    = color.New(color.FgYellow)
	SuccessColor = color.New(color.FgGreen, color.Bold)
	// Use a subtle color for dimmed text that works on both Windows and Linux
	DimColor = color.New(color.FgBlue)
)

// Constants
const (
	DefaultAPIURL     = "https://api.yok.ninja"
	DefaultSiteDomain = "yok.ninja"
)

// CreateHTTPClient returns an HTTP client with appropriate timeouts and settings
func CreateHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 30,
	}
}

// ResolvedConfig is the effective configuration after merging the config file,
// defaults, and environment overrides.
type ResolvedConfig struct {
	APIURL     string
	Token      string
	SiteDomain string
}

// ResolveConfig merges the local config file with defaults and YOK_* env overrides.
func ResolveConfig() ResolvedConfig {
	cfg, err := config.LoadConfig()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: could not read config file: %v\n", err)
	}
	rc := ResolvedConfig{
		APIURL:     cfg.APIURL,
		Token:      cfg.Token,
		SiteDomain: cfg.SiteDomain,
	}
	if rc.APIURL == "" {
		rc.APIURL = DefaultAPIURL
	}
	if rc.SiteDomain == "" {
		rc.SiteDomain = DefaultSiteDomain
	}
	if v := os.Getenv("YOK_API_URL"); v != "" {
		rc.APIURL = v
	}
	if v := os.Getenv("YOK_SITE_DOMAIN"); v != "" {
		rc.SiteDomain = v
	}
	if v := os.Getenv("YOK_TOKEN"); v != "" {
		rc.Token = v
	}
	return rc
}

// WithAuth attaches the bearer token to a request before it is sent.
func WithAuth(req *http.Request) *http.Request {
	if rc := ResolveConfig(); rc.Token != "" {
		req.Header.Set("Authorization", "Bearer "+rc.Token)
	}
	return req
}

// DeploymentURL returns the public URL of a deployment identifier under the configured site domain.
func DeploymentURL(identifier string) string {
	return fmt.Sprintf("https://%s.%s", identifier, ResolveConfig().SiteDomain)
}

// GetProjectIDOrExit loads the config and exits if no project ID is found
func GetProjectIDOrExit() types.Config {
	conf, err := config.LoadConfig()
	HandleError(err, "Error loading configuration")

	if conf.ProjectID == "" {
		ErrorColor.Println("No project configured. Run 'yok create' or 'yok deploy' first.")
		os.Exit(1)
	}

	return conf
}

// HandleError prints error messages and exits with non-zero code if err is not nil
func HandleError(err error, message string) {
	if err != nil {
		ErrorColor.Printf("[ERROR] %s: %v\n", message, err)
		os.Exit(1)
	}
}

// StartSpinner creates and starts a new spinner with the given message
func StartSpinner(message string) *spinner.Spinner {
	s := spinner.New(spinner.CharSets[25], 700*time.Millisecond)
	s.Suffix = " " + message
	s.Start()
	return s
}

// StopSpinner safely stops a spinner
func StopSpinner(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}

// FormatTableRow prints a row in the deployments table with colored status
func FormatTableRow(id string, status string, createdAt time.Time) {
	// Display the full ID without truncation
	fmt.Printf("%-36s ", id)
	switch status {
	case "COMPLETED":
		SuccessColor.Printf("%-12s ", status)
	case "FAILED":
		ErrorColor.Printf("%-12s ", status)
	case "PENDING", "QUEUED", "IN_PROGRESS":
		InfoColor.Printf("%-12s ", status)
	default:
		fmt.Printf("%-12s ", status)
	}
	fmt.Printf("%-20s\n", createdAt.Format("Jan 02 15:04:05"))
}

// DecodeJSON decodes JSON from a reader into a target struct
func DecodeJSON(r io.Reader, target any) error {
	return json.NewDecoder(r).Decode(target)
}

// GetSurveyOptions returns survey options configured for the current platform
// This fixes the PowerShell echo issue by properly configuring stdio
func GetSurveyOptions() survey.AskOpt {
	// Configure stdio to prevent echo issues in PowerShell
	// Use a simple stdio configuration that works across platforms
	return survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)
}

// LogRenderer handles the rendering of log entries to the terminal
type LogRenderer struct {
	showTimestamps bool
	useColors      bool
	rawOutput      bool
	lastDate       string
}

// NewLogRenderer creates a new LogRenderer with default settings
func NewLogRenderer() *LogRenderer {
	return &LogRenderer{
		showTimestamps: true,
		useColors:      !IsWindows(), // Disable colors on Windows by default
		rawOutput:      false,
	}
}

// RenderLogEntry displays a log entry in the terminal
func (lr *LogRenderer) RenderLogEntry(entry types.LogEntry) {
	// If raw output is requested, just print the log without any formatting
	if lr.rawOutput {
		fmt.Println(entry.Log)
		return
	}

	// Extract date and time from timestamp
	timestampParts := strings.Split(entry.Timestamp, " ")
	if len(timestampParts) >= 2 {
		date := timestampParts[0]
		timeStr := timestampParts[1]

		// Show date header if it's a new date
		if lr.lastDate != date {
			if lr.lastDate != "" {
				// Add a line break before new date
				fmt.Println()
			}

			if lr.useColors {
				DimColor.Printf("─── %s ───────────────────────────────────\n", date)
			} else {
				fmt.Printf("─── %s ───────────────────────────────────\n", date)
			}
			lr.lastDate = date
		}

		// Format timestamp as just the time if showing timestamps
		prefix := ""
		if lr.showTimestamps {
			if lr.useColors {
				prefix = DimColor.Sprintf("[%s] ", timeStr)
			} else {
				prefix = fmt.Sprintf("[%s] ", timeStr)
			}
		}

		// Process the log message
		logMessage := entry.Log

		// Print the log with appropriate styling
		fmt.Print(prefix)
		fmt.Println(logMessage)
	} else {
		// Fallback if timestamp format is unexpected
		fmt.Println(entry.Log)
	}
}

// WithTimestamps configures whether timestamps are shown
func (lr *LogRenderer) WithTimestamps(show bool) *LogRenderer {
	lr.showTimestamps = show
	return lr
}

// WithColors configures whether colors are used
func (lr *LogRenderer) WithColors(use bool) *LogRenderer {
	lr.useColors = use
	return lr
}

// WithRawOutput configures whether to display raw log output without formatting
func (lr *LogRenderer) WithRawOutput(raw bool) *LogRenderer {
	lr.rawOutput = raw
	return lr
}

// IsWindows checks if the current OS is Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
