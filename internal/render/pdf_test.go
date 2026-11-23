package render_test

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"strconv"
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

	// Custom PageConfig (0.6in vertical margins = 115.2px marginY, 1056px total pageHeight)
	customPage := render.PageConfig{
		Size:         "letter",
		MarginTop:    "0.6in",
		MarginBottom: "0.6in",
	}
	telemetry3 := render.CalculateLayoutTelemetry(800.0, 0, 1, bullets, customPage)
	if telemetry3.UsablePageHeightPX != 940.8 {
		t.Errorf("expected UsablePageHeightPX 940.8 for 0.6in margins, got %f", telemetry3.UsablePageHeightPX)
	}
	if telemetry3.PageHeightPX != 1056.0 {
		t.Errorf("expected PageHeightPX 1056.0, got %f", telemetry3.PageHeightPX)
	}

	// A4 PageConfig (297mm height ≈ 1122.52px, 20mm vertical margins ≈ 75.59px top + 75.59px bottom = 151.18px marginY)
	a4Page := render.PageConfig{
		Size:         "a4",
		MarginTop:    "20mm",
		MarginBottom: "20mm",
	}
	telemetryA4 := render.CalculateLayoutTelemetry(800.0, 0, 1, bullets, a4Page)
	expectedA4Height := 297.0 * 96.0 / 25.4
	expectedA4MarginY := 2.0 * (20.0 * 96.0 / 25.4)
	expectedA4Usable := expectedA4Height - expectedA4MarginY
	if math.Abs(telemetryA4.PageHeightPX-expectedA4Height) > 0.01 {
		t.Errorf("expected A4 PageHeightPX ~%f, got %f", expectedA4Height, telemetryA4.PageHeightPX)
	}
	if math.Abs(telemetryA4.UsablePageHeightPX-expectedA4Usable) > 0.01 {
		t.Errorf("expected A4 UsablePageHeightPX ~%f, got %f", expectedA4Usable, telemetryA4.UsablePageHeightPX)
	}
}

