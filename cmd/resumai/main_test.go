package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/version"
)

func TestVersionAndHelpCommands(t *testing.T) {
	// 1. Version command
	var out, errOut bytes.Buffer
	code := run([]string{"version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(out.String(), version.Version) {
		t.Errorf("expected version output to contain %s, got: %s", version.Version, out.String())
	}

	// 2. Help command
	out.Reset()
	errOut.Reset()
	code = run([]string{"help"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "resumai") || !strings.Contains(out.String(), "pii") {
		t.Errorf("expected help output to mention resumai and pii, got: %s", out.String())
	}

	// 3. Unknown command
	out.Reset()
	errOut.Reset()
	code = run([]string{"unknown-cmd"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected code 1 for unknown command, got %d", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("expected error message for unknown command, got: %s", errOut.String())
	}
}

func TestPIISetAndShowLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "vault_test.db")

	// 1. Show on empty vault
	var out, errOut bytes.Buffer
	code := run([]string{"pii", "show", "--vault", dbPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 on empty show, got %d; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "No candidate PII configured") {
		t.Errorf("expected empty notification, got: %s", out.String())
	}

	// Show on empty vault with --json
	out.Reset()
	errOut.Reset()
	code = run([]string{"pii", "show", "--vault", dbPath, "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 on empty show --json, got %d; stderr: %s", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("expected '{}', got: %s", out.String())
	}

	// 2. Set with missing fields should fail
	out.Reset()
	errOut.Reset()
	code = run([]string{"pii", "set", "--vault", dbPath}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected code 1 when no field flags passed, got %d", code)
	}
	if !strings.Contains(errOut.String(), "at least one field flag must be specified") {
		t.Errorf("expected error message, got: %s", errOut.String())
	}

	// 3. Set initial PII fields
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"--vault", dbPath,
		"pii", "set",
		"--name", "Logan Howlett",
		"--email", "wolverine@krakoa.mesh",
		"--location", "Krakoa / Alberta",
		"--github", "https://github.com/wolverine",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 on pii set, got %d; stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Logan Howlett") || !strings.Contains(out.String(), "wolverine@krakoa.mesh") {
		t.Errorf("expected confirmation output, got: %s", out.String())
	}

	// 4. Show populated PII (text output)
	out.Reset()
	errOut.Reset()
	code = run([]string{"pii", "show", "--vault", dbPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 on pii show, got %d; stderr: %s", code, errOut.String())
	}
	expectedStrings := []string{
		"Candidate Contact PII (Air-Gapped):",
		"Logan Howlett",
		"wolverine@krakoa.mesh",
		"Krakoa / Alberta",
		"https://github.com/wolverine",
	}
	for _, exp := range expectedStrings {
		if !strings.Contains(out.String(), exp) {
			t.Errorf("expected output to contain %q, got: %s", exp, out.String())
		}
	}

	// 5. Show populated PII (JSON output)
	out.Reset()
	errOut.Reset()
	code = run([]string{"pii", "show", "--vault", dbPath, "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 on pii show --json, got %d; stderr: %s", code, errOut.String())
	}
	var pii domain.PIIHeader
	if err := json.Unmarshal(out.Bytes(), &pii); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v; raw: %s", err, out.String())
	}
	if pii.FullName != "Logan Howlett" || pii.Email != "wolverine@krakoa.mesh" || pii.Location != "Krakoa / Alberta" {
		t.Errorf("unmarshaled PII mismatch: %+v", pii)
	}

	// 6. Incremental update: set phone and linkedin without wiping existing name/email/github
	out.Reset()
	errOut.Reset()
	code = run([]string{
		"pii", "set",
		"--vault", dbPath,
		"--phone", "+1-555-WEAPON-X",
		"--linkedin", "https://linkedin.com/in/logan",
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 on incremental set, got %d; stderr: %s", code, errOut.String())
	}

	// Verify all fields are preserved
	out.Reset()
	errOut.Reset()
	code = run([]string{"pii", "show", "--vault", dbPath, "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d; stderr: %s", code, errOut.String())
	}
	var updatedPII domain.PIIHeader
	if err := json.Unmarshal(out.Bytes(), &updatedPII); err != nil {
		t.Fatalf("failed to unmarshal updated JSON: %v", err)
	}
	if updatedPII.FullName != "Logan Howlett" {
		t.Errorf("expected FullName preserved, got %q", updatedPII.FullName)
	}
	if updatedPII.Email != "wolverine@krakoa.mesh" {
		t.Errorf("expected Email preserved, got %q", updatedPII.Email)
	}
	if updatedPII.Phone != "+1-555-WEAPON-X" {
		t.Errorf("expected Phone updated, got %q", updatedPII.Phone)
	}
	if updatedPII.LinkedInURL != "https://linkedin.com/in/logan" {
		t.Errorf("expected LinkedIn updated, got %q", updatedPII.LinkedInURL)
	}
	if updatedPII.GitHubURL != "https://github.com/wolverine" {
		t.Errorf("expected GitHub preserved, got %q", updatedPII.GitHubURL)
	}
}

func TestPIISubcommandErrors(t *testing.T) {
	var out, errOut bytes.Buffer

	// Empty pii command
	code := run([]string{"pii"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected code 1 for bare 'pii', got %d", code)
	}

	// Unknown subcommand
	out.Reset()
	errOut.Reset()
	code = run([]string{"pii", "invalid-subcmd"}, &out, &errOut)
	if code != 1 {
		t.Errorf("expected code 1 for invalid pii subcommand, got %d", code)
	}
	if !strings.Contains(errOut.String(), "unknown pii command") {
		t.Errorf("expected error message in stderr, got: %s", errOut.String())
	}

	// Help subcommand
	out.Reset()
	errOut.Reset()
	code = run([]string{"pii", "help"}, &out, &errOut)
	if code != 0 {
		t.Errorf("expected code 0 for 'pii help', got %d", code)
	}
	if !strings.Contains(out.String(), "Usage: resumai pii") {
		t.Errorf("expected usage output, got: %s", out.String())
	}
}
