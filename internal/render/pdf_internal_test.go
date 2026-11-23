package render

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
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

func TestPageContentWidthPX(t *testing.T) {
	tests := []struct {
		name     string
		cfg      PageConfig
		expected float64
	}{
		{
			name:     "empty defaults to US letter with 0.5in margins",
			cfg:      PageConfig{},
			expected: 720.0,
		},
		{
			name: "US letter custom margins",
			cfg: PageConfig{
				Size:        "letter",
				MarginLeft:  "0.75in",
				MarginRight: "1in",
			},
			expected: 648.0,
		},
		{
			name: "A4 with 20mm margins",
			cfg: PageConfig{
				Size:        "A4",
				MarginLeft:  "20mm",
				MarginRight: "20mm",
			},
			expected: (210.0 - 40.0) * DefaultDPI / 25.4,
		},
		{
			name: "excessive margins clamp to 1px minimum",
			cfg: PageConfig{
				Size:        "letter",
				MarginLeft:  "5in",
				MarginRight: "5in",
			},
			expected: 1.0,
		},
		{
			name: "non-finite or negative margins fallback to defaults",
			cfg: PageConfig{
				Size:        "letter",
				MarginLeft:  "NaNin",
				MarginRight: "-Infin",
			},
			expected: 720.0,
		},
		{
			name: "overflowing margin string falls back to default",
			cfg: PageConfig{
				Size:        "letter",
				MarginLeft:  "1e308in",
				MarginRight: "-1in",
			},
			expected: 720.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pageContentWidthPX(tt.cfg)
			if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-tt.expected) > 0.01 {
				t.Errorf("pageContentWidthPX(%+v) = %v; want %v", tt.cfg, got, tt.expected)
			}
		})
	}
}

func TestParseDimensionToPX_RejectsNonFiniteAndNegative(t *testing.T) {
	cases := []string{"NaNin", "Infin", "-Infin", "-1in", "1e308in", "1e307in"}
	for _, c := range cases {
		if got := parseDimensionToPX(c, 48.0); got != 48.0 {
			t.Errorf("parseDimensionToPX(%q, 48.0) = %v; want 48.0", c, got)
		}
		if got := parseDimensionToInches(c, 0.5); got != 0.5 {
			t.Errorf("parseDimensionToInches(%q, 0.5) = %v; want 0.5", c, got)
		}
	}
}

func TestTelemetryGuards_DegenerateVerticalMargins(t *testing.T) {
	for _, margin := range []string{"5.5in", "6in"} {
		cfg := PageConfig{
			Size:         "letter",
			MarginTop:    margin,
			MarginBottom: margin,
		}
		telemetry := CalculateLayoutTelemetry(500.0, 0, 1, nil, cfg)
		if telemetry.PageCount != 1 || telemetry.OverflowPX != 0 {
			t.Errorf("margin=%s: got PageCount=%d OverflowPX=%v; want 1, 0", margin, telemetry.PageCount, telemetry.OverflowPX)
		}
		if math.IsNaN(telemetry.LastPageFillRatio) || math.IsInf(telemetry.LastPageFillRatio, 0) {
			t.Errorf("margin=%s: non-finite LastPageFillRatio=%v", margin, telemetry.LastPageFillRatio)
		}
	}

	script := buildTelemetryScript(1, PageConfig{Size: "letter", MarginTop: "6in", MarginBottom: "6in"})
	if !strings.Contains(script, "Number.isFinite(rawUsableHeight) && rawUsableHeight >= 1.0") {
		t.Errorf("buildTelemetryScript missing finite usableHeight >= 1.0 guard")
	}
}

