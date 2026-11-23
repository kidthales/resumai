package gate0_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidthales/resumai/internal/gate0"
)

func TestVerifyWorkflowsDir_LiveRepoWorkflows(t *testing.T) {
	t.Parallel()

	wfDir := filepath.Join("..", "..", ".github", "workflows")
	violations, err := gate0.VerifyWorkflowsDir(wfDir)
	if err != nil {
		t.Fatalf("VerifyWorkflowsDir(%q) error: %v", wfDir, err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected zero Gate 0 violations on repo workflows, got: %v", violations)
	}
}

func TestVerifyWorkflowsDir_Errors(t *testing.T) {
	t.Parallel()

	t.Run("missing_dir", func(t *testing.T) {
		t.Parallel()
		_, err := gate0.VerifyWorkflowsDir(filepath.Join(t.TempDir(), "does-not-exist"))
		if err == nil || !strings.Contains(err.Error(), "Workflows directory not found") {
			t.Fatalf("expected missing directory error, got %v", err)
		}
	})

	t.Run("empty_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		_, err := gate0.VerifyWorkflowsDir(dir)
		if err == nil || !strings.Contains(err.Error(), "no workflow files") {
			t.Fatalf("expected empty workflow dir error, got %v", err)
		}
	})
}

func TestVerifyWorkflowYAML_Matrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		fileName    string
		yaml        string
		wantSubstrs []string
	}{
		{
			name:     "positive_non_canonical_with_block_and_inline_comments_and_sibling_job",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs: # jobs comment
  test-and-build:
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
    steps:
      - name: Checkout repository
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # pinned
        with:
          working-directory: ./sub
          env: ignored-action-input
      - name: Checkout flow with
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with: { working-directory: ./sub2, env: also-ignored }
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
      - name: Run CI Pipeline
        shell: pwsh
        continue-on-error: false
        run: ./scripts/ci-local.ps1 -CI # single source of truth
  notify:
    needs: [test-and-build]
    runs-on: ubuntu-latest
    env:
      WEBHOOK: ok
    steps:
      - name: "Dispatch consolidated status card to #dev-ci-builds"
        working-directory: ./notify-dir
        run: echo "done"
`,
		},
		{
			name:     "positive_flow_step_and_flow_steps_sequence",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps: [ { name: "Run CI", run: "./scripts/ci-local.ps1 -CI" } ]
`,
		},
		{
			name:     "negative_job_level_env_block_on_verify_job",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    env:
      GOFLAGS: "-mod=readonly"
    steps:
      - name: Run CI
        run: ./scripts/ci-local.ps1 -CI
`,
			wantSubstrs: []string{
				"ci.yaml: verify job 'test-and-build' must not define job-level 'env:' override to preserve single-source contract.",
			},
		},
		{
			name:     "negative_job_level_env_flow_on_custom_canonical_job",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  custom-ci:
    runs-on: ubuntu-latest
    env: { CGO_ENABLED: "0" }
    steps:
      - name: Run CI
        run: pwsh -File ./scripts/ci-local.ps1 -CI
`,
			wantSubstrs: []string{
				"ci.yaml: verify job 'custom-ci' must not define job-level 'env:' override to preserve single-source contract.",
			},
		},
		{
			name:     "negative_workflow_level_env_after_jobs",
			fileName: "release.yaml",
			yaml: `---
name: Release
jobs:
  verify:
    uses: ./.github/workflows/ci.yaml
env:
  GOFLAGS: "-race"
`,
			wantSubstrs: []string{
				"release.yaml: workflow must not define 'env:' override on verify job(s) (verify) to preserve single-source contract.",
			},
		},
		{
			name:     "negative_flow_step_mapping_working_dir_and_env",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - { name: "Run CI", run: "./scripts/ci-local.ps1 -CI", working-directory: "./flow-dir", env: { FOO: "bar" } }
`,
			wantSubstrs: []string{
				"ci.yaml: canonical CI step 'Run CI' in job 'test-and-build' must not define 'working-directory:' ('./flow-dir') to preserve single-source contract.",
				"ci.yaml: canonical CI step 'Run CI' in job 'test-and-build' must not define 'env:' override to preserve single-source contract.",
			},
		},
		{
			name:     "negative_multiline_flow_mapping_and_flow_steps",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps: [
      {
        name: "Run CI",
        run: "./scripts/ci-local.ps1 -CI",
        working-directory: "./multi-flow"
      }
    ]
`,
			wantSubstrs: []string{
				"ci.yaml: canonical CI step 'Run CI' in job 'test-and-build' must not define 'working-directory:' ('./multi-flow') to preserve single-source contract.",
			},
		},
		{
			name:     "negative_quoted_keys",
			fileName: "ci.yaml",
			yaml: `---
name: CI
"defaults":
  "run":
    "working-directory": "./quoted-wf"
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - "name": "Run CI"
        "run": "./scripts/ci-local.ps1 -CI"
        "working-directory": "./quoted-step"
        'env':
          FOO: bar
`,
			wantSubstrs: []string{
				"ci.yaml: canonical CI step 'Run CI' in job 'test-and-build' must not define 'working-directory:' ('./quoted-step') to preserve single-source contract.",
				"ci.yaml: canonical CI step 'Run CI' in job 'test-and-build' must not define 'env:' override to preserve single-source contract.",
				"ci.yaml: workflow must not define 'defaults.run.working-directory' ('./quoted-wf') on verify job(s) (test-and-build) to preserve single-source contract.",
			},
		},
		{
			name:     "negative_yaml_anchor_and_merge_key_rejected",
			fileName: "ci.yaml",
			yaml: `---
name: CI
x-step-overrides: &inj
  working-directory: ./sub
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI
        <<: *inj
        run: ./scripts/ci-local.ps1 -CI
`,
			wantSubstrs: []string{
				"YAML anchors and aliases are prohibited",
				"YAML merge keys ('<<') are prohibited",
			},
		},
		{
			name:     "negative_duplicate_mapping_keys_rejected",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI
        run: ./scripts/ci-local.ps1 -CI
        run: echo bypass
`,
			wantSubstrs: []string{
				`duplicate YAML mapping key "run"`,
			},
		},
		{
			name:     "negative_multiple_yaml_documents_rejected",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI
        run: ./scripts/ci-local.ps1 -CI
---
name: Shadow
`,
			wantSubstrs: []string{
				"multiple YAML documents are prohibited",
			},
		},
		{
			name:     "negative_invalid_yaml_fails_closed",
			fileName: "ci.yaml",
			yaml:     "jobs:\n  test-and-build:\n    steps: [unclosed\n",
			wantSubstrs: []string{
				"ci.yaml: invalid YAML:",
			},
		},
		{
			name:     "negative_step_and_job_if_continue_on_error_and_fail_fast_regressions",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    continue-on-error: ${{ inputs.soft_fail }}
    strategy:
      fail-fast: true
    steps:
      - name: Run CI
        if: always()
        continue-on-error: true
        run: ./scripts/ci-local.ps1 -CI
`,
			wantSubstrs: []string{
				"canonical CI step 'Run CI' in job 'test-and-build' must not define 'if:' conditional ('always()')",
				"canonical CI step 'Run CI' in job 'test-and-build' must not define 'continue-on-error: true'",
				"verify job 'test-and-build' must not define job-level 'if:' conditional",
				"verify job 'test-and-build' must not define job-level 'continue-on-error: true'",
				"verify job 'test-and-build' must not set 'strategy.fail-fast: true'",
			},
		},
		{
			name:     "negative_unallowlisted_inline_script_and_mismatched_allowlist",
			fileName: "ci.yaml",
			yaml: `---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Install govulncheck
        run: go install example.com/evil@latest
      - name: Sneaky step
        run: go test ./...
      - name: Run CI
        run: ./scripts/ci-local.ps1 -CI
`,
			wantSubstrs: []string{
				"does not match expected allowlisted command",
				"runs inline script ('go test ./...') instead of './scripts/ci-local.ps1 -CI'",
			},
		},
		{
			name:     "negative_reusable_verify_caller_with_env_and_wrong_uses",
			fileName: "release.yaml",
			yaml: `---
name: Release
jobs:
  verify:
    uses: ./.github/workflows/other.yaml
    env:
      BAD: "1"
  custom-caller:
    uses: ./.github/workflows/ci.yaml
    working-directory: ./bad
    env:
      BAD: "2"
`,
			wantSubstrs: []string{
				"release.yaml: verify job 'verify' must either invoke './scripts/ci-local.ps1 -CI' or call './.github/workflows/ci.yaml'",
				"release.yaml: verify job 'verify' must not define job-level 'env:' override",
				"release.yaml: verify job 'custom-caller' must not define job-level 'working-directory:' ('./bad')",
				"release.yaml: verify job 'custom-caller' must not define job-level 'env:' override",
			},
		},
		{
			name:     "negative_malformed_defaults_and_non_scalar_run",
			fileName: "ci.yaml",
			yaml: `---
name: CI
defaults: "not-a-map"
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI
        run: [not, scalar]
`,
			wantSubstrs: []string{
				"job 'test-and-build' step 'Run CI' 'run:' must be a scalar string",
				"verify job 'test-and-build' defines steps but does not invoke './scripts/ci-local.ps1 -CI'",
				"workflow defines malformed 'defaults:'",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := gate0.VerifyWorkflowYAML(tc.fileName, []byte(tc.yaml))
			if len(tc.wantSubstrs) == 0 {
				if len(got) != 0 {
					t.Fatalf("expected 0 violations, got: %v", got)
				}
				return
			}

			joined := strings.Join(got, "\n")
			for _, want := range tc.wantSubstrs {
				if !strings.Contains(joined, want) {
					t.Errorf("expected violations to contain %q\ngot:\n%s", want, joined)
				}
			}
		})
	}
}

func TestVerifyWorkflowsDir_DetectsViolationsAcrossFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ci.yaml"), []byte(`---
name: CI
env:
  GLOBAL_OVERRIDE: "1"
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI
        run: ./scripts/ci-local.ps1 -CI
`), 0o644); err != nil {
		t.Fatal(err)
	}

	violations, err := gate0.VerifyWorkflowsDir(dir)
	if err != nil {
		t.Fatalf("VerifyWorkflowsDir error: %v", err)
	}
	if len(violations) != 1 || !strings.Contains(violations[0], "workflow must not define 'env:' override") {
		t.Fatalf("unexpected violations: %v", violations)
	}
}
