package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/vault"
)

// runImportLegacy handles the `resumai import-legacy <dir> [flags]` command.
func runImportLegacy(args []string, stdout, stderr io.Writer, globalVault string) int {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			printImportLegacyUsage(stdout)
			return 0
		}
	}

	dir, defaultState, skipPII, vaultPath, err := parseImportLegacyFlags(args, globalVault)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printImportLegacyUsage(stderr)
		return 1
	}

	cleanDir := filepath.Clean(dir)
	info, err := os.Stat(cleanDir)
	if err != nil {
		fmt.Fprintf(stderr, "error reading legacy directory %s: %v\n", cleanDir, err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(stderr, "error: %s is not a directory\n", cleanDir)
		return 1
	}

	targetVault := vaultPath
	if targetVault == "" {
		targetVault, err = vault.DefaultPath()
		if err != nil {
			fmt.Fprintf(stderr, "error resolving default vault path: %v\n", err)
			return 1
		}
	}

	v, err := vault.OpenPath(targetVault)
	if err != nil {
		fmt.Fprintf(stderr, "error opening vault (%s): %v\n", targetVault, err)
		return 1
	}
	defer v.Close()

	ctx := context.Background()
	report, err := v.ImportLegacy(ctx, cleanDir, vault.LegacyImportOptions{
		DefaultEpistemicState: defaultState,
		SkipPII:               skipPII,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error importing legacy data from %s: %v\n", cleanDir, err)
		return 1
	}

	fmt.Fprintf(stdout, "Legacy data imported successfully from %s:\n", cleanDir)
	if report.PIIImported {
		fmt.Fprintln(stdout, "  - Contact PII: imported")
	} else if skipPII {
		fmt.Fprintln(stdout, "  - Contact PII: skipped (--skip-pii)")
	} else {
		fmt.Fprintln(stdout, "  - Contact PII: not present")
	}

	if report.LogisticsImported {
		fmt.Fprintln(stdout, "  - Logistics: imported")
	} else {
		fmt.Fprintln(stdout, "  - Logistics: not present")
	}

	fmt.Fprintf(stdout, "  - Organizations: %d\n", report.Organizations)
	fmt.Fprintf(stdout, "  - Roles: %d\n", report.Roles)
	fmt.Fprintf(stdout, "  - Career Facts: %d (default state: %s)\n", report.Facts, defaultState)
	fmt.Fprintf(stdout, "  - Education: %d\n", report.Education)
	fmt.Fprintf(stdout, "  - Archetypes: %d\n", report.Archetypes)

	return 0
}

func printImportLegacyUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage: resumai import-legacy <dir> [flags]

Import v0.1.0 Markdown files into the career vault.
Parses personal_info.md, education_history.md, work_history.md, extras.md, and archetypes/*.md.

Flags:
  --default-state <state>  Default epistemic state for facts: verified or raw_note (default: verified)
  --skip-pii               Skip importing personal contact PII from personal_info.md
  --vault <path>           Path to vault SQLite database (default: ~/.resumai/vault.db)
`)
}

func parseImportLegacyFlags(args []string, defaultVault string) (dir string, defaultState domain.EpistemicState, skipPII bool, vaultPath string, err error) {
	fs := flag.NewFlagSet("import-legacy", flag.ContinueOnError)
	stateStr := fs.String("default-state", string(domain.EpistemicVerified), "Default epistemic state (verified, raw_note)")
	skipPIIPtr := fs.Bool("skip-pii", false, "Skip importing personal contact PII")
	vaultPtr := fs.String("vault", defaultVault, "Path to vault SQLite database")

	var nonFlags []string
	var flagsOnly []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--skip-pii" || arg == "-skip-pii" {
			flagsOnly = append(flagsOnly, arg)
		} else if arg == "--vault" || arg == "-vault" {
			flagsOnly = append(flagsOnly, arg)
			if i+1 < len(args) {
				flagsOnly = append(flagsOnly, args[i+1])
				i++
			}
		} else if strings.HasPrefix(arg, "--vault=") || strings.HasPrefix(arg, "-vault=") {
			flagsOnly = append(flagsOnly, arg)
		} else if arg == "--default-state" || arg == "-default-state" {
			flagsOnly = append(flagsOnly, arg)
			if i+1 < len(args) {
				flagsOnly = append(flagsOnly, args[i+1])
				i++
			}
		} else if strings.HasPrefix(arg, "--default-state=") || strings.HasPrefix(arg, "-default-state=") {
			flagsOnly = append(flagsOnly, arg)
		} else if strings.HasPrefix(arg, "-") {
			flagsOnly = append(flagsOnly, arg)
		} else {
			nonFlags = append(nonFlags, arg)
		}
	}

	if err := fs.Parse(flagsOnly); err != nil {
		return "", "", false, "", err
	}

	if len(nonFlags) == 0 {
		return "", "", false, "", errors.New("missing source directory argument")
	}
	if len(nonFlags) > 1 {
		return "", "", false, "", fmt.Errorf("unexpected arguments: %v", nonFlags[1:])
	}

	eState := domain.EpistemicState(*stateStr)
	if err := eState.Validate(); err != nil {
		return "", "", false, "", fmt.Errorf("invalid --default-state: %w", err)
	}

	return nonFlags[0], eState, *skipPIIPtr, *vaultPtr, nil
}
