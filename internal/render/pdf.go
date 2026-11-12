package render

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kidthales/resumai/internal/domain"
)

var (
	// ErrChromiumNotFound is returned when neither Chrome nor Edge can be located on the host.
	ErrChromiumNotFound = errors.New("headless chromium binary (Chrome or Edge) not found on host")

	// ErrCDPTimeout is returned when a CDP operation times out.
	ErrCDPTimeout = errors.New("chrome devtools protocol operation timed out")
)

// LayoutTelemetry stores optical layout, pagination, and typography measurements
// extracted directly from the headless Chromium DOM via Range.getClientRects().
type LayoutTelemetry struct {
	PageCount          int               `json:"page_count"`
	LastPageFillRatio  float64           `json:"last_page_fill_ratio"`
	OverflowPX         float64           `json:"overflow_px"`
	ContentHeightPX    float64           `json:"content_height_px"`
	PageHeightPX       float64           `json:"page_height_px"`
	UsablePageHeightPX float64           `json:"usable_page_height_px"`
	Bullets            []BulletTelemetry `json:"bullets,omitempty"`
	OrphanBullets      []OrphanBullet    `json:"orphan_bullets,omitempty"`
}

// BulletTelemetry holds physical rendering measurements for an individual accomplishment bullet.
type BulletTelemetry struct {
	ID            string  `json:"id"`
	Text          string  `json:"text"`
	LineCount     int     `json:"line_count"`
	LastLineWidth float64 `json:"last_line_width"`
	LineWidth     float64 `json:"line_width"`
	LastLineRatio float64 `json:"last_line_ratio"`
	StartTop      float64 `json:"start_top"`
	EndBottom     float64 `json:"end_bottom"`
	StartPage     int     `json:"start_page"`
	EndPage       int     `json:"end_page"`
	IsOrphan      bool    `json:"is_orphan"`
	OrphanReason  string  `json:"orphan_reason,omitempty"`
}

// OrphanBullet identifies a bullet point suffering from typographic runts or split page boundaries.
type OrphanBullet struct {
	ID            string  `json:"id"`
	Text          string  `json:"text"`
	LineCount     int     `json:"line_count"`
	LastLineWidth float64 `json:"last_line_width"`
	LastLineRatio float64 `json:"last_line_ratio"`
	Reason        string  `json:"reason"`
}

// PDFPrinter manages headless host Chromium processes (Chrome/Edge) to render
// pixel-perfect PDFs via Page.printToPDF and extract DOM layout telemetry.
type PDFPrinter struct {
	chromiumPath string
	htmlRenderer *HTMLRenderer
	theme        *ThemeManifest
	targetPages  int
	timeout      time.Duration
}

// PDFPrinterOption configures a PDFPrinter instance.
type PDFPrinterOption func(*PDFPrinter)

// WithChromiumPath sets an explicit path to the Chrome/Edge binary.
func WithChromiumPath(path string) PDFPrinterOption {
	return func(p *PDFPrinter) {
		p.chromiumPath = path
	}
}

// WithHTMLRenderer configures an existing HTMLRenderer instance.
func WithHTMLRenderer(r *HTMLRenderer) PDFPrinterOption {
	return func(p *PDFPrinter) {
		p.htmlRenderer = r
	}
}

// WithTheme configures a ThemeManifest for the printer's HTMLRenderer.
func WithTheme(t *ThemeManifest) PDFPrinterOption {
	return func(p *PDFPrinter) {
		p.theme = t
	}
}

// WithTargetPages configures the target page budget for overflow calculations.
func WithTargetPages(pages int) PDFPrinterOption {
	return func(p *PDFPrinter) {
		if pages > 0 {
			p.targetPages = pages
		}
	}
}

// WithTimeout configures the execution deadline for headless browser operations.
func WithTimeout(d time.Duration) PDFPrinterOption {
	return func(p *PDFPrinter) {
		if d > 0 {
			p.timeout = d
		}
	}
}

