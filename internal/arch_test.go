package internal_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHexagonalBoundaryPurity verifies that package imports strictly obey Hexagonal Architecture:
// 1. domain must never import vault, render, mcp, or cmd.
// 2. vault and render must never import mcp or cmd.
func TestHexagonalBoundaryPurity(t *testing.T) {
	// Root of internal packages is the parent of current directory if run in internal,
	// or "." if running in internal.
	internalDir := "."
	if _, err := os.Stat("domain"); err != nil {
		// Try finding internal relative to working dir
		internalDir = "internal"
	}

	fset := token.NewFileSet()

	forbiddenForDomain := []string{
		"github.com/kidthales/resumai/internal/vault",
		"github.com/kidthales/resumai/internal/render",
		"github.com/kidthales/resumai/internal/mcp",
		"github.com/kidthales/resumai/cmd",
	}

	forbiddenForAdapters := []string{
		"github.com/kidthales/resumai/internal/mcp",
		"github.com/kidthales/resumai/cmd",
	}

	err := filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Errorf("failed to parse %s: %v", path, parseErr)
			return nil
		}

		normalizedPath := filepath.ToSlash(path)

		isDomain := strings.Contains(normalizedPath, "/domain/") || strings.HasPrefix(normalizedPath, "domain/")
		isVault := strings.Contains(normalizedPath, "/vault/") || strings.HasPrefix(normalizedPath, "vault/")
		isRender := strings.Contains(normalizedPath, "/render/") || strings.HasPrefix(normalizedPath, "render/")

		for _, imp := range node.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			if isDomain {
				for _, forbidden := range forbiddenForDomain {
					if strings.HasPrefix(importPath, forbidden) {
						t.Errorf("boundary violation in domain (%s): illegal import %q", path, importPath)
					}
				}
			}

			if isVault || isRender {
				for _, forbidden := range forbiddenForAdapters {
					if strings.HasPrefix(importPath, forbidden) {
						t.Errorf("boundary violation in adapter (%s): illegal import %q", path, importPath)
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk internal directory: %v", err)
	}
}
