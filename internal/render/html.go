package render

import (
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"strings"

	"github.com/kidthales/resumai/internal/domain"
)

// HTMLRenderer generates self-contained, offline-embedded HTML5/CSS3 resumes.
type HTMLRenderer struct {
	theme            *ThemeManifest
	fontInterRegB64  string
	fontInterBoldB64 string
	fontJBMonoRegB64 string
}

// NewHTMLRenderer creates a new renderer configured with the given theme.
// If theme is nil, DefaultThemeManifest() is used.
func NewHTMLRenderer(theme *ThemeManifest) (*HTMLRenderer, error) {
	if theme == nil {
		theme = DefaultThemeManifest()
	}
	if err := theme.Validate(); err != nil {
		return nil, fmt.Errorf("invalid theme for HTMLRenderer: %w", err)
	}

	// Pre-load and base64-encode offline embedded fonts
	interReg, err := GetFont(FontInterRegular)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", FontInterRegular, err)
	}

	interBold, err := GetFont(FontInterBold)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", FontInterBold, err)
	}

	jbMonoReg, err := GetFont(FontJetBrainsMonoReg)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", FontJetBrainsMonoReg, err)
	}

	return &HTMLRenderer{
		theme:            theme,
		fontInterRegB64:  base64.StdEncoding.EncodeToString(interReg),
		fontInterBoldB64: base64.StdEncoding.EncodeToString(interBold),
		fontJBMonoRegB64: base64.StdEncoding.EncodeToString(jbMonoReg),
	}, nil
}

// Theme returns the renderer's active ThemeManifest.
func (r *HTMLRenderer) Theme() *ThemeManifest {
	return r.theme
}

// Render compiles a ResumeDocument AST into standalone, printable HTML5 bytes.
// PII is injected strictly into an isolated document clone at render time to preserve
// prompt air-gapping and AST immutability.
func (r *HTMLRenderer) Render(doc *ResumeDocument, pii *domain.PIIHeader) ([]byte, error) {
	if doc == nil {
		return nil, errors.New("cannot render nil ResumeDocument")
	}

	// 1. Deep clone the document to prevent mutating the original in memory
	cloned, err := doc.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone ResumeDocument for render: %w", err)
	}

	// 2. Inject air-gapped personal contact information strictly at render time
	if pii != nil {
		cloned.InjectPII(pii)
	}

	// 3. Render HTML
	var sb strings.Builder
	r.renderDocument(&sb, cloned)

	return []byte(sb.String()), nil
}