// NewPDFPrinter initializes a headless Chrome/Edge PDF printer and telemetry extractor.
func NewPDFPrinter(opts ...PDFPrinterOption) (*PDFPrinter, error) {
	p := &PDFPrinter{
		targetPages: 1,
		timeout:     30 * time.Second,
	}

	for _, opt := range opts {
		opt(p)
	}

	if p.chromiumPath == "" {
		bin, err := FindChromiumBinary()
		if err != nil {
			return nil, err
		}
		p.chromiumPath = bin
	}

	if p.htmlRenderer == nil {
		hr, err := NewHTMLRenderer(p.theme)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize HTMLRenderer for PDFPrinter: %w", err)
		}
		p.htmlRenderer = hr
	}

	return p, nil
}

// ChromiumPath returns the resolved path to the Chromium executable.
func (p *PDFPrinter) ChromiumPath() string {
	return p.chromiumPath
}

// Print renders a ResumeDocument into PDF bytes and extracts DOM layout telemetry.
// PII is injected strictly into an isolated document clone at render time to enforce the air-gap invariant.
func (p *PDFPrinter) Print(doc *ResumeDocument, pii *domain.PIIHeader) ([]byte, *LayoutTelemetry, error) {
	if doc == nil {
		return nil, nil, errors.New("cannot print nil ResumeDocument")
	}

	targetPages := p.targetPages
	if doc.Metadata.PageBudget > 0 {
		targetPages = doc.Metadata.PageBudget
	}

	htmlBytes, err := p.htmlRenderer.Render(doc, pii)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to render HTML for PDF print: %w", err)
	}

	return p.executeCDP(htmlBytes, targetPages, true)
}

// Render implements the standard rendering interface, returning PDF bytes.
func (p *PDFPrinter) Render(doc *ResumeDocument, pii *domain.PIIHeader) ([]byte, error) {
	pdf, _, err := p.Print(doc, pii)
	return pdf, err
}

// PrintHTML renders arbitrary HTML bytes to PDF and extracts DOM layout telemetry.
func (p *PDFPrinter) PrintHTML(htmlBytes []byte) ([]byte, *LayoutTelemetry, error) {
	return p.executeCDP(htmlBytes, p.targetPages, true)
}

// ExtractTelemetry extracts DOM layout metrics from HTML bytes without generating PDF bytes.
func (p *PDFPrinter) ExtractTelemetry(htmlBytes []byte) (*LayoutTelemetry, error) {
	_, telemetry, err := p.executeCDP(htmlBytes, p.targetPages, false)
	return telemetry, err
}

// ExtractDocumentTelemetry renders the ResumeDocument to HTML and measures DOM layout metrics.
func (p *PDFPrinter) ExtractDocumentTelemetry(doc *ResumeDocument, pii *domain.PIIHeader) (*LayoutTelemetry, error) {
	if doc == nil {
		return nil, errors.New("cannot extract telemetry from nil ResumeDocument")
	}

	targetPages := p.targetPages
	if doc.Metadata.PageBudget > 0 {
		targetPages = doc.Metadata.PageBudget
	}

	htmlBytes, err := p.htmlRenderer.Render(doc, pii)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML for telemetry: %w", err)
	}

	_, telemetry, err := p.executeCDP(htmlBytes, targetPages, false)
	return telemetry, err
}

