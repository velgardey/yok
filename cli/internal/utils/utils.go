package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/gookit/color"
)

// Color definitions
var (
	InfoColor    = color.New(color.FgCyan)
	ErrorColor   = color.New(color.FgRed, color.Bold)
	WarnColor    = color.New(color.FgYellow)
	SuccessColor = color.New(color.FgGreen, color.Bold)
)

// Constants
const (
	ApiURL      = "http://api.yok.ninja"
	ConfigFile  = ".yok-config.json"
	HttpTimeout = 30 * time.Second
	UserAgent   = "Yok-CLI-Updater"
)

// CreateHTTPClient returns an HTTP client with appropriate timeouts and settings
func CreateHTTPClient() *http.Client {
	return &http.Client{
		Timeout: HttpTimeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
		},
	}
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

// FormatDeploymentStatus prints a deployment status with appropriate coloring
func FormatDeploymentStatus(status string) {
	switch status {
	case "COMPLETED":
		SuccessColor.Printf("Status: %s\n", status)
	case "FAILED":
		ErrorColor.Printf("Status: %s\n", status)
	case "PENDING", "QUEUED", "IN_PROGRESS":
		InfoColor.Printf("Status: %s\n", status)
	default:
		fmt.Printf("Status: %s\n", status)
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

// CompareVersions compares two version strings and returns true if latest is newer than current
func CompareVersions(current, latest string) bool {
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
	maxLen := max(len(currentParts), len(latestParts))

	for i := range maxLen {
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

// DecodeJSON decodes JSON from a reader into a target struct
func DecodeJSON(r io.Reader, target any) error {
	return json.NewDecoder(r).Decode(target)
}

// GetStdout returns os.Stdout
func GetStdout() io.Writer {
	return os.Stdout
}

// GetStderr returns os.Stderr
func GetStderr() io.Writer {
	return os.Stderr
}

// GetSurveyOptions returns survey options configured for the current platform
// This fixes the PowerShell echo issue by properly configuring stdio
func GetSurveyOptions() survey.AskOpt {
	// Configure stdio to prevent echo issues in PowerShell
	// Use a simple stdio configuration that works across platforms
	return survey.WithStdio(os.Stdin, os.Stdout, os.Stderr)
}

// IsValidURL checks if a string is a valid URL
func IsValidURL(str string) bool {
	if str == "" {
		return false
	}

	// Basic URL validation - check for common URL patterns
	return strings.HasPrefix(str, "http://") ||
		strings.HasPrefix(str, "https://") ||
		strings.HasPrefix(str, "git@") ||
		strings.Contains(str, "github.com") ||
		strings.Contains(str, "gitlab.com") ||
		strings.Contains(str, "bitbucket.org")
}

// TruncateString truncates a string to a maximum length with ellipsis
func TruncateString(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}

	if maxLen <= 3 {
		return str[:maxLen]
	}

	return str[:maxLen-3] + "..."
}

// SafeStringSlice safely gets a slice element or returns empty string
func SafeStringSlice(slice []string, index int) string {
	if index < 0 || index >= len(slice) {
		return ""
	}
	return slice[index]
}

// WrapError wraps an error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// HandleErrorWithMessage prints error with custom message and exits
func HandleErrorWithMessage(err error, message string, exitCode int) {
	if err != nil {
		ErrorColor.Printf("[ERROR] %s: %v\n", message, err)
		os.Exit(exitCode)
	}
}

// LogError logs an error without exiting
func LogError(err error, message string) {
	if err != nil {
		ErrorColor.Printf("[ERROR] %s: %v\n", message, err)
	}
}

// LogWarning logs a warning message
func LogWarning(message string) {
	WarnColor.Printf("Warning: %s\n", message)
}

// LogInfo logs an info message
func LogInfo(message string) {
	InfoColor.Printf("Info: %s\n", message)
}

// LogSuccess logs a success message
func LogSuccess(message string) {
	SuccessColor.Printf("[OK] %s\n", message)
}
