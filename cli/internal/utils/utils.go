package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/gookit/color"
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
	ApiURL      = "http://api.yok.ninja"
	ConfigFile  = ".yok-config.json"
	HttpTimeout = 30 * time.Second
	UserAgent   = "Yok-CLI-Updater"
)

// CreateHTTPClient returns an HTTP client with appropriate timeouts and settings
func CreateHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 30,
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

// WaitForInterrupt waits for an interrupt signal (Ctrl+C) or until the given stop channel is closed
// It returns true if the process completed naturally, false if it was interrupted
func WaitForInterrupt(stopChan chan bool) bool {
	// Setup signal catching
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Wait for either a signal or the stop channel to be closed
	select {
	case <-signals:
		// User interrupted with Ctrl+C
		close(stopChan)
		return false
	case result, ok := <-stopChan:
		// Channel was closed or received a value
		if !ok {
			// Channel was closed, meaning the process completed
			return true
		}
		// If we get here, the channel sent us a result
		return result
	}
}
