package render

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kidthales/resumai/internal/domain"
)

// DocxWriter produces pure-Go OpenXML (.docx) resumes with right-aligned tab stops,
// pagination guards (w:keepNext, w:keepLines), and strict linear ATS text ordering.
// Architectural Invariant: Zero <w:tbl> full-page wrapper tables.
type DocxWriter struct {
	theme *ThemeManifest
}

// NewDocxWriter creates a new DocxWriter configured with the given theme manifest.
// If theme is nil, DefaultThemeManifest() is used.
func NewDocxWriter(theme *ThemeManifest) (*DocxWriter, error) {
	if theme == nil {
		theme = DefaultThemeManifest()
	}
	if err := theme.Validate(); err != nil {
		return nil, fmt.Errorf("invalid theme for DocxWriter: %w", err)
	}
	return &DocxWriter{theme: theme}, nil
}

// Theme returns the writer's active ThemeManifest.
func (w *DocxWriter) Theme() *ThemeManifest {
	return w.theme
}

// Write renders a ResumeDocument AST into OpenXML (.docx) zip package bytes.
// Personal contact information (PII) is injected strictly into an isolated document
// clone at render time to enforce the air-gap invariant.
func (w *DocxWriter) Write(doc *ResumeDocument, pii *domain.PIIHeader) ([]byte, error) {
	return w.Render(doc, pii)
}

// Render renders a ResumeDocument AST into OpenXML (.docx) zip package bytes.
func (w *DocxWriter) Render(doc *ResumeDocument, pii *domain.PIIHeader) ([]byte, error) {
	if doc == nil {
		return nil, errors.New("cannot render nil ResumeDocument")
	}

	// 1. Deep clone the document to prevent mutating the original AST in memory
	cloned, err := doc.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone ResumeDocument for docx render: %w", err)
	}

	// 2. Inject air-gapped personal contact information strictly at render time
	if pii != nil {
		cloned.InjectPII(pii)
	}

	// 3. Assemble OpenXML parts and build the zip archive
	b := newDocxBuilder(w.theme)
	return b.build(cloned)
}

// docxBuilder coordinates relationship registration and OpenXML XML generation.
type docxBuilder struct {
	theme       *ThemeManifest
	rels        *relTracker
	pageWidth   int
	pageHeight  int
	marginTop   int
	marginRight int
	marginBott  int
	marginLeft  int
	contentW    int
}

func newDocxBuilder(theme *ThemeManifest) *docxBuilder {
	// Calculate twip dimensions (1 in = 1440 twips, 1 pt = 20 twips)
	pageW := 12240 // 8.5in (letter)
	pageH := 15840 // 11in
	if strings.ToLower(theme.Page.Size) == "a4" {
		pageW = 11906 // 210mm
		pageH = 16838 // 297mm
	}

	mTop := parseTwips(theme.Page.MarginTop, 720)
	mRight := parseTwips(theme.Page.MarginRight, 720)
	mBott := parseTwips(theme.Page.MarginBottom, 720)
	mLeft := parseTwips(theme.Page.MarginLeft, 720)
	contentW := pageW - mLeft - mRight
	if contentW <= 0 {
		contentW = 10800
	}

	return &docxBuilder{
		theme:       theme,
		rels:        newRelTracker(),
		pageWidth:   pageW,
		pageHeight:  pageH,
		marginTop:   mTop,
		marginRight: mRight,
		marginBott:  mBott,
		marginLeft:  mLeft,
		contentW:    contentW,
	}
}

