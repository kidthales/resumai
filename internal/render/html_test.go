package render_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/render"
)

func sampleResumeDocument() *render.ResumeDocument {
	doc := render.NewResumeDocument()
	doc.Metadata.DocumentID = "doc-001"
	doc.Metadata.TargetRole = "Principal Systems Architect"
	doc.Metadata.TargetCompany = "Acme Corp"

	// Initial un-injected header (PII is air-gapped before render)
	doc.Header = render.Header{
		FullName: "Candidate Pseudonym",
		Headline: "Principal Systems Architect | Distributed Systems",
	}

	doc.Summary = &render.SummarySection{
		Title: "Executive Summary",
		Text:  "High-performance systems architect with deep expertise in Go and Rust.",
		Runs: []render.InlineRun{
			{Text: "High-performance systems architect with deep expertise in "},
			{Text: "Go", Bold: true},
			{Text: " and "},
			{Text: "Rust", Bold: true},
			{Text: ". Built telemetry engines handling "},
			{Text: "100k+ req/sec", Bold: true, Italic: true},
			{Text: " under strict SLAs."},
		},
	}

	doc.Experience = []render.ExperienceItem{
		{
			ID:           "exp-1",
			RoleTitle:    "Staff Infrastructure Engineer",
			Organization: "CloudScale Inc.",
			Location:     "Seattle, WA",
			StartDate:    "2022",
			EndDate:      "Present",
			Duration:     "2022 - Present",
			Highlights: []render.Bullet{
				render.NewBullet("b-1", "Architected distributed raft cluster reducing p99 latency by **42%**.", []string{"f-1"}, true),
				render.NewBullet("b-2", "Implemented pure-Go `archive/zip` streaming writer saving *15GB* heap.", []string{"f-2"}, true),
			},
		},
	}

	doc.Education = []render.EducationItem{
		{
			ID:           "edu-1",
			Institution:  "University of Washington",
			Degree:       "Bachelor of Science",
			FieldOfStudy: "Computer Science",
			Location:     "Seattle, WA",
			Duration:     "2016 - 2020",
			Honors:       "Magna Cum Laude",
			Highlights: []render.Bullet{
				render.NewBullet("b-edu-1", "Undergraduate researcher in distributed consensus protocols.", []string{"f-3"}, true),
			},
		},
	}

	doc.Skills = []render.SkillCategory{
		{
			Name:   "Languages",
			Skills: []string{"Go", "Rust", "C++", "TypeScript", "SQL"},
		},
		{
			Name:   "Infrastructure",
			Skills: []string{"Kubernetes", "Linux Kernel", "eBPF", "Docker"},
		},
	}

	doc.Projects = []render.ProjectItem{
		{
			Name:         "FastCDP",
			Role:         "Creator & Maintainer",
			URL:          "https://github.com/fastcdp/fastcdp",
			Duration:     "2023 - Present",
			Technologies: []string{"Go", "CDP", "Chromium"},
			Highlights: []render.Bullet{
				render.NewBullet("b-proj-1", "Headless Chrome DevTools Protocol client with zero external allocations.", []string{"f-4"}, true),
			},
		},
	}

	doc.Certifications = []string{
		"Certified Kubernetes Administrator (CKA)",
		"AWS Certified Solutions Architect - Professional",
	}

	doc.CustomSections = []render.CustomSection{
		{
			Title: "Open Source Leadership",
			Highlights: []render.Bullet{
				render.NewBullet("b-custom-1", "Core contributor to open-source distributed tracing standards.", []string{"f-5"}, true),
			},
		},
	}

	return doc
}

func samplePIIHeader() *domain.PIIHeader {
	return &domain.PIIHeader{
		FullName:     "Ada Lovelace",
		Email:        "ada.lovelace@example.com",
		Phone:        "+1 (206) 555-0199",
		Location:     "Seattle, WA",
		LinkedInURL:  "https://linkedin.com/in/adalovelace",
		GitHubURL:    "https://github.com/adalovelace",
		PortfolioURL: "https://adalovelace.dev",
		UpdatedAt:    time.Now().UTC(),
	}
}