// executeCDP launches headless Chromium, navigates to the rendered HTML, evaluates DOM telemetry,
// and conditionally generates PDF bytes via Page.printToPDF.
func (p *PDFPrinter) executeCDP(htmlBytes []byte, targetPages int, generatePDF bool) ([]byte, *LayoutTelemetry, error) {
	if len(htmlBytes) == 0 {
		return nil, nil, errors.New("cannot execute CDP on empty HTML payload")
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	// 1. Write HTML to temporary file
	tmpHTML, err := os.CreateTemp("", "resumai-render-*.html")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary HTML file: %w", err)
	}
	tmpHTMLPath := tmpHTML.Name()
	defer os.Remove(tmpHTMLPath)

	if _, err := tmpHTML.Write(htmlBytes); err != nil {
		tmpHTML.Close()
		return nil, nil, fmt.Errorf("failed to write temporary HTML content: %w", err)
	}
	if err := tmpHTML.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to close temporary HTML file: %w", err)
	}

	// 2. Create isolated temporary Chromium user data directory
	tmpUserDataDir, err := os.MkdirTemp("", "resumai-chromium-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create chromium user data dir: %w", err)
	}
	defer os.RemoveAll(tmpUserDataDir)

	// 3. Launch host Chromium with modern headless mode
	args := []string{
		"--headless=new",
		"--remote-debugging-port=0",
		"--disable-gpu",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-extensions",
		"--disable-sync",
		"--disable-translate",
		"--metrics-recording-only",
		"--mute-audio",
		"--user-data-dir=" + tmpUserDataDir,
		"about:blank",
	}

	cmd := exec.CommandContext(ctx, p.chromiumPath, args...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to attach chromium stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start headless chromium: %w", err)
	}

	// Ensure process termination on exit
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	// 4. Read DevTools listening port/URL from stderr
	port, err := extractDevToolsPort(stderrPipe, 10*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse devtools port from chromium: %w", err)
	}

	// 5. Query /json/list to find the target page WebSocket URL
	pageWSURL, err := getPageWebSocketURL(ctx, port)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to obtain page websocket debugger url: %w", err)
	}

	// 6. Connect minimal RFC 6455 WebSocket client
	ws, err := dialCDPWebSocket(ctx, pageWSURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect CDP websocket: %w", err)
	}
	defer ws.Close()

	// 7. Initialize CDP domains
	if _, err := ws.Call(ctx, "Page.enable", nil); err != nil {
		return nil, nil, fmt.Errorf("CDP Page.enable failed: %w", err)
	}
	if _, err := ws.Call(ctx, "Runtime.enable", nil); err != nil {
		return nil, nil, fmt.Errorf("CDP Runtime.enable failed: %w", err)
	}

	// 8. Navigate to the temporary HTML file via file:/// URI
	fileURI := "file:///" + filepath.ToSlash(tmpHTMLPath)
	navParams := map[string]interface{}{
		"url": fileURI,
	}
	if _, err := ws.Call(ctx, "Page.navigate", navParams); err != nil {
		return nil, nil, fmt.Errorf("CDP Page.navigate failed: %w", err)
	}

	// 9. Await DOM load and embedded offline fonts readiness
	fontAwaitExpr := `new Promise((resolve) => {
		function check() {
			if (document.readyState === 'complete') {
				if (document.fonts && document.fonts.ready) {
					document.fonts.ready.then(() => resolve(true)).catch(() => resolve(true));
				} else {
					resolve(true);
				}
			} else {
				setTimeout(check, 10);
			}
		}
		check();
	})`
	evalFontParams := map[string]interface{}{
		"expression":    fontAwaitExpr,
		"awaitPromise":  true,
		"returnByValue": true,
	}
	if _, err := ws.Call(ctx, "Runtime.evaluate", evalFontParams); err != nil {
		return nil, nil, fmt.Errorf("CDP font wait evaluation failed: %w", err)
	}

	// 10. Extract DOM Layout Telemetry using Range.getClientRects()
	var pageCfg PageConfig
	if p.htmlRenderer != nil && p.htmlRenderer.Theme() != nil {
		pageCfg = p.htmlRenderer.Theme().Page
	} else if p.theme != nil {
		pageCfg = p.theme.Page
	}
	telemetryScript := buildTelemetryScript(targetPages, pageCfg)
	evalParams := map[string]interface{}{
		"expression":    telemetryScript,
		"awaitPromise":  true,
		"returnByValue": true,
	}

	telemetryRaw, err := ws.Call(ctx, "Runtime.evaluate", evalParams)
	if err != nil {
		return nil, nil, fmt.Errorf("CDP layout telemetry evaluation failed: %w", err)
	}

	var evalResp struct {
		Result struct {
			Value LayoutTelemetry `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(telemetryRaw, &evalResp); err != nil {
		return nil, nil, fmt.Errorf("failed to parse layout telemetry JSON: %w", err)
	}
	telemetry := evalResp.Result.Value

	// 11. Optionally generate PDF via Page.printToPDF
	var pdfBytes []byte
	if generatePDF {
		paperWidth, paperHeight, marginTop, marginBottom, marginLeft, marginRight := pageGeometryInches(pageCfg)
		printParams := map[string]interface{}{
			"printBackground":   true,
			"preferCSSPageSize": true,
			"paperWidth":        paperWidth,
			"paperHeight":       paperHeight,
			"marginTop":         marginTop,
			"marginBottom":      marginBottom,
			"marginLeft":        marginLeft,
			"marginRight":       marginRight,
			"generateTaggedPDF": true,
		}

		pdfRaw, err := ws.Call(ctx, "Page.printToPDF", printParams)
		if err != nil {
			return nil, nil, fmt.Errorf("CDP Page.printToPDF failed: %w", err)
		}

		var pdfResp struct {
			Data string `json:"data"`
		}
		if err := json.Unmarshal(pdfRaw, &pdfResp); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal printToPDF response: %w", err)
		}

		decoded, err := base64.StdEncoding.DecodeString(pdfResp.Data)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode base64 PDF bytes: %w", err)
		}
		pdfBytes = decoded
	}

	return pdfBytes, &telemetry, nil
}

// parseDimensionToPX converts CSS dimension strings to pixels at 96 DPI.
func parseDimensionToPX(dim string, defaultPX float64) float64 {
	dim = strings.TrimSpace(strings.ToLower(dim))
	if dim == "" {
		return defaultPX
	}
	if dim == "0" {
		return 0.0
	}
	if strings.HasSuffix(dim, "in") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "in"), 64); err == nil {
			return v * 96.0
		}
	}
	if strings.HasSuffix(dim, "pt") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "pt"), 64); err == nil {
			return v * 96.0 / 72.0
		}
	}
	if strings.HasSuffix(dim, "mm") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "mm"), 64); err == nil {
			return v * 96.0 / 25.4
		}
	}
	if strings.HasSuffix(dim, "cm") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "cm"), 64); err == nil {
			return v * 96.0 / 2.54
		}
	}
	if strings.HasSuffix(dim, "px") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "px"), 64); err == nil {
			return v
		}
	}
	return defaultPX
}

// parseDimensionToInches converts CSS dimension strings to inches.
func parseDimensionToInches(dim string, defaultInches float64) float64 {
	dim = strings.TrimSpace(strings.ToLower(dim))
	if dim == "" {
		return defaultInches
	}
	if dim == "0" {
		return 0.0
	}
	if strings.HasSuffix(dim, "in") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "in"), 64); err == nil {
			return v
		}
	}
	if strings.HasSuffix(dim, "pt") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "pt"), 64); err == nil {
			return v / 72.0
		}
	}
	if strings.HasSuffix(dim, "mm") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "mm"), 64); err == nil {
			return v / 25.4
		}
	}
	if strings.HasSuffix(dim, "cm") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "cm"), 64); err == nil {
			return v / 2.54
		}
	}
	if strings.HasSuffix(dim, "px") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(dim, "px"), 64); err == nil {
			return v / 96.0
		}
	}
	return defaultInches
}

// pageGeometryPX computes physical page height and vertical margins in pixels at 96 DPI.
func pageGeometryPX(page PageConfig) (pageHeight, marginY float64) {
	pageHeight = 1056.0 // US Letter default at 96 DPI: 11in = 1056px
	if strings.EqualFold(strings.TrimSpace(page.Size), "a4") {
		pageHeight = 297.0 * 96.0 / 25.4 // A4 height: 297mm ≈ 1122.52px
	}

	topPX := parseDimensionToPX(page.MarginTop, 48.0)    // default 0.5in = 48px
	botPX := parseDimensionToPX(page.MarginBottom, 48.0) // default 0.5in = 48px
	marginY = topPX + botPX
	return pageHeight, marginY
}

// pageGeometryInches computes physical paper dimensions and margins in inches for CDP printToPDF.
func pageGeometryInches(page PageConfig) (width, height, top, bottom, left, right float64) {
	width = 8.5   // default US Letter width in inches
	height = 11.0 // default US Letter height in inches
	if strings.EqualFold(strings.TrimSpace(page.Size), "a4") {
		width = 210.0 / 25.4  // A4 width: 210mm ≈ 8.27in
		height = 297.0 / 25.4 // A4 height: 297mm ≈ 11.69in
	}

	top = parseDimensionToInches(page.MarginTop, 0.5)
	bottom = parseDimensionToInches(page.MarginBottom, 0.5)
	left = parseDimensionToInches(page.MarginLeft, 0.5)
	right = parseDimensionToInches(page.MarginRight, 0.5)
	return width, height, top, bottom, left, right
}

// buildTelemetryScript compiles the in-browser JavaScript evaluation payload.
func buildTelemetryScript(targetPages int, page PageConfig) string {
	if targetPages <= 0 {
		targetPages = 1
	}

	pageHeight, marginY := pageGeometryPX(page)

	return fmt.Sprintf(`(() => {
		const resumePage = document.querySelector('.resume-page') || document.body;
		const rect = resumePage.getBoundingClientRect();
		const contentHeight = Math.max(resumePage.scrollHeight || 0, rect.height || 0);

		const pageHeight = %.2f;
		const marginY = %.2f;
		const usableHeight = pageHeight - marginY;

		const targetPages = %d;
		const allowedHeight = targetPages * usableHeight;
		const overflowPX = Math.max(0, contentHeight - allowedHeight);

		const pageCount = Math.max(1, Math.ceil(contentHeight / usableHeight));

		let lastPageFillRatio = 0.0;
		if (contentHeight > 0) {
			const remainder = contentHeight %% usableHeight;
			const lastPageContent = (remainder === 0 && contentHeight >= usableHeight) ? usableHeight : remainder;
			lastPageFillRatio = lastPageContent / usableHeight;
		}

		const bullets = [];
		const orphanBullets = [];
		const bulletEls = document.querySelectorAll('.resume-bullet');

		bulletEls.forEach((el) => {
			const id = el.getAttribute('data-bullet-id') || '';
			const text = (el.textContent || '').trim();

			let lineCount = 1;
			let lastLineWidth = 0.0;
			let maxLineWidth = 0.0;
			let startTop = 0.0;
			let endBottom = 0.0;

			// Range.getClientRects() accurately measures individual rendered text lines
			const range = document.createRange();
			range.selectNodeContents(el);
			const rects = range.getClientRects();

			if (rects && rects.length > 0) {
				lineCount = rects.length;
				startTop = rects[0].top;
				endBottom = rects[rects.length - 1].bottom;
				for (let i = 0; i < rects.length; i++) {
					if (rects[i].width > maxLineWidth) {
						maxLineWidth = rects[i].width;
					}
				}
				lastLineWidth = rects[rects.length - 1].width;
			} else {
				const bRect = el.getBoundingClientRect();
				startTop = bRect.top;
				endBottom = bRect.bottom;
				lastLineWidth = bRect.width;
				maxLineWidth = bRect.width;
			}

			const lastLineRatio = maxLineWidth > 0 ? (lastLineWidth / maxLineWidth) : 1.0;
			const startPage = Math.floor(Math.max(0, startTop) / usableHeight) + 1;
			const endPage = Math.floor(Math.max(0, endBottom) / usableHeight) + 1;

			let isOrphan = false;
			let reason = '';

			if (endPage > startPage) {
				isOrphan = true;
				reason = 'split_page_boundary';
			} else if (lineCount > 1 && (lastLineRatio < 0.20 || lastLineWidth < 75.0)) {
				isOrphan = true;
				reason = 'typographic_runt';
			}

			const bTelemetry = {
				id: id,
				text: text,
				line_count: lineCount,
				last_line_width: Math.round(lastLineWidth * 100) / 100,
				line_width: Math.round(maxLineWidth * 100) / 100,
				last_line_ratio: Math.round(lastLineRatio * 1000) / 1000,
				start_top: Math.round(startTop * 100) / 100,
				end_bottom: Math.round(endBottom * 100) / 100,
				start_page: startPage,
				end_page: endPage,
				is_orphan: isOrphan,
				orphan_reason: reason
			};
			bullets.push(bTelemetry);

			if (isOrphan) {
				orphanBullets.push({
					id: id,
					text: text,
					line_count: lineCount,
					last_line_width: bTelemetry.last_line_width,
					last_line_ratio: bTelemetry.last_line_ratio,
					reason: reason
				});
			}
		});

		return {
			page_count: pageCount,
			last_page_fill_ratio: Math.round(lastPageFillRatio * 1000) / 1000,
			overflow_px: Math.round(overflowPX * 100) / 100,
			content_height_px: Math.round(contentHeight * 100) / 100,
			page_height_px: pageHeight,
			usable_page_height_px: usableHeight,
			bullets: bullets,
			orphan_bullets: orphanBullets
		};
	})()`, pageHeight, marginY, targetPages)
}

// CalculateLayoutTelemetry provides a pure calculation model for layout metrics
// from raw geometric inputs, enabling headless/mock validation without browser execution.
func CalculateLayoutTelemetry(contentHeight, usableHeight float64, targetPages int, bullets []BulletTelemetry) LayoutTelemetry {
	if usableHeight <= 0 {
		usableHeight = 960.0
	}
	if targetPages <= 0 {
		targetPages = 1
	}

	pageHeight := usableHeight + 96.0
	allowedHeight := float64(targetPages) * usableHeight
	overflow := 0.0
	if contentHeight > allowedHeight {
		overflow = contentHeight - allowedHeight
	}

	pageCount := 1
	if contentHeight > 0 {
		pageCount = int((contentHeight + usableHeight - 1e-6) / usableHeight)
		if pageCount < 1 {
			pageCount = 1
		}
	}

	lastPageFill := 0.0
	if contentHeight > 0 {
		rem := float64(int(contentHeight*100)%int(usableHeight*100)) / 100.0
		if rem == 0 && contentHeight >= usableHeight {
			lastPageFill = 1.0
		} else {
			lastPageFill = rem / usableHeight
		}
	}

	var orphanBullets []OrphanBullet
	for _, b := range bullets {
		if b.IsOrphan {
			orphanBullets = append(orphanBullets, OrphanBullet{
				ID:            b.ID,
				Text:          b.Text,
				LineCount:     b.LineCount,
				LastLineWidth: b.LastLineWidth,
				LastLineRatio: b.LastLineRatio,
				Reason:        b.OrphanReason,
			})
		}
	}

	return LayoutTelemetry{
		PageCount:          pageCount,
		LastPageFillRatio:  float64(int(lastPageFill*1000)) / 1000.0,
		OverflowPX:         float64(int(overflow*100)) / 100.0,
		ContentHeightPX:    contentHeight,
		PageHeightPX:       pageHeight,
		UsablePageHeightPX: usableHeight,
		Bullets:            bullets,
		OrphanBullets:      orphanBullets,
	}
}

// FindChromiumBinary discovers Google Chrome or Microsoft Edge on the host operating system.
func FindChromiumBinary() (string, error) {
	// 1. Explicit environment variable overrides
	if env := os.Getenv("RESUMAI_CHROME_BIN"); env != "" {
		if fileExists(env) {
			return env, nil
		}
	}
	if env := os.Getenv("CHROME_BIN"); env != "" {
		if fileExists(env) {
			return env, nil
		}
	}

	// 2. Platform-specific known candidate paths
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LocalAppData")
		candidates := []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			filepath.Join(localAppData, `Google\Chrome\Application\chrome.exe`),
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			filepath.Join(localAppData, `Microsoft\Edge\Application\msedge.exe`),
		}
		for _, c := range candidates {
			if fileExists(c) {
				return c, nil
			}
		}

	case "darwin":
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, c := range candidates {
			if fileExists(c) {
				return c, nil
			}
		}

	case "linux":
		names := []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
			"microsoft-edge-stable",
			"msedge",
		}
		for _, name := range names {
			if path, err := exec.LookPath(name); err == nil {
				return path, nil
			}
		}
	}

	// 3. General PATH lookup fallback
	fallbacks := []string{"chrome", "google-chrome", "chromium", "msedge", "edge"}
	for _, f := range fallbacks {
		if path, err := exec.LookPath(f); err == nil {
			return path, nil
		}
	}

	return "", ErrChromiumNotFound
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// extractDevToolsPort parses the ephemeral port from Chromium's initial stderr output.
func extractDevToolsPort(r io.Reader, timeout time.Duration) (int, error) {
	scanner := bufio.NewScanner(r)
	portChan := make(chan int, 1)
	errChan := make(chan error, 1)

	re := regexp.MustCompile(`(?:DevTools listening on ws://(?:127\.0\.0\.1|localhost):|127\.0\.0\.1:)(\d+)`)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				p, err := strconv.Atoi(matches[1])
				if err == nil {
					portChan <- p
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			errChan <- err
		} else {
			errChan <- errors.New("chromium stderr closed without reporting devtools listening address")
		}
	}()

	select {
	case port := <-portChan:
		return port, nil
	case err := <-errChan:
		return 0, err
	case <-time.After(timeout):
		return 0, errors.New("timeout waiting for chromium devtools port")
	}
}

// getPageWebSocketURL queries Chromium's HTTP discovery API to find an active page target.
func getPageWebSocketURL(ctx context.Context, port int) (string, error) {
	endpoint := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from /json/list: %d", resp.StatusCode)
	}

	var targets []struct {
		ID                   string `json:"id"`
		Type                 string `json:"type"`
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", fmt.Errorf("failed to decode /json/list: %w", err)
	}

	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebuggerURL != "" {
			return t.WebSocketDebuggerURL, nil
		}
	}

	// If no page exists yet, create one
	newEndpoint := fmt.Sprintf("http://127.0.0.1:%d/json/new?about:blank", port)
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, newEndpoint, nil)
	if err != nil {
		return "", err
	}
	newResp, err := client.Do(putReq)
	if err != nil {
		return "", err
	}
	defer newResp.Body.Close()

	var newTarget struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(newResp.Body).Decode(&newTarget); err != nil {
		return "", fmt.Errorf("failed to decode /json/new: %w", err)
	}

	if newTarget.WebSocketDebuggerURL == "" {
		return "", errors.New("created target had empty webSocketDebuggerUrl")
	}

	return newTarget.WebSocketDebuggerURL, nil
}

