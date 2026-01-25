# dtvem installer for Windows
# Usage:
#   Standard (admin required): irm https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/install.ps1 | iex
#   User install (no admin):   iex "& { $(irm https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/install.ps1) } -UserInstall"

param(
    [switch]$UserInstall
)

$ErrorActionPreference = "Stop"

$REPO = "CodingWithCalvin/dtvem.cli"

# Get dtvem root directory
# Respects DTVEM_ROOT environment variable if set, otherwise uses default
function Get-DtvemRoot {
    if ($env:DTVEM_ROOT) {
        return $env:DTVEM_ROOT
    }
    return "$env:USERPROFILE\.dtvem"
}

$DTVEM_ROOT = Get-DtvemRoot
$INSTALL_DIR = "$DTVEM_ROOT\bin"
$SHIMS_DIR = "$DTVEM_ROOT\shims"

# This will be replaced with the actual version during release
# Format: $DTVEM_RELEASE_VERSION = "1.0.0"
# Leave empty to fetch latest
$DTVEM_RELEASE_VERSION = ""

function Write-Info {
    param([string]$Message)
    Write-Host "→ " -ForegroundColor Cyan -NoNewline
    Write-Host $Message
}

function Write-Success {
    param([string]$Message)
    Write-Host "✓ " -ForegroundColor Green -NoNewline
    Write-Host $Message
}

function Write-Error-Custom {
    param([string]$Message)
    Write-Host "✗ " -ForegroundColor Red -NoNewline
    Write-Host $Message
}

function Write-Warning-Custom {
    param([string]$Message)
    Write-Host "⚠ " -ForegroundColor Yellow -NoNewline
    Write-Host $Message
}

# Global variable to store release data
$script:ReleaseData = $null

function Get-ReleaseInfo {
    param([string]$Version)

    try {
        if ($Version) {
            $apiUrl = "https://api.github.com/repos/$REPO/releases/tags/$Version"
        }
        else {
            $apiUrl = "https://api.github.com/repos/$REPO/releases/latest"
        }

        $script:ReleaseData = Invoke-RestMethod -Uri $apiUrl
        return $script:ReleaseData.tag_name
    }
    catch {
        Write-Error-Custom "Failed to fetch release information: $_"
        exit 1
    }
}

function Get-AssetDigest {
    param([string]$AssetName)

    if (-not $script:ReleaseData) {
        return $null
    }

    # Find the asset with matching name
    $asset = $script:ReleaseData.assets | Where-Object { $_.name -eq $AssetName }

    if (-not $asset) {
        return $null
    }

    # GitHub returns digest in format "sha256:hash"
    if ($asset.digest -and $asset.digest.StartsWith("sha256:")) {
        return $asset.digest.Substring(7)
    }

    return $null
}

function Test-Checksum {
    param(
        [string]$FilePath,
        [string]$ExpectedHash
    )

    if (-not $ExpectedHash) {
        Write-Warning-Custom "No checksum available from GitHub API - skipping verification"
        return $true
    }

    # Calculate actual hash
    $actualHash = (Get-FileHash -Path $FilePath -Algorithm SHA256).Hash.ToLower()

    if ($ExpectedHash.ToLower() -ne $actualHash) {
        Write-Error-Custom "Checksum verification failed!"
        Write-Error-Custom "Expected: $ExpectedHash"
        Write-Error-Custom "Actual:   $actualHash"
        return $false
    }

    return $true
}

