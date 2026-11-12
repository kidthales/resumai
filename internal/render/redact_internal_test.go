package render

import (
	"strings"
	"testing"
)

func TestRedactSecretsCustom_MinimumLengthGuards(t *testing.T) {
	// Guard against HOME=/ (common in minimal containers) turning all slashes into [HOME]
	slashHomeInput := "/usr/bin/resumai --flag /etc/passwd"
	gotSlashHome := redactSecretsCustom(slashHomeInput, "/", "")
	if strings.Contains(gotSlashHome, "[HOME]") {
		t.Errorf("expected HOME='/' not to rewrite all slashes as [HOME], got: %s", gotSlashHome)
	}
	if gotSlashHome != slashHomeInput {
		t.Errorf("expected unchanged string for HOME='/', got: %s", gotSlashHome)
	}

	// Guard against root Windows drive letters (e.g. C:\) being scrubbed as [HOME]
	winRootInput := `C:\Windows\System32\drivers\etc\hosts`
	gotWinRoot := redactSecretsCustom(winRootInput, `C:\`, "")
	if strings.Contains(gotWinRoot, "[HOME]") {
		t.Errorf("expected HOME='C:\\' not to rewrite drive letter as [HOME], got: %s", gotWinRoot)
	}

	// Guard against root TEMP=/ turning all slashes into [TEMP]
	gotSlashTmp := redactSecretsCustom(slashHomeInput, "", "/")
	if strings.Contains(gotSlashTmp, "[TEMP]") {
		t.Errorf("expected TMP='/' not to rewrite all slashes as [TEMP], got: %s", gotSlashTmp)
	}

	// Valid length home and temp dirs should be redacted cleanly
	validHome := "/home/alice"
	validTmp := "/tmp/resumai-test"
	sampleMsg := "reading from " + validHome + "/data.json and writing to " + validTmp + "/out.pdf"
	gotValid := redactSecretsCustom(sampleMsg, validHome, validTmp)
	if strings.Contains(gotValid, validHome) {
		t.Errorf("expected valid home directory to be scrubbed, got: %s", gotValid)
	}
	if strings.Contains(gotValid, validTmp) {
		t.Errorf("expected valid temp directory to be scrubbed, got: %s", gotValid)
	}
	if !strings.Contains(gotValid, "[HOME]") || !strings.Contains(gotValid, "[TEMP]") {
		t.Errorf("expected [HOME] and [TEMP] placeholders in output, got: %s", gotValid)
	}
}

func TestRedactSecretsCustom_UnixUserPathAnchoring(t *testing.T) {
	// Paths with /home or /Users as a non-root subsegment must NOT be redacted
	optHomeInput := "found library at /opt/home/app/lib64/libssl.so"
	gotOpt := redactSecretsCustom(optHomeInput, "", "")
	if gotOpt != optHomeInput {
		t.Errorf("expected /opt/home path to remain unredacted, got: %s", gotOpt)
	}

	varUsersInput := "found spool at /var/Users/print/job.prn"
	gotVar := redactSecretsCustom(varUsersInput, "", "")
	if gotVar != varUsersInput {
		t.Errorf("expected /var/Users path to remain unredacted, got: %s", gotVar)
	}

	// Paths anchored by delimiters or start of string must be redacted
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "/home/charlie/.bashrc",
			expected: "[USER_HOME]/.bashrc",
		},
		{
			input:    "path=/home/charlie/resume.pdf",
			expected: "path=[USER_HOME]/resume.pdf",
		},
		{
			input:    "error in \"/Users/david/docs\"",
			expected: "error in \"[USER_HOME]/docs\"",
		},
		{
			input:    "config: '/home/eve/settings.json'",
			expected: "config: '[USER_HOME]/settings.json'",
		},
		{
			input:    "located at (/Users/frank/workspace)",
			expected: "located at ([USER_HOME]/workspace)",
		},
	}

	for _, tc := range tests {
		got := redactSecretsCustom(tc.input, "", "")
		if got != tc.expected {
			t.Errorf("input %q: expected %q, got %q", tc.input, tc.expected, got)
		}
	}
}
