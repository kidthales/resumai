package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpAndLoadCLI(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "source_vault.db")

	// 1. Seed vault with PII via CLI
	var out, errOut bytes.Buffer
	code := run([]string{
		"pii", "set",
		"--vault", dbPath,
		"--name", "Ororo Munroe",
		"--email", "storm@krakoa.mesh",
		"--location", "Krakoa / Cairo",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("pii set failed with code %d: %s", code, errOut.String())
	}

	// 2. Dump without PII
	bundleNoPII := filepath.Join(tempDir, "bundle-nopii")
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"dump", bundleNoPII,
		"--vault", dbPath,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("dump failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Vault dumped successfully") || !strings.Contains(out.String(), "PII included: false") {
		t.Errorf("expected dump success output, got: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(bundleNoPII, "pii.yaml")); !os.IsNotExist(err) {
		t.Errorf("pii.yaml should not exist in dump without pii")
	}

	// 3. Dump with PII (flag before directory)
	bundlePII := filepath.Join(tempDir, "bundle-pii")
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"dump",
		"--include-pii",
		"--vault", dbPath,
		bundlePII,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("dump with pii failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "PII included: true") {
		t.Errorf("expected PII included: true, got: %s", out.String())
	}
	if _, err := os.Stat(filepath.Join(bundlePII, "pii.yaml")); err != nil {
		t.Errorf("pii.yaml should exist in dump with pii: %v", err)
	}

	// 4. Load bundle into new target vault
	targetDB := filepath.Join(tempDir, "target_vault.db")
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"load", bundlePII,
		"--vault", targetDB,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("load failed with code %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Vault loaded successfully") {
		t.Errorf("expected load success output, got: %s", out.String())
	}

	// Verify loaded target vault has PII
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"pii", "show",
		"--vault", targetDB,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("pii show on target failed: %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Ororo Munroe") || !strings.Contains(out.String(), "storm@krakoa.mesh") {
		t.Errorf("expected loaded PII in target vault, got: %s", out.String())
	}
}

func TestDumpAndLoadErrorHandling(t *testing.T) {
	var out, errOut bytes.Buffer

	// 1. Dump missing directory argument
	code := run([]string{"dump"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected code 1 for bare dump, got %d", code)
	}
	if !strings.Contains(errOut.String(), "missing target directory argument") {
		t.Errorf("expected missing dir error message, got: %s", errOut.String())
	}

	// 2. Dump help
	out.Reset()
	errOut.Reset()
	code = run([]string{"dump", "--help"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected code 0 for dump --help, got %d", code)
	}
	if !strings.Contains(out.String(), "Usage: resumai dump") {
		t.Errorf("expected dump usage output, got: %s", out.String())
	}

	// 3. Load missing directory argument
	out.Reset()
	errOut.Reset()
	code = run([]string{"load"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected code 1 for bare load, got %d", code)
	}
	if !strings.Contains(errOut.String(), "missing source directory argument") {
		t.Errorf("expected missing dir error message, got: %s", errOut.String())
	}

	// 4. Load nonexistent directory
	out.Reset()
	errOut.Reset()
	code = run([]string{"load", "nonexistent-dir-12345"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected code 1 for nonexistent dir, got %d", code)
	}
	if !strings.Contains(errOut.String(), "error reading bundle directory") {
		t.Errorf("expected error message for nonexistent dir, got: %s", errOut.String())
	}

	// 5. Load help
	out.Reset()
	errOut.Reset()
	code = run([]string{"load", "--help"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected code 0 for load --help, got %d", code)
	}
	if !strings.Contains(out.String(), "Usage: resumai load") {
		t.Errorf("expected load usage output, got: %s", out.String())
	}
}