function Main {
    param([switch]$UserInstall)
    Write-Host ""
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host "   dtvem installer" -ForegroundColor Blue
    Write-Host "========================================" -ForegroundColor Blue
    Write-Host ""

    # Detect architecture
    $PROCESSOR_ARCH = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
    $ARCH = switch ($PROCESSOR_ARCH) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        default { "amd64" }
    }
    Write-Info "Detected platform: windows-$ARCH"
    if ($env:DTVEM_ROOT) {
        Write-Info "Using custom DTVEM_ROOT: $DTVEM_ROOT"
    }
    Write-Info "Install directory: $INSTALL_DIR"

    # Determine version to install
    $requestedVersion = $null
    if ($env:DTVEM_VERSION) {
        $requestedVersion = $env:DTVEM_VERSION
        Write-Info "Installing user-specified version: $requestedVersion"
    }
    elseif ($DTVEM_RELEASE_VERSION) {
        $requestedVersion = $DTVEM_RELEASE_VERSION
        Write-Info "Installing release version: $requestedVersion"
    }
    else {
        Write-Info "Fetching latest release..."
    }

    # Get release info from GitHub API
    $VERSION = Get-ReleaseInfo -Version $requestedVersion

    if (-not $VERSION) {
        Write-Error-Custom "Failed to determine version"
        exit 1
    }

    if (-not $requestedVersion) {
        Write-Success "Latest version: $VERSION"
    }

    # Strip "v" prefix from version for archive name
    $VERSION_NO_V = $VERSION.TrimStart('v')

    # Construct download URL
    $ARCHIVE_NAME = "dtvem-$VERSION_NO_V-windows-$ARCH.zip"
    $DOWNLOAD_URL = "https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE_NAME"

    Write-Info "Download URL: $DOWNLOAD_URL"

    # Get expected checksum from GitHub API
    Write-Info "Fetching checksum from GitHub API..."
    $EXPECTED_HASH = Get-AssetDigest -AssetName $ARCHIVE_NAME
    if ($EXPECTED_HASH) {
        Write-Success "Got checksum: $($EXPECTED_HASH.Substring(0, 16))..."
    }
    else {
        Write-Warning-Custom "Checksum not available from API (may be an older release)"
    }

    # Create temporary directory
    $TMP_DIR = Join-Path $env:TEMP "dtvem-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $TMP_DIR -Force | Out-Null

    try {
        # Download archive
        Write-Info "Downloading dtvem..."
        $ARCHIVE_PATH = Join-Path $TMP_DIR $ARCHIVE_NAME

        try {
            Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $ARCHIVE_PATH -UseBasicParsing
            Write-Success "Downloaded successfully"
        }
        catch {
            Write-Error-Custom "Failed to download dtvem: $_"
            Write-Error-Custom "URL: $DOWNLOAD_URL"
            exit 1
        }

        # Verify checksum
        Write-Info "Verifying checksum..."
        if (-not (Test-Checksum -FilePath $ARCHIVE_PATH -ExpectedHash $EXPECTED_HASH)) {
            Write-Error-Custom "Archive integrity check failed - aborting installation"
            exit 1
        }
        Write-Success "Checksum verified"

        # Extract archive
        Write-Info "Extracting archive..."
        Expand-Archive -Path $ARCHIVE_PATH -DestinationPath $TMP_DIR -Force
        Write-Success "Extracted successfully"

        # Create install directory
        Write-Info "Installing to $INSTALL_DIR..."
        New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null

        # Install binaries
        $dtvemExe = Join-Path $TMP_DIR "dtvem.exe"
        $shimExe = Join-Path $TMP_DIR "dtvem-shim.exe"

        if (Test-Path $dtvemExe) {
            Copy-Item $dtvemExe -Destination $INSTALL_DIR -Force
        }
        else {
            Write-Error-Custom "dtvem.exe not found in archive"
            exit 1
        }

        if (Test-Path $shimExe) {
            Copy-Item $shimExe -Destination $INSTALL_DIR -Force
        }
        else {
            Write-Warning-Custom "dtvem-shim.exe not found in archive"
        }

        Write-Success "Installation complete!"

        # Add install directory to PATH
        Write-Host ""
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$INSTALL_DIR*") {
            Write-Info "Adding $INSTALL_DIR to PATH..."

            try {
                # Add to BEGINNING of PATH for priority over system versions
                $newPath = if ($userPath) { "$INSTALL_DIR;$userPath" } else { $INSTALL_DIR }
                [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
                Write-Success "Added to PATH (at beginning for priority)"
                Write-Warning-Custom "Please restart your terminal for PATH changes to take effect"
            }
            catch {
                Write-Warning-Custom "Failed to add to PATH automatically: $_"
                Write-Info "Please add manually: $INSTALL_DIR"
            }
        }
        else {
            Write-Info "$INSTALL_DIR is already in PATH"
        }

        # Run init to add shims directory to PATH
        Write-Host ""
        Write-Info "Running dtvem init to add shims directory to PATH..."
        $dtvemPath = Join-Path $INSTALL_DIR "dtvem.exe"

        try {
            # Temporarily add to PATH for this session
            $env:Path = "$INSTALL_DIR;$env:Path"

            if ($UserInstall) {
                Write-Info "Using user-level PATH (no admin required)"
                & $dtvemPath init --user -y
            }
            else {
                & $dtvemPath init
            }
            Write-Success "dtvem is ready to use!"
            Write-Info "Both $INSTALL_DIR and $SHIMS_DIR have been added to PATH"
        }
        catch {
            Write-Warning-Custom "dtvem init failed - you may need to run it manually after restarting your terminal"
        }

        Write-Host ""
        Write-Host "========================================" -ForegroundColor Green
        Write-Host "   Installation successful!" -ForegroundColor Green
        Write-Host "========================================" -ForegroundColor Green
        Write-Host ""
        Write-Info "Next steps:"
        Write-Host "  1. Restart your terminal"
        Write-Host "  2. Run: dtvem install python 3.11.0"
        Write-Host "  3. Run: dtvem global python 3.11.0"
        Write-Host ""
        Write-Info "For help, run: dtvem help"
        Write-Host ""
    }
    finally {
        # Cleanup
        if (Test-Path $TMP_DIR) {
            Remove-Item -Path $TMP_DIR -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

Main -UserInstall:$UserInstall
