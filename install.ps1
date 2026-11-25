# Antigravity CLI Session Helper Windows Installer
# Usage in PowerShell:
#   irm https://raw.githubusercontent.com/Sidharth-Prabhu/antigravity-session-helper/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "Sidharth-Prabhu/antigravity-session-helper"
$BinaryName = "agy-session-helper.exe"
$InstallDir = "$HOME\.local\bin"

Write-Host ""
Write-Host "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓" -ForegroundColor Cyan
Write-Host "┃       Antigravity CLI Session Helper Windows Installer          ┃" -ForegroundColor Cyan
Write-Host "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛" -ForegroundColor Cyan
Write-Host ""

# Detect Architecture
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

Write-Host "[*] Detected System: Windows ($Arch)" -ForegroundColor White

# Ensure target folder exists
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$ZipName = "antigravity-session-helper-windows-$Arch.zip"
$ReleaseUrl = "https://github.com/$Repo/releases/latest/download/$ZipName"
$TempDir = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), [System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

$Installed = $false

try {
    Write-Host "[*] Checking for prebuilt release binary..." -ForegroundColor White
    $ZipPath = Join-Path $TempDir $ZipName
    Invoke-WebRequest -Uri $ReleaseUrl -OutFile $ZipPath -UseBasicParsing -ErrorAction SilentlyContinue

    if (Test-Path $ZipPath) {
        Write-Host "[*] Extracting prebuilt binary..." -ForegroundColor White
        Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
        $Extracted = Join-Path $TempDir $BinaryName
        if (Test-Path $Extracted) {
            Copy-Item -Path $Extracted -Destination (Join-Path $InstallDir $BinaryName) -Force
            $Installed = $true
        }
    }
} catch {
    # Fallback to Go build
}

# Fallback 1: Go install
if (-not $Installed) {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host "[*] Release archive unreachable. Compiling with Go..." -ForegroundColor Yellow
        $env:GOBIN = $InstallDir
        go install "github.com/$Repo/cmd/agy-helper@latest"
        $GoBuilt = Join-Path $InstallDir "agy-helper.exe"
        if (Test-Path $GoBuilt) {
            Move-Item -Path $GoBuilt -Destination (Join-Path $InstallDir $BinaryName) -Force
            $Installed = $true
        }
    }
}

# Fallback 2: Git clone & build
if (-not $Installed) {
    if ((Get-Command git -ErrorAction SilentlyContinue) -and (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "[*] Cloning repository to build from source..." -ForegroundColor Yellow
        $SrcDir = Join-Path $TempDir "src"
        git clone --depth 1 "https://github.com/$Repo.git" $SrcDir
        Push-Location $SrcDir
        go build -o (Join-Path $InstallDir $BinaryName) ./cmd/agy-helper
        Pop-Location
        $Installed = $true
    }
}

# Clean temp directory
Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue

if (-not $Installed) {
    Write-Host ""
    Write-Host "[!] Installation failed." -ForegroundColor Red
    Write-Host "Please download the binary manually from: https://github.com/$Repo/releases"
    exit 1
}

# Add to user PATH if not present
$UserPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "[*] Adding $InstallDir to User PATH..." -ForegroundColor Yellow
    [System.Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path = "$env:Path;$InstallDir"
}

# Configure PowerShell Profile
Write-Host "[*] Configuring PowerShell integration..." -ForegroundColor White
& (Join-Path $InstallDir $BinaryName) setup-shell

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host "✨ Antigravity Session Helper installed successfully!" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Green
Write-Host ""
Write-Host "To start using it:"
Write-Host "  1. Restart PowerShell (or reload your profile: . `$PROFILE)"
Write-Host "  2. Launch browser:   agy-session-helper"
Write-Host "  3. Start a session:  agy"
Write-Host ""
