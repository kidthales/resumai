package render_test

import (
	"testing"

	"github.com/kidthales/resumai/internal/render"
)

func TestSanitizeText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean string remains clean",
			input:    "Engineered robust distributed systems.",
			expected: "Engineered robust distributed systems.",
		},
		{
			name:     "strips basic html tags",
			input:    "Engineered <b>distributed</b> systems with <i>zero</i> downtime.",
			expected: "Engineered distributed systems with zero downtime.",
		},
		{
			name:     "strips malicious script blocks",
			input:    "Hello <script>alert('xss')</script>World",
			expected: "Hello World",
		},
		{
			name:     "strips embedded style tags",
			input:    "Normal text <style>body{color:red}</style> continues here.",
			expected: "Normal text continues here.",
		},
		{
			name:     "unescapes html entities",
			input:    "AT&amp;T &lt;Bell Labs&gt; &#39;1972&#39;",
			expected: "AT&T <Bell Labs> '1972'",
		},
		{
			name:     "strips non-printable control characters but keeps newlines and tabs",
			input:    "Line 1\x00\x07\x1b\n\tLine 2",
			expected: "Line 1\n\tLine 2",
		},
		{
			name:     "normalizes excessive whitespace",
			input:    "   Engineered    high-throughput    pipeline.   ",
			expected: "Engineered high-throughput pipeline.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := render.SanitizeText(tt.input)
			if actual != tt.expected {
				t.Errorf("SanitizeText(%q) = %q; expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestIsSafeURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://github.com/kidthales", true},
		{"http://localhost:8080/api", true},
		{"mailto:engineer@resumai.dev", true},
		{"javascript:alert(1)", false},
		{"data:text/html,<script>alert(1)</script>", false},
		{"vbscript:msgbox", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		actual := render.IsSafeURL(tt.url)
		if actual != tt.expected {
			t.Errorf("IsSafeURL(%q) = %v; expected %v", tt.url, actual, tt.expected)
		}
	}
}

func TestParseInlineRuns(t *testing.T) {
	t.Run("plain text without formatting", func(t *testing.T) {
		runs := render.ParseInlineRuns("Simple bullet text")
		if len(runs) != 1 {
			t.Fatalf("expected 1 run, got %d", len(runs))
		}
		if runs[0].Text != "Simple bullet text" || runs[0].Bold || runs[0].Italic || runs[0].Code {
			t.Errorf("unexpected run attributes: %+v", runs[0])
		}
	})

	t.Run("bold formatting with asterisks", func(t *testing.T) {
		runs := render.ParseInlineRuns("Led **distributed consensus** team.")
		if len(runs) != 3 {
			t.Fatalf("expected 3 runs, got %d", len(runs))
		}
		if runs[0].Text != "Led " || runs[0].Bold {
			t.Errorf("run[0] mismatch: %+v", runs[0])
		}
		if runs[1].Text != "distributed consensus" || !runs[1].Bold {
			t.Errorf("run[1] mismatch: %+v", runs[1])
		}
		if runs[2].Text != " team." || runs[2].Bold {
			t.Errorf("run[2] mismatch: %+v", runs[2])
		}
	})

	t.Run("bold formatting with underscores", func(t *testing.T) {
		runs := render.ParseInlineRuns("Maintained __high availability__ cluster.")
		if len(runs) != 3 {
			t.Fatalf("expected 3 runs, got %d", len(runs))
		}
		if runs[1].Text != "high availability" || !runs[1].Bold {
			t.Errorf("run[1] mismatch: %+v", runs[1])
		}
	})

	t.Run("italic formatting", func(t *testing.T) {
		runs := render.ParseInlineRuns("Published in *ACM SIGMOD* journal.")
		if len(runs) != 3 {
			t.Fatalf("expected 3 runs, got %d", len(runs))
		}
		if runs[1].Text != "ACM SIGMOD" || !runs[1].Italic {
			t.Errorf("run[1] mismatch: %+v", runs[1])
		}
	})

	t.Run("bold and italic formatting", func(t *testing.T) {
		runs := render.ParseInlineRuns("Achieved ***unprecedented throughput*** metrics.")
		if len(runs) != 3 {
			t.Fatalf("expected 3 runs, got %d", len(runs))
		}
		if runs[1].Text != "unprecedented throughput" || !runs[1].Bold || !runs[1].Italic {
			t.Errorf("run[1] mismatch: %+v", runs[1])
		}
	})

	t.Run("inline code", func(t *testing.T) {
		runs := render.ParseInlineRuns("Migrated services from `gRPC` to `REST`.")
		if len(runs) != 5 {
			t.Fatalf("expected 5 runs, got %d", len(runs))
		}
		if runs[1].Text != "gRPC" || !runs[1].Code {
			t.Errorf("run[1] mismatch: %+v", runs[1])
		}
		if runs[3].Text != "REST" || !runs[3].Code {
			t.Errorf("run[3] mismatch: %+v", runs[3])
		}
	})

	t.Run("valid markdown link", func(t *testing.T) {
		runs := render.ParseInlineRuns("See [Design Doc](https://resumai.dev/design) for details.")
		if len(runs) != 3 {
			t.Fatalf("expected 3 runs, got %d", len(runs))
		}
		if runs[1].Text != "Design Doc" || runs[1].Link != "https://resumai.dev/design" {
			t.Errorf("run[1] mismatch: %+v", runs[1])
		}
	})

	t.Run("malicious markdown link is sanitized", func(t *testing.T) {
		runs := render.ParseInlineRuns("Click [here](javascript:stealTokens()) now.")
		if len(runs) != 3 {
			t.Fatalf("expected 3 runs, got %d", len(runs))
		}
		if runs[1].Text != "here" {
			t.Errorf("run[1] text mismatch: %+v", runs[1])
		}
		if runs[1].Link != "" {
			t.Errorf("expected empty Link for unsafe scheme, got %q", runs[1].Link)
		}
	})

	t.Run("reconstructed PlainText matches input content without formatting markup", func(t *testing.T) {
		input := "Spearheaded **distributed event mesh** migrating from *legacy polling*, reducing latency by `45%`."
		runs := render.ParseInlineRuns(input)
		plain := render.PlainText(runs)
		expected := "Spearheaded distributed event mesh migrating from legacy polling, reducing latency by 45%."
		if plain != expected {
			t.Errorf("PlainText mismatch:\n got:      %q\n expected: %q", plain, expected)
		}
	})

	t.Run("unclosed delimiter handled safely as literal", func(t *testing.T) {
		runs := render.ParseInlineRuns("A *dangling asterisk and `unclosed backtick")
		if len(runs) == 0 {
			t.Fatal("expected runs, got 0")
		}
		plain := render.PlainText(runs)
		if plain == "" {
			t.Error("expected non-empty plain text")
		}
	})
}
