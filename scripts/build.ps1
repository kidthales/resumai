<#
.SYNOPSIS
    Builds the ResumAI production binary with single-source versioning and lock rotation.
.DESCRIPTION
    Cleans up any stale bin/*.old* and bin/*~ artifacts left over from previous builds,
    extracts git metadata, compiles bin/resumai(.exe) with stripped symbol table and injected
    version information, and installs to %USERPROFILE%\go\bin\resumai.exe (or ~/go/bin/resumai)
    with lock rotation.
#>
[CmdletBinding()]
param(
    [string]$OutputDir = "bin",
    [switch]$SkipTests,
    [switch]$SkipGlobal
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = Split-Path -Parent $ScriptDir

$IsWin = ($PSVersionTable.PSEdition -eq "Desktop") -or ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows))
$ExeName = if ($IsWin) { "resumai.exe" } else { "resumai" }

# 1. Automatic cleanup for stale rotated artifacts in local bin
$BinDir = Join-Path $RootDir $OutputDir
if (Test-Path $BinDir) {
    Get-ChildItem -Path $BinDir -Filter "*.old*" -File -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
    Get-ChildItem -Path $BinDir -Filter "*~" -File -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
} else {
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
}

# 2. Extract version and git commit metadata
$PluginManifest = Get-Content -Raw (Join-Path $RootDir "plugin.json") | ConvertFrom-Json
$Version = $PluginManifest.version
$Commit = (git -C $RootDir rev-parse --short HEAD 2>$null)
if (-not $Commit) { $Commit = "none" }
$Date = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")

$LdFlags = "-s -w " +
    "-X github.com/kidthales/resumai/internal/version.Version=$Version " +
    "-X github.com/kidthales/resumai/internal/version.Commit=$Commit " +
    "-X github.com/kidthales/resumai/internal/version.Date=$Date"

# 3. Optional pre-build tests
if (-not $SkipTests) {
    Write-Host "==> Running test suite..." -ForegroundColor Cyan
    go -C $RootDir test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Unit tests failed. Aborting build."
    }
}

# 4. Compile local binary with lock rotation
$TargetExe = Join-Path $BinDir $ExeName
if (Test-Path $TargetExe) {
    # If the binary is locked by a running process (e.g. MCP server), rotate it to .old
    $StaleTarget = Join-Path $BinDir ("$ExeName.old." + (Get-Random))
    Move-Item -Path $TargetExe -Destination $StaleTarget -Force -ErrorAction SilentlyContinue
}
Write-Host "==> Compiling $TargetExe ($Version @ $Commit)..." -ForegroundColor Cyan
go -C $RootDir build -ldflags $LdFlags -o $TargetExe ./cmd/resumai
if ($LASTEXITCODE -ne 0) {
    throw "Build failed for resumai ($TargetExe)."
}

# 5. Compile / Install to GOPATH/bin with lock rotation
if (-not $SkipGlobal) {
    $HomeDir = if ($env:USERPROFILE) { $env:USERPROFILE } else { $env:HOME }
    $GoPathBin = if ($env:GOPATH) { Join-Path $env:GOPATH "bin" } else { Join-Path (Join-Path $HomeDir "go") "bin" }
    if (-not (Test-Path $GoPathBin)) {
        New-Item -ItemType Directory -Path $GoPathBin -Force | Out-Null
    }

    $GlobalExe = Join-Path $GoPathBin $ExeName
    if (Test-Path $GlobalExe) {
        $StaleGlobal = Join-Path $GoPathBin ("$ExeName.old." + (Get-Random))
        Move-Item -Path $GlobalExe -Destination $StaleGlobal -Force -ErrorAction SilentlyContinue
    }

    Write-Host "==> Installing to $GlobalExe..." -ForegroundColor Cyan
    Copy-Item -Path $TargetExe -Destination $GlobalExe -Force

    # Best-effort cleanup of stale rotated binaries in GOPATH/bin
    Get-ChildItem -Path $GoPathBin -Filter "$ExeName.old*" -File -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
}

# 6. Best-effort cleanup of rotated stale binaries in local bin
Get-ChildItem -Path $BinDir -Filter "*.old*" -File -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue
Get-ChildItem -Path $BinDir -Filter "*~" -File -ErrorAction SilentlyContinue | Remove-Item -Force -ErrorAction SilentlyContinue

Write-Host "==> Build succeeded: $TargetExe" -ForegroundColor Green
& $TargetExe --version