func (b *docxBuilder) build(doc *ResumeDocument) ([]byte, error) {
	// First pass: render document.xml to discover all external hyperlinks
	var docBody bytes.Buffer
	b.renderDocumentBody(&docBody, doc)

	var docXML bytes.Buffer
	docXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	docXML.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" `)
	docXML.WriteString(`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + "\n")
	docXML.WriteString("  <w:body>\n")
	docXML.Write(docBody.Bytes())

	// Page Geometry Section Properties (at end of body)
	fmt.Fprintf(&docXML, `    <w:sectPr>
      <w:pgSz w:w="%d" w:h="%d" w:orient="portrait"/>
      <w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="0" w:footer="0" w:gutter="0"/>
      <w:cols w:num="1" w:space="720"/>
      <w:docGrid w:linePitch="360"/>
    </w:sectPr>
`, b.pageWidth, b.pageHeight, b.marginTop, b.marginRight, b.marginBott, b.marginLeft)
	docXML.WriteString("  </w:body>\n</w:document>\n")

	// Package zip archive
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	files := []struct {
		name    string
		content []byte
	}{
		{"[Content_Types].xml", []byte(b.contentTypesXML())},
		{"_rels/.rels", []byte(b.packageRelsXML())},
		{"docProps/core.xml", []byte(b.corePropsXML(doc))},
		{"docProps/app.xml", []byte(b.appPropsXML())},
		{"word/_rels/document.xml.rels", []byte(b.documentRelsXML())},
		{"word/styles.xml", []byte(b.stylesXML())},
		{"word/numbering.xml", []byte(b.numberingXML())},
		{"word/document.xml", docXML.Bytes()},
	}

	for _, f := range files {
		h := &zip.FileHeader{
			Name:   f.name,
			Method: zip.Deflate,
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create zip entry %s: %w", f.name, err)
		}
		if _, err := w.Write(f.content); err != nil {
			return nil, fmt.Errorf("failed to write zip entry %s: %w", f.name, err)
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize docx zip package: %w", err)
	}

	return zipBuf.Bytes(), nil
}

func (b *docxBuilder) renderDocumentBody(sb *bytes.Buffer, doc *ResumeDocument) {
	// Header block
	b.renderHeader(sb, &doc.Header)

	// Sections in configured order
	order := b.theme.Sections.Order
	if len(order) == 0 {
		order = []string{"summary", "experience", "education", "skills", "projects", "certifications", "custom"}
	}

	for _, sec := range order {
		switch strings.ToLower(sec) {
		case "summary":
			if doc.Summary != nil && (doc.Summary.Text != "" || len(doc.Summary.Runs) > 0) {
				b.renderSummary(sb, doc.Summary)
			}
		case "experience":
			if len(doc.Experience) > 0 {
				b.renderExperience(sb, doc.Experience)
			}
		case "education":
			if len(doc.Education) > 0 {
				b.renderEducation(sb, doc.Education)
			}
		case "skills":
			if len(doc.Skills) > 0 {
				b.renderSkills(sb, doc.Skills)
			}
		case "projects":
			if len(doc.Projects) > 0 {
				b.renderProjects(sb, doc.Projects)
			}
		case "certifications":
			if len(doc.Certifications) > 0 {
				b.renderCertifications(sb, doc.Certifications)
			}
		case "custom":
			if len(doc.CustomSections) > 0 {
				b.renderCustomSections(sb, doc.CustomSections)
			}
		}
	}
}

func (b *docxBuilder) renderHeader(sb *bytes.Buffer, h *Header) {
	t := b.theme
	primaryColor := cleanHex(t.Colors.Primary)
	accentColor := cleanHex(t.Colors.Accent)
	secColor := cleanHex(t.Colors.Secondary)
	mutedColor := cleanHex(t.Colors.Muted)
	borderColor := cleanHex(t.Colors.Border)

	titleSz := parseHalfPoints(t.Typography.FontSizeTitle, 36)
	subSz := parseHalfPoints(t.Typography.FontSizeSubtitle, 21)
	smallSz := parseHalfPoints(t.Typography.FontSizeSmall, 18)

	headerAlign := "left"
	if strings.ToLower(t.Header.Layout) == "centered" {
		headerAlign = "center"
	}

	// 1. Full Name
	fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:jc w:val="%s"/>
        <w:keepNext/>
        <w:keepLines/>
        <w:spacing w:before="0" w:after="40" w:line="240" w:lineRule="auto"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:b/>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
    </w:p>
`, headerAlign, titleSz, titleSz, primaryColor, xmlEscape(h.FullName))

	// 2. Headline
	if t.Header.ShowHeadline && h.Headline != "" {
		fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:jc w:val="%s"/>
        <w:keepNext/>
        <w:keepLines/>
        <w:spacing w:before="0" w:after="60" w:line="240" w:lineRule="auto"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:b/>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
    </w:p>
`, headerAlign, subSz, subSz, accentColor, xmlEscape(h.Headline))
	}

	// 3. Contact items
	type contactEntry struct {
		label string
		url   string
	}
	var entries []contactEntry
	if h.Email != "" {
		entries = append(entries, contactEntry{label: h.Email, url: "mailto:" + h.Email})
	}
	if h.Phone != "" {
		entries = append(entries, contactEntry{label: h.Phone})
	}
	if h.Location != "" {
		entries = append(entries, contactEntry{label: h.Location})
	}
	if h.LinkedInURL != "" {
		entries = append(entries, contactEntry{label: cleanDisplayURL(h.LinkedInURL), url: h.LinkedInURL})
	}
	if h.GitHubURL != "" {
		entries = append(entries, contactEntry{label: cleanDisplayURL(h.GitHubURL), url: h.GitHubURL})
	}
	if h.PortfolioURL != "" {
		entries = append(entries, contactEntry{label: cleanDisplayURL(h.PortfolioURL), url: h.PortfolioURL})
	}
	if h.WebsiteURL != "" {
		entries = append(entries, contactEntry{label: cleanDisplayURL(h.WebsiteURL), url: h.WebsiteURL})
	}

	if len(entries) > 0 {
		fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:jc w:val="%s"/>
        <w:keepNext/>
        <w:keepLines/>
        <w:spacing w:before="0" w:after="80" w:line="240" w:lineRule="auto"/>
      </w:pPr>
`, headerAlign)

		for i, entry := range entries {
			if i > 0 {
				// Bullet separator between contact items
				fmt.Fprintf(sb, `      <w:r>
        <w:rPr>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">  •  </w:t>
      </w:r>
`, smallSz, smallSz, mutedColor)
			}

			if entry.url != "" && IsSafeURL(entry.url) {
				relID := b.rels.addExternal(entry.url)
				fmt.Fprintf(sb, `      <w:hyperlink r:id="%s" w:history="1">
        <w:r>
          <w:rPr>
            <w:rStyle w:val="Hyperlink"/>
            <w:sz w:val="%d"/>
            <w:szCs w:val="%d"/>
            <w:color w:val="%s"/>
            <w:u w:val="single"/>
          </w:rPr>
          <w:t xml:space="preserve">%s</w:t>
        </w:r>
      </w:hyperlink>
`, relID, smallSz, smallSz, accentColor, xmlEscape(entry.label))
			} else {
				fmt.Fprintf(sb, `      <w:r>
        <w:rPr>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
`, smallSz, smallSz, secColor, xmlEscape(entry.label))
			}
		}

		sb.WriteString("    </w:p>\n")
	}

	// 4. Header Divider
	if t.Header.ShowDivider {
		headerGapTwips := parseTwips(t.Spacing.HeaderGap, 120)
		fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:pBdr>
          <w:bottom w:val="single" w:sz="6" w:space="1" w:color="%s"/>
        </w:pBdr>
        <w:spacing w:before="40" w:after="%d" w:line="240" w:lineRule="auto"/>
      </w:pPr>
    </w:p>
`, borderColor, headerGapTwips)
	}
}

func (b *docxBuilder) renderSectionHeader(sb *bytes.Buffer, title string) {
	t := b.theme
	if t.Sections.Uppercase {
		title = strings.ToUpper(title)
	}

	primaryColor := cleanHex(t.Colors.Primary)
	borderColor := cleanHex(t.Colors.Border)
	secSz := parseHalfPoints(t.Typography.FontSizeSection, 23)
	secGapTwips := parseTwips(t.Spacing.SectionGap, 220)

	sb.WriteString("    <w:p>\n      <w:pPr>\n        <w:pStyle w:val=\"Heading1\"/>\n        <w:keepNext/>\n        <w:keepLines/>\n")
	fmt.Fprintf(sb, "        <w:spacing w:before=\"%d\" w:after=\"60\" w:line=\"240\" w:lineRule=\"auto\"/>\n", secGapTwips)

	if t.Sections.BorderBottom {
		fmt.Fprintf(sb, `        <w:pBdr>
          <w:bottom w:val="single" w:sz="6" w:space="4" w:color="%s"/>
        </w:pBdr>
`, borderColor)
	}

	fmt.Fprintf(sb, `      </w:pPr>
      <w:r>
        <w:rPr>
          <w:b/>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
    </w:p>
`, secSz, secSz, primaryColor, xmlEscape(title))
}

func (b *docxBuilder) renderSummary(sb *bytes.Buffer, s *SummarySection) {
	title := s.Title
	if title == "" {
		title = "Summary"
	}
	b.renderSectionHeader(sb, title)

	lineTwips := int(b.theme.Typography.LineHeight * 240.0)
	sb.WriteString("    <w:p>\n      <w:pPr>\n")
	fmt.Fprintf(sb, "        <w:spacing w:before=\"40\" w:after=\"80\" w:line=\"%d\" w:lineRule=\"auto\"/>\n", lineTwips)
	sb.WriteString("      </w:pPr>\n")

	if len(s.Runs) > 0 {
		b.renderInlineRuns(sb, s.Runs)
	} else if s.Text != "" {
		b.renderInlineRuns(sb, ParseInlineRuns(s.Text))
	}

	sb.WriteString("    </w:p>\n")
}

func (b *docxBuilder) renderExperience(sb *bytes.Buffer, items []ExperienceItem) {
	b.renderSectionHeader(sb, "Experience")

	for _, item := range items {
		meta := formatMeta(item.Duration, item.Location)
		b.renderSplitHeader(sb, item.RoleTitle, item.Organization, meta)

		if len(item.Highlights) > 0 {
			b.renderBullets(sb, item.Highlights)
		}
	}
}

func (b *docxBuilder) renderEducation(sb *bytes.Buffer, items []EducationItem) {
	b.renderSectionHeader(sb, "Education")

	for _, item := range items {
		degreeTitle := item.Degree
		if item.FieldOfStudy != "" {
			degreeTitle = fmt.Sprintf("%s in %s", item.Degree, item.FieldOfStudy)
		}

		meta := formatMeta(item.Duration, item.Location)
		b.renderSplitHeader(sb, degreeTitle, item.Institution, meta)

		if item.Honors != "" {
			b.renderSubline(sb, item.Honors)
		}

		if len(item.Highlights) > 0 {
			b.renderBullets(sb, item.Highlights)
		}
	}
}

func (b *docxBuilder) renderSkills(sb *bytes.Buffer, skills []SkillCategory) {
	b.renderSectionHeader(sb, "Skills")

	primaryColor := cleanHex(b.theme.Colors.Primary)
	secColor := cleanHex(b.theme.Colors.Secondary)
	baseSz := parseHalfPoints(b.theme.Typography.FontSizeBase, 20)
	lineTwips := int(b.theme.Typography.LineHeight * 240.0)

	for _, cat := range skills {
		fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:spacing w:before="30" w:after="30" w:line="%d" w:lineRule="auto"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:b/>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s: </w:t>
      </w:r>
      <w:r>
        <w:rPr>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
    </w:p>
`, lineTwips, baseSz, baseSz, primaryColor, xmlEscape(cat.Name),
			baseSz, baseSz, secColor, xmlEscape(strings.Join(cat.Skills, ", ")))
	}
}

