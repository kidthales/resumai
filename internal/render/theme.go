package render

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	dimensionRegex = regexp.MustCompile(`^(?:0|[0-9]+(?:\.[0-9]+)?(?:in|pt|mm|cm|px|rem|em|%))$`)
	hexColorRegex  = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
)

// PageConfig holds page geometry and print margin dimensions.
type PageConfig struct {
	Size         string `json:"size" yaml:"size"`                   // "letter" or "a4"
	MarginTop    string `json:"margin_top" yaml:"margin_top"`       // e.g. "0.5in"
	MarginBottom string `json:"margin_bottom" yaml:"margin_bottom"` // e.g. "0.5in"
	MarginLeft   string `json:"margin_left" yaml:"margin_left"`     // e.g. "0.5in"
	MarginRight  string `json:"margin_right" yaml:"margin_right"`   // e.g. "0.5in"
}

// TypographyConfig defines font stacks, sizes, and line height.
type TypographyConfig struct {
	FontFamilyPrimary string  `json:"font_family_primary" yaml:"font_family_primary"` // e.g. "Inter"
	FontFamilyMono    string  `json:"font_family_mono" yaml:"font_family_mono"`       // e.g. "JetBrains Mono"
	FontSizeBase      string  `json:"font_size_base" yaml:"font_size_base"`           // e.g. "10pt"
	FontSizeTitle     string  `json:"font_size_title" yaml:"font_size_title"`         // e.g. "18pt"
	FontSizeSubtitle  string  `json:"font_size_subtitle" yaml:"font_size_subtitle"`   // e.g. "10.5pt"
	FontSizeSection   string  `json:"font_size_section" yaml:"font_size_section"`     // e.g. "11.5pt"
	FontSizeSmall     string  `json:"font_size_small" yaml:"font_size_small"`         // e.g. "9pt"
	LineHeight        float64 `json:"line_height" yaml:"line_height"`                 // e.g. 1.35
}

// ColorsConfig defines accessible palette tokens for print and screen rendering.
type ColorsConfig struct {
	Primary    string `json:"primary" yaml:"primary"`       // e.g. "#111827"
	Secondary  string `json:"secondary" yaml:"secondary"`   // e.g. "#374151"
	Accent     string `json:"accent" yaml:"accent"`         // e.g. "#0f766e"
	Muted      string `json:"muted" yaml:"muted"`           // e.g. "#6b7280"
	Border     string `json:"border" yaml:"border"`         // e.g. "#d1d5db"
	Background string `json:"background" yaml:"background"` // e.g. "#ffffff"
}

// SpacingConfig controls vertical rhythm, section margins, and gap dimensions.
type SpacingConfig struct {
	SectionGap string `json:"section_gap" yaml:"section_gap"` // e.g. "11pt"
	ItemGap    string `json:"item_gap" yaml:"item_gap"`       // e.g. "6pt"
	BulletGap  string `json:"bullet_gap" yaml:"bullet_gap"`   // e.g. "2.5pt"
	HeaderGap  string `json:"header_gap" yaml:"header_gap"`   // e.g. "6pt"
}

// HeaderConfig controls header layout and visual elements.
type HeaderConfig struct {
	Layout       string `json:"layout" yaml:"layout"`               // "left", "centered", "split"
	ShowHeadline bool   `json:"show_headline" yaml:"show_headline"` // show target title/headline
	ShowDivider  bool   `json:"show_divider" yaml:"show_divider"`   // show horizontal rule
}

// SectionStyleConfig configures the presentation of resume sections.
type SectionStyleConfig struct {
	Uppercase    bool     `json:"uppercase" yaml:"uppercase"`         // e.g. true
	BorderBottom bool     `json:"border_bottom" yaml:"border_bottom"` // e.g. true
	Order        []string `json:"order,omitempty" yaml:"order,omitempty"`
}

// ThemeManifest is the root declarative specification for typeset resumes.
type ThemeManifest struct {
	Name        string             `json:"name" yaml:"name"`
	Version     string             `json:"version" yaml:"version"`
	Description string             `json:"description,omitempty" yaml:"description,omitempty"`
	Page        PageConfig         `json:"page" yaml:"page"`
	Typography  TypographyConfig   `json:"typography" yaml:"typography"`
	Colors      ColorsConfig       `json:"colors" yaml:"colors"`
	Spacing     SpacingConfig      `json:"spacing" yaml:"spacing"`
	Header      HeaderConfig       `json:"header" yaml:"header"`
	Sections    SectionStyleConfig `json:"sections" yaml:"sections"`
}

// LoadThemeManifest parses and validates a ThemeManifest from YAML bytes.
func LoadThemeManifest(data []byte) (*ThemeManifest, error) {
	if len(data) == 0 {
		return nil, errors.New("empty theme manifest data")
	}

	var theme ThemeManifest
	if err := yaml.Unmarshal(data, &theme); err != nil {
		return nil, fmt.Errorf("failed to parse theme manifest yaml: %w", err)
	}

	if err := theme.Validate(); err != nil {
		return nil, fmt.Errorf("invalid theme manifest %q: %w", theme.Name, err)
	}

	return &theme, nil
}