// cdpWebSocket provides a minimal, dependency-free RFC 6455 WebSocket client for CDP.
type cdpWebSocket struct {
	conn    net.Conn
	r       *bufio.Reader
	writeMu sync.Mutex

	nextID  int64
	mu      sync.Mutex
	pending map[int64]chan cdpResponse
	closed  chan struct{}
}

type cdpResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func dialCDPWebSocket(ctx context.Context, wsURL string) (*cdpWebSocket, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("invalid websocket URL %q: %w", wsURL, err)
	}

	host := u.Host
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, "80")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, fmt.Errorf("failed to dial CDP host %q: %w", host, err)
	}

	// Generate 16 random bytes for Sec-WebSocket-Key
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		conn.Close()
		return nil, err
	}
	secKey := base64.StdEncoding.EncodeToString(nonce)

	// Send HTTP WebSocket upgrade request
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\n"+
		"Sec-WebSocket-Version: 13\r\n\r\n", path, u.Host, secKey)

	if _, err := conn.Write([]byte(req)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to write websocket upgrade: %w", err)
	}

	reader := bufio.NewReader(conn)
	respLine, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read websocket handshake response: %w", err)
	}

	if !strings.Contains(respLine, "101") {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade rejected: %s", strings.TrimSpace(respLine))
	}

	// Read remaining HTTP headers until empty line
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("error reading handshake headers: %w", err)
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	ws := &cdpWebSocket{
		conn:    conn,
		r:       reader,
		pending: make(map[int64]chan cdpResponse),
		closed:  make(chan struct{}),
	}

	go ws.readLoop()

	return ws, nil
}