func TestNewHTMLRenderer_Valid(t *testing.T) {
	r, err := render.NewHTMLRenderer(nil)
	if err != nil {
		t.Fatalf("failed to create default HTMLRenderer: %v", err)
	}
	if r.Theme() == nil {
		t.Error("expected non-nil theme")
	}

	modernTheme, err := render.LoadEmbeddedTheme("modern")
	if err != nil {
		t.Fatalf("failed to load modern theme: %v", err)
	}

	rModern, err := render.NewHTMLRenderer(modernTheme)
	if err != nil {
		t.Fatalf("failed to create modern HTMLRenderer: %v", err)
	}
	if rModern.Theme().Name != "modern" {
		t.Errorf("expected modern theme name, got %q", rModern.Theme().Name)
	}
}

func TestNewHTMLRenderer_InvalidTheme(t *testing.T) {
	invalidTheme := render.DefaultThemeManifest()
	invalidTheme.Name = "" // invalid

	_, err := render.NewHTMLRenderer(invalidTheme)
	if err == nil {
		t.Fatal("expected error with invalid theme, got nil")
	}
}

func TestHTMLRenderer_Render_NilDoc(t *testing.T) {
	r, err := render.NewHTMLRenderer(nil)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	_, err = r.Render(nil, nil)
	if err == nil {
		t.Fatal("expected error rendering nil ResumeDocument, got nil")
	}
}

func TestHTMLRenderer_Render_PIIAirGapped(t *testing.T) {
	r, err := render.NewHTMLRenderer(nil)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	doc := sampleResumeDocument()
	originalFullName := doc.Header.FullName
	pii := samplePIIHeader()

	outBytes, err := r.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render HTML: %v", err)
	}

	out := string(outBytes)

	// 1. Verify original doc was NOT mutated (Air-Gap Invariant)
	if doc.Header.FullName != originalFullName {
		t.Errorf("original doc.Header.FullName mutated from %q to %q", originalFullName, doc.Header.FullName)
	}
	if doc.Header.Email != "" {
		t.Errorf("original doc.Header.Email was mutated to %q", doc.Header.Email)
	}

	// 2. Verify rendered output contains injected PII
	if !strings.Contains(out, "Ada Lovelace") {
		t.Errorf("rendered output missing injected FullName 'Ada Lovelace'")
	}
	if !strings.Contains(out, "ada.lovelace@example.com") {
		t.Errorf("rendered output missing injected Email")
	}
	if !strings.Contains(out, "+1 (206) 555-0199") {
		t.Errorf("rendered output missing injected Phone")
	}
	if !strings.Contains(out, "https://linkedin.com/in/adalovelace") {
		t.Errorf("rendered output missing injected LinkedIn URL")
	}
	if !strings.Contains(out, "linkedin.com/in/adalovelace") {
		t.Errorf("rendered output missing cleaned display LinkedIn URL")
	}
}

func TestHTMLRenderer_Render_OfflineAndSelfContained(t *testing.T) {
	r, err := render.NewHTMLRenderer(nil)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	outBytes, err := r.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render HTML: %v", err)
	}

	out := string(outBytes)

	// Invariant: Zero external network @import calls or webfont URLs
	if strings.Contains(out, "@import") {
		t.Error("rendered HTML contains forbidden @import directive")
	}
	if strings.Contains(out, "fonts.googleapis.com") || strings.Contains(out, "fonts.gstatic.com") {
		t.Error("rendered HTML contains external Google Fonts link")
	}

	// Invariant: Base64 data URIs for Inter and JetBrains Mono offline fonts
	dataURISig := "data:font/woff2;charset=utf-8;base64,"
	if !strings.Contains(out, dataURISig) {
		t.Errorf("rendered HTML missing offline base64 font data URI signature %q", dataURISig)
	}

	// Invariant: Zero <table wrapper tags (ATS Linear Invariant)
	if strings.Contains(out, "<table") {
		t.Errorf("rendered HTML contains <table> tag, violating zero full-page wrapper table ATS invariant")
	}
}

