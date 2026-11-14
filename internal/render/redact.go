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
	unixUserPathRe         = regexp.MustCompile(`(^|[\s"'=:(])/(?:home|Users)/[^\s/]+`)
)

// RedactCredentials scrubs sensitive credentials (Discord webhooks and GitHub tokens)
// without rewriting local filesystem paths.
func RedactCredentials(s string) string {
	if s == "" {
		return ""
	}
	s = discordWebhookRedactRe.ReplaceAllString(s, "https://discord.com/api/webhooks/[REDACTED]")
	s = githubTokenRedactRe.ReplaceAllString(s, "[REDACTED]")
	return s
}

// RedactSecrets scrubs sensitive credentials (GitHub tokens, Discord webhooks)
// and local filesystem path segments (user home directories, temp dirs, OS profile paths)
// from diagnostic strings, logs, and error messages before surfacing via CLI/MCP/Sidecar.
func RedactSecrets(s string) string {
	home, _ := os.UserHomeDir()
	return redactSecretsCustom(s, home, os.TempDir())
}

// redactSecretsCustom scrubs secrets using explicit home and temp directories,
// allowing deterministic testing of edge-case environments without mutating process env.
func redactSecretsCustom(s, home, tmp string) string {
	if s == "" {
		return ""
	}

	// 1-2. Scrub Discord webhooks and GitHub tokens
	s = RedactCredentials(s)

	// 3. Scrub user home directory if discoverable and long enough (skip minimal paths like "/" or "C:\")
	homeTrimmed := strings.TrimRight(home, `/\`)
	if len(homeTrimmed) > 3 {
		s = strings.ReplaceAll(s, home, "[HOME]")
		s = strings.ReplaceAll(s, homeTrimmed, "[HOME]")
		s = strings.ReplaceAll(s, filepath.ToSlash(homeTrimmed), "[HOME]")
		s = strings.ReplaceAll(s, filepath.FromSlash(homeTrimmed), "[HOME]")
	}

	// 4. Scrub temporary directory if discoverable and long enough
	tmpTrimmed := strings.TrimRight(tmp, `/\`)
	if len(tmpTrimmed) > 3 {
		s = strings.ReplaceAll(s, tmp, "[TEMP]")
		s = strings.ReplaceAll(s, tmpTrimmed, "[TEMP]")
		s = strings.ReplaceAll(s, filepath.ToSlash(tmpTrimmed), "[TEMP]")
		s = strings.ReplaceAll(s, filepath.FromSlash(tmpTrimmed), "[TEMP]")
	}

	// 5. Scrub Windows user profile paths (e.g. C:\Users\john)
	s = windowsUserPathRe.ReplaceAllString(s, `[USER_HOME]`)

	// 6. Scrub Unix/macOS user paths (e.g. /home/john or /Users/john)
	s = unixUserPathRe.ReplaceAllString(s, `${1}[USER_HOME]`)

	return s
}
