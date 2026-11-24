package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCLI(t *testing.T) {
	t.Parallel()

	t.Run("missing_workflows_flag", func(t *testing.T) {
		t.Parallel()
		if code := run([]string{}); code != 1 {
			t.Fatalf("expected exit 1, got %d", code)
		}
	})

	t.Run("valid_workflow_text_and_json", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "ci.yaml"), []byte(`---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    steps:
      - name: Run CI
        run: ./scripts/ci-local.ps1 -CI
`), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := run([]string{"-workflows", dir}); code != 0 {
			t.Fatalf("expected exit 0 in text mode, got %d", code)
		}
		if code := run([]string{"-json", "-workflows", dir}); code != 0 {
			t.Fatalf("expected exit 0 in json mode, got %d", code)
		}
	})

	t.Run("violation_returns_nonzero_in_text_mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "ci.yaml"), []byte(`---
name: CI
jobs:
  test-and-build:
    runs-on: ubuntu-latest
    env:
      BAD: "1"
    steps:
      - name: Run CI
        run: ./scripts/ci-local.ps1 -CI
`), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := run([]string{"-workflows", dir}); code != 1 {
			t.Fatalf("expected exit 1 on violation, got %d", code)
		}
		if code := run([]string{"-json", "-workflows", dir}); code != 0 {
			t.Fatalf("expected exit 0 emitting json violations, got %d", code)
		}
	})
}
