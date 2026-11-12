package render

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// InlineRun represents a styled span of inline text.
type InlineRun struct {
	Text   string `json:"text" yaml:"text"`
	Bold   bool   `json:"bold,omitempty" yaml:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty" yaml:"italic,omitempty"`
	Code   bool   `json:"code,omitempty" yaml:"code,omitempty"`
	Link   string `json:"link,omitempty" yaml:"link,omitempty"`
}

var (
	// Script and style block stripper
	scriptBlockRegex = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	// Generic HTML tag stripper
	htmlTagRegex = regexp.MustCompile(`(?i)<[^>]*>`)
	// Dangerous executable HTML tags stripper (strips script/style tags unescaped from entities)
	scriptTagRegex = regexp.MustCompile(`(?i)</?(?:script|style|iframe|object|embed|applet)[^>]*>`)
	// Markdown link pattern: [label](url)
	linkRegex = regexp.MustCompile(`^\[([^\]]+)\]\(([^)]+)\)`)
	// Multiple consecutive horizontal spaces pattern
	multiSpaceRegex = regexp.MustCompile(`[ ]{2,}`)
)

// SanitizeText removes harmful control characters, strips script/style blocks and HTML tags,
// and normalizes whitespace while preserving valid punctuation and unicode characters.
func SanitizeText(s string) string {
	if s == "" {
		return ""
	}

	// 1. Remove dangerous script and style blocks entirely
	cleaned := scriptBlockRegex.ReplaceAllString(s, "")

	// 2. Strip raw HTML tags to prevent XSS or broken markup
	cleaned = htmlTagRegex.ReplaceAllString(cleaned, "")

	// 3. Unescape common HTML entities if present (e.g. &amp; -> &, &lt; -> <)
	cleaned = html.UnescapeString(cleaned)

	// 4. Strip script/style blocks and tags unescaped from entities to prevent bypass
	cleaned = scriptBlockRegex.ReplaceAllString(cleaned, "")
	cleaned = scriptTagRegex.ReplaceAllString(cleaned, "")

	// 5. Filter control characters (keep \t, \n, \r)
	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		if unicode.IsControl(r) {
			if r == '\t' || r == '\n' || r == '\r' {
				b.WriteRune(r)
			}
			continue
		}
		b.WriteRune(r)
	}

	// 6. Normalize multiple consecutive horizontal spaces
	cleaned = multiSpaceRegex.ReplaceAllString(b.String(), " ")

	return strings.TrimSpace(cleaned)
}

// SanitizeRunText sanitizes an individual InlineRun's text without trimming outer whitespace,
// ensuring inter-word spacing is preserved across runs.
func SanitizeRunText(s string) string {
	if s == "" {
		return ""
	}

	cleaned := scriptBlockRegex.ReplaceAllString(s, "")
	cleaned = htmlTagRegex.ReplaceAllString(cleaned, "")
	cleaned = html.UnescapeString(cleaned)
	cleaned = scriptBlockRegex.ReplaceAllString(cleaned, "")
	cleaned = scriptTagRegex.ReplaceAllString(cleaned, "")

	var b strings.Builder
	b.Grow(len(cleaned))
	for _, r := range cleaned {
		if unicode.IsControl(r) {
			if r == '\t' || r == '\n' || r == '\r' {
				b.WriteRune(r)
			}
			continue
		}
		b.WriteRune(r)
	}

	return multiSpaceRegex.ReplaceAllString(b.String(), " ")
}

// IsSafeURL verifies that a link uses an approved safe scheme (http, https, mailto).
func IsSafeURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	return scheme == "http" || scheme == "https" || scheme == "mailto"
}

// SanitizeInlineRuns sanitizes text and validates URLs across a slice of InlineRuns.
func SanitizeInlineRuns(runs []InlineRun) []InlineRun {
	result := make([]InlineRun, 0, len(runs))
	for _, run := range runs {
		text := SanitizeRunText(run.Text)
		if text == "" {
			continue
		}
		sanitizedRun := InlineRun{
			Text:   text,
			Bold:   run.Bold,
			Italic: run.Italic,
			Code:   run.Code,
		}
		if run.Link != "" && IsSafeURL(run.Link) {
			sanitizedRun.Link = strings.TrimSpace(run.Link)
		}
		result = append(result, sanitizedRun)
	}
	return result
}