func (ws *cdpWebSocket) Close() error {
	ws.mu.Lock()
	select {
	case <-ws.closed:
		ws.mu.Unlock()
		return nil
	default:
		close(ws.closed)
	}
	ws.mu.Unlock()

	return ws.conn.Close()
}

func (ws *cdpWebSocket) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&ws.nextID, 1)

	respChan := make(chan cdpResponse, 1)
	ws.mu.Lock()
	ws.pending[id] = respChan
	ws.mu.Unlock()

	defer func() {
		ws.mu.Lock()
		delete(ws.pending, id)
		ws.mu.Unlock()
	}()

	cmd := map[string]interface{}{
		"id":     id,
		"method": method,
	}
	if params != nil {
		cmd["params"] = params
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CDP command: %w", err)
	}

	if err := ws.writeTextFrame(data); err != nil {
		return nil, fmt.Errorf("failed to send CDP frame: %w", err)
	}

	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("CDP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ws.closed:
		return nil, errors.New("CDP websocket closed")
	}
}

func (ws *cdpWebSocket) writeTextFrame(payload []byte) error {
	ws.writeMu.Lock()
	defer ws.writeMu.Unlock()

	length := len(payload)
	b0 := byte(0x81) // FIN=1, Opcode=1 (Text)

	var maskKey [4]byte
	if _, err := rand.Read(maskKey[:]); err != nil {
		maskKey = [4]byte{0x23, 0x45, 0x67, 0x89}
	}

	var header []byte
	if length < 126 {
		header = []byte{b0, byte(0x80 | length)}
	} else if length <= 65535 {
		header = make([]byte, 4)
		header[0] = b0
		header[1] = byte(0x80 | 126)
		binary.BigEndian.PutUint16(header[2:4], uint16(length))
	} else {
		header = make([]byte, 10)
		header[0] = b0
		header[1] = byte(0x80 | 127)
		binary.BigEndian.PutUint64(header[2:10], uint64(length))
	}

	maskedPayload := make([]byte, length)
	for i := range payload {
		maskedPayload[i] = payload[i] ^ maskKey[i%4]
	}

	var buf bytes.Buffer
	buf.Grow(len(header) + 4 + length)
	buf.Write(header)
	buf.Write(maskKey[:])
	buf.Write(maskedPayload)

	_, err := ws.conn.Write(buf.Bytes())
	return err
}

