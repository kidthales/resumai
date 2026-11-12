package render

import (
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

func TestParseDimensionToPX(t *testing.T) {
	tests := []struct {
		name      string
		dim       string
		defaultPX float64
		expected  float64
	}{
		{"empty string", "", 48.0, 48.0},
		{"zero value", "0", 48.0, 0.0},
		{"zero inches", "0in", 48.0, 0.0},
		{"zero px", "0px", 48.0, 0.0},
		{"inches", "0.5in", 48.0, 48.0},
		{"one inch", "1in", 0.0, 96.0},
		{"points", "72pt", 0.0, 96.0},
		{"millimeters", "25.4mm", 0.0, 96.0},
		{"centimeters", "2.54cm", 0.0, 96.0},
		{"pixels", "96px", 0.0, 96.0},
		{"invalid fallback", "invalid", 48.0, 48.0},
		{"trimmed spaces", "  0.75in  ", 0.0, 72.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDimensionToPX(tt.dim, tt.defaultPX)
			if math.Abs(got-tt.expected) > 0.001 {
				t.Errorf("parseDimensionToPX(%q, %v) = %v; want %v", tt.dim, tt.defaultPX, got, tt.expected)
			}
		})
	}
}

func TestParseDimensionToInches(t *testing.T) {
	tests := []struct {
		name          string
		dim           string
		defaultInches float64
		expected      float64
	}{
		{"empty string", "", 0.5, 0.5},
		{"zero value", "0", 0.5, 0.0},
		{"zero inches", "0in", 0.5, 0.0},
		{"inches", "0.5in", 0.0, 0.5},
		{"one inch", "1in", 0.0, 1.0},
		{"points", "72pt", 0.0, 1.0},
		{"millimeters", "25.4mm", 0.0, 1.0},
		{"centimeters", "2.54cm", 0.0, 1.0},
		{"pixels", "96px", 0.0, 1.0},
		{"invalid fallback", "unknown", 0.75, 0.75},
		{"trimmed spaces", "  1.5in  ", 0.0, 1.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDimensionToInches(tt.dim, tt.defaultInches)
			if math.Abs(got-tt.expected) > 0.001 {
				t.Errorf("parseDimensionToInches(%q, %v) = %v; want %v", tt.dim, tt.defaultInches, got, tt.expected)
			}
		})
	}
}

func TestPageGeometryInches(t *testing.T) {
	t.Run("default US letter geometry", func(t *testing.T) {
		cfg := PageConfig{
			Size:         "letter",
			MarginTop:    "0.5in",
			MarginBottom: "0.5in",
			MarginLeft:   "0.5in",
			MarginRight:  "0.5in",
		}
		w, h, top, bot, left, right := pageGeometryInches(cfg)
		if w != 8.5 || h != 11.0 {
			t.Errorf("expected 8.5x11, got %vx%v", w, h)
		}
		if top != 0.5 || bot != 0.5 || left != 0.5 || right != 0.5 {
			t.Errorf("expected 0.5 margins, got top=%v bot=%v left=%v right=%v", top, bot, left, right)
		}
	})

	t.Run("A4 page geometry with metric margins", func(t *testing.T) {
		cfg := PageConfig{
			Size:         "a4",
			MarginTop:    "25.4mm",
			MarginBottom: "25.4mm",
			MarginLeft:   "12.7mm",
			MarginRight:  "12.7mm",
		}
		w, h, top, bot, left, right := pageGeometryInches(cfg)
		expectedW := 210.0 / 25.4
		expectedH := 297.0 / 25.4
		if math.Abs(w-expectedW) > 0.001 || math.Abs(h-expectedH) > 0.001 {
			t.Errorf("expected A4 %vx%v, got %vx%v", expectedW, expectedH, w, h)
		}
		if math.Abs(top-1.0) > 0.001 || math.Abs(bot-1.0) > 0.001 {
			t.Errorf("expected 1.0in vertical margins, got top=%v bot=%v", top, bot)
		}
		if math.Abs(left-0.5) > 0.001 || math.Abs(right-0.5) > 0.001 {
			t.Errorf("expected 0.5in horizontal margins, got left=%v right=%v", left, right)
		}
	})

	t.Run("empty page config defaults to US letter and 0.5in margins", func(t *testing.T) {
		cfg := PageConfig{}
		w, h, top, bot, left, right := pageGeometryInches(cfg)
		if w != 8.5 || h != 11.0 {
			t.Errorf("expected default 8.5x11, got %vx%v", w, h)
		}
		if top != 0.5 || bot != 0.5 || left != 0.5 || right != 0.5 {
			t.Errorf("expected default 0.5 margins, got top=%v bot=%v left=%v right=%v", top, bot, left, right)
		}
	})
}

