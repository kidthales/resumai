package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	discordWebhookRedactRe = regexp.MustCompile(`https://(?:ptb\.|canary\.)?discord(?:app)?\.com/api/webhooks/\d+/[A-Za-z0-9_-]+`)
	githubTokenRedactRe    = regexp.MustCompile(`\b(?:ghp_[A-Za-z0-9]{36,255}|github_pat_[A-Za-z0-9_]{22,255}|gho_[A-Za-z0-9]{36,255}|ghu_[A-Za-z0-9]{36,255}|ghs_[A-Za-z0-9]{36,255}|ghr_[A-Za-z0-9]{36,255})\b`)
	windowsUserPathRe      = regexp.MustCompile(`(?i)[a-zA-Z]:\\Users\\[^\s\\/:*?"<>|]+`)
	unixUserPathRe         = regexp.MustCompile(`/(?:home|Users)/[^\s/]+`)
)

// RedactSecrets scrubs sensitive credentials (GitHub tokens, Discord webhooks)
// and local filesystem path segments (user home directories, temp dirs, OS profile paths)
// from diagnostic strings, logs, and error messages before surfacing via CLI/MCP/Sidecar.
func RedactSecrets(s string) string {
	if s == "" {
		return ""
	}

	// 1. Scrub Discord webhooks
	s = discordWebhookRedactRe.ReplaceAllString(s, "https://discord.com/api/webhooks/[REDACTED]")

	// 2. Scrub GitHub tokens
	s = githubTokenRedactRe.ReplaceAllString(s, "[REDACTED]")

	// 3. Scrub user home directory if discoverable
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		s = strings.ReplaceAll(s, home, "[HOME]")
		s = strings.ReplaceAll(s, filepath.ToSlash(home), "[HOME]")
		s = strings.ReplaceAll(s, filepath.FromSlash(home), "[HOME]")
	}

	// 4. Scrub temporary directory if discoverable
	if tmp := os.TempDir(); tmp != "" {
		tmpTrimmed := strings.TrimRight(tmp, `/\`)
		if len(tmpTrimmed) > 3 {
			s = strings.ReplaceAll(s, tmpTrimmed, "[TEMP]")
			s = strings.ReplaceAll(s, filepath.ToSlash(tmpTrimmed), "[TEMP]")
			s = strings.ReplaceAll(s, filepath.FromSlash(tmpTrimmed), "[TEMP]")
		}
	}

	// 5. Scrub Windows user profile paths (e.g. C:\Users\john)
	s = windowsUserPathRe.ReplaceAllString(s, `[USER_HOME]`)

	// 6. Scrub Unix/macOS user paths (e.g. /home/john or /Users/john)
	s = unixUserPathRe.ReplaceAllString(s, `[USER_HOME]`)

	return s
}