func TestPageViewportPX(t *testing.T) {
	tests := []struct {
		name       string
		cfg        PageConfig
		wantWidth  int
		wantHeight int
	}{
		{
			name:       "US letter default",
			cfg:        PageConfig{},
			wantWidth:  720,
			wantHeight: 960,
		},
		{
			name: "A4 with 20mm margins rounds up to integer viewport",
			cfg: PageConfig{
				Size:         "a4",
				MarginTop:    "20mm",
				MarginBottom: "20mm",
				MarginLeft:   "20mm",
				MarginRight:  "20mm",
			},
			wantWidth:  int(math.Ceil((210.0 - 40.0) * DefaultDPI / 25.4)),
			wantHeight: int(math.Ceil((297.0 - 40.0) * DefaultDPI / 25.4)),
		},
		{
			name: "excessive vertical margins fallback to full page height",
			cfg: PageConfig{
				Size:         "letter",
				MarginTop:    "6in",
				MarginBottom: "6in",
				MarginLeft:   "5in",
				MarginRight:  "5in",
			},
			wantWidth:  1,
			wantHeight: 1056,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := pageViewportPX(tt.cfg)
			if gotW != tt.wantWidth || gotH != tt.wantHeight {
				t.Errorf("pageViewportPX(%+v) = (%d, %d); want (%d, %d)", tt.cfg, gotW, gotH, tt.wantWidth, tt.wantHeight)
			}
		})
	}
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
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "")
		args := chromiumLaunchArgs("/tmp/resumai-test-profile")
		joined := strings.Join(args, " ")

		for _, flag := range []string{"--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"} {
			if !strings.Contains(joined, flag) {
				t.Errorf("expected CI invocation to include %q, got %v", flag, args)
			}
		}
	})

	t.Run("desktop linux keeps sandbox active when not root container or CI", func(t *testing.T) {
		t.Setenv("CI", "")
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "")

		if shouldDisableChromiumSandboxWithWriter("linux", 1000, false, io.Discard) {
			t.Fatal("expected desktop Linux non-root non-container non-CI host to keep Chromium sandbox enabled")
		}

		args := chromiumLaunchArgsFor("/tmp/resumai-test-profile", "linux", false)
		joined := strings.Join(args, " ")
		for _, forbidden := range []string{"--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage", "--no-zygote"} {
			if strings.Contains(joined, forbidden) {
				t.Errorf("expected desktop Linux invocation not to include %q, got %v", forbidden, args)
			}
		}
	})

	t.Run("falsy CI env does not disable sandbox", func(t *testing.T) {
		t.Setenv("CI", "false")
		t.Setenv("GITHUB_ACTIONS", "0")
		t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "")

		if shouldDisableChromiumSandboxWithWriter("linux", 1000, false, io.Discard) {
			t.Fatal("expected CI=false and GITHUB_ACTIONS=0 not to disable Chromium sandbox")
		}
	})

	t.Run("explicit opt-out, container, and root disable sandbox", func(t *testing.T) {
		t.Setenv("CI", "")
		t.Setenv("GITHUB_ACTIONS", "")
		t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "")

		if !shouldDisableChromiumSandboxWithWriter("linux", 0, false, io.Discard) {
			t.Error("expected euid=0 to disable sandbox")
		}
		if !shouldDisableChromiumSandboxWithWriter("linux", 1000, true, io.Discard) {
			t.Error("expected /.dockerenv to disable sandbox")
		}

		t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "1")
		if !shouldDisableChromiumSandboxWithWriter("linux", 1000, false, io.Discard) {
			t.Error("expected RESUMAI_CHROMIUM_NO_SANDBOX=1 to disable sandbox")
		}

		args := chromiumLaunchArgsFor("/tmp/resumai-test-profile", "linux", true)
		joined := strings.Join(args, " ")
		for _, flag := range []string{"--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage", "--no-zygote"} {
			if !strings.Contains(joined, flag) {
				t.Errorf("expected sandbox-disabled Linux invocation to include %q, got %v", flag, args)
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

func TestFilePathToURI(t *testing.T) {
	if got := filePathToURI("/tmp/resumai-render-123.html"); got != "file:///tmp/resumai-render-123.html" {
		t.Errorf("unix temp path URI = %q, want %q", got, "file:///tmp/resumai-render-123.html")
	}
	if got := filePathToURI("/tmp/dir with spaces/resumai.html"); got != "file:///tmp/dir%20with%20spaces/resumai.html" {
		t.Errorf("unix space path URI = %q, want %q", got, "file:///tmp/dir%20with%20spaces/resumai.html")
	}
	if runtime.GOOS == "windows" {
		if got := filePathToURI(`C:\Users\dev\Temp\resumai-render-123.html`); got != "file:///C:/Users/dev/Temp/resumai-render-123.html" {
			t.Errorf("windows path URI = %q, want %q", got, "file:///C:/Users/dev/Temp/resumai-render-123.html")
		}
	}
}

func readClientWSFrame(r *bufio.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	masked := (header[1] & 0x80) != 0
	length := int(header[1] & 0x7F)
	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint16(ext))
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(r, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return payload, nil
}

func writeServerWSFrame(w io.Writer, payload []byte) error {
	var buf bytes.Buffer
	buf.WriteByte(0x81)
	if len(payload) < 126 {
		buf.WriteByte(byte(len(payload)))
	} else {
		buf.WriteByte(126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(len(payload)))
		buf.Write(ext[:])
	}
	buf.Write(payload)
	_, err := w.Write(buf.Bytes())
	return err
}

func TestCDPWebSocket_EventsAndContextRetry(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	ws := &cdpWebSocket{
		conn:    clientConn,
		r:       bufio.NewReader(clientConn),
		pending: make(map[int64]chan cdpResponse),
		events:  make(map[string]map[uint64]chan json.RawMessage),
		closed:  make(chan struct{}),
	}
	go ws.readLoop()
	defer ws.Close()

	loadCh, unsub := ws.Subscribe("Page.loadEventFired")
	defer unsub()

	errCh := make(chan error, 1)
	go func() {
		sr := bufio.NewReader(serverConn)
		// 1. Expect Page.navigate
		rawNav, err := readClientWSFrame(sr)
		if err != nil {
			errCh <- err
			return
		}
		var navCmd struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(rawNav, &navCmd); err != nil {
			errCh <- err
			return
		}
		// Reply to Page.navigate, then emit Page.loadEventFired
		if err := writeServerWSFrame(serverConn, []byte(`{"id":1,"result":{"frameId":"f1","loaderId":"l1"}}`)); err != nil {
			errCh <- err
			return
		}
		if err := writeServerWSFrame(serverConn, []byte(`{"method":"Page.loadEventFired","params":{"timestamp":123.45}}`)); err != nil {
			errCh <- err
			return
		}

		// 2. First Runtime.evaluate -> transient -32000 Execution context was destroyed
		rawEval1, err := readClientWSFrame(sr)
		if err != nil {
			errCh <- err
			return
		}
		var evalCmd1 struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(rawEval1, &evalCmd1)
		resp1, _ := json.Marshal(map[string]any{
			"id": evalCmd1.ID,
			"error": map[string]any{
				"code":    -32000,
				"message": "Execution context was destroyed.",
			},
		})
		if err := writeServerWSFrame(serverConn, resp1); err != nil {
			errCh <- err
			return
		}

		// 3. Second Runtime.evaluate -> resolves false (stale about:blank load event)
		rawEval2, err := readClientWSFrame(sr)
		if err != nil {
			errCh <- err
			return
		}
		var evalCmd2 struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(rawEval2, &evalCmd2)
		resp2, _ := json.Marshal(map[string]any{
			"id":     evalCmd2.ID,
			"result": map[string]any{"result": map[string]any{"value": false}},
		})
		if err := writeServerWSFrame(serverConn, resp2); err != nil {
			errCh <- err
			return
		}

		// Emit committed Page.loadEventFired before third evaluation
		if err := writeServerWSFrame(serverConn, []byte(`{"method":"Page.loadEventFired","params":{"timestamp":124.00}}`)); err != nil {
			errCh <- err
			return
		}

		// 4. Third Runtime.evaluate -> resolves true
		rawEval3, err := readClientWSFrame(sr)
		if err != nil {
			errCh <- err
			return
		}
		var evalCmd3 struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(rawEval3, &evalCmd3)
		resp3, _ := json.Marshal(map[string]any{
			"id":     evalCmd3.ID,
			"result": map[string]any{"result": map[string]any{"value": true}},
		})
		if err := writeServerWSFrame(serverConn, resp3); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := ws.Call(ctx, "Page.navigate", map[string]any{"url": "file:///tmp/test.html"}); err != nil {
		t.Fatalf("Page.navigate failed: %v", err)
	}
	select {
	case <-loadCh:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for Page.loadEventFired: %v", ctx.Err())
	}

	if err := awaitDocumentAndFonts(ctx, ws, loadCh); err != nil {
		t.Fatalf("awaitDocumentAndFonts failed: %v", err)
	}
	if serr := <-errCh; serr != nil {
		t.Fatalf("mock CDP server error: %v", serr)
	}
}

func testServerPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("failed to parse httptest server URL %q: %v", srv.URL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("failed to parse httptest server port %q: %v", u.Port(), err)
	}
	return port
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFetchPageWebSocketURL_SingleFetchAndRetries(t *testing.T) {
	t.Run("positive single /json/list fetch returns page target without /json/new", func(t *testing.T) {
		var listCalls atomic.Int32
		var newCalls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/json/list":
				listCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `[{"id":"sw1","type":"service_worker","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/sw/1"},{"id":"p1","type":"page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/p1"}]`)
			case "/json/new":
				newCalls.Add(1)
				http.Error(w, "unexpected /json/new", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		wsURL, err := getPageWebSocketURL(ctx, testServerPort(t, srv))
		if err != nil {
			t.Fatalf("getPageWebSocketURL failed: %v", err)
		}
		if wsURL != "ws://127.0.0.1:9222/devtools/page/p1" {
			t.Errorf("got wsURL = %q, want %q", wsURL, "ws://127.0.0.1:9222/devtools/page/p1")
		}
		if got := listCalls.Load(); got != 1 {
			t.Errorf("expected exactly 1 /json/list GET request, got %d", got)
		}
		if got := newCalls.Load(); got != 0 {
			t.Errorf("expected 0 /json/new requests, got %d", got)
		}
	})

	t.Run("positive retries transient 503 with single fetch per iteration", func(t *testing.T) {
		var listCalls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/json/list" {
				http.NotFound(w, r)
				return
			}
			attempt := listCalls.Add(1)
			if attempt == 1 {
				http.Error(w, "starting up", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `[{"id":"p2","type":"page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/p2"}]`)
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		wsURL, err := getPageWebSocketURLWithRetry(ctx, testServerPort(t, srv), 5*time.Second, time.Millisecond)
		if err != nil {
			t.Fatalf("getPageWebSocketURLWithRetry failed after transient 503: %v", err)
		}
		if wsURL != "ws://127.0.0.1:9222/devtools/page/p2" {
			t.Errorf("got wsURL = %q, want %q", wsURL, "ws://127.0.0.1:9222/devtools/page/p2")
		}
		if got := listCalls.Load(); got != 2 {
			t.Errorf("expected exactly 2 /json/list calls across 2 attempts, got %d", got)
		}
	})

	t.Run("positive falls back to PUT /json/new when no page target exists", func(t *testing.T) {
		var listCalls atomic.Int32
		var newMethod string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/json/list":
				listCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `[{"id":"bg","type":"background_page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/bg"}]`)
			case "/json/new":
				newMethod = r.Method
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"created","type":"page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/created"}`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		wsURL, err := fetchPageWebSocketURLOnce(ctx, srv.Client(), testServerPort(t, srv))
		if err != nil {
			t.Fatalf("fetchPageWebSocketURLOnce failed: %v", err)
		}
		if wsURL != "ws://127.0.0.1:9222/devtools/page/created" {
			t.Errorf("got wsURL = %q, want created target URL", wsURL)
		}
		if newMethod != http.MethodPut {
			t.Errorf("expected PUT /json/new, got %q", newMethod)
		}
		if got := listCalls.Load(); got != 1 {
			t.Errorf("expected 1 /json/list call, got %d", got)
		}
	})

	t.Run("negative cancelled context short-circuits without HTTP requests", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			http.Error(w, "should not be called", http.StatusInternalServerError)
		}))
		defer srv.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := getPageWebSocketURL(ctx, testServerPort(t, srv))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		if got := calls.Load(); got != 0 {
			t.Errorf("expected 0 HTTP requests on pre-cancelled context, got %d", got)
		}
	})

	t.Run("negative invalid JSON, /json/new failures, oversized payloads, and retry exhaustion propagate errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/json/list":
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{not-json`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := fetchPageWebSocketURLOnce(ctx, srv.Client(), testServerPort(t, srv))
		if err == nil || !strings.Contains(err.Error(), "failed to decode /json/list response") {
			t.Fatalf("expected JSON decode error, got %v", err)
		}

		// /json/new non-200, invalid JSON, empty webSocketDebuggerUrl, and transport errors must propagate.
		newErrorCases := []struct {
			name        string
			newStatus   int
			newBody     string
			wantErrPart string
		}{
			{
				name:        "HTTP 500",
				newStatus:   http.StatusInternalServerError,
				newBody:     "boom",
				wantErrPart: "/json/new returned HTTP 500",
			},
			{
				name:        "invalid JSON",
				newStatus:   http.StatusOK,
				newBody:     `{bad`,
				wantErrPart: "failed to decode /json/new response",
			},
			{
				name:        "missing webSocketDebuggerUrl",
				newStatus:   http.StatusOK,
				newBody:     `{"id":"p1","type":"page"}`,
				wantErrPart: "/json/new response missing webSocketDebuggerUrl",
			},
		}
		for _, tc := range newErrorCases {
			t.Run("/json/new "+tc.name, func(t *testing.T) {
				newErrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/json/list":
						w.Header().Set("Content-Type", "application/json")
						_, _ = io.WriteString(w, `[]`)
					case "/json/new":
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(tc.newStatus)
						_, _ = io.WriteString(w, tc.newBody)
					default:
						http.NotFound(w, r)
					}
				}))
				defer newErrSrv.Close()

				_, err := fetchPageWebSocketURLOnce(ctx, newErrSrv.Client(), testServerPort(t, newErrSrv))
				if err == nil || !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrPart, err)
				}
			})
		}

		transportErr := errors.New("simulated /json/new transport failure")
		transportClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/json/list" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`[]`)),
						Header:     make(http.Header),
					}, nil
				}
				return nil, transportErr
			}),
		}
		_, err = fetchPageWebSocketURLOnce(ctx, transportClient, 9222)
		if !errors.Is(err, transportErr) {
			t.Fatalf("expected /json/new transport error %v, got %v", transportErr, err)
		}

		// Oversized discovery responses are bounded by io.LimitReader and fail decode cleanly.
		oversizedListClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				payload := `[{"id":"x","type":"page","webSocketDebuggerUrl":"ws://127.0.0.1:9222/p","pad":"` + strings.Repeat("a", maxCDPDiscoveryBodyBytes) + `"}]`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(payload)),
					Header:     make(http.Header),
				}, nil
			}),
		}
		_, err = fetchPageWebSocketURLOnce(ctx, oversizedListClient, 9222)
		if err == nil || !strings.Contains(err.Error(), "failed to decode /json/list response") {
			t.Fatalf("expected bounded decode error on oversized /json/list response, got %v", err)
		}

		oversizedNewClient := &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/json/list" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`[]`)),
						Header:     make(http.Header),
					}, nil
				}
				payload := `{"id":"x","type":"page","pad":"` + strings.Repeat("b", maxCDPDiscoveryBodyBytes) + `","webSocketDebuggerUrl":"ws://127.0.0.1:9222/p"}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(payload)),
					Header:     make(http.Header),
				}, nil
			}),
		}
		_, err = fetchPageWebSocketURLOnce(ctx, oversizedNewClient, 9222)
		if err == nil || !strings.Contains(err.Error(), "failed to decode /json/new response") {
			t.Fatalf("expected bounded decode error on oversized /json/new response, got %v", err)
		}

		// Retry exhaustion preserves errors across /json/list and /json/new failures.
		var exhaustCalls atomic.Int32
		exhaustSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/json/list":
				if exhaustCalls.Add(1) == 1 {
					http.Error(w, "starting up", http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `[]`)
			case "/json/new":
				http.Error(w, "unavailable", http.StatusNotFound)
			default:
				http.NotFound(w, r)
			}
		}))
		defer exhaustSrv.Close()

		_, err = getPageWebSocketURLWithRetry(ctx, testServerPort(t, exhaustSrv), 25*time.Millisecond, time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "retries exhausted connecting to /json/list") || !strings.Contains(err.Error(), "/json/new returned HTTP 404") {
			t.Fatalf("expected retry exhaustion preserving /json/new HTTP 404 error, got %v", err)
		}

		emptyCtx, emptyCancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		defer emptyCancel()

		_, err = getPageWebSocketURLWithRetry(emptyCtx, testServerPort(t, exhaustSrv), 10*time.Second, time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded when /json/list stays empty until deadline, got %v", err)
		}
	})
}

func TestParseAndDialCDPLoopbackWebSocketURL(t *testing.T) {
	t.Run("positive explicit loopback hosts normalize dial address", func(t *testing.T) {
		tests := []struct {
			name     string
			rawURL   string
			wantAddr string
		}{
			{
				name:     "IPv4 loopback",
				rawURL:   "ws://127.0.0.1:9222/devtools/page/abc",
				wantAddr: "127.0.0.1:9222",
			},
			{
				name:     "localhost mixed case normalizes to 127.0.0.1",
				rawURL:   "ws://LocalHost:9333/devtools/page/abc",
				wantAddr: "127.0.0.1:9333",
			},
			{
				name:     "localhost default ws port 80 normalizes to 127.0.0.1",
				rawURL:   "ws://localhost/devtools/page/abc",
				wantAddr: "127.0.0.1:80",
			},
			{
				name:     "IPv6 loopback",
				rawURL:   "ws://[::1]:9222/devtools/page/abc",
				wantAddr: "[::1]:9222",
			},
			{
				name:     "default ws port 80",
				rawURL:   "ws://127.0.0.1/devtools/page/abc",
				wantAddr: "127.0.0.1:80",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				u, dialAddr, err := parseCDPLoopbackWebSocketURL(tc.rawURL)
				if err != nil {
					t.Fatalf("parseCDPLoopbackWebSocketURL(%q) returned unexpected error: %v", tc.rawURL, err)
				}
				if u == nil {
					t.Fatal("expected non-nil parsed URL")
				}
				if dialAddr != tc.wantAddr {
					t.Errorf("dialAddr = %q, want %q", dialAddr, tc.wantAddr)
				}
			})
		}
	})

	t.Run("positive dialCDPWebSocket sends normalized Host header on handshake", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen on loopback: %v", err)
		}
		defer ln.Close()

		port := ln.Addr().(*net.TCPAddr).Port
		wantHost := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

		type handshakeResult struct {
			host    string
			uri     string
			upgrade string
			err     error
		}
		resCh := make(chan handshakeResult, 1)
		doneCh := make(chan struct{})
		defer close(doneCh)

		go func() {
			conn, err := ln.Accept()
			if err != nil {
				resCh <- handshakeResult{err: err}
				return
			}
			defer conn.Close()

			req, err := http.ReadRequest(bufio.NewReader(conn))
			if err != nil {
				resCh <- handshakeResult{err: err}
				return
			}
			resCh <- handshakeResult{
				host:    req.Host,
				uri:     req.RequestURI,
				upgrade: req.Header.Get("Upgrade"),
			}
			_, _ = io.WriteString(conn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
			<-doneCh
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ws, err := dialCDPWebSocket(ctx, "ws://localhost:"+strconv.Itoa(port)+"/devtools/page/abc")
		if err != nil {
			t.Fatalf("dialCDPWebSocket failed: %v", err)
		}
		defer ws.Close()

		got := <-resCh
		if got.err != nil {
			t.Fatalf("mock WS server failed: %v", got.err)
		}
		if got.host != wantHost {
			t.Errorf("handshake Host = %q, want %q", got.host, wantHost)
		}
		if got.uri != "/devtools/page/abc" {
			t.Errorf("handshake RequestURI = %q, want %q", got.uri, "/devtools/page/abc")
		}
		if !strings.EqualFold(got.upgrade, "websocket") {
			t.Errorf("handshake Upgrade = %q, want websocket", got.upgrade)
		}

		// Verify explicit :80 default port Host header via in-memory pipe without binding port 80.
		for _, defaultURL := range []string{"ws://127.0.0.1/devtools/page/default", "ws://localhost/devtools/page/default"} {
			clientConn, serverConn := net.Pipe()
			var dialedNetwork, dialedAddr string
			pipeResCh := make(chan handshakeResult, 1)
			pipeDone := make(chan struct{})

			go func() {
				defer serverConn.Close()
				req, err := http.ReadRequest(bufio.NewReader(serverConn))
				if err != nil {
					pipeResCh <- handshakeResult{err: err}
					return
				}
				pipeResCh <- handshakeResult{host: req.Host, uri: req.RequestURI}
				_, _ = io.WriteString(serverConn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
				<-pipeDone
			}()

			wsPipe, err := dialCDPWebSocketWithDialer(ctx, defaultURL, func(_ context.Context, network, addr string) (net.Conn, error) {
				dialedNetwork = network
				dialedAddr = addr
				return clientConn, nil
			})
			if err != nil {
				close(pipeDone)
				t.Fatalf("dialCDPWebSocketWithDialer(%q) failed: %v", defaultURL, err)
			}

			pipeGot := <-pipeResCh
			_ = wsPipe.Close()
			close(pipeDone)
			if pipeGot.err != nil {
				t.Fatalf("pipe WS server failed for %q: %v", defaultURL, pipeGot.err)
			}
			if dialedNetwork != "tcp" || dialedAddr != "127.0.0.1:80" {
				t.Errorf("dialed (%q, %q), want (\"tcp\", \"127.0.0.1:80\")", dialedNetwork, dialedAddr)
			}
			if pipeGot.host != "127.0.0.1:80" {
				t.Errorf("default port handshake Host for %q = %q, want %q", defaultURL, pipeGot.host, "127.0.0.1:80")
			}
		}
	})

	t.Run("negative non-loopback hosts, invalid schemes, userinfo, and bad ports fail closed", func(t *testing.T) {
		dummyToken := "ghp_" + strings.Repeat("d", 36)
		tests := []struct {
			name        string
			rawURL      string
			wantErrPart string
		}{
			{
				name:        "empty URL",
				rawURL:      "   ",
				wantErrPart: "empty CDP websocket URL",
			},
			{
				name:        "wss scheme rejected",
				rawURL:      "wss://127.0.0.1:9222/devtools/page/1",
				wantErrPart: "refusing non-ws CDP websocket scheme",
			},
			{
				name:        "http scheme rejected",
				rawURL:      "http://127.0.0.1:9222/json/list",
				wantErrPart: "refusing non-ws CDP websocket scheme",
			},
			{
				name:        "userinfo rejected",
				rawURL:      "ws://admin:secret@127.0.0.1:9222/devtools/page/1",
				wantErrPart: "userinfo",
			},
			{
				name:        "external domain rejected",
				rawURL:      "ws://attacker.example:9222/devtools/page/1",
				wantErrPart: "refusing non-loopback CDP websocket host",
			},
			{
				name:        "LAN IPv4 rejected",
				rawURL:      "ws://192.168.1.50:9222/devtools/page/1",
				wantErrPart: "refusing non-loopback CDP websocket host",
			},
			{
				name:        "wildcard 0.0.0.0 rejected",
				rawURL:      "ws://0.0.0.0:9222/devtools/page/1",
				wantErrPart: "refusing non-loopback CDP websocket host",
			},
			{
				name:        "non-127.0.0.1 loopback-block address rejected",
				rawURL:      "ws://127.0.0.2:9222/devtools/page/1",
				wantErrPart: "refusing non-loopback CDP websocket host",
			},
			{
				name:        "non-::1 IPv6 rejected",
				rawURL:      "ws://[::2]:9222/devtools/page/1",
				wantErrPart: "refusing non-loopback CDP websocket host",
			},
			{
				name:        "port zero rejected",
				rawURL:      "ws://127.0.0.1:0/devtools/page/1",
				wantErrPart: "invalid CDP websocket port",
			},
			{
				name:        "port > 65535 rejected",
				rawURL:      "ws://127.0.0.1:65536/devtools/page/1",
				wantErrPart: "invalid CDP websocket port",
			},
			{
				name:        "invalid URL syntax redacts secrets",
				rawURL:      "ws://127.0.0.1:9222/%\x7f" + dummyToken,
				wantErrPart: "invalid websocket URL",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := parseCDPLoopbackWebSocketURL(tc.rawURL)
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.rawURL)
				}
				if !strings.Contains(err.Error(), tc.wantErrPart) {
					t.Errorf("error %q missing expected substring %q", err.Error(), tc.wantErrPart)
				}
				if strings.Contains(err.Error(), dummyToken) {
					t.Errorf("error leaked raw token: %s", err.Error())
				}

				ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
				defer cancel()
				_, dialErr := dialCDPWebSocket(ctx, tc.rawURL)
				if dialErr == nil {
					t.Fatalf("expected dialCDPWebSocket to reject %q", tc.rawURL)
				}
				if !strings.Contains(dialErr.Error(), tc.wantErrPart) {
					t.Errorf("dialCDPWebSocket error %q missing expected substring %q", dialErr.Error(), tc.wantErrPart)
				}
				if strings.Contains(dialErr.Error(), dummyToken) {
					t.Errorf("dialCDPWebSocket error leaked raw token: %s", dialErr.Error())
				}
			})
		}
	})
}

func TestCDPError_RedactionAndTransientCheck(t *testing.T) {
	t.Run("positive transient context errors preserve match while redacting Error output", func(t *testing.T) {
		dummyToken := "ghp_" + strings.Repeat("e", 36)
		dummyWebhook := "https://" + "discord.com/api/webhooks/111222333/secret-token-value"
		msg := `Execution context was destroyed near C:\Users\alice\AppData\Local\Temp\r.html with ` + dummyToken + ` and ` + dummyWebhook
		err := &cdpError{
			Code:    -32000,
			Message: msg,
		}

		if !isTransientCDPContextErr(err) {
			t.Fatal("expected isTransientCDPContextErr to match raw transient -32000 message")
		}

		formatted := err.Error()
		if strings.Contains(formatted, dummyToken) {
			t.Errorf("cdpError.Error() leaked GitHub token: %s", formatted)
		}
		if strings.Contains(formatted, "111222333/secret-token-value") {
			t.Errorf("cdpError.Error() leaked Discord webhook: %s", formatted)
		}
		if strings.Contains(formatted, `C:\Users\alice`) {
			t.Errorf("cdpError.Error() leaked user profile path: %s", formatted)
		}
		if !strings.Contains(formatted, "[REDACTED]") {
			t.Errorf("expected [REDACTED] in formatted cdpError, got: %s", formatted)
		}
		if !strings.Contains(formatted, "CDP error -32000: Execution context was destroyed") {
			t.Errorf("expected preserved error prefix and code, got: %s", formatted)
		}

		cannotFindErr := &cdpError{Code: -32000, Message: "Cannot find context with specified id"}
		if !isTransientCDPContextErr(cannotFindErr) {
			t.Error("expected isTransientCDPContextErr to match 'Cannot find context'")
		}
	})

	t.Run("negative nil and non-transient errors", func(t *testing.T) {
		if isTransientCDPContextErr(nil) {
			t.Error("expected nil error not to match isTransientCDPContextErr")
		}

		wrongCode := &cdpError{Code: -32602, Message: "Execution context was destroyed."}
		if isTransientCDPContextErr(wrongCode) {
			t.Error("expected non--32000 code not to match isTransientCDPContextErr")
		}

		wrongMsg := &cdpError{Code: -32000, Message: "Target closed"}
		if isTransientCDPContextErr(wrongMsg) {
			t.Error("expected unrelated -32000 message not to match isTransientCDPContextErr")
		}

		if isTransientCDPContextErr(errors.New("plain error")) {
			t.Error("expected non-cdpError not to match isTransientCDPContextErr")
		}
	})
}

func TestShouldDisableChromiumSandbox_NonLinuxWarning(t *testing.T) {
	t.Run("positive emits diagnostic warning on windows and darwin for sandbox env overrides", func(t *testing.T) {
		for _, goos := range []string{"windows", "darwin"} {
			t.Run(goos, func(t *testing.T) {
				t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "1")
				t.Setenv("CI", "true")
				t.Setenv("GITHUB_ACTIONS", "")

				var warnBuf bytes.Buffer
				if !shouldDisableChromiumSandboxWithWriter(goos, 1000, false, &warnBuf) {
					t.Fatalf("expected sandbox disabled on %s when overrides are truthy", goos)
				}

				warnText := warnBuf.String()
				if !strings.Contains(warnText, "resumai warning: disabling Chromium renderer sandbox on non-Linux host ("+goos+") due to RESUMAI_CHROMIUM_NO_SANDBOX, CI") {
					t.Errorf("unexpected warning text on %s: %q", goos, warnText)
				}

				warnBuf.Reset()
				args := chromiumLaunchArgsWithWriter("/tmp/resumai-profile", goos, 1000, false, &warnBuf)
				joined := strings.Join(args, " ")
				for _, flag := range []string{"--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage"} {
					if !strings.Contains(joined, flag) {
						t.Errorf("expected %s args to include %q, got %v", goos, flag, args)
					}
				}
				if strings.Contains(joined, "--no-zygote") {
					t.Errorf("expected non-Linux %s args to omit --no-zygote, got %v", goos, args)
				}
				if warnBuf.Len() == 0 {
					t.Errorf("expected chromiumLaunchArgsWithWriter to emit warning on %s", goos)
				}
			})
		}
	})

	t.Run("negative suppresses warning on linux, clean desktop hosts, container/root fallbacks, and nil writer", func(t *testing.T) {
		t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "1")
		t.Setenv("CI", "true")
		t.Setenv("GITHUB_ACTIONS", "true")

		var linuxBuf bytes.Buffer
		if !shouldDisableChromiumSandboxWithWriter("linux", 1000, false, &linuxBuf) {
			t.Fatal("expected sandbox disabled on linux when env overrides are set")
		}
		if linuxBuf.Len() != 0 {
			t.Errorf("expected zero non-Linux warning output on linux, got %q", linuxBuf.String())
		}

		// Nil writer must never panic on non-Linux hosts.
		if !shouldDisableChromiumSandboxWithWriter("windows", 1000, false, nil) {
			t.Fatal("expected true with nil writer when override is active")
		}

		// Clear env overrides: falsy env vars or root/container fallbacks emit zero off-Linux env warning.
		t.Setenv("RESUMAI_CHROMIUM_NO_SANDBOX", "0")
		t.Setenv("CI", "false")
		t.Setenv("GITHUB_ACTIONS", "no")

		var desktopBuf bytes.Buffer
		if shouldDisableChromiumSandboxWithWriter("windows", 1000, false, &desktopBuf) {
			t.Fatal("expected clean Windows desktop with falsy env vars to keep sandbox enabled")
		}
		if desktopBuf.Len() != 0 {
			t.Errorf("expected no warning when sandbox stays enabled, got %q", desktopBuf.String())
		}

		args := chromiumLaunchArgsWithWriter("/tmp/resumai-profile", "darwin", 1000, false, &desktopBuf)
		if strings.Contains(strings.Join(args, " "), "--no-sandbox") {
			t.Errorf("expected clean darwin args to keep sandbox enabled, got %v", args)
		}
		if desktopBuf.Len() != 0 {
			t.Errorf("expected zero warning for clean darwin host, got %q", desktopBuf.String())
		}

		var rootBuf bytes.Buffer
		if !shouldDisableChromiumSandboxWithWriter("darwin", 0, true, &rootBuf) {
			t.Fatal("expected root/docker fallback to disable sandbox")
		}
		if rootBuf.Len() != 0 {
			t.Errorf("expected no env-override warning when only root/docker triggered disablement, got %q", rootBuf.String())
		}
	})
}