func (b *docxBuilder) renderProjects(sb *bytes.Buffer, projects []ProjectItem) {
	b.renderSectionHeader(sb, "Projects")

	for _, proj := range projects {
		title := proj.Name
		if proj.Role != "" {
			title = fmt.Sprintf("%s (%s)", proj.Name, proj.Role)
		}

		b.renderSplitHeader(sb, title, "", proj.Duration)

		if proj.URL != "" {
			disp := cleanDisplayURL(proj.URL)
			b.renderProjectLink(sb, proj.URL, disp)
		}

		if len(proj.Technologies) > 0 {
			tech := "Technologies: " + strings.Join(proj.Technologies, ", ")
			b.renderSubline(sb, tech)
		}

		if len(proj.Highlights) > 0 {
			b.renderBullets(sb, proj.Highlights)
		}
	}
}

func (b *docxBuilder) renderCertifications(sb *bytes.Buffer, certs []string) {
	b.renderSectionHeader(sb, "Certifications")

	var bullets []Bullet
	for _, cert := range certs {
		bullets = append(bullets, Bullet{Text: cert})
	}
	b.renderBullets(sb, bullets)
}

func (b *docxBuilder) renderCustomSections(sb *bytes.Buffer, sections []CustomSection) {
	for _, sec := range sections {
		b.renderSectionHeader(sb, sec.Title)
		if len(sec.Highlights) > 0 {
			b.renderBullets(sb, sec.Highlights)
		}
	}
}

