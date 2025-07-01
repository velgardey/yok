# Yok CLI Installer for Windows
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"  # Makes downloads faster

# Function to handle errors and keep the window open
function Handle-Error {
    param(
        [Parameter(Mandatory=$true)][string]$ErrorMessage,
        [Parameter(Mandatory=$false)][object]$ErrorDetail = $null
    )
    
    Write-Host "`n====== ERROR ======" -ForegroundColor Red
    Write-Host $ErrorMessage -ForegroundColor Red
    
    if ($ErrorDetail) {
        Write-Host "`nError details:" -ForegroundColor Red
        Write-Host $ErrorDetail.Exception.Message -ForegroundColor Red
    }
    
    Write-Host "`nPress Enter to exit..." -ForegroundColor Yellow
    Read-Host
    exit 1
}

try {
    Write-Host "Installing Yok CLI..." -ForegroundColor Cyan

    # Variables
    $GithubRepo = "velgardey/yok"
    $InstallDir = "$env:LOCALAPPDATA\Programs\yok"
    $BinaryPath = "$InstallDir\yok.exe"

    # Create installation directory if it doesn't exist
    if (-not (Test-Path $InstallDir)) {
        Write-Host "Creating installation directory..." -ForegroundColor Cyan
        try {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        } catch {
            Handle-Error "Failed to create installation directory." $_
        }
    }

    # Get latest release info
    Write-Host "Fetching the latest version..." -ForegroundColor Cyan
    try {
        $LatestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/$GithubRepo/releases/latest"
        $Version = $LatestRelease.tag_name
        Write-Host "Latest version: $Version" -ForegroundColor Cyan
    } catch {
        Handle-Error "Failed to get latest release information. Please check your internet connection." $_
    }

    # Download the archive
    $ArchiveName = "yok_${Version}_windows_amd64.zip"
    $DownloadUrl = "https://github.com/$GithubRepo/releases/download/$Version/$ArchiveName"
    $ZipPath = "$env:TEMP\$ArchiveName"

    Write-Host "Downloading $ArchiveName from $DownloadUrl..." -ForegroundColor Cyan
    try {
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipPath
    } catch {
        Handle-Error "Failed to download $DownloadUrl" $_
    }

    # Verify the download
    if (-not (Test-Path $ZipPath)) {
        Handle-Error "Downloaded file not found at $ZipPath"
    } else {
        $FileSize = (Get-Item $ZipPath).Length
        Write-Host "Download complete, file size: $FileSize bytes" -ForegroundColor Green
    }

    # Extract the binary
    Write-Host "Extracting..." -ForegroundColor Cyan
    try {
        Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force
    } catch {
        Handle-Error "Failed to extract archive" $_
    }

    # Verify the extraction
    if (-not (Test-Path "$InstallDir\yok.exe")) {
        Handle-Error "Extracted file not found at $InstallDir\yok.exe. Archive may be corrupted or have a different structure."
    }

    # Cleanup the ZIP file
    Remove-Item $ZipPath -Force

    # Add to PATH if not already present
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        Write-Host "Adding Yok CLI to your PATH..." -ForegroundColor Cyan
        try {
            [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
        } catch {
            Handle-Error "Failed to update PATH environment variable" $_
        }
    }

    Write-Host "`n✅ Yok CLI installed successfully!" -ForegroundColor Green
    Write-Host "Please restart your terminal or run 'refreshenv' if you're using Chocolatey." -ForegroundColor Cyan
    Write-Host "Run 'yok --help' to get started" -ForegroundColor Cyan

    # Keep the window open
    Write-Host "`nPress Enter to exit..." -ForegroundColor Yellow
    Read-Host
} catch {
    Handle-Error "An unexpected error occurred" $_
} 