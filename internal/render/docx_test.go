package render_test

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/kidthales/resumai/internal/render"
)

func TestNewDocxWriter_Valid(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create default DocxWriter: %v", err)
	}
	if w.Theme() == nil {
		t.Error("expected non-nil theme")
	}

	modernTheme, err := render.LoadEmbeddedTheme("modern")
	if err != nil {
		t.Fatalf("failed to load modern theme: %v", err)
	}

	wModern, err := render.NewDocxWriter(modernTheme)
	if err != nil {
		t.Fatalf("failed to create modern DocxWriter: %v", err)
	}
	if wModern.Theme().Name != "modern" {
		t.Errorf("expected modern theme name, got %q", wModern.Theme().Name)
	}
}

func TestNewDocxWriter_InvalidTheme(t *testing.T) {
	invalidTheme := render.DefaultThemeManifest()
	invalidTheme.Name = "" // invalid

	_, err := render.NewDocxWriter(invalidTheme)
	if err == nil {
		t.Fatal("expected error on invalid theme manifest, got nil")
	}
}

func TestDocxWriter_Render_NilDoc(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	_, err = w.Render(nil, nil)
	if err == nil {
		t.Fatal("expected error when rendering nil ResumeDocument, got nil")
	}

	_, err = w.Write(nil, nil)
	if err == nil {
		t.Fatal("expected error when writing nil ResumeDocument, got nil")
	}
}

func TestDocxWriter_Render_ZipPackageIntegrity(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	docxBytes, err := w.Write(doc, pii)
	if err != nil {
		t.Fatalf("failed to write docx: %v", err)
	}

	if len(docxBytes) == 0 {
		t.Fatal("expected non-empty docx bytes")
	}

	// Verify zip package integrity
	zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	if err != nil {
		t.Fatalf("failed to read zip archive: %v", err)
	}

	expectedFiles := map[string]bool{
		"[Content_Types].xml":           false,
		"_rels/.rels":                   false,
		"docProps/core.xml":             false,
		"docProps/app.xml":              false,
		"word/_rels/document.xml.rels": false,
		"word/styles.xml":               false,
		"word/numbering.xml":            false,
		"word/document.xml":             false,
	}

	for _, f := range zr.File {
		if _, ok := expectedFiles[f.Name]; ok {
			expectedFiles[f.Name] = true
		}

		// Verify every XML file is well-formed XML
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open zip entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("failed to read zip entry %s: %v", f.Name, err)
		}

		var parsed interface{}
		if err := xml.Unmarshal(data, &parsed); err != nil {
			t.Errorf("zip entry %s contains malformed XML: %v", f.Name, err)
		}
	}

	for name, found := range expectedFiles {
		if !found {
			t.Errorf("missing expected package part: %s", name)
		}
	}
}

func TestDocxWriter_Render_ZeroTableInvariant(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	docxBytes, err := w.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render docx: %v", err)
	}

	docXML := extractZipFile(t, docxBytes, "word/document.xml")

	// Strict Architectural Invariant: Zero <w:tbl> full-page wrapper tables
	if strings.Contains(docXML, "<w:tbl") {
		t.Errorf("architectural invariant violated: found <w:tbl> in docx output")
	}
	if strings.Contains(docXML, "<w:tr") || strings.Contains(docXML, "<w:tc") {
		t.Errorf("architectural invariant violated: found table rows or cells in docx output")
	}
}

func TestDocxWriter_Render_RightAlignedTabStops(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	docxBytes, err := w.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render docx: %v", err)
	}

	docXML := extractZipFile(t, docxBytes, "word/document.xml")

	// Verify w:tabs with right alignment tab stop exists
	if !strings.Contains(docXML, `<w:tab w:val="right"`) {
		t.Errorf("expected right-aligned tab-stop <w:tab w:val=\"right\" .../> in document.xml")
	}

	// Verify tab run <w:tab/> exists
	if !strings.Contains(docXML, "<w:tab/>") {
		t.Errorf("expected tab run <w:tab/> in document.xml for split headers")
	}
}

func TestDocxWriter_Render_KeepNextAndKeepLines(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	docxBytes, err := w.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render docx: %v", err)
	}

	docXML := extractZipFile(t, docxBytes, "word/document.xml")

	// Verify pagination properties w:keepNext and w:keepLines exist on headings and item headers
	if !strings.Contains(docXML, "<w:keepNext/>") {
		t.Errorf("expected <w:keepNext/> paragraph property in document.xml")
	}
	if !strings.Contains(docXML, "<w:keepLines/>") {
		t.Errorf("expected <w:keepLines/> paragraph property in document.xml")
	}
}

