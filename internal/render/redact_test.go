package render_test

import (
	"os"
	"strings"
	"testing"

	"github.com/kidthales/resumai/internal/render"
)

func TestRedactSecrets_Empty(t *testing.T) {
	if got := render.RedactSecrets(""); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRedactSecrets_TokensAndWebhooks(t *testing.T) {
	dummyToken := "ghp_" + strings.Repeat("a", 36)
	dummyWebhook := "https://" + "discord.com/api/webhooks/123456789/abcdef-ghijk"
	input := "error sending to " + dummyWebhook + " with token " + dummyToken
	got := render.RedactSecrets(input)

	if strings.Contains(got, "123456789/abcdef-ghijk") {
		t.Errorf("expected discord webhook secret to be redacted, got %s", got)
	}
	if strings.Contains(got, dummyToken) {
		t.Errorf("expected github token to be redacted, got %s", got)
	}
	if !strings.Contains(got, "https://discord.com/api/webhooks/[REDACTED]") {
		t.Errorf("expected redacted webhook placeholder, got %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("expected [REDACTED] in output, got %s", got)
	}
}

func TestRedactSecrets_UserPaths(t *testing.T) {
	winPath := `failed to read C:\Users\administrator\AppData\Local\Temp\resumai-1234\profile.lock`
	gotWin := render.RedactSecrets(winPath)
	if strings.Contains(gotWin, `C:\Users\administrator`) {
		t.Errorf("expected windows user path to be scrubbed, got %s", gotWin)
	}
	if !strings.Contains(gotWin, `[USER_HOME]`) {
		t.Errorf("expected [USER_HOME] placeholder in windows path, got %s", gotWin)
	}

	unixPath := `failed to open /home/deployer/.cache/resumai/chromium.sock`
	gotUnix := render.RedactSecrets(unixPath)
	if strings.Contains(gotUnix, `/home/deployer`) {
		t.Errorf("expected unix user path to be scrubbed, got %s", gotUnix)
	}
	if !strings.Contains(gotUnix, `[USER_HOME]`) {
		t.Errorf("expected [USER_HOME] placeholder in unix path, got %s", gotUnix)
	}
}

func TestRedactSecrets_UserHomeAndTempDirBranches(t *testing.T) {
	// Exercise the os.UserHomeDir() live branch
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		homeTrimmed := strings.TrimRight(home, `/\`)
		if len(homeTrimmed) > 3 {
			input := "reading config from " + homeTrimmed + "/.resumai/config.yaml"
			got := render.RedactSecrets(input)
			if strings.Contains(got, homeTrimmed) {
				t.Errorf("expected live UserHomeDir %q to be scrubbed, got %s", homeTrimmed, got)
			}
			if !strings.Contains(got, "[HOME]") && !strings.Contains(got, "[USER_HOME]") {
				t.Errorf("expected [HOME] or [USER_HOME] in redacted output, got %s", got)
			}
		}
	}

	// Exercise the os.TempDir() live branch
	if tmp := os.TempDir(); tmp != "" {
		tmpTrimmed := strings.TrimRight(tmp, `/\`)
		if len(tmpTrimmed) > 3 {
			input := "cached intermediate file at " + tmpTrimmed + "/resumai-test.bin"
			got := render.RedactSecrets(input)
			if strings.Contains(got, tmpTrimmed) {
				t.Errorf("expected live TempDir %q to be scrubbed, got %s", tmpTrimmed, got)
			}
			if !strings.Contains(got, "[TEMP]") {
				t.Errorf("expected [TEMP] in redacted output, got %s", got)
			}
		}
	}
}

func TestRedactSecrets_UnixPathAnchoring(t *testing.T) {
	// Subpaths like /opt/home/x must not be mangled
	optPath := "/opt/home/service/config.json"
	gotOpt := render.RedactSecrets("check " + optPath)
	if !strings.Contains(gotOpt, optPath) {
		t.Errorf("expected /opt/home path to remain unredacted, got %s", gotOpt)
	}

	// Explicit user paths must be scrubbed
	userPath := "/home/alice/resume.pdf"
	gotUser := render.RedactSecrets("path: " + userPath)
	if strings.Contains(gotUser, "/home/alice") {
		t.Errorf("expected /home/alice to be scrubbed, got %s", gotUser)
	}
	if !strings.Contains(gotUser, "[USER_HOME]") {
		t.Errorf("expected [USER_HOME] in output, got %s", gotUser)
	}
}