// renderDocument builds the full HTML5 page markup.
func (r *HTMLRenderer) renderDocument(sb *strings.Builder, doc *ResumeDocument) {
	sb.WriteString("<!DOCTYPE html>\n")
	sb.WriteString("<html lang=\"en\">\n")
	sb.WriteString("<head>\n")
	sb.WriteString("  <meta charset=\"utf-8\">\n")
	sb.WriteString("  <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	fmt.Fprintf(sb, "  <title>%s - Resume</title>\n", html.EscapeString(doc.Header.FullName))

	// Embedded CSS Stylesheet
	sb.WriteString("  <style>\n")
	r.renderCSS(sb)
	sb.WriteString("  </style>\n")
	sb.WriteString("</head>\n")
	sb.WriteString("<body>\n")
	sb.WriteString("  <div class=\"resume-page\">\n")

	// Header block
	r.renderHeader(sb, &doc.Header)

	// Sections in configured order
	order := r.theme.Sections.Order
	if len(order) == 0 {
		order = []string{"summary", "experience", "education", "skills", "projects", "certifications", "custom"}
	}

	for _, sec := range order {
		switch strings.ToLower(sec) {
		case "summary":
			if doc.Summary != nil && (doc.Summary.Text != "" || len(doc.Summary.Runs) > 0) {
				r.renderSummary(sb, doc.Summary)
			}
		case "experience":
			if len(doc.Experience) > 0 {
				r.renderExperience(sb, doc.Experience)
			}
		case "education":
			if len(doc.Education) > 0 {
				r.renderEducation(sb, doc.Education)
			}
		case "skills":
			if len(doc.Skills) > 0 {
				r.renderSkills(sb, doc.Skills)
			}
		case "projects":
			if len(doc.Projects) > 0 {
				r.renderProjects(sb, doc.Projects)
			}
		case "certifications":
			if len(doc.Certifications) > 0 {
				r.renderCertifications(sb, doc.Certifications)
			}
		case "custom":
			if len(doc.CustomSections) > 0 {
				r.renderCustomSections(sb, doc.CustomSections)
			}
		}
	}

	sb.WriteString("  </div>\n")
	sb.WriteString("</body>\n")
	sb.WriteString("</html>\n")
}

// renderCSS writes all print and screen styles with embedded base64 fonts.
func (r *HTMLRenderer) renderCSS(sb *strings.Builder) {
	t := r.theme

	// Print page size & margins
	fmt.Fprintf(sb, `    @page {
      size: %s;
      margin: %s %s %s %s;
    }
`, t.Page.Size, t.Page.MarginTop, t.Page.MarginRight, t.Page.MarginBottom, t.Page.MarginLeft)

	// Offline embedded @font-face declarations
	fmt.Fprintf(sb, `    @font-face {
      font-family: 'Inter';
      src: url('data:font/woff2;charset=utf-8;base64,%s') format('woff2');
      font-weight: 400;
      font-style: normal;
      font-display: block;
    }
    @font-face {
      font-family: 'Inter';
      src: url('data:font/woff2;charset=utf-8;base64,%s') format('woff2');
      font-weight: 700;
      font-style: normal;
      font-display: block;
    }
    @font-face {
      font-family: 'JetBrains Mono';
      src: url('data:font/woff2;charset=utf-8;base64,%s') format('woff2');
      font-weight: 400;
      font-style: normal;
      font-display: block;
    }
`, r.fontInterRegB64, r.fontInterBoldB64, r.fontJBMonoRegB64)

	// Reset & Core Layout
	fmt.Fprintf(sb, `    *, *::before, *::after {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
    }
    html, body {
      background-color: %s;
      color: %s;
      font-family: '%s', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
      font-size: %s;
      line-height: %.2f;
      -webkit-font-smoothing: antialiased;
      -moz-osx-font-smoothing: grayscale;
    }
    .resume-page {
      width: 100%%;
      max-width: 8.5in;
      margin: 0 auto;
      background: %s;
    }
`, t.Colors.Background, t.Colors.Primary, t.Typography.FontFamilyPrimary, t.Typography.FontSizeBase, t.Typography.LineHeight, t.Colors.Background)

	// Typography & Section Elements
	headerAlign := "left"
	if strings.ToLower(t.Header.Layout) == "centered" {
		headerAlign = "center"
	}

	fmt.Fprintf(sb, `    /* Header */
    .resume-header {
      margin-bottom: %s;
      text-align: %s;
    }
    .resume-header .name {
      font-size: %s;
      font-weight: 700;
      color: %s;
      letter-spacing: -0.02em;
      line-height: 1.15;
    }
    .resume-header .headline {
      font-size: %s;
      font-weight: 600;
      color: %s;
      margin-top: 2pt;
    }
    .resume-header .contact-bar {
      display: flex;
      flex-wrap: wrap;
      gap: 4pt 10pt;
      margin-top: 4pt;
      font-size: %s;
      color: %s;
`, t.Spacing.HeaderGap, headerAlign, t.Typography.FontSizeTitle, t.Colors.Primary, t.Typography.FontSizeSubtitle, t.Colors.Accent, t.Typography.FontSizeSmall, t.Colors.Secondary)

	if strings.ToLower(t.Header.Layout) == "centered" {
		sb.WriteString("      justify-content: center;\n")
	}

	fmt.Fprintf(sb, `    }
    .resume-header .contact-item {
      display: inline-flex;
      align-items: center;
      text-decoration: none;
      color: inherit;
    }
    .resume-header .contact-item a {
      color: %s;
      text-decoration: none;
    }
    .resume-header .contact-item a:hover {
      text-decoration: underline;
    }
    .header-divider {
      margin-top: 6pt;
      border: 0;
      border-top: 1px solid %s;
    }
`, t.Colors.Accent, t.Colors.Border)

	// Section Titles
	transform := "none"
	if t.Sections.Uppercase {
		transform = "uppercase"
	}
	borderStyle := "none"
	if t.Sections.BorderBottom {
		borderStyle = fmt.Sprintf("1px solid %s", t.Colors.Border)
	}

	fmt.Fprintf(sb, `    /* Sections */
    .resume-section {
      margin-top: %s;
    }
    .section-title {
      font-size: %s;
      font-weight: 700;
      color: %s;
      text-transform: %s;
      letter-spacing: 0.04em;
      border-bottom: %s;
      padding-bottom: 2pt;
      margin-bottom: %s;
    }
    .section-content {
      font-size: %s;
    }
`, t.Spacing.SectionGap, t.Typography.FontSizeSection, t.Colors.Primary, transform, borderStyle, t.Spacing.ItemGap, t.Typography.FontSizeBase)

	// Items & Tab-Stop Header Layout
	fmt.Fprintf(sb, `    /* Items & ATS Linear Headers */
    .resume-item {
      margin-bottom: %s;
    }
    .resume-item:last-child {
      margin-bottom: 0;
    }
    .item-header {
      display: flex;
      justify-content: space-between;
      align-items: baseline;
    }
    .item-title-block {
      font-weight: 700;
      color: %s;
    }
    .item-subtitle-block {
      font-size: %s;
      color: %s;
      font-weight: 400;
    }
    .item-meta-block {
      font-size: %s;
      color: %s;
      text-align: right;
      white-space: nowrap;
    }
`, t.Spacing.ItemGap, t.Colors.Primary, t.Typography.FontSizeSmall, t.Colors.Secondary, t.Typography.FontSizeSmall, t.Colors.Secondary)

	// Bullets & Inline Runs
	fmt.Fprintf(sb, `    /* Bullets */
    ul.bullet-list {
      list-style-type: disc;
      margin-left: 14pt;
      margin-top: 2pt;
    }
    li.resume-bullet {
      margin-bottom: %s;
      color: %s;
    }
    li.resume-bullet:last-child {
      margin-bottom: 0;
    }
    code {
      font-family: '%s', monospace;
      font-size: 0.9em;
      background: #f3f4f6;
      padding: 1px 3px;
      border-radius: 2px;
    }
    a {
      color: %s;
      text-decoration: underline;
    }
    strong {
      font-weight: 700;
      color: %s;
    }
`, t.Spacing.BulletGap, t.Colors.Primary, t.Typography.FontFamilyMono, t.Colors.Accent, t.Colors.Primary)

	// Skill categories
	fmt.Fprintf(sb, `    /* Skills */
    .skills-grid {
      display: flex;
      flex-direction: column;
      gap: 3pt;
    }
    .skill-row {
      display: flex;
      gap: 6pt;
    }
    .skill-category-name {
      font-weight: 700;
      min-width: 110pt;
      color: %s;
    }
    .skill-category-items {
      color: %s;
    }
`, t.Colors.Primary, t.Colors.Secondary)
}

// renderHeader generates the top contact and headline block.
func (r *HTMLRenderer) renderHeader(sb *strings.Builder, h *Header) {
	sb.WriteString("    <header class=\"resume-header\" data-section=\"header\">\n")
	fmt.Fprintf(sb, "      <div class=\"name\">%s</div>\n", html.EscapeString(h.FullName))

	if r.theme.Header.ShowHeadline && h.Headline != "" {
		fmt.Fprintf(sb, "      <div class=\"headline\">%s</div>\n", html.EscapeString(h.Headline))
	}

	// Contact row
	var contacts []string
	if h.Email != "" {
		contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\"><a href=\"mailto:%s\">%s</a></span>",
			html.EscapeString(h.Email), html.EscapeString(h.Email)))
	}
	if h.Phone != "" {
		contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\">%s</span>", html.EscapeString(h.Phone)))
	}
	if h.Location != "" {
		contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\">%s</span>", html.EscapeString(h.Location)))
	}
	if h.LinkedInURL != "" {
		label := cleanDisplayURL(h.LinkedInURL)
		if IsSafeURL(h.LinkedInURL) {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\"><a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">%s</a></span>",
				html.EscapeString(h.LinkedInURL), html.EscapeString(label)))
		} else {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\">%s</span>", html.EscapeString(label)))
		}
	}
	if h.GitHubURL != "" {
		label := cleanDisplayURL(h.GitHubURL)
		if IsSafeURL(h.GitHubURL) {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\"><a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">%s</a></span>",
				html.EscapeString(h.GitHubURL), html.EscapeString(label)))
		} else {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\">%s</span>", html.EscapeString(label)))
		}
	}
	if h.PortfolioURL != "" {
		label := cleanDisplayURL(h.PortfolioURL)
		if IsSafeURL(h.PortfolioURL) {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\"><a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">%s</a></span>",
				html.EscapeString(h.PortfolioURL), html.EscapeString(label)))
		} else {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\">%s</span>", html.EscapeString(label)))
		}
	}
	if h.WebsiteURL != "" {
		label := cleanDisplayURL(h.WebsiteURL)
		if IsSafeURL(h.WebsiteURL) {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\"><a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">%s</a></span>",
				html.EscapeString(h.WebsiteURL), html.EscapeString(label)))
		} else {
			contacts = append(contacts, fmt.Sprintf("<span class=\"contact-item\">%s</span>", html.EscapeString(label)))
		}
	}

	if len(contacts) > 0 {
		sb.WriteString("      <div class=\"contact-bar\">\n")
		for _, c := range contacts {
			fmt.Fprintf(sb, "        %s\n", c)
		}
		sb.WriteString("      </div>\n")
	}

	if r.theme.Header.ShowDivider {
		sb.WriteString("      <hr class=\"header-divider\">\n")
	}

	sb.WriteString("    </header>\n")
}