func TestDocxWriter_Render_LinearATSTextOrderParity(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	docxBytes, err := w.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render docx: %v", err)
	}

	docXML := extractZipFile(t, docxBytes, "word/document.xml")

	// Extract linear text from document.xml exactly as an ATS parser does
	text := extractLinearDocxText(t, docXML)

	// Verify sequential text reading order matches AST:
	// 1. Candidate Name
	// 2. Headline
	// 3. Contact info
	// 4. Executive Summary
	// 5. Experience
	// 6. Education
	// 7. Skills
	// 8. Projects
	// 9. Certifications
	// 10. Open Source Leadership (Custom)

	indices := []struct {
		name string
		sub  string
		pos  int
	}{
		{name: "FullName", sub: "Ada Lovelace"},
		{name: "Headline", sub: "Principal Systems Architect | Distributed Systems"},
		{name: "Email", sub: "ada.lovelace@example.com"},
		{name: "SummaryTitle", sub: "EXECUTIVE SUMMARY"},
		{name: "SummaryText", sub: "High-performance systems architect with deep expertise in Go and Rust."},
		{name: "ExperienceTitle", sub: "EXPERIENCE"},
		{name: "RoleTitle", sub: "Staff Infrastructure Engineer"},
		{name: "Organization", sub: "CloudScale Inc."},
		{name: "Duration", sub: "2022 - Present | Seattle, WA"},
		{name: "Bullet1", sub: "Architected distributed raft cluster reducing p99 latency by 42%."},
		{name: "Bullet2", sub: "Implemented pure-Go archive/zip streaming writer saving 15GB heap."},
		{name: "EducationTitle", sub: "EDUCATION"},
		{name: "Degree", sub: "Bachelor of Science in Computer Science"},
		{name: "Institution", sub: "University of Washington"},
		{name: "Honors", sub: "Magna Cum Laude"},
		{name: "SkillsTitle", sub: "SKILLS"},
		{name: "SkillLanguages", sub: "Languages: Go, Rust, C++, TypeScript, SQL"},
		{name: "SkillInfra", sub: "Infrastructure: Kubernetes, Linux Kernel, eBPF, Docker"},
		{name: "ProjectsTitle", sub: "PROJECTS"},
		{name: "ProjectName", sub: "FastCDP (Creator & Maintainer)"},
		{name: "ProjectURL", sub: "[github.com/fastcdp/fastcdp]"},
		{name: "Technologies", sub: "Technologies: Go, CDP, Chromium"},
		{name: "CertificationsTitle", sub: "CERTIFICATIONS"},
		{name: "Certification1", sub: "Certified Kubernetes Administrator (CKA)"},
		{name: "CustomTitle", sub: "OPEN SOURCE LEADERSHIP"},
		{name: "CustomBullet", sub: "Core contributor to open-source distributed tracing standards."},
	}

	lastPos := -1
	for i := range indices {
		idx := strings.Index(text, indices[i].sub)
		if idx == -1 {
			t.Errorf("missing expected text in linear ATS stream: %q (context: %s)", indices[i].sub, indices[i].name)
			continue
		}
		indices[i].pos = idx
		if idx <= lastPos {
			t.Errorf("linear ATS text order inversion: %s (%q at pos %d) appeared before or at previous item (last pos %d)",
				indices[i].name, indices[i].sub, idx, lastPos)
		}
		lastPos = idx
	}
}

func TestDocxWriter_Render_PIIAirGapped(t *testing.T) {
	w, err := render.NewDocxWriter(nil)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	doc := sampleResumeDocument()

	// 1. Without PII injected, original pseudonym must remain
	unInjectedBytes, err := w.Render(doc, nil)
	if err != nil {
		t.Fatalf("failed to render un-injected docx: %v", err)
	}
	unInjectedXML := extractZipFile(t, unInjectedBytes, "word/document.xml")
	if !strings.Contains(unInjectedXML, "Candidate Pseudonym") {
		t.Error("expected 'Candidate Pseudonym' in un-injected output")
	}
	if strings.Contains(unInjectedXML, "Ada Lovelace") {
		t.Error("unexpected real PII 'Ada Lovelace' in un-injected output")
	}

	// 2. Original doc AST must NOT be modified in place
	if doc.Header.FullName != "Candidate Pseudonym" {
		t.Errorf("original AST mutated in place: got %q, want 'Candidate Pseudonym'", doc.Header.FullName)
	}

	// 3. With PII injected at render time, real name must replace pseudonym in output
	pii := samplePIIHeader()
	injectedBytes, err := w.Render(doc, pii)
	if err != nil {
		t.Fatalf("failed to render injected docx: %v", err)
	}
	injectedXML := extractZipFile(t, injectedBytes, "word/document.xml")
	if !strings.Contains(injectedXML, "Ada Lovelace") {
		t.Error("expected 'Ada Lovelace' in injected output")
	}
	if strings.Contains(injectedXML, "Candidate Pseudonym") {
		t.Error("pseudonym should not remain when real PII is injected")
	}

	// 4. Original AST must STILL remain untouched
	if doc.Header.FullName != "Candidate Pseudonym" {
		t.Errorf("original AST mutated after render with PII: got %q", doc.Header.FullName)
	}
}

