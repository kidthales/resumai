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

    function Remove-YamlInlineComment {
        param([string]$Text)
        $inSingle = $false
        $inDouble = $false
        for ($idx = 0; $idx -lt $Text.Length; $idx++) {
            $ch = $Text[$idx]
            if ($ch -eq "'" -and -not $inDouble) {
                if ($inSingle -and ($idx + 1 -lt $Text.Length) -and $Text[$idx + 1] -eq "'") {
                    $idx++
                    continue
                }
                $inSingle = -not $inSingle
            } elseif ($ch -eq '"' -and -not $inSingle) {
                if ($inDouble) {
                    $bsCount = 0
                    $k = $idx - 1
                    while ($k -ge 0 -and $Text[$k] -eq '\') {
                        $bsCount++
                        $k--
                    }
                    if (($bsCount % 2) -eq 1) {
                        continue
                    }
                }
                $inDouble = -not $inDouble
            } elseif ($ch -eq '#' -and -not $inSingle -and -not $inDouble) {
                if ($idx -eq 0 -or [char]::IsWhiteSpace($Text[$idx - 1])) {
                    return $Text.Substring(0, $idx).TrimEnd()
                }
            }
        }
        return $Text
    }

    function Unquote-YamlScalar {
        param([string]$Val)
        $v = $Val.Trim()
        if ($v.Length -ge 2 -and (($v.StartsWith("'") -and $v.EndsWith("'")) -or ($v.StartsWith('"') -and $v.EndsWith('"')))) {
            return $v.Substring(1, $v.Length - 2)
        }
        return $v
    }

    function Get-InlineFlowWorkingDir {
        param(
            [string]$FlowText,
            [bool]$RequireRunKey
        )
        $pattern = if ($RequireRunKey) {
            '(?:^|[{,\s])run\s*:\s*\{.*(?:^|[{,\s])working-directory\s*:\s*([^,}\s]+|"[^"]*"|''[^'']*'')'
        } else {
            '(?:^|[{,\s])working-directory\s*:\s*([^,}\s]+|"[^"]*"|''[^'']*'')'
        }
        if ($FlowText -match $pattern) {
            return @{
                Found = $true
                Value = (Unquote-YamlScalar -Val $matches[1])
            }
        }
        return @{
            Found = $false
            Value = ""
        }
    }

    function New-DefaultsState {
        return @{
            InDefaults     = $false
            DefaultsIndent = -1
            ChildIndent    = -1
            InRun          = $false
            RunIndent      = -1
            RunChildIndent = -1
            HasWorkingDir  = $false
            WorkingDirVal  = ""
        }
    }

    function Enter-DefaultsBlock {
        param(
            [hashtable]$State,
            [int]$Indent,
            [string]$PropVal
        )
        $State.InDefaults = $true
        $State.DefaultsIndent = $Indent
        $State.ChildIndent = -1
        $State.InRun = $false
        $State.RunIndent = -1
        $State.RunChildIndent = -1
        if ($PropVal -ne "") {
            $flowMatch = Get-InlineFlowWorkingDir -FlowText $PropVal -RequireRunKey $true
            if ($flowMatch.Found) {
                $State.HasWorkingDir = $true
                $State.WorkingDirVal = $flowMatch.Value
            }
        }
    }

    function Step-DefaultsWorkingDir {
        param(
            [hashtable]$State,
            [int]$Indent,
            [string]$CleanTrimmed
        )
        if ($Indent -le $State.DefaultsIndent) {
            return $false
        }
        if ($State.ChildIndent -eq -1) {
            $State.ChildIndent = $Indent
        }
        if ($State.InRun -and $Indent -le $State.RunIndent) {
            $State.InRun = $false
            $State.RunIndent = -1
            $State.RunChildIndent = -1
        }
        $colonIdx = $CleanTrimmed.IndexOf(":")
        if ($colonIdx -gt 0) {
            $propKey = $CleanTrimmed.Substring(0, $colonIdx).Trim()
            $propVal = Unquote-YamlScalar -Val $CleanTrimmed.Substring($colonIdx + 1)
            if (-not $State.InRun -and $Indent -eq $State.ChildIndent -and $propKey -eq "run") {
                $State.InRun = $true
                $State.RunIndent = $Indent
                $State.RunChildIndent = -1
                if ($propVal -ne "") {
                    $flowMatch = Get-InlineFlowWorkingDir -FlowText $propVal -RequireRunKey $false
                    if ($flowMatch.Found) {
                        $State.HasWorkingDir = $true
                        $State.WorkingDirVal = $flowMatch.Value
                    }
                }
            } elseif ($State.InRun -and $Indent -gt $State.RunIndent) {
                if ($State.RunChildIndent -eq -1) {
                    $State.RunChildIndent = $Indent
                }
                if ($Indent -eq $State.RunChildIndent -and $propKey -eq "working-directory") {
                    $State.HasWorkingDir = $true
                    $State.WorkingDirVal = $propVal
                }
            }
        }
        return $true
    }

    # Exact job-id/step-name allowlist with exact required run text (or $null if command is unconstrained)
    $ExactStepAllowlist = @{
        "test-and-build/Install govulncheck"                         = "go install golang.org/x/vuln/cmd/govulncheck@v1.9.0"
        "notify/Dispatch consolidated status card to #dev-ci-builds" = $null
        "release/Build cross-platform stripped release binaries"     = $null
        "release/Verify tagged commit is on main"                    = $null
        "release/Generate SHA256SUMS"                                = $null
        "release/Verify resumai version output"                      = $null
        "release/Attach release assets"                              = $null
    }

    # Canonical single-source CI invocation strings accepted without allowlisting
    $CanonicalCiRuns = @(
        "./scripts/ci-local.ps1 -CI",
        "pwsh ./scripts/ci-local.ps1 -CI",
        "pwsh -File ./scripts/ci-local.ps1 -CI"
    )

    $wfFiles = @(Get-ChildItem -Path $TargetWorkflowsDir -File | Where-Object { $_.Extension -in @(".yml", ".yaml") })
    if ($wfFiles.Count -eq 0) {
        throw "Gate 0 failed: no workflow files (*.yml, *.yaml) found in $TargetWorkflowsDir"
    }

    $violations = @()

    foreach ($wf in $wfFiles) {
        $lines = @(Get-Content -Path $wf.FullName)
        $inJobs = $false
        $jobsIndent = -1
        $workflowDefaults = New-DefaultsState
        $workflowHasVerifyJob = $false
        $workflowVerifyJobNames = @()

        $currentJob = ""
        $jobIndent = -1
        $jobPropIndent = -1
        $jobDefaults = New-DefaultsState

        $inSteps = $false
        $stepsIndent = -1
        $inStepItem = $false
        $stepItemIndent = -1
        $stepPropIndent = -1
        $currentStepName = ""
        $currentStepHasRun = $false
        $currentStepRunLines = @()
        $currentStepHasIf = $false
        $currentStepIfVal = ""
        $currentStepContinueOnError = $false
        $currentStepHasWorkingDir = $false
        $currentStepWorkingDirVal = ""
        $currentStepHasEnv = $false
        $inRunBlock = $false
        $runBlockIndent = -1

        $jobHasSteps = $false
        $jobUsesReusableWorkflow = ""
        $jobRunsCanonicalCi = $false
        $jobHasIf = $false
        $jobIfVal = ""
        $jobContinueOnError = $false
        $jobHasStrategy = $false
        $inJobStrategy = $false
        $jobStrategyIndent = -1
        $jobStrategyFailFast = ""

        for ($i = 0; $i -le $lines.Count; $i++) {
            $rawLine = if ($i -lt $lines.Count) { $lines[$i] } else { "" }
            $trimmed = $rawLine.Trim()

            if ($i -lt $lines.Count -and ($trimmed -eq "" -or $trimmed.StartsWith("#"))) {
                continue
            }

            if ($trimmed -match '^steps:\s*\[') {
                $violations += "$($wf.Name): flow-style 'steps: [...]' is prohibited to ensure deterministic Gate 0 auditing."
            }
            if ($trimmed -match '^\s*(jobs|steps):\s*&' -or $trimmed -match '^\s*-\s*\*[a-zA-Z0-9_-]+') {
                $violations += "$($wf.Name): YAML anchors and aliases are prohibited in workflow steps to ensure deterministic Gate 0 auditing."
            }

            $indent = $rawLine.Length - $rawLine.TrimStart().Length

            if ($inRunBlock) {
                if ($i -lt $lines.Count -and $indent -gt $runBlockIndent) {
                    $currentStepRunLines += $trimmed
                    continue
                } else {
                    $inRunBlock = $false
                }
            }

            $cleanRawLine = if ($i -lt $lines.Count) { Remove-YamlInlineComment -Text $rawLine } else { "" }
            $cleanTrimmed = $cleanRawLine.Trim()
            if ($i -lt $lines.Count -and ($cleanTrimmed -eq "" -or ($indent -eq 0 -and $cleanTrimmed -eq "---"))) {
                continue
            }

            $shouldFlushStep = $false
            if ($inStepItem) {
                if ($i -eq $lines.Count) {
                    $shouldFlushStep = $true
                } elseif ($indent -le $jobIndent) {
                    $shouldFlushStep = $true
                } elseif ($indent -le $stepsIndent -and -not $trimmed.StartsWith("-")) {
                    $shouldFlushStep = $true
                } elseif (($trimmed.StartsWith("- ") -or $trimmed -eq "-") -and ($stepItemIndent -eq -1 -or $indent -le $stepItemIndent)) {
                    $shouldFlushStep = $true
                }
            }

            if ($shouldFlushStep) {
                if ($currentStepHasRun) {
                    $runText = ($currentStepRunLines -join "`n").Trim()
                    $pairKey = "$currentJob/$currentStepName"
                    $isCanonicalCi = $CanonicalCiRuns -contains $runText
                    $isAllowlisted = $ExactStepAllowlist.ContainsKey($pairKey)

                    if ($isCanonicalCi) {
                        $jobRunsCanonicalCi = $true
                        if ($currentStepHasIf) {
                            $violations += "$($wf.Name): canonical CI step '$currentStepName' in job '$currentJob' must not define 'if:' conditional ('$currentStepIfVal') to preserve single-source contract."
                        }
                        if ($currentStepContinueOnError) {
                            $violations += "$($wf.Name): canonical CI step '$currentStepName' in job '$currentJob' must not define 'continue-on-error: true' to preserve single-source contract."
                        }
                        if ($currentStepHasWorkingDir) {
                            $violations += "$($wf.Name): canonical CI step '$currentStepName' in job '$currentJob' must not define 'working-directory:' ('$currentStepWorkingDirVal') to preserve single-source contract."
                        }
                        if ($currentStepHasEnv) {
                            $violations += "$($wf.Name): canonical CI step '$currentStepName' in job '$currentJob' must not define 'env:' override to preserve single-source contract."
                        }
                    }

                    if ($isAllowlisted) {
                        $expectedRun = $ExactStepAllowlist[$pairKey]
                        if ($expectedRun -ne $null -and $runText -ne $expectedRun) {
                            $violations += "$($wf.Name): job '$currentJob' step '$currentStepName' run script ('$runText') does not match expected allowlisted command ('$expectedRun')."
                        }
                    } elseif (-not $isCanonicalCi) {
                        $violations += "$($wf.Name): job '$currentJob' step '$currentStepName' runs inline script ('$runText') instead of './scripts/ci-local.ps1 -CI' and pair '$pairKey' is not in the Gate 0 exact allowlist."
                    }
                }
                $inStepItem = $false
                $stepItemIndent = -1
                $stepPropIndent = -1
                $currentStepName = ""
                $currentStepHasRun = $false
                $currentStepRunLines = @()
                $currentStepHasIf = $false
                $currentStepIfVal = ""
                $currentStepContinueOnError = $false
                $currentStepHasWorkingDir = $false
                $currentStepWorkingDirVal = ""
                $currentStepHasEnv = $false
            }

            $isEndingJob = $false
            if ($currentJob -ne "") {
                if ($i -eq $lines.Count) {
                    $isEndingJob = $true
                } elseif ($indent -eq 0 -and $cleanTrimmed -match '^[A-Za-z0-9_-]+\s*:') {
                    $isEndingJob = $true
                } elseif ($jobIndent -ne -1 -and $indent -eq $jobIndent -and -not $trimmed.StartsWith("-") -and $cleanTrimmed.EndsWith(":")) {
                    $isEndingJob = $true
                }
            }

            if ($isEndingJob) {
                $isVerifyJob = ($currentJob -in @("test-and-build", "verify")) -or $jobRunsCanonicalCi
                if ($isVerifyJob) {
                    $workflowHasVerifyJob = $true
                    $workflowVerifyJobNames += $currentJob
                    if ($jobDefaults.HasWorkingDir) {
                        $violations += "$($wf.Name): verify job '$currentJob' must not define 'defaults.run.working-directory' ('$($jobDefaults.WorkingDirVal)') to preserve single-source contract."
                    }
                }
                if ($currentJob -in @("test-and-build", "verify")) {
                    if ($jobHasSteps -and -not $jobRunsCanonicalCi) {
                        $violations += "$($wf.Name): verify job '$currentJob' defines steps but does not invoke './scripts/ci-local.ps1 -CI'."
                    } elseif (-not $jobHasSteps -and $jobUsesReusableWorkflow -notin @("./.github/workflows/ci.yml", "./.github/workflows/ci.yaml")) {
                        $violations += "$($wf.Name): verify job '$currentJob' must either invoke './scripts/ci-local.ps1 -CI' or call './.github/workflows/ci.yaml' (got uses: '$jobUsesReusableWorkflow')."
                    }
                    if ($jobHasIf) {
                        $violations += "$($wf.Name): verify job '$currentJob' must not define job-level 'if:' conditional ('$jobIfVal') to preserve single-source contract."
                    }
                    if ($jobContinueOnError) {
                        $violations += "$($wf.Name): verify job '$currentJob' must not define job-level 'continue-on-error: true' to preserve single-source contract."
                    }
                    if ($jobHasStrategy -and $jobStrategyFailFast -eq "true") {
                        $violations += "$($wf.Name): verify job '$currentJob' must not set 'strategy.fail-fast: true' to prevent matrix cancellation."
                    }
                }
                $currentJob = ""
                $jobPropIndent = -1
                $jobDefaults = New-DefaultsState
                $inJobStrategy = $false
                $jobStrategyIndent = -1
            }

            if ($i -eq $lines.Count) {
                break
            }

            if ($indent -eq 0 -and $cleanTrimmed -match '^([A-Za-z0-9_-]+)\s*:(.*)$') {
                $sectionName = $matches[1].Trim()
                $sectionVal = $matches[2].Trim()
                $workflowDefaults.InDefaults = $false
                if ($sectionName -eq "jobs") {
                    $inJobs = $true
                    $jobsIndent = 0
                    $currentJob = ""
                    $inSteps = $false
                } elseif ($sectionName -eq "defaults") {
                    $inJobs = $false
                    $currentJob = ""
                    $inSteps = $false
                    Enter-DefaultsBlock -State $workflowDefaults -Indent 0 -PropVal $sectionVal
                } else {
                    $inJobs = $false
                    $currentJob = ""
                    $inSteps = $false
                }
                continue
            }

            if ($workflowDefaults.InDefaults) {
                [void](Step-DefaultsWorkingDir -State $workflowDefaults -Indent $indent -CleanTrimmed $cleanTrimmed)
                continue
            }

            if (-not $inJobs) {
                continue
            }

            if (($jobIndent -eq -1 -or $indent -eq $jobIndent) -and $indent -gt $jobsIndent -and -not $trimmed.StartsWith("-") -and $cleanTrimmed.EndsWith(":")) {
                $jobCandidate = $cleanTrimmed.Substring(0, $cleanTrimmed.Length - 1).Trim()
                if ($jobIndent -eq -1) {
                    $jobIndent = $indent
                }
                if ($indent -eq $jobIndent) {
                    $currentJob = $jobCandidate
                    $jobPropIndent = -1
                    $inSteps = $false
                    $stepsIndent = -1
                    $jobDefaults = New-DefaultsState
                    $jobHasSteps = $false
                    $jobUsesReusableWorkflow = ""
                    $jobRunsCanonicalCi = $false
                    $jobHasIf = $false
                    $jobIfVal = ""
                    $jobContinueOnError = $false
                    $jobHasStrategy = $false
                    $inJobStrategy = $false
                    $jobStrategyIndent = -1
                    $jobStrategyFailFast = ""
                    continue
                }
            }

            if (-not $currentJob) {
                continue
            }

            if ($inSteps -and $indent -le $stepsIndent -and -not $trimmed.StartsWith("-")) {
                $inSteps = $false
            }

            if (-not $inSteps -and $indent -gt $jobIndent) {
                if ($jobPropIndent -eq -1) {
                    $jobPropIndent = $indent
                }

                if ($jobDefaults.InDefaults) {
                    if (Step-DefaultsWorkingDir -State $jobDefaults -Indent $indent -CleanTrimmed $cleanTrimmed) {
                        continue
                    }
                    $jobDefaults.InDefaults = $false
                }

                if ($inJobStrategy -and $indent -le $jobStrategyIndent) {
                    $inJobStrategy = $false
                    $jobStrategyIndent = -1
                }

                if ($indent -eq $jobPropIndent -and $cleanTrimmed -eq "steps:") {
                    $inSteps = $true
                    $stepsIndent = $indent
                    $jobHasSteps = $true
                    continue
                }

                if ($indent -eq $jobPropIndent -and $cleanTrimmed.StartsWith("uses:")) {
                    $jobUsesReusableWorkflow = Unquote-YamlScalar -Val $cleanTrimmed.Substring(5)
                    continue
                }

                $colonIdx = $cleanTrimmed.IndexOf(":")
                if ($colonIdx -gt 0) {
                    $propKey = $cleanTrimmed.Substring(0, $colonIdx).Trim()
                    $propVal = Unquote-YamlScalar -Val $cleanTrimmed.Substring($colonIdx + 1)

                    if ($indent -eq $jobPropIndent) {
                        if ($propKey -eq "defaults") {
                            Enter-DefaultsBlock -State $jobDefaults -Indent $indent -PropVal $propVal
                        } elseif ($propKey -eq "if") {
                            $jobHasIf = $true
                            $jobIfVal = $propVal
                        } elseif ($propKey -eq "continue-on-error") {
                            if ($propVal -ne "false") {
                                $jobContinueOnError = $true
                            }
                        } elseif ($propKey -eq "strategy") {
                            $jobHasStrategy = $true
                            $inJobStrategy = $true
                            $jobStrategyIndent = $indent
                        }
                    } elseif ($inJobStrategy -and $propKey -eq "fail-fast") {
                        $jobStrategyFailFast = $propVal
                    }
                }
                continue
            }

            if ($inSteps) {
                $propText = $cleanTrimmed
                $isStepStart = $false
                if (($trimmed.StartsWith("- ") -or $trimmed -eq "-") -and (-not $inStepItem -or $indent -le $stepItemIndent)) {
                    $isStepStart = $true
                    $inStepItem = $true
                    $stepItemIndent = $indent
                    $currentStepName = ""
                    $currentStepHasRun = $false
                    $currentStepRunLines = @()
                    $currentStepHasIf = $false
                    $currentStepIfVal = ""
                    $currentStepContinueOnError = $false
                    $currentStepHasWorkingDir = $false
                    $currentStepWorkingDirVal = ""
                    $currentStepHasEnv = $false
                    $dashIdx = $cleanRawLine.IndexOf("-")
                    $afterDash = $cleanRawLine.Substring($dashIdx + 1)
                    $propText = $afterDash.Trim()
                    if ($propText -ne "") {
                        $leadingAfterDash = $afterDash.Length - $afterDash.TrimStart().Length
                        $stepPropIndent = $dashIdx + 1 + $leadingAfterDash
                    } else {
                        $stepPropIndent = -1
                    }
                }

                if ($inStepItem -and $propText -ne "") {
                    $isDirectStepProp = $false
                    if ($isStepStart) {
                        $isDirectStepProp = $true
                    } elseif ($stepPropIndent -eq -1 -and $indent -gt $stepItemIndent) {
                        $stepPropIndent = $indent
                        $isDirectStepProp = $true
                    } elseif ($indent -eq $stepPropIndent) {
                        $isDirectStepProp = $true
                    }

                    if ($isDirectStepProp) {
                        $colonIdx = $propText.IndexOf(":")
                        if ($colonIdx -gt 0) {
                            $propKey = $propText.Substring(0, $colonIdx).Trim()
                            $propVal = Unquote-YamlScalar -Val $propText.Substring($colonIdx + 1)

                            if ($propKey -eq "name") {
                                $currentStepName = $propVal
                            } elseif ($propKey -eq "run") {
                                $currentStepHasRun = $true
                                if ($propVal -eq "|" -or $propVal -eq ">" -or $propVal -eq "|-" -or $propVal -eq ">-" -or $propVal -eq "") {
                                    $inRunBlock = $true
                                    $runBlockIndent = $indent
                                } else {
                                    $currentStepRunLines += $propVal
                                }
                            } elseif ($propKey -eq "if") {
                                $currentStepHasIf = $true
                                $currentStepIfVal = $propVal
                            } elseif ($propKey -eq "continue-on-error") {
                                if ($propVal -ne "false") {
                                    $currentStepContinueOnError = $true
                                }
                            } elseif ($propKey -eq "working-directory") {
                                $currentStepHasWorkingDir = $true
                                $currentStepWorkingDirVal = $propVal
                            } elseif ($propKey -eq "env") {
                                $currentStepHasEnv = $true
                            }
                        }
                    }
                }
            }
        }

        if ($workflowDefaults.HasWorkingDir -and $workflowHasVerifyJob) {
            $verifyList = $workflowVerifyJobNames -join ", "
            $violations += "$($wf.Name): workflow must not define 'defaults.run.working-directory' ('$($workflowDefaults.WorkingDirVal)') on verify job(s) ($verifyList) to preserve single-source contract."
        }
    }

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
