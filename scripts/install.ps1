# Yok CLI Installer for Windows
$ErrorActionPreference = "Stop"

Write-Host "Installing Yok CLI..." -ForegroundColor Cyan

# Variables
$GithubRepo = "velgardey/yok"
$InstallDir = "$env:LOCALAPPDATA\Programs\yok"
$BinaryPath = "$InstallDir\yok.exe"

# Create installation directory if it doesn't exist
if (-not (Test-Path $InstallDir)) {
    Write-Host "Creating installation directory..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Get latest release info
Write-Host "Fetching the latest version..." -ForegroundColor Cyan
try {
    $LatestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/$GithubRepo/releases/latest"
    $Version = $LatestRelease.tag_name
} catch {
    Write-Host "Failed to get latest release information. Please check your internet connection." -ForegroundColor Red
    exit 1
}

Write-Host "Latest version: $Version" -ForegroundColor Cyan

# Download the archive
$ArchiveName = "yok_${Version}_windows_amd64.zip"
$DownloadUrl = "https://github.com/$GithubRepo/releases/download/$Version/$ArchiveName"
$ZipPath = "$env:TEMP\$ArchiveName"

Write-Host "Downloading $ArchiveName..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath
} catch {
    Write-Host "Failed to download $DownloadUrl" -ForegroundColor Red
    exit 1
}

# Extract the binary
Write-Host "Extracting..." -ForegroundColor Cyan
try {
    Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force
} catch {
    Write-Host "Failed to extract archive" -ForegroundColor Red
    exit 1
}

# Cleanup the ZIP file
Remove-Item $ZipPath -Force

# Add to PATH if not already present
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "Adding Yok CLI to your PATH..." -ForegroundColor Cyan
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
}

Write-Host "✅ Yok CLI installed successfully!" -ForegroundColor Green
Write-Host "Please restart your terminal or run 'refreshenv' if you're using Chocolatey." -ForegroundColor Cyan
Write-Host "Run 'yok --help' to get started" -ForegroundColor Cyan 