func TestDocxWriter_Render_ThemeSwitching(t *testing.T) {
	themes := []string{"default", "modern", "classic"}
	doc := sampleResumeDocument()
	pii := samplePIIHeader()

	for _, themeName := range themes {
		t.Run("Theme_"+themeName, func(t *testing.T) {
			theme, err := render.LoadEmbeddedTheme(themeName)
			if err != nil {
				t.Fatalf("failed to load theme %q: %v", themeName, err)
			}

			w, err := render.NewDocxWriter(theme)
			if err != nil {
				t.Fatalf("failed to create writer for %q: %v", themeName, err)
			}

			outBytes, err := w.Render(doc, pii)
			if err != nil {
				t.Fatalf("failed to render docx for %q: %v", themeName, err)
			}

			stylesXML := extractZipFile(t, outBytes, "word/styles.xml")
			// Primary font from theme should be declared in styles.xml
			if !strings.Contains(stylesXML, theme.Typography.FontFamilyPrimary) {
				t.Errorf("expected primary font %q in styles.xml for theme %q",
					theme.Typography.FontFamilyPrimary, themeName)
			}

			docXML := extractZipFile(t, outBytes, "word/document.xml")
			if strings.Contains(docXML, "<w:tbl") {
				t.Errorf("found <w:tbl> in docx output for theme %q", themeName)
			}
		})
	}
}

func TestDocxWriter_Render_PageGeometry(t *testing.T) {
	// Test letter vs a4
	doc := sampleResumeDocument()

	letterTheme := render.DefaultThemeManifest()
	letterTheme.Page.Size = "letter"
	wLetter, err := render.NewDocxWriter(letterTheme)
	if err != nil {
		t.Fatalf("failed to create letter writer: %v", err)
	}
	letterBytes, err := wLetter.Render(doc, nil)
	if err != nil {
		t.Fatalf("failed to render letter: %v", err)
	}
	letterXML := extractZipFile(t, letterBytes, "word/document.xml")
	if !strings.Contains(letterXML, `w:w="12240" w:h="15840"`) {
		t.Errorf("expected letter dimensions in document.xml: w:w=\"12240\" w:h=\"15840\"")
	}

	a4Theme := render.DefaultThemeManifest()
	a4Theme.Page.Size = "a4"
	wA4, err := render.NewDocxWriter(a4Theme)
	if err != nil {
		t.Fatalf("failed to create A4 writer: %v", err)
	}
	a4Bytes, err := wA4.Render(doc, nil)
	if err != nil {
		t.Fatalf("failed to render A4: %v", err)
	}
	a4XML := extractZipFile(t, a4Bytes, "word/document.xml")
	if !strings.Contains(a4XML, `w:w="11906" w:h="16838"`) {
		t.Errorf("expected A4 dimensions in document.xml: w:w=\"11906\" w:h=\"16838\"")
	}
}

// Helpers

func extractZipFile(t *testing.T, zipBytes []byte, filename string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("failed to open zip reader: %v", err)
	}

	for _, f := range zr.File {
		if f.Name == filename {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to open %s in zip: %v", filename, err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("failed to read %s in zip: %v", filename, err)
			}
			return string(data)
		}
	}

	t.Fatalf("file %s not found in zip archive", filename)
	return ""
}

// extractLinearDocxText scans document.xml in document stream order and extracts
// all <w:t> content, converting <w:tab/> to \t and <w:p> boundaries to \n.
func extractLinearDocxText(t *testing.T, docXML string) string {
	t.Helper()
	decoder := xml.NewDecoder(strings.NewReader(docXML))
	var sb strings.Builder

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch elem := token.(type) {
		case xml.StartElement:
			if elem.Name.Local == "t" {
				var text string
				if err := decoder.DecodeElement(&text, &elem); err == nil {
					sb.WriteString(text)
				}
			} else if elem.Name.Local == "tab" {
				sb.WriteString("\t")
			}
		case xml.EndElement:
			if elem.Name.Local == "p" {
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}