// renderSplitHeader outputs an ATS-friendly single-paragraph split header with right-aligned tab-stop.
// Architectural Invariant: Emits w:tabs and w:tab instead of <w:tbl> table elements.
func (b *docxBuilder) renderSplitHeader(sb *bytes.Buffer, leftTitle, leftSubtitle, rightMeta string) {
	t := b.theme
	primaryColor := cleanHex(t.Colors.Primary)
	secColor := cleanHex(t.Colors.Secondary)
	mutedColor := cleanHex(t.Colors.Muted)

	subSz := parseHalfPoints(t.Typography.FontSizeSubtitle, 21)
	smallSz := parseHalfPoints(t.Typography.FontSizeSmall, 18)
	itemGapTwips := parseTwips(t.Spacing.ItemGap, 120)

	fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:pStyle w:val="JobTitle"/>
        <w:keepNext/>
        <w:keepLines/>
        <w:tabs>
          <w:tab w:val="right" w:pos="%d"/>
        </w:tabs>
        <w:spacing w:before="%d" w:after="20" w:line="240" w:lineRule="auto"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:b/>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
`, b.contentW, itemGapTwips, subSz, subSz, primaryColor, xmlEscape(leftTitle))

	if leftSubtitle != "" {
		fmt.Fprintf(sb, `      <w:r>
        <w:rPr>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve"> — %s</w:t>
      </w:r>
`, subSz, subSz, secColor, xmlEscape(leftSubtitle))
	}

	if rightMeta != "" {
		fmt.Fprintf(sb, `      <w:r>
        <w:tab/>
      </w:r>
      <w:r>
        <w:rPr>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
`, smallSz, smallSz, mutedColor, xmlEscape(rightMeta))
	}

	sb.WriteString("    </w:p>\n")
}

func (b *docxBuilder) renderSubline(sb *bytes.Buffer, text string) {
	secColor := cleanHex(b.theme.Colors.Secondary)
	smallSz := parseHalfPoints(b.theme.Typography.FontSizeSmall, 18)

	fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:keepNext/>
        <w:keepLines/>
        <w:spacing w:before="0" w:after="30" w:line="240" w:lineRule="auto"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:i/>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
    </w:p>
`, smallSz, smallSz, secColor, xmlEscape(text))
}

