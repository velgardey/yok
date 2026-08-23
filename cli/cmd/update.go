package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
	"github.com/velgardey/yok/cli/internal/utils"
)

const releaseBaseURL = "https://github.com/velgardey/yok/releases"

// checkForUpdates checks for newer version on GitHub
func checkForUpdates() (string, bool, error) {
	currentVersion := getCurrentVersion()

	if runtime.GOOS == "windows" {
		// Use non-API method for Windows
		latestVersionStr, err := getLatestVersionNoAPI()
		if err != nil {
			return "", false, fmt.Errorf("failed to check for updates: %w", err)
		}
		hasUpdate, err := isNewerVersion(currentVersion, latestVersionStr)
		if err != nil {
			return "", false, err
		}
		return latestVersionStr, hasUpdate, nil
	}

	// Use GitHub API for non-Windows platforms
	latest, found, err := selfupdate.DetectLatest("velgardey/yok")
	if err != nil {
		return "", false, fmt.Errorf("error checking for updates: %w", err)
	}
	if !found {
		return "", false, fmt.Errorf("no release found for velgardey/yok")
	}

	if isDevVersion(currentVersion) {
		return latest.Version.String(), true, nil // Always update dev versions
	}

	v, err := semver.Parse(currentVersion)
	if err != nil {
		return "", false, fmt.Errorf("failed to parse current version: %w", err)
	}
	return latest.Version.String(), latest.Version.GT(v), nil
}

func isDevVersion(version string) bool {
	return version == "dev" || version == "development"
}

func isNewerVersion(currentVersion, latestVersionStr string) (bool, error) {
	if isDevVersion(currentVersion) {
		return true, nil
	}
	currentSemver, err := semver.Parse(currentVersion)
	if err != nil {
		return false, fmt.Errorf("failed to parse current version: %w", err)
	}
	latestSemver, err := semver.Parse(latestVersionStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse latest version: %w", err)
	}
	return latestSemver.GT(currentSemver), nil
}

// getCurrentVersion returns the current version without the 'v' prefix
func getCurrentVersion() string {
	return strings.TrimPrefix(version, "v")
}

// getLatestVersionNoAPI makes an HTTP request to GitHub releases page
// and extracts the latest version from the redirect URL
func getLatestVersionNoAPI() (string, error) {
	client := utils.CreateHTTPClient()

	// Disable following redirects so we can capture the redirect URL
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(releaseBaseURL + "/latest")
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no redirect location found")
	}

	parts := strings.Split(location, "/")
	tag := parts[len(parts)-1]
	if !strings.HasPrefix(tag, "v") || len(parts) < 2 {
		return "", fmt.Errorf("invalid version format: %s", tag)
	}

	return strings.TrimPrefix(tag, "v"), nil
}

// detectInstallLocation returns the appropriate directory for binary installation
func detectInstallLocation() (string, error) {
	var defaultLocations []string
	switch runtime.GOOS {
	case "windows":
		defaultLocations = []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "yok"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "yok"),
		}
	default:
		defaultLocations = []string{
			"/usr/local/bin",
			"/opt/homebrew/bin",
			"/usr/bin",
			"/bin",
			filepath.Join(os.Getenv("HOME"), ".local", "bin"),
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get current executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}
	execDir := filepath.Dir(execPath)

	// If current executable is in a standard location, use that
	for _, dir := range defaultLocations {
		if execDir == dir {
			return dir, nil
		}
	}

	for _, dir := range candidateLocations(runtime.GOOS, defaultLocations) {
		if isLocationWritable(dir) {
			return dir, nil
		}
	}

	return "", fmt.Errorf("no writable installation location found")
}

func candidateLocations(goos string, defaults []string) []string {
	if goos == "windows" {
		return append([]string{filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "yok")}, defaults...)
	}
	return append([]string{"/usr/local/bin"}, defaults...)
}

// isLocationWritable checks if a directory is writable without side effects;
// missing directories are simply not writable candidates here.
func isLocationWritable(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	testFile := filepath.Join(dir, ".yok-write-test")
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	os.Remove(testFile)
	return true
}

