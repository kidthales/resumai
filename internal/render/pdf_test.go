package render_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/render"
)

func TestFindChromiumBinary(t *testing.T) {
	path, err := render.FindChromiumBinary()
	if err != nil {
		t.Logf("FindChromiumBinary returned: %v (expected if no chromium on host)", err)
		return
	}
	if path == "" {
		t.Errorf("expected non-empty path when error is nil")
	}
}

func TestNewPDFPrinter_Defaults(t *testing.T) {
	bin, err := render.FindChromiumBinary()
	if err != nil {
		t.Skip("skipping NewPDFPrinter test: chromium binary not found on host")
	}

	p, err := render.NewPDFPrinter()
	if err != nil {
		t.Fatalf("failed to create default PDFPrinter: %v", err)
	}

	if p.ChromiumPath() != bin {
		t.Errorf("expected chromium path %q, got %q", bin, p.ChromiumPath())
	}
}

func TestNewPDFPrinter_WithOptions(t *testing.T) {
	tm := render.DefaultThemeManifest()
	hr, err := render.NewHTMLRenderer(tm)
	if err != nil {
		t.Fatalf("failed to create HTMLRenderer: %v", err)
	}

	p, err := render.NewPDFPrinter(
		render.WithChromiumPath("custom-chrome"),
		render.WithHTMLRenderer(hr),
		render.WithTargetPages(2),
		render.WithTimeout(15*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to create PDFPrinter with options: %v", err)
	}

	if p.ChromiumPath() != "custom-chrome" {
		t.Errorf("expected custom chromium path, got %q", p.ChromiumPath())
	}
}

func TestCalculateLayoutTelemetry_PureCalculations(t *testing.T) {
	bullets := []render.BulletTelemetry{
		{
			ID:            "b-1",
			Text:          "Architected real-time streaming pipeline processing 100k events/sec.",
			LineCount:     1,
			LastLineWidth: 650.0,
			LineWidth:     650.0,
			LastLineRatio: 1.0,
			StartTop:      120.0,
			EndBottom:     135.0,
			StartPage:     1,
			EndPage:       1,
			IsOrphan:      false,
		},
		{
			ID:            "b-2",
			Text:          "Dangling runt bullet with one trailing word.",
			LineCount:     2,
			LastLineWidth: 45.0,
			LineWidth:     650.0,
			LastLineRatio: 0.069,
			StartTop:      140.0,
			EndBottom:     170.0,
			StartPage:     1,
			EndPage:       1,
			IsOrphan:      true,
			OrphanReason:  "typographic_runt",
		},
		{
			ID:            "b-3",
			Text:          "Long bullet crossing physical page boundaries.",
			LineCount:     3,
			LastLineWidth: 300.0,
			LineWidth:     650.0,
			LastLineRatio: 0.46,
			StartTop:      950.0,
			EndBottom:     980.0,
			StartPage:     1,
			EndPage:       2,
			IsOrphan:      true,
			OrphanReason:  "split_page_boundary",
		},
	}

	// 1 Page Target: Content fits in 1 page (800px < 960px usable)
	telemetry1 := render.CalculateLayoutTelemetry(800.0, 960.0, 1, bullets)
	if telemetry1.PageCount != 1 {
		t.Errorf("expected PageCount 1, got %d", telemetry1.PageCount)
	}
	if telemetry1.OverflowPX != 0 {
		t.Errorf("expected OverflowPX 0, got %f", telemetry1.OverflowPX)
	}
	if telemetry1.LastPageFillRatio < 0.83 || telemetry1.LastPageFillRatio > 0.84 {
		t.Errorf("expected LastPageFillRatio ~0.833, got %f", telemetry1.LastPageFillRatio)
	}
	if len(telemetry1.OrphanBullets) != 2 {
		t.Fatalf("expected 2 orphan bullets, got %d", len(telemetry1.OrphanBullets))
	}
	if telemetry1.OrphanBullets[0].Reason != "typographic_runt" {
		t.Errorf("expected typographic_runt, got %s", telemetry1.OrphanBullets[0].Reason)
	}
	if telemetry1.OrphanBullets[1].Reason != "split_page_boundary" {
		t.Errorf("expected split_page_boundary, got %s", telemetry1.OrphanBullets[1].Reason)
	}

	// Overflow condition: Content height 1200px vs 1 page target (960px)
	telemetry2 := render.CalculateLayoutTelemetry(1200.0, 960.0, 1, bullets)
	if telemetry2.PageCount != 2 {
		t.Errorf("expected PageCount 2, got %d", telemetry2.PageCount)
	}
	if telemetry2.OverflowPX != 240.0 {
		t.Errorf("expected OverflowPX 240.0, got %f", telemetry2.OverflowPX)
	}
	// Last page content is 240px / 960px = 0.25 (25%)
	if telemetry2.LastPageFillRatio != 0.25 {
		t.Errorf("expected LastPageFillRatio 0.25, got %f", telemetry2.LastPageFillRatio)
	}
}

func TestPDFPrinter_Print_LiveChromium(t *testing.T) {
	_, err := render.FindChromiumBinary()
	if err != nil {
		t.Skip("skipping live chromium print test: chromium binary not found on host")
	}

	printer, err := render.NewPDFPrinter(
		render.WithTimeout(20 * time.Second),
		render.WithTargetPages(1),
	)
	if err != nil {
		t.Fatalf("failed to initialize PDFPrinter: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	pdfBytes, telemetry, err := printer.Print(doc, pii)
	if err != nil {
		t.Fatalf("failed to render PDF with live Chromium: %v", err)
	}

	// 1. Verify valid PDF byte signature (%PDF-)
	if len(pdfBytes) < 100 {
		t.Fatalf("generated PDF too small: %d bytes", len(pdfBytes))
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("output does not begin with %%PDF- magic header: %s", string(pdfBytes[:20]))
	}

	// 2. Verify DOM Layout Telemetry extraction
	if telemetry == nil {
		t.Fatal("expected non-nil LayoutTelemetry from live printer")
	}

	if telemetry.PageCount < 1 {
		t.Errorf("expected page_count >= 1, got %d", telemetry.PageCount)
	}
	if telemetry.LastPageFillRatio <= 0.0 || telemetry.LastPageFillRatio > 1.0 {
		t.Errorf("expected last_page_fill_ratio in (0, 1], got %f", telemetry.LastPageFillRatio)
	}
	if telemetry.ContentHeightPX <= 0.0 {
		t.Errorf("expected positive content_height_px, got %f", telemetry.ContentHeightPX)
	}
	if telemetry.UsablePageHeightPX != 960.0 {
		t.Errorf("expected 960px usable page height, got %f", telemetry.UsablePageHeightPX)
	}

	// 3. Verify Bullet measurements via Range.getClientRects()
	if len(telemetry.Bullets) == 0 {
		t.Errorf("expected extracted bullets in telemetry, got 0")
	}

	foundExpBullet := false
	for _, b := range telemetry.Bullets {
		if b.ID == "b-1" {
			foundExpBullet = true
			if b.LineCount < 1 {
				t.Errorf("expected bullet b-1 to have at least 1 line, got %d", b.LineCount)
			}
			if b.LineWidth <= 0 {
				t.Errorf("expected bullet b-1 to have positive line width, got %f", b.LineWidth)
			}
			if b.StartPage != 1 {
				t.Errorf("expected bullet b-1 start page 1, got %d", b.StartPage)
			}
		}
	}
	if !foundExpBullet {
		t.Errorf("expected bullet b-1 in telemetry bullets list")
	}
}

func TestPDFPrinter_ExtractTelemetry_HTML(t *testing.T) {
	_, err := render.FindChromiumBinary()
	if err != nil {
		t.Skip("skipping live chromium telemetry test: chromium binary not found on host")
	}

	printer, err := render.NewPDFPrinter(render.WithTimeout(20 * time.Second))
	if err != nil {
		t.Fatalf("failed to initialize PDFPrinter: %v", err)
	}

	htmlContent := `<!DOCTYPE html>
<html>
<head>
  <style>
    body { margin: 0; padding: 0; font-family: sans-serif; font-size: 10pt; }
    .resume-page { width: 816px; margin: 0 auto; padding: 48px; }
    ul { list-style-type: disc; margin-left: 20px; }
    li.resume-bullet { margin-bottom: 4px; }
  </style>
</head>
<body>
  <div class="resume-page">
    <h1>Jane Doe</h1>
    <ul>
      <li class="resume-bullet" data-bullet-id="bullet-test-1">
        Engineered distributed event log in Go processing 250,000 transactions per second.
      </li>
      <li class="resume-bullet" data-bullet-id="bullet-test-2">
        A very short bullet.
      </li>
    </ul>
  </div>
</body>
</html>`

	telemetry, err := printer.ExtractTelemetry([]byte(htmlContent))
	if err != nil {
		t.Fatalf("failed to extract telemetry from HTML: %v", err)
	}

	if telemetry.PageCount != 1 {
		t.Errorf("expected 1 page, got %d", telemetry.PageCount)
	}
	if len(telemetry.Bullets) != 2 {
		t.Fatalf("expected 2 bullets, got %d", len(telemetry.Bullets))
	}
	if telemetry.Bullets[0].ID != "bullet-test-1" {
		t.Errorf("expected ID bullet-test-1, got %s", telemetry.Bullets[0].ID)
	}
}

func TestPDFPrinter_Render_PIIAirGapped(t *testing.T) {
	_, err := render.FindChromiumBinary()
	if err != nil {
		t.Skip("skipping live chromium PII test: chromium binary not found on host")
	}

	printer, err := render.NewPDFPrinter(render.WithTimeout(20 * time.Second))
	if err != nil {
		t.Fatalf("failed to initialize PDFPrinter: %v", err)
	}

	doc := sampleResumeDocument()
	// Document starts with placeholder PII
	if doc.Header.Email != "" || doc.Header.Phone != "" {
		t.Fatalf("baseline document must have empty PII to verify air-gapping")
	}

	pii := &domain.PIIHeader{
		FullName: "Air-Gapped Candidate",
		Email:    "confidential@domain.test",
		Phone:    "+1 (555) 019-9999",
	}

	pdfBytes, err := printer.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render PDF: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Fatal("expected non-empty PDF bytes")
	}

	// Verify original AST remained unmutated (air-gap preserved)
	if doc.Header.Email != "" {
		t.Errorf("doc.Header.Email was mutated in place: %q", doc.Header.Email)
	}
	if doc.Header.Phone != "" {
		t.Errorf("doc.Header.Phone was mutated in place: %q", doc.Header.Phone)
	}
}

func TestPDFPrinter_ErrorHandling(t *testing.T) {
	p, err := render.NewPDFPrinter(render.WithChromiumPath("non-existent-binary-path"))
	if err != nil {
		t.Fatalf("NewPDFPrinter with explicit path should not fail constructor: %v", err)
	}

	// 1. Nil document
	_, _, err = p.Print(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "nil ResumeDocument") {
		t.Errorf("expected error printing nil ResumeDocument, got: %v", err)
	}

	// 2. Empty HTML payload
	_, _, err = p.PrintHTML(nil)
	if err == nil || !strings.Contains(err.Error(), "empty HTML") {
		t.Errorf("expected error on empty HTML payload, got: %v", err)
	}

	// 3. Non-existent binary execution failure
	doc := sampleResumeDocument()
	_, _, err = p.Print(doc, nil)
	if err == nil {
		t.Errorf("expected execution error with invalid binary path, got nil")
	}
}