func TestPDFPrinter_Print_LiveChromium(t *testing.T) {
	_, err := render.FindChromiumBinary()
	if err != nil {
		t.Skip("skipping live chromium print test: chromium binary not found on host")
	}

	printer, err := render.NewPDFPrinter(
		render.WithTimeout(30*time.Second),
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

	printer, err := render.NewPDFPrinter(render.WithTimeout(30 * time.Second))
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

	printer, err := render.NewPDFPrinter(render.WithTimeout(30 * time.Second))
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

var (
	pdfPagesDictRe = regexp.MustCompile(`(?s)<<[^>]*?/Type\s*/Pages\b[^>]*?/Count\s+(\d+)[^>]*?>>|<<[^>]*?/Count\s+(\d+)[^>]*?/Type\s*/Pages\b[^>]*?>>`)
	pdfPageLeafRe  = regexp.MustCompile(`/Type\s*/Page\b[^sS]`)
)

func countPDFPages(pdfBytes []byte) (int, error) {
	if len(pdfBytes) == 0 {
		return 0, fmt.Errorf("empty PDF bytes")
	}
	if matches := pdfPagesDictRe.FindAllSubmatch(pdfBytes, -1); len(matches) > 0 {
		maxCount := 0
		for _, m := range matches {
			for i := 1; i < len(m); i++ {
				if len(m[i]) == 0 {
					continue
				}
				n, err := strconv.Atoi(string(m[i]))
				if err == nil && n > maxCount {
					maxCount = n
				}
			}
		}
		if maxCount > 0 {
			return maxCount, nil
		}
	}
	if leaves := pdfPageLeafRe.FindAll(pdfBytes, -1); len(leaves) > 0 {
		return len(leaves), nil
	}
	return 0, fmt.Errorf("unable to determine page count from generated PDF (%d bytes)", len(pdfBytes))
}

func buildWrapParityDocument(bulletCount int) *render.ResumeDocument {
	doc := render.NewResumeDocument()
	doc.Header = render.Header{
		FullName: "Jordan Rivera",
		Headline: "Principal Systems Architect",
	}

	bullets := make([]render.Bullet, 0, bulletCount)
	for i := 1; i <= bulletCount; i++ {
		bullets = append(bullets, render.NewBullet(
			fmt.Sprintf("b-wrap-%d", i),
			fmt.Sprintf("Designed fault-tolerant distributed storage engine #%d sustaining 120,000 writes/sec with deterministic consensus recovery across multi-region clusters.", i),
			nil,
			true,
		))
	}

	doc.Experience = []render.ExperienceItem{
		{
			ID:           "exp-wrap-1",
			RoleTitle:    "Principal Infrastructure Engineer",
			Organization: "CloudScale Systems",
			Location:     "Seattle, WA",
			Duration:     "2021 - Present",
			Highlights:   bullets,
		},
	}
	return doc
}

func TestPDFPrinter_PrintWrapParity_LetterAndA4(t *testing.T) {
	_, err := render.FindChromiumBinary()
	if err != nil {
		t.Skip("skipping live chromium print-wrap parity test: chromium binary not found on host")
	}

	tests := []struct {
		name        string
		page        render.PageConfig
		bulletCount int
		wantPages   int
	}{
		{
			name: "US Letter 1-page default margins",
			page: render.PageConfig{
				Size:         "letter",
				MarginTop:    "0.5in",
				MarginBottom: "0.5in",
				MarginLeft:   "0.5in",
				MarginRight:  "0.5in",
			},
			bulletCount: 10,
			wantPages:   1,
		},
		{
			name: "US Letter 2-page narrow printable width",
			page: render.PageConfig{
				Size:         "letter",
				MarginTop:    "0.5in",
				MarginBottom: "0.5in",
				MarginLeft:   "1.25in",
				MarginRight:  "1.25in",
			},
			bulletCount: 22,
			wantPages:   2,
		},
		{
			name: "A4 1-page 20mm margins",
			page: render.PageConfig{
				Size:         "a4",
				MarginTop:    "20mm",
				MarginBottom: "20mm",
				MarginLeft:   "20mm",
				MarginRight:  "20mm",
			},
			bulletCount: 10,
			wantPages:   1,
		},
		{
			name: "A4 2-page 20mm margins",
			page: render.PageConfig{
				Size:         "a4",
				MarginTop:    "20mm",
				MarginBottom: "20mm",
				MarginLeft:   "20mm",
				MarginRight:  "20mm",
			},
			bulletCount: 24,
			wantPages:   2,
		},
	}

	pii := samplePIIHeader()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			theme := render.DefaultThemeManifest()
			theme.Page = tt.page

			printer, err := render.NewPDFPrinter(
				render.WithTheme(theme),
				render.WithTargetPages(tt.wantPages),
				render.WithTimeout(30*time.Second),
			)
			if err != nil {
				t.Fatalf("failed to initialize PDFPrinter: %v", err)
			}

			doc := buildWrapParityDocument(tt.bulletCount)
			pdfBytes, telemetry, err := printer.Print(doc, pii)
			if err != nil {
				t.Fatalf("printer.Print failed: %v", err)
			}

			pdfPages, err := countPDFPages(pdfBytes)
			if err != nil {
				t.Fatalf("countPDFPages failed: %v", err)
			}

			if telemetry.PageCount != tt.wantPages {
				t.Errorf("telemetry.PageCount = %d, want %d (contentHeight=%.2f, usablePageHeight=%.2f)",
					telemetry.PageCount, tt.wantPages, telemetry.ContentHeightPX, telemetry.UsablePageHeightPX)
			}
			if pdfPages != tt.wantPages {
				t.Errorf("generated PDF page count = %d, want %d", pdfPages, tt.wantPages)
			}
			if telemetry.PageCount != pdfPages {
				t.Errorf("print-wrap parity mismatch: telemetry.PageCount=%d vs generated PDF pages=%d", telemetry.PageCount, pdfPages)
			}
		})
	}
}

