<#
.SYNOPSIS
    Runs local continuous integration suite for ResumAI.
.DESCRIPTION
    Executes the single-source test and verification pipeline locally (via seldon ci run)
    and in GitHub Actions (.github/workflows/ci.yaml via ./scripts/ci-local.ps1 -CI):
    0. Gate 0: Workflow verify-job single-source contract (ci-local.ps1 -CI + exact job-id/step-name allowlist)
    1. Gate 1: Workflow action 40-char commit SHA pin verification
    2. Gate 2: Go format check (gofmt -l cmd internal)
    3. Gate 3: Go vet (go vet ./...)
    4. Gate 4: Unit & hexagonal boundary purity tests (go test -count=1 -v ./...)
    5. Gate 5: Vulnerability audit (govulncheck)
    6. Gate 6: Unit tests with race detector (CGO_ENABLED=1 go test -race -count=1 ./...)
    7. Gate 7: PowerShell AST syntax validation (scripts/*.ps1)
    8. Gate 8: Production build and version inspection (scripts/build.ps1 -SkipTests -SkipGlobal)
#>
[CmdletBinding()]
param(
    [switch]$CI,
    [switch]$SkipRace,
    [switch]$SkipBuild,
    [switch]$SkipVulnCheck,
    [switch]$Quick,
    [switch]$Gate0Only,
    [string]$WorkflowsPath = "",
    [string]$RepoRoot = ""
)

Set-StrictMode -Version Latest
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

# Fail-closed guard for -CI mode: reject any skip, quick, or partial-run switch immediately
if ($CI) {
    $rejectedFlags = @()
    if ($Quick) { $rejectedFlags += "-Quick" }
    if ($SkipRace) { $rejectedFlags += "-SkipRace" }
    if ($SkipBuild) { $rejectedFlags += "-SkipBuild" }
    if ($SkipVulnCheck) { $rejectedFlags += "-SkipVulnCheck" }
    if ($Gate0Only) { $rejectedFlags += "-Gate0Only" }
    if ($WorkflowsPath) { $rejectedFlags += "-WorkflowsPath" }
    if ($RepoRoot) { $rejectedFlags += "-RepoRoot" }
    if ($rejectedFlags.Count -gt 0) {
        $flagList = $rejectedFlags -join ", "
        [Console]::Error.WriteLine("ERROR: -CI mode is fail-closed and cannot be combined with skip or partial flags ($flagList).")
        exit 1
    }

    # Verify required tools upfront under -CI so missing binaries never silently degrade
    $requiredTools = @(
        @{ Name = "go"; Hint = "Install Go 1.27.x (or Go 1.26.x with GOTOOLCHAIN auto-download enabled) and ensure 'go' is on PATH." },
        @{ Name = "govulncheck"; Hint = "Install via 'go install golang.org/x/vuln/cmd/govulncheck@v1.9.0' and ensure GOPATH/bin is on PATH." },
        @{ Name = "gcc"; Hint = "Go -race requires CGO and a 64-bit C compiler (MinGW-w64 gcc on Windows; build-essential on Linux)." }
    )
    foreach ($tool in $requiredTools) {
        if (-not (Get-Command $tool.Name -ErrorAction SilentlyContinue)) {
            [Console]::Error.WriteLine("ERROR: -CI mode requires '$($tool.Name)' in PATH, but it was not found. $($tool.Hint)")
            exit 1
        }
    }
}

function Test-Gate0Workflows {
    param([string]$TargetWorkflowsDir)

    if (-not (Test-Path $TargetWorkflowsDir)) {
        throw "Workflows directory not found: $TargetWorkflowsDir"
    }

    $goCmd = Get-Command "go" -ErrorAction SilentlyContinue
    if (-not $goCmd) {
        throw "Gate 0 failed: 'go' executable is required on PATH for workflow YAML verification."
    }

    $toolRepoRoot = Split-Path -Parent $ScriptDir
    $hasPrevGoFlags = Test-Path Env:GOFLAGS
    $prevGoFlags = if ($hasPrevGoFlags) { $env:GOFLAGS } else { $null }
    $hasPrevGoWork = Test-Path Env:GOWORK
    $prevGoWork = if ($hasPrevGoWork) { $env:GOWORK } else { $null }
    try {
        # Pin module and workspace flags so caller environment overrides cannot alter Gate 0 helper compilation.
        $env:GOFLAGS = "-mod=readonly"
        $env:GOWORK = "off"
        $jsonLines = @(& $goCmd.Source -C $toolRepoRoot run ./internal/gate0/cmd -json -workflows $TargetWorkflowsDir)
        $exitCode = $LASTEXITCODE
    } finally {
        if ($hasPrevGoFlags) {
            $env:GOFLAGS = $prevGoFlags
        } else {
            Remove-Item Env:GOFLAGS -ErrorAction SilentlyContinue
        }
        if ($hasPrevGoWork) {
            $env:GOWORK = $prevGoWork
        } else {
            Remove-Item Env:GOWORK -ErrorAction SilentlyContinue
        }
    }
    if ($exitCode -ne 0) {
        throw "Gate 0 workflow verification helper failed with exit code $exitCode."
    }

    $jsonText = ($jsonLines -join "`n").Trim()
    if (-not $jsonText) {
        throw "Gate 0 workflow verification helper produced empty output."
    }

    $result = $jsonText | ConvertFrom-Json -ErrorAction Stop
    $violations = @($result.violations)
    if ($violations.Count -gt 0) {
        foreach ($v in $violations) {
            [Console]::Error.WriteLine("Gate 0 violation: $v")
        }
        throw "Gate 0 workflow single-source verification failed ($($violations.Count) violation(s))."
    }
}

$GateResults = [ordered]@{
    "gate0_workflow_contract" = "not-applicable"
    "gate1_action_pins"       = "not-applicable"
    "gate2_gofmt"             = "not-applicable"
    "gate3_go_vet"            = "not-applicable"
    "gate4_unit_tests"        = "not-applicable"
    "gate5_govulncheck"       = "not-applicable"
    "gate6_race_detector"     = "not-applicable"
    "gate7_pwsh_ast"          = "not-applicable"
    "gate8_build"             = "not-applicable"
}

Write-Host "Starting ResumAI local CI runner in $RootDir$(if ($CI) { ' [-CI Fail-Closed Mode]' } else { '' })..." -ForegroundColor Cyan

# -------------------------------------------------------------
# Gate 0: Workflow Single-Source Contract
# -------------------------------------------------------------
$EffectiveWorkflowsDir = if ($WorkflowsPath) { $WorkflowsPath } else { [System.IO.Path]::Combine($RootDir, ".github", "workflows") }
Enter-CiGate "Gate 0: Workflow Single-Source Contract"
try {
    Test-Gate0Workflows -TargetWorkflowsDir $EffectiveWorkflowsDir
    $GateResults["gate0_workflow_contract"] = "pass"
    Write-Host "All workflow run steps conform to ci-local.ps1 -CI or exact job-id/step-name allowlist." -ForegroundColor Green
} finally {
    Exit-CiGate
}

if ($Gate0Only) {
    exit 0
}

# -------------------------------------------------------------
# Gate 1: GitHub Actions Commit SHA Pin Verification
# -------------------------------------------------------------
Enter-CiGate "Gate 1: GitHub Actions Commit SHA Pin Verification"
try {
    if (Test-Path $EffectiveWorkflowsDir) {
        $WorkflowFiles = @(Get-ChildItem -Path $EffectiveWorkflowsDir -File | Where-Object { $_.Extension -in @(".yml", ".yaml") })
        $UnpinnedFound = $false
        foreach ($wf in $WorkflowFiles) {
            $lines = @(Get-Content $wf.FullName)
            for ($i = 0; $i -lt $lines.Count; $i++) {
                $line = $lines[$i]
                if ($line -match '^\s*(?:-\s+)?uses:\s*[''"]?([^''"\s#]+)[''"]?') {
                    $target = $matches[1]
                    if ($target -notmatch '^(\./|\.\./|docker://)') {
                        if ($target -notmatch '@[0-9a-fA-F]{40}') {
                            Write-Error "Unpinned action in $($wf.Name) line $($i+1): $target (must pin to 40-char commit SHA)"
                            $UnpinnedFound = $true
                        }
                    }
                }
            }
        }
        if ($UnpinnedFound) {
            throw "GitHub Actions commit SHA pin verification failed."
        }
        $GateResults["gate1_action_pins"] = "pass"
        Write-Host "All workflow actions pinned to 40-character SHAs." -ForegroundColor Green
    } else {
        throw "Workflows directory not found: $EffectiveWorkflowsDir"
    }
} finally {
    Exit-CiGate
}

# -------------------------------------------------------------
# Gate 2: Go Format Check
# -------------------------------------------------------------
Enter-CiGate "Gate 2: Go Format Verification"
try {
    $unformatted = @(& gofmt -l (Join-Path $RootDir "cmd") (Join-Path $RootDir "internal"))
    if ($unformatted.Count -gt 0) {
        Write-Host "Unformatted Go files detected:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
        throw "Go formatting check failed. Run 'gofmt -w .' to fix."
    }
    $GateResults["gate2_gofmt"] = "pass"
    Write-Host "All Go source files formatted properly." -ForegroundColor Green
} finally {
    Exit-CiGate
}

# -------------------------------------------------------------
# Gate 3: Go Vet
# -------------------------------------------------------------
Enter-CiGate "Gate 3: Go Vet"
try {
    go -C $RootDir vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go vet failed."
    }
    $GateResults["gate3_go_vet"] = "pass"
    Write-Host "go vet passed cleanly." -ForegroundColor Green
} finally {
    Exit-CiGate
}

# -------------------------------------------------------------
# Gate 4: Unit & Boundary Tests
# -------------------------------------------------------------
Enter-CiGate "Gate 4: Unit & Boundary Tests"
try {
    go -C $RootDir test -count=1 -v ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Unit/boundary tests failed."
    }
    $GateResults["gate4_unit_tests"] = "pass"
    Write-Host "All unit and boundary tests passed." -ForegroundColor Green
} finally {
    Exit-CiGate
}

# -------------------------------------------------------------
# Gate 5: Vulnerability Audit (govulncheck)
# -------------------------------------------------------------
Enter-CiGate "Gate 5: Vulnerability Audit (govulncheck)"
try {
    if (-not $SkipVulnCheck -and -not $Quick) {
        $GovulncheckCmd = Get-Command "govulncheck" -ErrorAction SilentlyContinue
        if ($GovulncheckCmd) {
            $output = & $GovulncheckCmd.Source -C $RootDir ./... 2>&1
            $exitCode = $LASTEXITCODE
            $outputStr = ($output | Out-String)
            if ($outputStr) {
                Write-Host $outputStr.TrimEnd()
            }
            if ($exitCode -ne 0) {
                if ($outputStr -match 'panic:' -or ($outputStr -notmatch 'Vulnerability #' -and $outputStr -notmatch 'vulnerabilities found' -and $outputStr -notmatch 'Your code is affected')) {
                    throw "govulncheck crashed (no scan result): $($outputStr.Trim())"
                }
                throw "govulncheck detected vulnerabilities."
            }
        } elseif ($CI) {
            throw "ERROR: -CI mode requires 'govulncheck' installed on PATH."
        } else {
            $output = go -C $RootDir run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -C $RootDir ./... 2>&1
            $exitCode = $LASTEXITCODE
            $outputStr = ($output | Out-String)
            if ($outputStr) {
                Write-Host $outputStr.TrimEnd()
            }
            if ($exitCode -ne 0) {
                if ($outputStr -match 'panic:' -or ($outputStr -notmatch 'Vulnerability #' -and $outputStr -notmatch 'vulnerabilities found' -and $outputStr -notmatch 'Your code is affected')) {
                    throw "govulncheck crashed (no scan result): $($outputStr.Trim())"
                }
                throw "govulncheck detected vulnerabilities."
            }
        }
        $GateResults["gate5_govulncheck"] = "pass"
        Write-Host "Vulnerability audit passed." -ForegroundColor Green
    } else {
        $GateResults["gate5_govulncheck"] = "not-applicable"
        Write-Host "Skipping vulnerability audit (-SkipVulnCheck or -Quick)." -ForegroundColor DarkGray
    }
} finally {
    Exit-CiGate
}

# -------------------------------------------------------------
# Gate 6: Race Detector
# -------------------------------------------------------------
Enter-CiGate "Gate 6: Race Detection"
try {
    if ($SkipRace -or $Quick) {
        $GateResults["gate6_race_detector"] = "not-applicable"
        Write-Host "Skipping race detector as requested (-SkipRace or -Quick)." -ForegroundColor DarkGray
    } else {
        $hasGcc = Get-Command "gcc" -ErrorAction SilentlyContinue
        if ($hasGcc) {
            $prevCgo = $env:CGO_ENABLED
            try {
                $env:CGO_ENABLED = "1"
                go -C $RootDir test -race -count=1 ./...
                if ($LASTEXITCODE -ne 0) {
                    throw "Race detection tests failed."
                }
            } finally {
                $env:CGO_ENABLED = $prevCgo
            }
            $GateResults["gate6_race_detector"] = "pass"
            Write-Host "Race detection passed cleanly." -ForegroundColor Green
        } elseif ($CI) {
            throw "ERROR: -CI mode requires 'gcc' for 'go test -race', but gcc was not found on PATH."
        } else {
            $GateResults["gate6_race_detector"] = "not-applicable"
            Write-Host "Notice: gcc not found in PATH; skipping race detector in local non-CI mode." -ForegroundColor DarkYellow
        }
    }
} finally {
    Exit-CiGate
}

# -------------------------------------------------------------
# Gate 7: PowerShell Script AST Validation
# -------------------------------------------------------------
Enter-CiGate "Gate 7: PowerShell AST Validation"
try {
    $psFiles = @(Get-ChildItem -Path (Join-Path $RootDir "scripts") -Filter "*.ps1" -File -Recurse)
    foreach ($file in $psFiles) {
        $tokens = $null
        $errors = $null
        [System.Management.Automation.Language.Parser]::ParseFile($file.FullName, [ref]$tokens, [ref]$errors) | Out-Null
        $errList = @($errors)
        if ($errList.Count -gt 0) {
            Write-Host "Syntax error in script $($file.FullName):" -ForegroundColor Red
            $errList | ForEach-Object { Write-Host "  Line $($_.Extent.StartLineNumber): $($_.Message)" -ForegroundColor Red }
            throw "PowerShell AST validation failed for $($file.Name)."
        }
    }
    $GateResults["gate7_pwsh_ast"] = "pass"
    Write-Host "All $($psFiles.Count) PowerShell scripts passed AST validation." -ForegroundColor Green
} finally {
    Exit-CiGate
}

# -------------------------------------------------------------
# Gate 8: Production Build & Version Verification
# -------------------------------------------------------------
Enter-CiGate "Gate 8: Build & Version Verification"
try {
    if ($SkipBuild) {
        $GateResults["gate8_build"] = "not-applicable"
        Write-Host "Skipping build verification as requested (-SkipBuild)." -ForegroundColor DarkGray
    } else {
        $BuildScript = Join-Path $RootDir "scripts\build.ps1"
        & pwsh -File $BuildScript -SkipTests -SkipGlobal
        if ($LASTEXITCODE -ne 0) {
            throw "Build script execution failed."
        }
        $GateResults["gate8_build"] = "pass"
    }
} finally {
    Exit-CiGate
}

Write-Host "`nGate Summary:" -ForegroundColor Cyan
foreach ($k in $GateResults.Keys) {
    $v = "$($GateResults[$k])".ToUpperInvariant()
    Write-Host "  [${k}: $v]"
}

Write-Host "`n[CI SUCCESS] All ResumAI CI gates passed 100% green." -ForegroundColor Green
exit 0