func TestPageGeometryPX(t *testing.T) {
	t.Run("letter height and margin calculation", func(t *testing.T) {
		cfg := PageConfig{
			Size:         "letter",
			MarginTop:    "0.5in",
			MarginBottom: "0.5in",
		}
		h, marginY := pageGeometryPX(cfg)
		if h != 1056.0 {
			t.Errorf("expected 1056px height, got %v", h)
		}
		if marginY != 96.0 {
			t.Errorf("expected 96px marginY, got %v", marginY)
		}
	})

	t.Run("A4 height calculation", func(t *testing.T) {
		cfg := PageConfig{
			Size:         "A4",
			MarginTop:    "1in",
			MarginBottom: "1in",
		}
		h, marginY := pageGeometryPX(cfg)
		expectedH := 297.0 * 96.0 / 25.4
		if math.Abs(h-expectedH) > 0.001 {
			t.Errorf("expected A4 %v px, got %v", expectedH, h)
		}
		if marginY != 192.0 {
			t.Errorf("expected 192px marginY, got %v", marginY)
		}
	})
}

func TestChromiumLaunchArgs(t *testing.T) {
	t.Run("headless base flags", func(t *testing.T) {
		args := chromiumLaunchArgs("/tmp/resumai-test-profile")
		joined := strings.Join(args, " ")

		baseFlags := []string{
			"--headless=new",
			"--remote-debugging-port=0",
			"--disable-software-rasterizer",
			"--user-data-dir=/tmp/resumai-test-profile",
		}

		for _, flag := range baseFlags {
			if !strings.Contains(joined, flag) {
				t.Errorf("expected chromiumLaunchArgs to include %q, got %v", flag, args)
			}
		}
	})

	t.Run("ci environment sandbox disable flags", func(t *testing.T) {
		t.Setenv("CI", "true")
		args := chromiumLaunchArgs("/tmp/resumai-test-profile")
		joined := strings.Join(args, " ")

		for _, flag := range []string{"--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"} {
			if !strings.Contains(joined, flag) {
				t.Errorf("expected CI invocation to include %q, got %v", flag, args)
			}
		}
	})
}

func TestExtractDevToolsPort(t *testing.T) {
	t.Run("extracts port and drains remaining stderr", func(t *testing.T) {
		stderrInput := "Fontconfig warning: ignoring UTF-8\nDevTools listening on ws://127.0.0.1:41923/devtools/browser/abc-123\nTrailing log line\n"
		port, err := extractDevToolsPort(strings.NewReader(stderrInput), 2*time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if port != 41923 {
			t.Errorf("expected port 41923, got %d", port)
		}
	})

	t.Run("includes collected stderr when stream closes without port", func(t *testing.T) {
		stderrInput := "[0101/000000.000:ERROR:zygote_host_impl_linux.cc] Running as root without --no-sandbox is not supported.\n"
		_, err := extractDevToolsPort(strings.NewReader(stderrInput), 2*time.Second)
		if err == nil {
			t.Fatal("expected error when stderr closes without devtools port")
		}
		if !strings.Contains(err.Error(), "Running as root without --no-sandbox") {
			t.Errorf("expected error to include stderr diagnostics, got: %v", err)
		}
	})

	t.Run("includes collected stderr on timeout", func(t *testing.T) {
		pr, pw := io.Pipe()
		defer pr.Close()
		defer pw.Close()

		go func() {
			_, _ = pw.Write([]byte("[0101/000000.000:WARNING:dbus] D-Bus connection stalled\n"))
		}()

		_, err := extractDevToolsPort(pr, 80*time.Millisecond)
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !strings.Contains(err.Error(), "timeout waiting for chromium devtools port") {
			t.Errorf("expected timeout message, got: %v", err)
		}
		if !strings.Contains(err.Error(), "D-Bus connection stalled") {
			t.Errorf("expected timeout error to include collected stderr output, got: %v", err)
		}
	})

	t.Run("redacts secrets and user paths in error diagnostics", func(t *testing.T) {
		dummyToken := "ghp_" + strings.Repeat("b", 36)
		stderrInput := "[0101/000000.000:ERROR] Failed to bind to C:\\Users\\secretdev\\AppData\\Local\\Temp with token " + dummyToken + "\n"
		_, err := extractDevToolsPort(strings.NewReader(stderrInput), 2*time.Second)
		if err == nil {
			t.Fatal("expected error")
		}
		errStr := err.Error()
		if strings.Contains(errStr, "secretdev") {
			t.Errorf("expected username in path to be redacted, got: %s", errStr)
		}
		if strings.Contains(errStr, dummyToken) {
			t.Errorf("expected token to be redacted, got: %s", errStr)
		}
		if !strings.Contains(errStr, "[REDACTED]") {
			t.Errorf("expected [REDACTED] in error, got: %s", errStr)
		}
	})
}
