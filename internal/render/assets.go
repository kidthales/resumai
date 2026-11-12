package render

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed assets/fonts/*.woff2 assets/themes/*.yaml
var assetsFS embed.FS

// Known font asset file names embedded offline.
const (
	FontInterRegular    = "Inter-Regular.woff2"
	FontInterBold       = "Inter-Bold.woff2"
	FontJetBrainsMonoReg = "JetBrainsMono-Regular.woff2"
)

// GetFont loads an embedded offline font by filename.
func GetFont(filename string) ([]byte, error) {
	cleanName := path.Base(filename)
	filePath := path.Join("assets", "fonts", cleanName)
	data, err := assetsFS.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded font %q: %w", cleanName, err)
	}
	return data, nil
}

// GetThemeYAML loads raw YAML bytes for an embedded theme by name.
func GetThemeYAML(name string) ([]byte, error) {
	cleanName := strings.TrimSuffix(path.Base(name), ".yaml")
	filePath := path.Join("assets", "themes", cleanName+".yaml")
	data, err := assetsFS.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("embedded theme %q not found: %w", cleanName, err)
	}
	return data, nil
}

// ListEmbeddedThemes returns the names of all embedded themes available.
func ListEmbeddedThemes() ([]string, error) {
	entries, err := fs.ReadDir(assetsFS, "assets/themes")
	if err != nil {
		return nil, fmt.Errorf("failed to list embedded themes: %w", err)
	}

	var themes []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			themes = append(themes, strings.TrimSuffix(entry.Name(), ".yaml"))
		}
	}
	return themes, nil
}