// runUnixUpdate installs the new binary atomically: the downloaded artifact is
// verified against the release checksums, then staged next to the target and
// moved into place with a rename - which, unlike copying over the file,
// succeeds even while the old binary is still running.
func runUnixUpdate(targetPath string, version string) error {
	archiveName := fmt.Sprintf("yok_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	downloadURL := releaseBaseURL + "/download/v" + version

	tmpDir, err := os.MkdirTemp("", "yok-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	utils.InfoColor.Printf("Downloading update from %s/%s...\n", downloadURL, archiveName)
	archivePath := filepath.Join(tmpDir, "update.tar.gz")
	if err := downloadFile(downloadURL+"/"+archiveName, archivePath); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	utils.InfoColor.Println("Verifying download...")
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	if err := downloadFile(downloadURL+"/checksums.txt", checksumsPath); err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	if err := verifyChecksum(archivePath, checksumsPath, archiveName); err != nil {
		return fmt.Errorf("download verification failed: %w", err)
	}

	utils.InfoColor.Println("Extracting update...")
	extractedBinaryPath, err := extractBinary(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("failed to extract update: %w", err)
	}
	if err := os.Chmod(extractedBinaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	utils.InfoColor.Println("Installing update...")
	stagedPath := targetPath + ".new"
	sudoCmd := exec.Command("sudo", "cp", extractedBinaryPath, stagedPath)
	sudoCmd.Stdin, sudoCmd.Stdout, sudoCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := sudoCmd.Run(); err != nil {
		return fmt.Errorf("failed to stage update with sudo: %w", err)
	}

	chmodCmd := exec.Command("sudo", "chmod", "755", stagedPath)
	chmodCmd.Stdin, chmodCmd.Stdout, chmodCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := chmodCmd.Run(); err != nil {
		return fmt.Errorf("failed to set permissions with sudo: %w", err)
	}

	// Atomic replace: rename(2) swaps the directory entry, so this works even
	// while the currently running binary is executing from the same path.
	mvCmd := exec.Command("sudo", "mv", "-f", stagedPath, targetPath)
	mvCmd.Stdin, mvCmd.Stdout, mvCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := mvCmd.Run(); err != nil {
		_ = exec.Command("sudo", "rm", "-f", stagedPath).Run()
		return fmt.Errorf("failed to install update: %w", err)
	}

	utils.SuccessColor.Printf("\n[OK] Yok CLI has been updated to v%s successfully!\n", version)
	fmt.Println("Run 'yok version' to verify the update.")
	return nil
}

// verifyChecksum checks that fileName's sha256 matches the entry published in
// the goreleaser-generated checksums.txt for the release.
func verifyChecksum(filePath, checksumsPath, fileName string) error {
	file, err := os.Open(checksumsPath)
	if err != nil {
		return err
	}
	defer file.Close()

	expected := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == fileName {
			expected = fields[0]
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if expected == "" {
		return fmt.Errorf("no checksum published for %s", fileName)
	}

	actualHash := sha256.New()
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(actualHash, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(actualHash.Sum(nil))

	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// downloadFile downloads a file from the given URL
func downloadFile(url string, destPath string) error {
	resp, err := utils.CreateHTTPClient().Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractBinary extracts the binary named 'yok' from a tar.gz archive
func extractBinary(archivePath string, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(header.Name) != "yok" {
			continue
		}

		extractedPath := filepath.Join(destDir, "yok")
		outFile, err := os.OpenFile(extractedPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(outFile, tarReader); err != nil {
			outFile.Close()
			return "", err
		}
		outFile.Close()
		return extractedPath, nil
	}

	return "", fmt.Errorf("binary not found in archive")
}

// runWindowsUpdate handles the update process for Windows
func runWindowsUpdate(targetPath string, version string) error {
	scriptPath, err := createWindowsUpdateScript(targetPath, version)
	if err != nil {
		return err
	}

	utils.InfoColor.Println("Starting update process...")
	utils.InfoColor.Println("The CLI will exit and a new process will complete the update.")

	cmd := exec.Command("powershell.exe", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr

	// Start (not Run) so the CLI can exit and unblock the running executable.
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start update process: %v", err)
	}

	fmt.Println("Update in progress... please wait.")
	os.Exit(0)
	return nil // unreachable
}

const windowsUpdateScriptTemplate = `# Yok CLI Self-Update Script
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"  # Makes downloads faster

function Handle-Error {
    param(
        [Parameter(Mandatory=$true)][string]$ErrorMessage,
        [Parameter(Mandatory=$false)][object]$ErrorDetail = $null
    )
    Write-Host "` + "`n" + `====== ERROR ======" -ForegroundColor Red
    Write-Host $ErrorMessage -ForegroundColor Red
    if ($ErrorDetail) {
        Write-Host "` + "`n" + `Error details:" -ForegroundColor Red
        Write-Host $ErrorDetail.Exception.Message -ForegroundColor Red
    }
    if (Test-Path "%[1]s") { Restore-Backup }
    Start-Sleep -Seconds 5
    Remove-Item -Path $PSCommandPath -Force -ErrorAction SilentlyContinue
    exit 1
}

function Restore-Backup {
    Write-Host "Restoring from backup..." -ForegroundColor Yellow
    try {
        Copy-Item -Path "%[1]s" -Destination "%[2]s" -Force
        Write-Host "Restored successfully." -ForegroundColor Green
    } catch {
        Write-Host "Failed to restore from backup: $_" -ForegroundColor Red
    }
}

try {
    # Wait for the main process to exit
    Start-Sleep -Seconds 2
    Write-Host "Updating Yok CLI to v%[4]s..." -ForegroundColor Cyan

    $updateDir = "$env:TEMP\yok_update"
    if (Test-Path $updateDir) { Remove-Item -Path $updateDir -Recurse -Force }
    New-Item -ItemType Directory -Path $updateDir -Force | Out-Null

    Write-Host "Downloading update from %[3]s..." -ForegroundColor Cyan
    try {
        Invoke-WebRequest -Uri "%[3]s" -OutFile "$updateDir\yok.zip"
        Invoke-WebRequest -Uri "%[5]s" -OutFile "$updateDir\checksums.txt"
    } catch {
        Handle-Error "Failed to download the update package" $_
    }

    Write-Host "Verifying download..." -ForegroundColor Cyan
    try {
        $expected = (Select-String -Path "$updateDir\checksums.txt" -Pattern "%[6]s").Line.Split(' ')[0]
        $actual = (Get-FileHash -Path "$updateDir\yok.zip" -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) { throw "checksum mismatch" }
    } catch {
        Handle-Error "Download verification failed" $_
    }

    Write-Host "Creating backup..." -ForegroundColor Cyan
    try {
        Copy-Item -Path "%[2]s" -Destination "%[1]s" -Force
    } catch {
        Handle-Error "Failed to create backup" $_
    }

    Write-Host "Installing update..." -ForegroundColor Cyan
    try {
        Expand-Archive -Path "$updateDir\yok.zip" -DestinationPath $updateDir -Force
        Copy-Item -Path "$updateDir\yok.exe" -Destination "%[2]s" -Force
    } catch {
        Handle-Error "Failed to install the update" $_
    }

    Write-Host "Cleaning up..." -ForegroundColor Cyan
    Remove-Item -Path $updateDir -Recurse -Force -ErrorAction SilentlyContinue

    Write-Host "` + "`n" + `[OK] Yok CLI has been updated to v%[4]s successfully!" -ForegroundColor Green
    Write-Host "Run 'yok version' to verify the update." -ForegroundColor Cyan
    Start-Sleep -Seconds 1
    Remove-Item -Path $PSCommandPath -Force -ErrorAction SilentlyContinue
} catch {
    Handle-Error "An unexpected error occurred during update" $_
}
`

// createWindowsUpdateScript generates a PowerShell script for updating the
// Windows binary. The script verifies the download against the release
// checksums and keeps a backup for rollback on failure.
func createWindowsUpdateScript(targetPath, version string) (string, error) {
	tmpDir := os.TempDir()
	scriptPath := filepath.Join(tmpDir, "yok_update.ps1")

	zipName := fmt.Sprintf("yok_%s_windows_amd64.zip", version)
	downloadURL := releaseBaseURL + "/download/v" + version
	script := fmt.Sprintf(windowsUpdateScriptTemplate,
		targetPath+".backup",
		targetPath,
		downloadURL+"/"+zipName,
		version,
		downloadURL+"/checksums.txt",
		zipName,
	)

	return scriptPath, os.WriteFile(scriptPath, []byte(script), 0o700)
}

// getExePath returns the directory of the running binary and the binary name
func getExePath() (string, string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("failed to get executable path: %v", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve symlinks: %v", err)
	}

	targetName := "yok"
	if runtime.GOOS == "windows" {
		targetName += ".exe"
	}

	installDir := filepath.Dir(execPath)
	base := filepath.Base(execPath)

	// Test builds may live outside the install location; fall back to detection.
	if strings.HasSuffix(base, ".new") || strings.HasSuffix(base, ".test") {
		installDir, err = detectInstallLocation()
		if err != nil {
			return "", "", fmt.Errorf("failed to detect installation location: %v", err)
		}
	}

	return installDir, targetName, nil
}

// runSelfUpdate implements the update logic
func runSelfUpdate(_ *cobra.Command, force bool, checkOnly bool) error {
	spinner := utils.StartSpinner("Checking for updates...")
	latestVersionStr, hasUpdate, err := checkForUpdates()
	utils.StopSpinner(spinner)

	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	currentVersion := getCurrentVersion()

	if checkOnly {
		if hasUpdate {
			utils.InfoColor.Printf("\nUpdate available: v%s (current: %s)\n", latestVersionStr, currentVersion)
			fmt.Printf("Run 'yok self-update' to update to the latest version\n")
		} else {
			utils.SuccessColor.Printf("You're already using the latest version (v%s)\n", currentVersion)
		}
		return nil
	}

	if !hasUpdate && !force {
		utils.SuccessColor.Printf("You're already using the latest version (v%s)\n", currentVersion)
		return nil
	}

	utils.InfoColor.Printf("\nAvailable update:\n")
	fmt.Printf("Current version: v%s\n", currentVersion)
	fmt.Printf("Latest version: v%s\n", latestVersionStr)
	fmt.Printf("Release page: %s/tag/v%s\n", releaseBaseURL, latestVersionStr)

	if !force {
		updateConfirm := false
		updatePrompt := &survey.Confirm{
			Message: fmt.Sprintf("Do you want to update from v%s to v%s?", currentVersion, latestVersionStr),
			Default: true,
		}
		opts := utils.GetSurveyOptions()
		if err := survey.AskOne(updatePrompt, &updateConfirm, opts); err != nil {
			return fmt.Errorf("update cancelled: %v", err)
		}
		if !updateConfirm {
			utils.InfoColor.Println("Update cancelled")
			return nil
		}
	}

	installDir, targetName, err := getExePath()
	if err != nil {
		return err
	}
	targetPath := filepath.Join(installDir, targetName)

	if runtime.GOOS == "windows" {
		return runWindowsUpdate(targetPath, latestVersionStr)
	}
	return runUnixUpdate(targetPath, latestVersionStr)
}

var updateCmd *cobra.Command

func init() {
	var (
		force     bool
		checkOnly bool
	)

	updateCmd = &cobra.Command{
		Use:     "self-update",
		Short:   "Update Yok CLI to the latest version",
		Long:    `Update Yok CLI to the latest version from GitHub releases.`,
		Aliases: []string{"update"},
		Run: func(cmd *cobra.Command, args []string) {
			if err := runSelfUpdate(cmd, force, checkOnly); err != nil {
				handleUpdateFailure(err)
			}
		},
	}

	updateCmd.Flags().BoolVarP(&force, "force", "f", false, "Force update without confirmation")
	updateCmd.Flags().BoolVarP(&checkOnly, "check", "c", false, "Only check for updates without installing")

	RootCmd.AddCommand(updateCmd)
}

func handleUpdateFailure(err error) {
	utils.ErrorColor.Printf("Update failed: %v\n", err)

	utils.WarnColor.Println("\nTroubleshooting tips:")
	fmt.Println("1. Check your internet connection")
	fmt.Println("2. Make sure you have permission to write to the installation directory")
	if runtime.GOOS == "windows" {
		fmt.Println("3. Try running with administrator privileges")
		fmt.Println("4. Ensure PowerShell execution policy allows running scripts")
	} else {
		fmt.Println("3. Try running with elevated privileges (sudo/admin)")
	}
	fmt.Println("4. Check if GitHub is accessible from your network")

	os.Exit(1)
}