func (b *docxBuilder) renderProjectLink(sb *bytes.Buffer, rawURL, display string) {
	accentColor := cleanHex(b.theme.Colors.Accent)
	secColor := cleanHex(b.theme.Colors.Secondary)
	smallSz := parseHalfPoints(b.theme.Typography.FontSizeSmall, 18)

	if rawURL != "" && IsSafeURL(rawURL) {
		relID := b.rels.addExternal(rawURL)
		fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:keepNext/>
        <w:keepLines/>
        <w:spacing w:before="0" w:after="30" w:line="240" w:lineRule="auto"/>
      </w:pPr>
      <w:hyperlink r:id="%s" w:history="1">
        <w:r>
          <w:rPr>
            <w:rStyle w:val="Hyperlink"/>
            <w:sz w:val="%d"/>
            <w:szCs w:val="%d"/>
            <w:color w:val="%s"/>
            <w:u w:val="single"/>
          </w:rPr>
          <w:t xml:space="preserve">[%s]</w:t>
        </w:r>
      </w:hyperlink>
    </w:p>
`, relID, smallSz, smallSz, accentColor, xmlEscape(display))
	} else if display != "" {
		fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:keepNext/>
        <w:keepLines/>
        <w:spacing w:before="0" w:after="30" w:line="240" w:lineRule="auto"/>
      </w:pPr>
      <w:r>
        <w:rPr>
          <w:sz w:val="%d"/>
          <w:szCs w:val="%d"/>
          <w:color w:val="%s"/>
        </w:rPr>
        <w:t xml:space="preserve">[%s]</w:t>
      </w:r>
    </w:p>
`, smallSz, smallSz, secColor, xmlEscape(display))
	}
}