// PlainText concatenates all InlineRun text values into a single plain text string.
func PlainText(runs []InlineRun) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// ParseInlineRuns parses inline markdown formatting (**bold**, *italic*, `code`, [label](url))
// and returns a sanitized slice of styled InlineRun structs.
func ParseInlineRuns(input string) []InlineRun {
	sanitized := SanitizeText(input)
	if sanitized == "" {
		return nil
	}

	rawRuns := tokenizeInline(sanitized)
	return SanitizeInlineRuns(rawRuns)
}

func tokenizeInline(s string) []InlineRun {
	var runs []InlineRun
	cursor := 0
	n := len(s)

	for cursor < n {
		sub := s[cursor:]

		// 1. Check for Link: [label](url)
		if strings.HasPrefix(sub, "[") {
			if loc := linkRegex.FindStringSubmatchIndex(sub); loc != nil && loc[0] == 0 {
				label := sub[loc[2]:loc[3]]
				rawLink := sub[loc[4]:loc[5]]
				runs = append(runs, InlineRun{
					Text: label,
					Link: rawLink,
				})
				cursor += loc[1]
				continue
			}
		}

		// 2. Check for Inline Code: `code`
		if strings.HasPrefix(sub, "`") && len(sub) > 2 {
			end := strings.Index(sub[1:], "`")
			if end != -1 {
				content := sub[1 : end+1]
				runs = append(runs, InlineRun{
					Text: content,
					Code: true,
				})
				cursor += end + 2
				continue
			}
		}

		// 3. Check for Bold + Italic: ***text***
		if strings.HasPrefix(sub, "***") && len(sub) > 6 {
			end := strings.Index(sub[3:], "***")
			if end != -1 {
				content := sub[3 : end+3]
				runs = append(runs, InlineRun{
					Text:   content,
					Bold:   true,
					Italic: true,
				})
				cursor += end + 6
				continue
			}
		}

		// 4. Check for Bold: **text**
		if strings.HasPrefix(sub, "**") && len(sub) > 4 {
			end := strings.Index(sub[2:], "**")
			if end != -1 {
				content := sub[2 : end+2]
				runs = append(runs, InlineRun{
					Text: content,
					Bold: true,
				})
				cursor += end + 4
				continue
			}
		}

		// 5. Check for Bold with underscores: __text__
		if strings.HasPrefix(sub, "__") && len(sub) > 4 {
			end := strings.Index(sub[2:], "__")
			if end != -1 {
				content := sub[2 : end+2]
				runs = append(runs, InlineRun{
					Text: content,
					Bold: true,
				})
				cursor += end + 4
				continue
			}
		}

		// 6. Check for Italic: *text* (ensure not followed by *)
		if strings.HasPrefix(sub, "*") && len(sub) > 2 && !strings.HasPrefix(sub, "**") {
			end := strings.Index(sub[1:], "*")
			if end != -1 && (end+2 >= len(sub) || sub[end+2] != '*') {
				content := sub[1 : end+1]
				runs = append(runs, InlineRun{
					Text:   content,
					Italic: true,
				})
				cursor += end + 2
				continue
			}
		}

		// 7. Check for Italic with underscore: _text_
		if strings.HasPrefix(sub, "_") && len(sub) > 2 && !strings.HasPrefix(sub, "__") {
			end := strings.Index(sub[1:], "_")
			if end != -1 && (end+2 >= len(sub) || sub[end+2] != '_') {
				content := sub[1 : end+1]
				runs = append(runs, InlineRun{
					Text:   content,
					Italic: true,
				})
				cursor += end + 2
				continue
			}
		}

		// Otherwise, accumulate plain text until the next formatting marker
		nextSpecial := findNextMarker(s, cursor+1)
		plainChunk := s[cursor:nextSpecial]
		runs = append(runs, InlineRun{
			Text: plainChunk,
		})
		cursor = nextSpecial
	}

	return runs
}

func findNextMarker(s string, start int) int {
	markers := []string{"[", "`", "***", "**", "__", "*", "_"}
	minPos := len(s)
	for _, m := range markers {
		pos := strings.Index(s[start:], m)
		if pos != -1 && start+pos < minPos {
			minPos = start + pos
		}
	}
	return minPos
}