func TestPDFPrinter_Telemetry_InlineRunLineGrouping(t *testing.T) {
	_, err := render.FindChromiumBinary()
	if err != nil {
		t.Skip("skipping live chromium inline-run telemetry test: chromium binary not found on host")
	}

	printer, err := render.NewPDFPrinter(render.WithTimeout(30 * time.Second))
	if err != nil {
		t.Fatalf("failed to initialize PDFPrinter: %v", err)
	}

	doc := render.NewResumeDocument()
	doc.Header = render.Header{
		FullName: "Jordan Rivera",
		Headline: "Systems Architect",
	}
	doc.Experience = []render.ExperienceItem{
		{
			ID:           "exp-inline",
			RoleTitle:    "Staff Engineer",
			Organization: "Acme Systems",
			Duration:     "2023 - Present",
			Highlights: []render.Bullet{
				{
					ID:   "b-plain-single",
					Text: "Engineered telemetry pipeline in Go and Rust for production clusters.",
					Runs: []render.InlineRun{
						{Text: "Engineered telemetry pipeline in Go and Rust for production clusters."},
					},
				},
				{
					ID:   "b-link-single",
					Text: "Engineered telemetry pipeline in Go and Rust for production clusters.",
					Runs: []render.InlineRun{
						{Text: "Engineered "},
						{Text: "telemetry pipeline", Link: "https://example.com/telemetry"},
						{Text: " in "},
						{Text: "Go", Link: "https://go.dev"},
						{Text: " and Rust for production clusters."},
					},
				},
				{
					ID:   "b-strong-link-single",
					Text: "Built distributed telemetry engine in Go and Rust.",
					Runs: []render.InlineRun{
						{Text: "Built "},
						{Text: "distributed", Bold: true},
						{Text: " telemetry engine in "},
						{Text: "Go", Link: "https://go.dev"},
						{Text: " and "},
						{Text: "Rust", Bold: true},
						{Text: "."},
					},
				},
				{
					ID:   "b-inline-multiline-ok",
					Text: "Architected high-throughput distributed consensus engine across multi-region clusters using Raft, gRPC streaming, and deterministic replay.",
					Runs: []render.InlineRun{
						{Text: "Architected high-throughput distributed consensus engine across multi-region clusters using "},
						{Text: "Raft", Bold: true},
						{Text: ", "},
						{Text: "gRPC streaming", Link: "https://example.com/grpc"},
						{Text: ", and "},
						{Text: "deterministic replay", Bold: true},
						{Text: "."},
					},
				},
				{
					ID:   "b-inline-multiline-runt",
					Text: "Architected high-throughput distributed consensus engine across multi-region production clusters with zero loss.",
					Runs: []render.InlineRun{
						{Text: "Architected high-throughput distributed consensus engine across multi-region "},
						{Text: "production", Link: "https://example.com/prod"},
						{Text: " clusters with zero "},
						{Text: "loss", Bold: true},
						{Text: "."},
					},
				},
			},
		},
	}

	telemetry, err := printer.ExtractDocumentTelemetry(doc, samplePIIHeader())
	if err != nil {
		t.Fatalf("ExtractDocumentTelemetry failed: %v", err)
	}

	byID := make(map[string]render.BulletTelemetry, len(telemetry.Bullets))
	for _, b := range telemetry.Bullets {
		byID[b.ID] = b
	}

	plain, ok := byID["b-plain-single"]
	if !ok {
		t.Fatalf("missing b-plain-single in telemetry")
	}
	if plain.LineCount != 1 {
		t.Fatalf("b-plain-single LineCount = %d, want 1", plain.LineCount)
	}

	linkSingle, ok := byID["b-link-single"]
	if !ok {
		t.Fatalf("missing b-link-single in telemetry")
	}
	if linkSingle.LineCount != 1 {
		t.Errorf("b-link-single LineCount = %d, want 1 (inline <a> spans must group onto same visual line)", linkSingle.LineCount)
	}
	if linkSingle.IsOrphan {
		t.Errorf("b-link-single IsOrphan = true (%s), want false", linkSingle.OrphanReason)
	}
	if math.Abs(linkSingle.LastLineWidth-linkSingle.LineWidth) > 0.5 {
		t.Errorf("b-link-single LastLineWidth=%.2f != LineWidth=%.2f", linkSingle.LastLineWidth, linkSingle.LineWidth)
	}
	if math.Abs(linkSingle.LastLineWidth-plain.LastLineWidth) > 1.5 {
		t.Errorf("b-link-single LastLineWidth=%.2f diverges from plain baseline %.2f", linkSingle.LastLineWidth, plain.LastLineWidth)
	}

	strongLinkSingle, ok := byID["b-strong-link-single"]
	if !ok {
		t.Fatalf("missing b-strong-link-single in telemetry")
	}
	if strongLinkSingle.LineCount != 1 {
		t.Errorf("b-strong-link-single LineCount = %d, want 1", strongLinkSingle.LineCount)
	}
	if strongLinkSingle.IsOrphan {
		t.Errorf("b-strong-link-single IsOrphan = true (%s), want false", strongLinkSingle.OrphanReason)
	}
	if strongLinkSingle.LastLineWidth < 200.0 {
		t.Errorf("b-strong-link-single LastLineWidth = %.2f, expected full single-line width >= 200px", strongLinkSingle.LastLineWidth)
	}

	multiOK, ok := byID["b-inline-multiline-ok"]
	if !ok {
		t.Fatalf("missing b-inline-multiline-ok in telemetry")
	}
	if multiOK.LineCount != 2 {
		t.Errorf("b-inline-multiline-ok LineCount = %d, want 2", multiOK.LineCount)
	}
	if multiOK.LastLineWidth < 150.0 {
		t.Errorf("b-inline-multiline-ok LastLineWidth = %.2f, want >= 150px across grouped 2nd line", multiOK.LastLineWidth)
	}
	if multiOK.IsOrphan {
		t.Errorf("b-inline-multiline-ok IsOrphan = true (%s), want false", multiOK.OrphanReason)
	}

	multiRunt, ok := byID["b-inline-multiline-runt"]
	if !ok {
		t.Fatalf("missing b-inline-multiline-runt in telemetry")
	}
	if multiRunt.LineCount != 2 {
		t.Errorf("b-inline-multiline-runt LineCount = %d, want 2", multiRunt.LineCount)
	}
	if multiRunt.LastLineWidth >= 75.0 {
		t.Errorf("b-inline-multiline-runt LastLineWidth = %.2f, want < 75px for true typographic runt", multiRunt.LastLineWidth)
	}
	if !multiRunt.IsOrphan || multiRunt.OrphanReason != "typographic_runt" {
		t.Errorf("b-inline-multiline-runt IsOrphan=%v reason=%q, want true / typographic_runt", multiRunt.IsOrphan, multiRunt.OrphanReason)
	}
}