func (b *docxBuilder) renderBullets(sb *bytes.Buffer, bullets []Bullet) {
	bulletGapTwips := parseTwips(b.theme.Spacing.BulletGap, 50)
	lineTwips := int(b.theme.Typography.LineHeight * 240.0)

	for _, bullet := range bullets {
		fmt.Fprintf(sb, `    <w:p>
      <w:pPr>
        <w:pStyle w:val="ListBullet"/>
        <w:numPr>
          <w:ilvl w:val="0"/>
          <w:numId w:val="1"/>
        </w:numPr>
        <w:ind w:left="360" w:hanging="240"/>
        <w:spacing w:before="15" w:after="%d" w:line="%d" w:lineRule="auto"/>
      </w:pPr>
`, bulletGapTwips, lineTwips)

		if len(bullet.Runs) > 0 {
			b.renderInlineRuns(sb, bullet.Runs)
		} else {
			b.renderInlineRuns(sb, ParseInlineRuns(bullet.Text))
		}

		sb.WriteString("    </w:p>\n")
	}
}

func (b *docxBuilder) renderInlineRuns(sb *bytes.Buffer, runs []InlineRun) {
	primaryFont := b.theme.Typography.FontFamilyPrimary
	monoFont := b.theme.Typography.FontFamilyMono
	primaryColor := cleanHex(b.theme.Colors.Primary)
	accentColor := cleanHex(b.theme.Colors.Accent)
	baseSz := parseHalfPoints(b.theme.Typography.FontSizeBase, 20)

	for _, run := range runs {
		text := SanitizeRunText(run.Text)
		if text == "" {
			continue
		}

		escaped := xmlEscape(text)
		fontName := primaryFont
		colorVal := primaryColor

		if run.Code {
			fontName = monoFont
			colorVal = accentColor
		}

		var rPr bytes.Buffer
		rPr.WriteString("<w:rPr>")

		// Font
		escapedFont := xmlEscape(fontName)
		fmt.Fprintf(&rPr, `<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>`, escapedFont, escapedFont, escapedFont)

		if run.Bold {
			rPr.WriteString("<w:b/>")
		}
		if run.Italic {
			rPr.WriteString("<w:i/>")
		}
		if run.Link != "" {
			colorVal = accentColor
			rPr.WriteString(`<w:u w:val="single"/>`)
		}

		fmt.Fprintf(&rPr, `<w:color w:val="%s"/>`, colorVal)
		fmt.Fprintf(&rPr, `<w:sz w:val="%d"/><w:szCs w:val="%d"/>`, baseSz, baseSz)
		rPr.WriteString("</w:rPr>")

		if run.Link != "" && IsSafeURL(run.Link) {
			relID := b.rels.addExternal(run.Link)
			fmt.Fprintf(sb, `      <w:hyperlink r:id="%s" w:history="1">
        <w:r>
          %s
          <w:t xml:space="preserve">%s</w:t>
        </w:r>
      </w:hyperlink>
`, relID, rPr.String(), escaped)
		} else {
			fmt.Fprintf(sb, `      <w:r>
        %s
        <w:t xml:space="preserve">%s</w:t>
      </w:r>
`, rPr.String(), escaped)
		}
	}
}

