package main

import (
	"bytes"
	"encoding/json"
	"os"
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

func TestMCPSubcommandStdio(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "mcp_cli_test.db")

	in := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"resumai_vault_status","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"resumai_save_triage","arguments":{"application_id":"app-cli","dossier":{"role_title":"Staff Go","company":"Acme","raw_jd":"Go systems"}}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"resumai_render_bundle","arguments":{"application_id":"app-cli"}}}`,
	}, "\n") + "\n")

	var out, errOut bytes.Buffer
	code := runMCPWithIO([]string{"--vault", dbPath}, in, &out, &errOut, "")
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, errOut.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 JSON-RPC responses on stdout, got %d: %s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"protocolVersion"`) {
		t.Errorf("expected initialize response in line 0, got: %s", lines[0])
	}
	var toolResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &toolResp); err != nil {
		t.Fatalf("failed to unmarshal tool response: %v; raw: %s", err, lines[1])
	}
	if len(toolResp.Result.Content) == 0 || !strings.Contains(toolResp.Result.Content[0].Text, `"organizations_count": 0`) {
		t.Errorf("expected vault status JSON in line 1 content, got: %+v", toolResp)
	}
	if strings.Contains(lines[3], "packet materializer is not configured") {
		t.Errorf("expected PacketMaterializer to be wired in runMCPWithIO, got: %s", lines[3])
	}
	if !strings.Contains(lines[3], "invalid application state transition") {
		t.Errorf("expected state transition error from wired materializer path, got: %s", lines[3])
	}
}

func TestMCPSubcommandStderrRedaction(t *testing.T) {
	leakedPAT := "ghp_" + strings.Repeat("x", 36)
	// Point --vault inside a regular file so vault.OpenPath fails deterministically and surfaces path + error.
	blockingFile := filepath.Join(t.TempDir(), "not-a-dir-"+leakedPAT)
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	targetVault := filepath.Join(blockingFile, "vault.db")

	var out, errOut bytes.Buffer
	code := runMCPWithIO([]string{"--vault", targetVault}, strings.NewReader(""), &out, &errOut, "")
	if code != 1 {
		t.Fatalf("expected exit code 1 when vault open fails, got %d", code)
	}
	stderrStr := errOut.String()
	if strings.Contains(stderrStr, leakedPAT) {
		t.Fatalf("expected stderr to redact GitHub token, got: %s", stderrStr)
	}
	if !strings.Contains(stderrStr, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker in stderr, got: %s", stderrStr)
	}
	if tempDir := t.TempDir(); len(tempDir) > 3 && strings.Contains(stderrStr, filepath.Dir(tempDir)) {
		t.Fatalf("expected temp directory segment to be scrubbed from stderr, got: %s", stderrStr)
	}
}