func (ws *cdpWebSocket) readLoop() {
	for {
		msg, err := ws.readMessage()
		if err != nil {
			_ = ws.Close()
			return
		}

		var resp cdpResponse
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID != 0 {
			ws.mu.Lock()
			ch, ok := ws.pending[resp.ID]
			ws.mu.Unlock()
			if ok {
				ch <- resp
			}
		}
	}
}

func (ws *cdpWebSocket) readMessage() ([]byte, error) {
	var fullMessage bytes.Buffer

	for {
		header := make([]byte, 2)
		if _, err := io.ReadFull(ws.r, header); err != nil {
			return nil, err
		}

		fin := (header[0] & 0x80) != 0
		opcode := header[0] & 0x0F

		if opcode == 0x08 { // Connection Close
			return nil, io.EOF
		}

		isMasked := (header[1] & 0x80) != 0
		payloadLen := int64(header[1] & 0x7F)

		if payloadLen == 126 {
			ext := make([]byte, 2)
			if _, err := io.ReadFull(ws.r, ext); err != nil {
				return nil, err
			}
			payloadLen = int64(binary.BigEndian.Uint16(ext))
		} else if payloadLen == 127 {
			ext := make([]byte, 8)
			if _, err := io.ReadFull(ws.r, ext); err != nil {
				return nil, err
			}
			payloadLen = int64(binary.BigEndian.Uint64(ext))
		}

		var mask [4]byte
		if isMasked {
			if _, err := io.ReadFull(ws.r, mask[:]); err != nil {
				return nil, err
			}
		}

		if payloadLen > 50*1024*1024 {
			return nil, fmt.Errorf("CDP message payload exceeds 50MB: %d", payloadLen)
		}

		chunk := make([]byte, payloadLen)
		if _, err := io.ReadFull(ws.r, chunk); err != nil {
			return nil, err
		}

		if isMasked {
			for i := range chunk {
				chunk[i] ^= mask[i%4]
			}
		}

		fullMessage.Write(chunk)
		if fin {
			break
		}
	}

	return fullMessage.Bytes(), nil
}
