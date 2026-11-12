<#
.SYNOPSIS
    Runs local continuous integration suite for ResumAI.
.DESCRIPTION
    Executes the full test and verification pipeline locally and in GitHub Actions:
    1. Gate 1: Go format check (gofmt -l)
    2. Gate 2: Go vet (go vet ./...)
    3. Gate 3: Unit & hexagonal boundary purity tests (go test -v ./...)
    4. Gate 4: Unit tests with race detector (go test -race ./...)
    5. Gate 5: PowerShell AST syntax validation (scripts/*.ps1)
    6. Gate 6: Production build and version inspection (scripts/build.ps1)
#>
[CmdletBinding()]
param(
    [switch]$CI,
    [switch]$SkipRace,
    [switch]$SkipBuild,
    [string]$RepoRoot = ""
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = if ($RepoRoot) { $RepoRoot } else { Split-Path -Parent $ScriptDir }

function Enter-CiGate {
    param([string]$Title)
    if ($CI) {
        Write-Host "::group::$Title"
    }
    Write-Host "`n=== $Title ===" -ForegroundColor Yellow
}

function Exit-CiGate {
    if ($CI) {
        Write-Host "::endgroup::"
    }
}

# Fail-closed guard for -CI mode: reject skip flags
if ($CI) {
    $rejectedFlags = @()
    if ($SkipRace) { $rejectedFlags += "-SkipRace" }
    if ($SkipBuild) { $rejectedFlags += "-SkipBuild" }
    if ($rejectedFlags.Count -gt 0) {
        $flagList = $rejectedFlags -join ", "
        [Console]::Error.WriteLine("ERROR: -CI mode is fail-closed and cannot be combined with skip flags ($flagList).")
        exit 1
    }

    # Verify required tools upfront under -CI
    $requiredTools = @(
        @{ Name = "go"; Hint = "Ensure 'go' is in PATH." }
    )
    foreach ($tool in $requiredTools) {
        if (-not (Get-Command $tool.Name -ErrorAction SilentlyContinue)) {
            [Console]::Error.WriteLine("ERROR: -CI mode requires '$($tool.Name)' in PATH, but it was not found. $($tool.Hint)")
            exit 1
        }
    }
}

$GateResults = [ordered]@{
    "gate1_gofmt"       = "not-applicable"
    "gate2_go_vet"      = "not-applicable"
    "gate3_unit_tests"  = "not-applicable"
    "gate4_race_detect" = "not-applicable"
    "gate5_pwsh_ast"    = "not-applicable"
    "gate6_build"       = "not-applicable"
}

Write-Host "Starting ResumAI local CI runner in $RootDir..." -ForegroundColor Cyan

# -------------------------------------------------------------
# Gate 1: Go Format Check
# -------------------------------------------------------------
Enter-CiGate "Gate 1: Go Format Verification"
$unformatted = & gofmt -l (Join-Path $RootDir "cmd") (Join-Path $RootDir "internal")
if ($unformatted) {
    Write-Host "Unformatted Go files detected:" -ForegroundColor Red
    $unformatted | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    throw "Go formatting check failed. Run 'gofmt -w .' to fix."
}
$GateResults["gate1_gofmt"] = "pass"
Write-Host "All Go source files formatted properly." -ForegroundColor Green
Exit-CiGate

# -------------------------------------------------------------
# Gate 2: Go Vet
# -------------------------------------------------------------
Enter-CiGate "Gate 2: Go Vet"
go -C $RootDir vet ./...
if ($LASTEXITCODE -ne 0) {
    throw "go vet failed."
}
$GateResults["gate2_go_vet"] = "pass"
Write-Host "go vet passed cleanly." -ForegroundColor Green
Exit-CiGate

# -------------------------------------------------------------
# Gate 3: Unit & Boundary Tests
# -------------------------------------------------------------
Enter-CiGate "Gate 3: Unit & Boundary Tests"
go -C $RootDir test -v ./...
if ($LASTEXITCODE -ne 0) {
    throw "Unit/boundary tests failed."
}
$GateResults["gate3_unit_tests"] = "pass"
Write-Host "All unit and boundary tests passed." -ForegroundColor Green
Exit-CiGate

# -------------------------------------------------------------
# Gate 4: Race Detector
# -------------------------------------------------------------
Enter-CiGate "Gate 4: Race Detection"
if ($SkipRace) {
    $GateResults["gate4_race_detect"] = "not-applicable"
    Write-Host "Skipping race detector as requested (-SkipRace)." -ForegroundColor DarkGray
} else {
    $hasGcc = Get-Command "gcc" -ErrorAction SilentlyContinue
    if (-not $hasGcc -and -not $CI) {
        $GateResults["gate4_race_detect"] = "not-applicable"
        Write-Host "Notice: gcc not found in PATH; skipping race detector in local non-CI mode." -ForegroundColor DarkYellow
    } else {
        go -C $RootDir test -race ./...
        if ($LASTEXITCODE -ne 0) {
            throw "Race detection tests failed."
        }
        $GateResults["gate4_race_detect"] = "pass"
        Write-Host "Race detection passed cleanly." -ForegroundColor Green
    }
}
Exit-CiGate

# -------------------------------------------------------------
# Gate 5: PowerShell Script AST Validation
# -------------------------------------------------------------
Enter-CiGate "Gate 5: PowerShell AST Validation"
$psFiles = Get-ChildItem -Path (Join-Path $RootDir "scripts") -Filter "*.ps1" -File -Recurse
foreach ($file in $psFiles) {
    $tokens = $null
    $errors = $null
    [System.Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$tokens, [ref]$errors) | Out-Null
    if ($errors.Count -gt 0) {
        Write-Host "Syntax error in script $($file.FullName):" -ForegroundColor Red
        $errors | ForEach-Object { Write-Host "  Line $($_.Extent.StartLineNumber): $($_.Message)" -ForegroundColor Red }
        throw "PowerShell AST validation failed for $($file.Name)."
    }
}
$GateResults["gate5_pwsh_ast"] = "pass"
Write-Host "All $($psFiles.Count) PowerShell scripts passed AST validation." -ForegroundColor Green
Exit-CiGate

# -------------------------------------------------------------
# Gate 6: Production Build & Version Verification
# -------------------------------------------------------------
Enter-CiGate "Gate 6: Build & Version Verification"
if ($SkipBuild) {
    $GateResults["gate6_build"] = "not-applicable"
    Write-Host "Skipping build verification as requested (-SkipBuild)." -ForegroundColor DarkGray
} else {
    $BuildScript = Join-Path $RootDir "scripts\build.ps1"
    & pwsh -File $BuildScript -SkipTests
    if ($LASTEXITCODE -ne 0) {
        throw "Build script execution failed."
    }
    $GateResults["gate6_build"] = "pass"
}
Exit-CiGate

Write-Host "`nGate Summary:" -ForegroundColor Cyan
foreach ($k in $GateResults.Keys) {
    $v = "$($GateResults[$k])".ToUpperInvariant()
    Write-Host "  [${k}: $v]"
}

Write-Host "`n[CI SUCCESS] All ResumAI CI gates passed 100% green." -ForegroundColor Green
exit 0
