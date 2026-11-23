package internal_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runGate0Workflows(t *testing.T, pwshPath, repoRoot, workflowsDir string) (string, int, error) {
	t.Helper()

	scriptPath := filepath.Join(repoRoot, "scripts", "ci-local.ps1")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		pwshPath,
		"-NoProfile",
		"-File", scriptPath,
		"-Gate0Only",
		"-WorkflowsPath", workflowsDir,
	)
	cmd.Dir = repoRoot

	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), err
	}
	return string(out), -1, err
}

// TestGate0Workflows_CanonicalStepAndDefaultsGuards verifies that Gate 0 in
// scripts/ci-local.ps1 passes clean single-source workflows and fails closed
// with clear violation messages when canonical -CI steps or verify jobs
// inject working-directory, env, or defaults.run.working-directory overrides.
func TestGate0Workflows_CanonicalStepAndDefaultsGuards(t *testing.T) {
	pwshPath, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("skipping Gate 0 PowerShell tests: pwsh not found on PATH")
	}

	repoRoot := findRepoRoot(t)

	cases := []struct {
		name           string
		files          map[string]string
		wantFail       bool
		wantSubstrs    []string
		notWantSubstrs []string
	}{
		{
			name: "positive_baseline_canonical_ci",
			files: map[string]string{
				"ci.yaml": `---
name: CI
on: [push]
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
      - name: Run CI Pipeline (Single Source of Truth)
        shell: pwsh
        continue-on-error: false
        run: ./scripts/ci-local.ps1 -CI
`,
				"release.yaml": `---
name: Release
on:
  push:
    tags: ["v*"]
jobs:
  verify:
    uses: ./.github/workflows/ci.yaml
  release:
    needs: [verify]
    runs-on: ubuntu-latest
    steps:
      - name: Attach release assets
        env:
          GH_TOKEN: ${{ github.token }}
        run: gh release upload "$TAG" SHA256SUMS --clobber
`,
			},
			wantFail:       false,
			wantSubstrs:    []string{"All workflow run steps conform to ci-local.ps1 -CI"},
			notWantSubstrs: []string{"Gate 0 violation:"},
		},
		{
			name: "positive_non_verify_workflow_defaults_allowed",
			files: map[string]string{
				"release-only.yaml": `---
name: Release Assets
defaults:
  run:
    working-directory: ./dist
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Generate SHA256SUMS
        run: sha256sum * > SHA256SUMS
`,
			},
			wantFail:       false,
			wantSubstrs:    []string{"All workflow run steps conform to ci-local.ps1 -CI"},
			notWantSubstrs: []string{"Gate 0 violation:"},
		},
		{
			name: "negative_step_working_directory_after_run",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI Pipeline
        shell: pwsh
        run: ./scripts/ci-local.ps1 -CI
        working-directory: ./subpkg
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: canonical CI step 'Run CI Pipeline' in job 'test-and-build' must not define 'working-directory:' ('./subpkg') to preserve single-source contract.",
				"Gate 0 workflow single-source verification failed (1 violation(s)).",
			},
		},
		{
			name: "negative_step_working_directory_before_run_pwsh_prefix",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  verify:
    runs-on: windows-latest
    steps:
      - name: Run CI via pwsh
        working-directory: "nested/dir"
        run: pwsh ./scripts/ci-local.ps1 -CI
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: canonical CI step 'Run CI via pwsh' in job 'verify' must not define 'working-directory:' ('nested/dir') to preserve single-source contract.",
			},
		},
		{
			name: "negative_step_env_block_override",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI Pipeline
        shell: pwsh
        env:
          GOFLAGS: "-mod=readonly"
        run: ./scripts/ci-local.ps1 -CI
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: canonical CI step 'Run CI Pipeline' in job 'test-and-build' must not define 'env:' override to preserve single-source contract.",
			},
		},
		{
			name: "negative_step_env_flow_override",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI Pipeline
        run: pwsh -File ./scripts/ci-local.ps1 -CI
        env: { CGO_ENABLED: "0" }
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: canonical CI step 'Run CI Pipeline' in job 'test-and-build' must not define 'env:' override to preserve single-source contract.",
			},
		},
		{
			name: "negative_job_defaults_working_directory_block",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    defaults:
      run:
        shell: pwsh
        working-directory: ./subdir
    steps:
      - name: Run CI Pipeline
        run: ./scripts/ci-local.ps1 -CI
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: verify job 'test-and-build' must not define 'defaults.run.working-directory' ('./subdir') to preserve single-source contract.",
			},
		},
		{
			name: "negative_job_defaults_working_directory_flow_custom_verify_job",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  custom-ci:
    runs-on: ubuntu-latest
    defaults: { run: { working-directory: ./flow-sub } }
    steps:
      - name: Run CI Pipeline
        run: ./scripts/ci-local.ps1 -CI
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: verify job 'custom-ci' must not define 'defaults.run.working-directory' ('./flow-sub') to preserve single-source contract.",
			},
		},
		{
			name: "negative_workflow_defaults_working_directory_block",
			files: map[string]string{
				"ci.yaml": `---
name: CI
defaults:
  run:
    shell: pwsh
    working-directory: ./repo-sub
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI Pipeline
        run: ./scripts/ci-local.ps1 -CI
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: workflow must not define 'defaults.run.working-directory' ('./repo-sub') on verify job(s) (test-and-build) to preserve single-source contract.",
			},
		},
		{
			name: "negative_workflow_defaults_working_directory_flow_reusable_verify",
			files: map[string]string{
				"release.yaml": `---
name: Release
defaults: { run: { working-directory: ./wf-flow } }
jobs:
  verify:
    uses: ./.github/workflows/ci.yaml
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"release.yaml: workflow must not define 'defaults.run.working-directory' ('./wf-flow') on verify job(s) (verify) to preserve single-source contract.",
			},
		},
		{
			name: "negative_job_defaults_mixed_block_and_flow_run",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    defaults:
      run: { working-directory: './mixed-sub' }
    steps:
      - name: Run CI Pipeline
        run: ./scripts/ci-local.ps1 -CI
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: verify job 'test-and-build' must not define 'defaults.run.working-directory' ('./mixed-sub') to preserve single-source contract.",
			},
		},
		{
			name: "negative_multiline_run_with_both_step_overrides",
			files: map[string]string{
				"ci.yaml": `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI Pipeline
        run: |
          ./scripts/ci-local.ps1 -CI
        working-directory: ./multi-sub
        env:
          FOO: bar
`,
			},
			wantFail: true,
			wantSubstrs: []string{
				"Gate 0 violation:",
				"ci.yaml: canonical CI step 'Run CI Pipeline' in job 'test-and-build' must not define 'working-directory:' ('./multi-sub') to preserve single-source contract.",
				"ci.yaml: canonical CI step 'Run CI Pipeline' in job 'test-and-build' must not define 'env:' override to preserve single-source contract.",
				"Gate 0 workflow single-source verification failed (2 violation(s)).",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wfDir := t.TempDir()
			for filename, content := range tc.files {
				dst := filepath.Join(wfDir, filename)
				if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
					t.Fatalf("write workflow fixture %s: %v", filename, err)
				}
			}

			out, exitCode, runErr := runGate0Workflows(t, pwshPath, repoRoot, wfDir)
			if tc.wantFail {
				if runErr == nil || exitCode == 0 {
					t.Fatalf("expected non-zero exit code for %s, got code=%d err=%v\noutput:\n%s", tc.name, exitCode, runErr, out)
				}
				if exitCode < 0 {
					t.Fatalf("expected process exit error for %s, got non-exit error: %v\noutput:\n%s", tc.name, runErr, out)
				}
			} else if runErr != nil {
				t.Fatalf("expected exit code 0 for %s, got code=%d err=%v\noutput:\n%s", tc.name, exitCode, runErr, out)
			}

			for _, substr := range tc.wantSubstrs {
				if !strings.Contains(out, substr) {
					t.Errorf("expected output to contain %q\nfull output:\n%s", substr, out)
				}
			}
			for _, forbid := range tc.notWantSubstrs {
				if strings.Contains(out, forbid) {
					t.Errorf("expected output NOT to contain %q\nfull output:\n%s", forbid, out)
				}
			}
		})
	}
}
