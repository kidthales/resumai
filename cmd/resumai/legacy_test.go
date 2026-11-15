package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportLegacyCLI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "legacy_imported.db")

	contentDir := filepath.Join("..", "..", "content")
	if _, err := os.Stat(contentDir); err != nil {
		t.Fatalf("could not locate content directory: %v", err)
	}

	var out, errOut bytes.Buffer
	code := run([]string{
		"import-legacy",
		"--vault", dbPath,
		contentDir,
	}, &out, &errOut)

	if code != 0 {
		t.Fatalf("import-legacy failed with code %d: %s", code, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "Legacy data imported successfully") {
		t.Errorf("expected success message in stdout, got: %s", output)
	}
	if !strings.Contains(output, "Contact PII: imported") {
		t.Errorf("expected Contact PII: imported, got: %s", output)
	}
	if !strings.Contains(output, "Organizations: 10") {
		t.Errorf("expected Organizations: 10, got: %s", output)
	}
	if !strings.Contains(output, "Roles: 10") {
		t.Errorf("expected Roles: 10, got: %s", output)
	}
	if !strings.Contains(output, "Career Facts: 30") {
		t.Errorf("expected Career Facts: 30, got: %s", output)
	}
	if !strings.Contains(output, "Education: 2") {
		t.Errorf("expected Education: 2, got: %s", output)
	}
	if !strings.Contains(output, "Archetypes: 7") {
		t.Errorf("expected Archetypes: 7, got: %s", output)
	}

	// Verify CLI pii show reveals imported PII
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"pii", "show",
		"--vault", dbPath,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("pii show failed: %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Jane Doe") || !strings.Contains(out.String(), "jane.doe@example.com") {
		t.Errorf("expected imported Jane Doe in pii show, got: %s", out.String())
	}
}

func TestImportLegacyCLISkipPII(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "legacy_no_pii.db")
	contentDir := filepath.Join("..", "..", "content")

	var out, errOut bytes.Buffer
	code := run([]string{
		"import-legacy",
		"--skip-pii",
		"--vault", dbPath,
		contentDir,
	}, &out, &errOut)

	if code != 0 {
		t.Fatalf("import-legacy failed: %d: %s", code, errOut.String())
	}

	if !strings.Contains(out.String(), "Contact PII: skipped (--skip-pii)") {
		t.Errorf("expected skipped PII notice, got: %s", out.String())
	}

	// Verify pii show confirms no PII configured
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"pii", "show",
		"--vault", dbPath,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected pii show to succeed with empty notice, got code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "No candidate PII configured in vault") {
		t.Errorf("expected 'No candidate PII configured in vault', got: %s", out.String())
	}
}

func TestImportLegacyCLIErrorHandling(t *testing.T) {
	var out, errOut bytes.Buffer

	// 1. Missing directory argument
	code := run([]string{"import-legacy"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1 for missing dir, got %d", code)
	}
	if !strings.Contains(errOut.String(), "missing source directory argument") {
		t.Errorf("expected missing source dir error, got: %s", errOut.String())
	}

	// 2. Invalid epistemic state flag
	out.Reset()
	errOut.Reset()
	code = run([]string{"import-legacy", "--default-state", "bogus_state", "."}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1 for invalid state, got %d", code)
	}
	if !strings.Contains(errOut.String(), "invalid --default-state") {
		t.Errorf("expected invalid state error, got: %s", errOut.String())
	}

	// 3. Help flag
	out.Reset()
	errOut.Reset()
	code = run([]string{"import-legacy", "--help"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0 for help, got %d", code)
	}
	if !strings.Contains(out.String(), "Usage: resumai import-legacy") {
		t.Errorf("expected help text, got: %s", out.String())
	}
}