// relTracker tracks external relationships for word/_rels/document.xml.rels.
type relTracker struct {
	nextID int
	items  map[string]string // url -> rId
	list   []struct{ id, target string }
}

func newRelTracker() *relTracker {
	return &relTracker{
		nextID: 3, // rId1 is styles, rId2 is numbering
		items:  make(map[string]string),
	}
}

func (r *relTracker) addExternal(target string) string {
	if id, exists := r.items[target]; exists {
		return id
	}
	id := fmt.Sprintf("rId%d", r.nextID)
	r.nextID++
	r.items[target] = id
	r.list = append(r.list, struct{ id, target string }{id: id, target: target})
	return id
}

// XML Templates

func (b *docxBuilder) contentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>
`
}

func (b *docxBuilder) packageRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>
`
}

func (b *docxBuilder) documentRelsXML() string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + "\n")
	sb.WriteString(`  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` + "\n")
	sb.WriteString(`  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>` + "\n")

	for _, item := range b.rels.list {
		fmt.Fprintf(&sb, `  <Relationship Id="%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="%s" TargetMode="External"/>`+"\n",
			item.id, xmlEscape(item.target))
	}

	sb.WriteString("</Relationships>\n")
	return sb.String()
}

func (b *docxBuilder) corePropsXML(doc *ResumeDocument) string {
	title := doc.Header.FullName
	if doc.Metadata.TargetRole != "" {
		title = fmt.Sprintf("%s - %s", doc.Header.FullName, doc.Metadata.TargetRole)
	}

	created := doc.Metadata.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	updated := doc.Metadata.UpdatedAt
	if updated.IsZero() {
		updated = time.Now().UTC()
	}

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title>%s</dc:title>
  <dc:creator>ResumAI</dc:creator>
  <cp:lastModifiedBy>ResumAI</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">%s</dcterms:modified>
</cp:coreProperties>
`, xmlEscape(title), created.Format(time.RFC3339), updated.Format(time.RFC3339))
}

func (b *docxBuilder) appPropsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>ResumAI Typesetter</Application>
  <DocSecurity>0</DocSecurity>
  <Lines>1</Lines>
  <Paragraphs>1</Paragraphs>
  <ScaleCrop>false</ScaleCrop>
  <LinksUpToDate>false</LinksUpToDate>
  <SharedDoc>false</SharedDoc>
  <HyperlinksChanged>false</HyperlinksChanged>
  <AppVersion>16.0000</AppVersion>
</Properties>
`
}

func (b *docxBuilder) numberingXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="0">
    <w:multiLevelType w:val="hybridMultilevel"/>
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:numFmt w:val="bullet"/>
      <w:lvlText w:val="&#x2022;"/>
      <w:lvlJc w:val="left"/>
      <w:pPr>
        <w:ind w:left="360" w:hanging="240"/>
      </w:pPr>
      <w:rPr>
        <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:hint="default"/>
      </w:rPr>
    </w:lvl>
  </w:abstractNum>
  <w:num w:numId="1">
    <w:abstractNumId w:val="0"/>
  </w:num>