// LoadEmbeddedTheme loads and validates a built-in theme by name (e.g. "default", "modern", "classic").
func LoadEmbeddedTheme(name string) (*ThemeManifest, error) {
	if name == "" {
		name = "default"
	}

	data, err := GetThemeYAML(name)
	if err != nil {
		return nil, err
	}

	return LoadThemeManifest(data)
}

// DefaultThemeManifest returns the verified default theme manifest.
func DefaultThemeManifest() *ThemeManifest {
	theme, err := LoadEmbeddedTheme("default")
	if err != nil {
		// Fallback code defaults in case embedded FS is unavailable in test harness
		return &ThemeManifest{
			Name:        "default",
			Version:     "1.0.0",
			Description: "Professional ATS-optimized single-column executive theme",
			Page: PageConfig{
				Size:         "letter",
				MarginTop:    "0.5in",
				MarginBottom: "0.5in",
				MarginLeft:   "0.5in",
				MarginRight:  "0.5in",
			},
			Typography: TypographyConfig{
				FontFamilyPrimary: "Inter",
				FontFamilyMono:    "JetBrains Mono",
				FontSizeBase:      "10pt",
				FontSizeTitle:     "18pt",
				FontSizeSubtitle:  "10.5pt",
				FontSizeSection:   "11.5pt",
				FontSizeSmall:     "9pt",
				LineHeight:        1.35,
			},
			Colors: ColorsConfig{
				Primary:    "#111827",
				Secondary:  "#374151",
				Accent:     "#0f766e",
				Muted:      "#6b7280",
				Border:     "#d1d5db",
				Background: "#ffffff",
			},
			Spacing: SpacingConfig{
				SectionGap: "11pt",
				ItemGap:    "6pt",
				BulletGap:  "2.5pt",
				HeaderGap:  "6pt",
			},
			Header: HeaderConfig{
				Layout:       "left",
				ShowHeadline: true,
				ShowDivider:  true,
			},
			Sections: SectionStyleConfig{
				Uppercase:    true,
				BorderBottom: true,
				Order: []string{
					"summary",
					"experience",
					"education",
					"skills",
					"projects",
					"certifications",
					"custom",
				},
			},
		}
	}
	return theme
}

// Validate checks all manifest properties against geometric and typographical invariants.
func (m *ThemeManifest) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("theme manifest name is required")
	}

	// Validate page size
	switch strings.ToLower(m.Page.Size) {
	case "letter", "a4":
	default:
		return fmt.Errorf("unsupported page size %q (expected 'letter' or 'a4')", m.Page.Size)
	}

	// Validate page margins
	margins := map[string]string{
		"page.margin_top":    m.Page.MarginTop,
		"page.margin_bottom": m.Page.MarginBottom,
		"page.margin_left":   m.Page.MarginLeft,
		"page.margin_right":  m.Page.MarginRight,
	}
	for name, val := range margins {
		if !dimensionRegex.MatchString(val) {
			return fmt.Errorf("invalid dimension for %s: %q", name, val)
		}
	}

	// Validate typography
	if strings.TrimSpace(m.Typography.FontFamilyPrimary) == "" {
		return errors.New("typography.font_family_primary is required")
	}
	if strings.TrimSpace(m.Typography.FontFamilyMono) == "" {
		return errors.New("typography.font_family_mono is required")
	}
	if m.Typography.LineHeight <= 0 || m.Typography.LineHeight > 3.0 {
		return fmt.Errorf("invalid typography.line_height: %v (expected 0.5 - 3.0)", m.Typography.LineHeight)
	}

	fontSizes := map[string]string{
		"typography.font_size_base":     m.Typography.FontSizeBase,
		"typography.font_size_title":    m.Typography.FontSizeTitle,
		"typography.font_size_subtitle": m.Typography.FontSizeSubtitle,
		"typography.font_size_section":  m.Typography.FontSizeSection,
		"typography.font_size_small":    m.Typography.FontSizeSmall,
	}
	for name, val := range fontSizes {
		if !dimensionRegex.MatchString(val) {
			return fmt.Errorf("invalid dimension for %s: %q", name, val)
		}
	}

	// Validate colors
	colors := map[string]string{
		"colors.primary":    m.Colors.Primary,
		"colors.secondary":  m.Colors.Secondary,
		"colors.accent":     m.Colors.Accent,
		"colors.muted":      m.Colors.Muted,
		"colors.border":     m.Colors.Border,
		"colors.background": m.Colors.Background,
	}
	for name, val := range colors {
		if !hexColorRegex.MatchString(val) {
			return fmt.Errorf("invalid hex color for %s: %q", name, val)
		}
	}

	// Validate spacing
	spacings := map[string]string{
		"spacing.section_gap": m.Spacing.SectionGap,
		"spacing.item_gap":    m.Spacing.ItemGap,
		"spacing.bullet_gap":  m.Spacing.BulletGap,
		"spacing.header_gap":  m.Spacing.HeaderGap,
	}
	for name, val := range spacings {
		if !dimensionRegex.MatchString(val) {
			return fmt.Errorf("invalid dimension for %s: %q", name, val)
		}
	}

	// Validate header layout
	switch strings.ToLower(m.Header.Layout) {
	case "left", "centered", "split":
	default:
		return fmt.Errorf("invalid header layout %q (expected 'left', 'centered', or 'split')", m.Header.Layout)
	}

	return nil
}