func TestHTMLRenderer_Render_LayoutTelemetryHooks(t *testing.T) {
	r, err := render.NewHTMLRenderer(nil)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	outBytes, err := r.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render HTML: %v", err)
	}

	out := string(outBytes)

	// Verify data attributes for Chrome CDP Range.getClientRects() telemetry
	expectedHooks := []string{
		`data-bullet-id="b-1"`,
		`data-bullet-id="b-2"`,
		`data-bullet-id="b-edu-1"`,
		`data-bullet-id="b-proj-1"`,
		`data-bullet-id="b-custom-1"`,
		`class="resume-bullet"`,
		`data-item-id="exp-1"`,
		`data-item-id="edu-1"`,
	}

	for _, hook := range expectedHooks {
		if !strings.Contains(out, hook) {
			t.Errorf("rendered HTML missing layout telemetry hook %s", hook)
		}
	}
}

func TestHTMLRenderer_Render_InlineRunFormatting(t *testing.T) {
	r, err := render.NewHTMLRenderer(nil)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	doc := sampleResumeDocument()
	outBytes, err := r.Render(doc, nil)
	if err != nil {
		t.Fatalf("failed to render HTML: %v", err)
	}

	out := string(outBytes)

	// Verify bold, italic, code styling
	if !strings.Contains(out, "<strong>Go</strong>") {
		t.Errorf("rendered HTML missing bold formatting for 'Go'")
	}
	if !strings.Contains(out, "<strong><em>100k+ req/sec</em></strong>") {
		t.Errorf("rendered HTML missing bold+italic formatting for '100k+ req/sec'")
	}
	if !strings.Contains(out, "<strong>42%</strong>") {
		t.Errorf("rendered HTML missing markdown parsed bold '42%%'")
	}
	if !strings.Contains(out, "<code>archive/zip</code>") {
		t.Errorf("rendered HTML missing code formatting for 'archive/zip'")
	}
}

func TestHTMLRenderer_Render_ThemeSwitching(t *testing.T) {
	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	for _, themeName := range []string{"default", "modern", "classic"} {
		theme, err := render.LoadEmbeddedTheme(themeName)
		if err != nil {
			t.Fatalf("failed to load theme %q: %v", themeName, err)
		}

		renderer, err := render.NewHTMLRenderer(theme)
		if err != nil {
			t.Fatalf("failed to create renderer for %q: %v", themeName, err)
		}

		outBytes, err := renderer.Render(doc, pii)
		if err != nil {
			t.Fatalf("failed to render HTML with theme %q: %v", themeName, err)
		}

		out := string(outBytes)
		if !strings.Contains(out, "<!DOCTYPE html>") {
			t.Errorf("theme %q output missing DOCTYPE", themeName)
		}
		if !strings.Contains(out, "Ada Lovelace") {
			t.Errorf("theme %q output missing header name", themeName)
		}
	}
}

func TestHTMLRenderer_Render_UnsafeURLSanitization(t *testing.T) {
	doc := sampleResumeDocument()
	doc.Projects = []render.ProjectItem{
		{
			Name: "Malicious Project",
			URL:  "javascript:alert('project')",
		},
	}
	doc.Summary.Runs = append(doc.Summary.Runs, render.InlineRun{
		Text: "Unsafe Link",
		Link: "javascript:alert('inline')",
	})

	pii := samplePIIHeader()
	pii.LinkedInURL = "javascript:alert('linkedin')"
	pii.GitHubURL = "data:text/html,<script>alert(1)</script>"

	renderer, err := render.NewHTMLRenderer(nil)
	if err != nil {
		t.Fatalf("failed to create renderer: %v", err)
	}

	outBytes, err := renderer.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render HTML: %v", err)
	}

	out := string(outBytes)
	if strings.Contains(out, "href=\"javascript:") || strings.Contains(out, "href=\"data:") {
		t.Errorf("rendered HTML contains unsafe URL schemes: %s", out)
	}
}