// renderSummary outputs the profile / summary section.
func (r *HTMLRenderer) renderSummary(sb *strings.Builder, s *SummarySection) {
	title := s.Title
	if title == "" {
		title = "Summary"
	}
	sb.WriteString("    <section class=\"resume-section\" data-section=\"summary\">\n")
	fmt.Fprintf(sb, "      <h2 class=\"section-title\">%s</h2>\n", html.EscapeString(title))
	sb.WriteString("      <div class=\"section-content\">\n")

	if len(s.Runs) > 0 {
		fmt.Fprintf(sb, "        <p>%s</p>\n", RenderInlineRunsHTML(s.Runs))
	} else if s.Text != "" {
		fmt.Fprintf(sb, "        <p>%s</p>\n", html.EscapeString(s.Text))
	}

	sb.WriteString("      </div>\n")
	sb.WriteString("    </section>\n")
}

// renderExperience outputs professional work history.
func (r *HTMLRenderer) renderExperience(sb *strings.Builder, items []ExperienceItem) {
	sb.WriteString("    <section class=\"resume-section\" data-section=\"experience\">\n")
	sb.WriteString("      <h2 class=\"section-title\">Experience</h2>\n")
	sb.WriteString("      <div class=\"section-content\">\n")

	for _, item := range items {
		fmt.Fprintf(sb, "        <article class=\"resume-item\" data-item-id=\"%s\">\n", html.EscapeString(item.ID))
		sb.WriteString("          <div class=\"item-header\">\n")
		fmt.Fprintf(sb, "            <div class=\"item-title-block\">%s <span class=\"item-subtitle-block\">— %s</span></div>\n",
			html.EscapeString(item.RoleTitle), html.EscapeString(item.Organization))

		meta := formatMeta(item.Duration, item.Location)
		if meta != "" {
			fmt.Fprintf(sb, "            <div class=\"item-meta-block\">%s</div>\n", html.EscapeString(meta))
		}
		sb.WriteString("          </div>\n")

		if len(item.Highlights) > 0 {
			r.renderBullets(sb, item.Highlights)
		}
		sb.WriteString("        </article>\n")
	}

	sb.WriteString("      </div>\n")
	sb.WriteString("    </section>\n")
}

