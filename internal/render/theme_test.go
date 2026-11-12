package render_test

import (
	"bytes"
	"testing"

	"github.com/kidthales/resumai/internal/render"
)

func TestEmbeddedThemes(t *testing.T) {
	themes, err := render.ListEmbeddedThemes()
	if err != nil {
		t.Fatalf("failed to list embedded themes: %v", err)
	}

	if len(themes) < 3 {
		t.Errorf("expected at least 3 embedded themes, got %d (%v)", len(themes), themes)
	}

	expectedThemes := []string{"default", "modern", "classic"}
	for _, expected := range expectedThemes {
		found := false
		for _, name := range themes {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected theme %q not found in %v", expected, themes)
		}

		theme, err := render.LoadEmbeddedTheme(expected)
		if err != nil {
			t.Errorf("failed to load embedded theme %q: %v", expected, err)
			continue
		}
		if theme.Name != expected {
			t.Errorf("expected theme name %q, got %q", expected, theme.Name)
		}
		if err := theme.Validate(); err != nil {
			t.Errorf("theme %q validation failed: %v", expected, err)
		}
	}
}

func TestDefaultThemeManifest(t *testing.T) {
	theme := render.DefaultThemeManifest()
	if theme == nil {
		t.Fatal("DefaultThemeManifest returned nil")
	}
	if theme.Name != "default" {
		t.Errorf("expected default theme name 'default', got %q", theme.Name)
	}
	if err := theme.Validate(); err != nil {
		t.Errorf("default theme validation failed: %v", err)
	}
}

func TestThemeManifest_Validate_Errors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(m *render.ThemeManifest)
		wantErr string
	}{
		{
			name: "empty name",
			mutate: func(m *render.ThemeManifest) {
				m.Name = ""
			},
			wantErr: "theme manifest name is required",
		},
		{
			name: "unsupported page size",
			mutate: func(m *render.ThemeManifest) {
				m.Page.Size = "tabloid"
			},
			wantErr: "unsupported page size",
		},
		{
			name: "invalid page margin",
			mutate: func(m *render.ThemeManifest) {
				m.Page.MarginTop = "invalid_margin"
			},
			wantErr: "invalid dimension for page.margin_top",
		},
		{
			name: "missing primary font",
			mutate: func(m *render.ThemeManifest) {
				m.Typography.FontFamilyPrimary = "   "
			},
			wantErr: "typography.font_family_primary is required",
		},
		{
			name: "missing mono font",
			mutate: func(m *render.ThemeManifest) {
				m.Typography.FontFamilyMono = ""
			},
			wantErr: "typography.font_family_mono is required",
		},
		{
			name: "invalid line height",
			mutate: func(m *render.ThemeManifest) {
				m.Typography.LineHeight = 0
			},
			wantErr: "invalid typography.line_height",
		},
		{
			name: "invalid font size",
			mutate: func(m *render.ThemeManifest) {
				m.Typography.FontSizeBase = "10furlongs"
			},
			wantErr: "invalid dimension for typography.font_size_base",
		},
		{
			name: "invalid hex color",
			mutate: func(m *render.ThemeManifest) {
				m.Colors.Primary = "blue"
			},
			wantErr: "invalid hex color for colors.primary",
		},
		{
			name: "invalid spacing",
			mutate: func(m *render.ThemeManifest) {
				m.Spacing.SectionGap = "-5pt"
			},
			wantErr: "invalid dimension for spacing.section_gap",
		},
		{
			name: "invalid header layout",
			mutate: func(m *render.ThemeManifest) {
				m.Header.Layout = "diagonal"
			},
			wantErr: "invalid header layout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := render.DefaultThemeManifest()
			tt.mutate(m)
			err := m.Validate()
			if err == nil {
				t.Fatalf("expected validation error containing %q, got nil", tt.wantErr)
			}
			if !bytes.Contains([]byte(err.Error()), []byte(tt.wantErr)) {
				t.Errorf("expected error %q to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadThemeManifest_InvalidYAML(t *testing.T) {
	_, err := render.LoadThemeManifest([]byte("invalid: yaml: : ["))
	if err == nil {
		t.Fatal("expected error parsing invalid YAML, got nil")
	}

	_, err = render.LoadThemeManifest([]byte{})
	if err == nil {
		t.Fatal("expected error parsing empty YAML, got nil")
	}
}

func TestEmbeddedFonts(t *testing.T) {
	fonts := []string{
		render.FontInterRegular,
		render.FontInterBold,
		render.FontJetBrainsMonoReg,
	}

	woff2Magic := []byte("wOF2")

	for _, fontName := range fonts {
		data, err := render.GetFont(fontName)
		if err != nil {
			t.Fatalf("failed to read embedded font %q: %v", fontName, err)
		}
		if len(data) < 1000 {
			t.Errorf("font data %q unexpectedly small: %d bytes", fontName, len(data))
		}
		if !bytes.HasPrefix(data, woff2Magic) {
			t.Errorf("font %q does not start with WOFF2 magic header 'wOF2'", fontName)
		}
	}

	// Non-existent font
	_, err := render.GetFont("ComicSans.woff2")
	if err == nil {
		t.Error("expected error loading non-existent font, got nil")
	}
}