</w:numbering>
`
}

func (b *docxBuilder) stylesXML() string {
	t := b.theme
	primaryFont := xmlEscape(t.Typography.FontFamilyPrimary)
	primaryColor := cleanHex(t.Colors.Primary)
	accentColor := cleanHex(t.Colors.Accent)
	baseSz := parseHalfPoints(t.Typography.FontSizeBase, 20)
	secSz := parseHalfPoints(t.Typography.FontSizeSection, 23)
	subSz := parseHalfPoints(t.Typography.FontSizeSubtitle, 21)
	lineTwips := int(t.Typography.LineHeight * 240.0)

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>
        <w:sz w:val="%d"/>
        <w:szCs w:val="%d"/>
        <w:color w:val="%s"/>
        <w:lang w:val="en-US"/>
      </w:rPr>
    </w:rPrDefault>
    <w:pPrDefault>
      <w:pPr>
        <w:spacing w:line="%d" w:lineRule="auto"/>
      </w:pPr>
    </w:pPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:qFormat/>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:next w:val="Normal"/>
    <w:qFormat/>
    <w:rPr>
      <w:b/>
      <w:sz w:val="%d"/>
      <w:szCs w:val="%d"/>
      <w:color w:val="%s"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="JobTitle">
    <w:name w:val="Job Title"/>
    <w:basedOn w:val="Normal"/>
    <w:rPr>
      <w:b/>
      <w:sz w:val="%d"/>
      <w:szCs w:val="%d"/>
      <w:color w:val="%s"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListBullet">
    <w:name w:val="List Bullet"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr>
      <w:numPr>
        <w:ilvl w:val="0"/>
        <w:numId w:val="1"/>
      </w:numPr>
      <w:ind w:left="360" w:hanging="240"/>
    </w:pPr>
  </w:style>
  <w:style w:type="character" w:styleId="Hyperlink">
    <w:name w:val="Hyperlink"/>
    <w:rPr>
      <w:color w:val="%s"/>
      <w:u w:val="single"/>
    </w:rPr>
  </w:style>
</w:styles>
`, primaryFont, primaryFont, primaryFont, baseSz, baseSz, primaryColor, lineTwips,
		secSz, secSz, primaryColor,
		subSz, subSz, primaryColor,
		accentColor)
}

// Helpers

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func cleanHex(c string) string {
	c = strings.TrimPrefix(strings.TrimSpace(c), "#")
	if len(c) == 3 {
		return strings.ToUpper(fmt.Sprintf("%c%c%c%c%c%c", c[0], c[0], c[1], c[1], c[2], c[2]))
	}
	if len(c) >= 6 {
		return strings.ToUpper(c[:6])
	}
	return "000000"
}

func parseTwips(dim string, defaultTwips int) int {
	dim = strings.TrimSpace(strings.ToLower(dim))
	if dim == "" {
		return defaultTwips
	}
	if strings.HasSuffix(dim, "in") {
		valStr := strings.TrimSuffix(dim, "in")
		if v, err := strconv.ParseFloat(valStr, 64); err == nil {
			return int(v * 1440.0)
		}
	}
	if strings.HasSuffix(dim, "pt") {
		valStr := strings.TrimSuffix(dim, "pt")
		if v, err := strconv.ParseFloat(valStr, 64); err == nil {
			return int(v * 20.0)
		}
	}
	if strings.HasSuffix(dim, "mm") {
		valStr := strings.TrimSuffix(dim, "mm")
		if v, err := strconv.ParseFloat(valStr, 64); err == nil {
			return int(v * 1440.0 / 25.4)
		}
	}
	if strings.HasSuffix(dim, "cm") {
		valStr := strings.TrimSuffix(dim, "cm")
		if v, err := strconv.ParseFloat(valStr, 64); err == nil {
			return int(v * 1440.0 / 2.54)
		}
	}
	if strings.HasSuffix(dim, "px") {
		valStr := strings.TrimSuffix(dim, "px")
		if v, err := strconv.ParseFloat(valStr, 64); err == nil {
			return int(v * 15.0)
		}
	}
	return defaultTwips
}

func parseHalfPoints(sizeStr string, defaultHalfPoints int) int {
	sizeStr = strings.TrimSpace(strings.ToLower(sizeStr))
	if sizeStr == "" {
		return defaultHalfPoints
	}
	if strings.HasSuffix(sizeStr, "pt") {
		valStr := strings.TrimSuffix(sizeStr, "pt")
		if v, err := strconv.ParseFloat(valStr, 64); err == nil {
			return int(v * 2.0)
		}
	}
	return defaultHalfPoints
}