// renderEducation outputs academic credentials.
func (r *HTMLRenderer) renderEducation(sb *strings.Builder, items []EducationItem) {
	sb.WriteString("    <section class=\"resume-section\" data-section=\"education\">\n")
	sb.WriteString("      <h2 class=\"section-title\">Education</h2>\n")
	sb.WriteString("      <div class=\"section-content\">\n")

	for _, item := range items {
		fmt.Fprintf(sb, "        <article class=\"resume-item\" data-item-id=\"%s\">\n", html.EscapeString(item.ID))
		sb.WriteString("          <div class=\"item-header\">\n")

		degreeTitle := item.Degree
		if item.FieldOfStudy != "" {
			degreeTitle = fmt.Sprintf("%s in %s", item.Degree, item.FieldOfStudy)
		}
		fmt.Fprintf(sb, "            <div class=\"item-title-block\">%s <span class=\"item-subtitle-block\">— %s</span></div>\n",
			html.EscapeString(degreeTitle), html.EscapeString(item.Institution))

		meta := formatMeta(item.Duration, item.Location)
		if meta != "" {
			fmt.Fprintf(sb, "            <div class=\"item-meta-block\">%s</div>\n", html.EscapeString(meta))
		}
		sb.WriteString("          </div>\n")

		if item.Honors != "" {
			fmt.Fprintf(sb, "          <div class=\"item-subtitle-block\">%s</div>\n", html.EscapeString(item.Honors))
		}

		if len(item.Highlights) > 0 {
			r.renderBullets(sb, item.Highlights)
		}
		sb.WriteString("        </article>\n")
	}

	sb.WriteString("      </div>\n")
	sb.WriteString("    </section>\n")
}

// renderSkills outputs technical and functional skill categories.
func (r *HTMLRenderer) renderSkills(sb *strings.Builder, skills []SkillCategory) {
	sb.WriteString("    <section class=\"resume-section\" data-section=\"skills\">\n")
	sb.WriteString("      <h2 class=\"section-title\">Skills</h2>\n")
	sb.WriteString("      <div class=\"section-content skills-grid\">\n")

	for _, cat := range skills {
		sb.WriteString("        <div class=\"skill-row\">\n")
		fmt.Fprintf(sb, "          <div class=\"skill-category-name\">%s:</div>\n", html.EscapeString(cat.Name))
		fmt.Fprintf(sb, "          <div class=\"skill-category-items\">%s</div>\n", html.EscapeString(strings.Join(cat.Skills, ", ")))
		sb.WriteString("        </div>\n")
	}

	sb.WriteString("      </div>\n")
	sb.WriteString("    </section>\n")
}

// renderProjects outputs project accomplishments.
func (r *HTMLRenderer) renderProjects(sb *strings.Builder, projects []ProjectItem) {
	sb.WriteString("    <section class=\"resume-section\" data-section=\"projects\">\n")
	sb.WriteString("      <h2 class=\"section-title\">Projects</h2>\n")
	sb.WriteString("      <div class=\"section-content\">\n")

	for _, proj := range projects {
		sb.WriteString("        <article class=\"resume-item\">\n")
		sb.WriteString("          <div class=\"item-header\">\n")

		title := proj.Name
		if proj.Role != "" {
			title = fmt.Sprintf("%s (%s)", proj.Name, proj.Role)
		}
		fmt.Fprintf(sb, "            <div class=\"item-title-block\">%s", html.EscapeString(title))
		if proj.URL != "" {
			disp := cleanDisplayURL(proj.URL)
			if IsSafeURL(proj.URL) {
				fmt.Fprintf(sb, " <span class=\"item-subtitle-block\"><a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">[%s]</a></span>",
					html.EscapeString(proj.URL), html.EscapeString(disp))
			} else {
				fmt.Fprintf(sb, " <span class=\"item-subtitle-block\">[%s]</span>", html.EscapeString(disp))
			}
		}
		sb.WriteString("</div>\n")

		if proj.Duration != "" {
			fmt.Fprintf(sb, "            <div class=\"item-meta-block\">%s</div>\n", html.EscapeString(proj.Duration))
		}
		sb.WriteString("          </div>\n")

		if len(proj.Technologies) > 0 {
			fmt.Fprintf(sb, "          <div class=\"item-subtitle-block\"><em>Technologies: %s</em></div>\n",
				html.EscapeString(strings.Join(proj.Technologies, ", ")))
		}

		if len(proj.Highlights) > 0 {
			r.renderBullets(sb, proj.Highlights)
		}
		sb.WriteString("        </article>\n")
	}

	sb.WriteString("      </div>\n")
	sb.WriteString("    </section>\n")
}

// renderCertifications outputs professional certifications.
func (r *HTMLRenderer) renderCertifications(sb *strings.Builder, certs []string) {
	sb.WriteString("    <section class=\"resume-section\" data-section=\"certifications\">\n")
	sb.WriteString("      <h2 class=\"section-title\">Certifications</h2>\n")
	sb.WriteString("      <div class=\"section-content\">\n")
	sb.WriteString("        <ul class=\"bullet-list\">\n")
	for _, cert := range certs {
		fmt.Fprintf(sb, "          <li class=\"resume-bullet\">%s</li>\n", html.EscapeString(cert))
	}
	sb.WriteString("        </ul>\n")
	sb.WriteString("      </div>\n")
	sb.WriteString("    </section>\n")
}

// renderCustomSections outputs any user-defined custom sections.
func (r *HTMLRenderer) renderCustomSections(sb *strings.Builder, sections []CustomSection) {
	for _, sec := range sections {
		sb.WriteString("    <section class=\"resume-section\" data-section=\"custom\">\n")
		fmt.Fprintf(sb, "      <h2 class=\"section-title\">%s</h2>\n", html.EscapeString(sec.Title))
		sb.WriteString("      <div class=\"section-content\">\n")
		if len(sec.Highlights) > 0 {
			r.renderBullets(sb, sec.Highlights)
		}
		sb.WriteString("      </div>\n")
		sb.WriteString("    </section>\n")
	}
}

// renderBullets writes accomplishment bullets with data-bullet-id attributes for layout telemetry.
func (r *HTMLRenderer) renderBullets(sb *strings.Builder, bullets []Bullet) {
	sb.WriteString("          <ul class=\"bullet-list\">\n")
	for _, b := range bullets {
		idAttr := ""
		if b.ID != "" {
			idAttr = fmt.Sprintf(" data-bullet-id=\"%s\"", html.EscapeString(b.ID))
		}

		content := ""
		if len(b.Runs) > 0 {
			content = RenderInlineRunsHTML(b.Runs)
		} else {
			content = html.EscapeString(b.Text)
		}

		fmt.Fprintf(sb, "            <li class=\"resume-bullet\"%s>%s</li>\n", idAttr, content)
	}
	sb.WriteString("          </ul>\n")
}

// RenderInlineRunsHTML transforms a slice of InlineRuns into escaped, styled HTML.
func RenderInlineRunsHTML(runs []InlineRun) string {
	if len(runs) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, run := range runs {
		escaped := html.EscapeString(run.Text)
		if escaped == "" {
			continue
		}

		inner := escaped
		if run.Code {
			inner = "<code>" + inner + "</code>"
		}
		if run.Bold && run.Italic {
			inner = "<strong><em>" + inner + "</em></strong>"
		} else if run.Bold {
			inner = "<strong>" + inner + "</strong>"
		} else if run.Italic {
			inner = "<em>" + inner + "</em>"
		}

		if run.Link != "" && IsSafeURL(run.Link) {
			inner = fmt.Sprintf("<a href=\"%s\" target=\"_blank\" rel=\"noopener noreferrer\">%s</a>",
				html.EscapeString(run.Link), inner)
		}

		sb.WriteString(inner)
	}
	return sb.String()
}

func formatMeta(duration, location string) string {
	if duration != "" && location != "" {
		return duration + " | " + location
	}
	if duration != "" {
		return duration
	}
	return location
}

func cleanDisplayURL(rawURL string) string {
	cleaned := strings.TrimPrefix(rawURL, "https://")
	cleaned = strings.TrimPrefix(cleaned, "http://")
	cleaned = strings.TrimPrefix(cleaned, "www.")
	cleaned = strings.TrimSuffix(cleaned, "/")
	return cleaned
}
