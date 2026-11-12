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

	"github.com/kidthales/resumai/internal/vault"
)

// runDump handles the `resumai dump <dir> [--include-pii] [--vault <path>]` command.
func runDump(args []string, stdout, stderr io.Writer, globalVault string) int {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			printDumpUsage(stdout)
			return 0
		}
	}

	dir, includePII, vaultPath, err := parseDumpFlags(args, globalVault)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printDumpUsage(stderr)
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

	cleanDir := filepath.Clean(dir)
	ctx := context.Background()
	if err := v.Dump(ctx, cleanDir, vault.DumpOptions{IncludePII: includePII}); err != nil {
		fmt.Fprintf(stderr, "error dumping vault to %s: %v\n", cleanDir, err)
		return 1
	}

	fmt.Fprintf(stdout, "Vault dumped successfully to %s (PII included: %t)\n", cleanDir, includePII)
	return 0
}

func printDumpUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage: resumai dump <dir> [flags]

Dump career vault contents to a structured YAML directory bundle.
By default, contact PII (pii_header) is strictly omitted unless --include-pii is set.

Flags:
  --include-pii    Include air-gapped candidate contact PII in dump
  --vault <path>   Path to vault SQLite database (default: ~/.resumai/vault.db)
`)
}

func parseDumpFlags(args []string, defaultVault string) (dir string, includePII bool, vaultPath string, err error) {
	fs := flag.NewFlagSet("dump", flag.ContinueOnError)
	includePIIPtr := fs.Bool("include-pii", false, "Include candidate contact PII in dump")
	vaultPtr := fs.String("vault", defaultVault, "Path to vault SQLite database")

	var nonFlags []string
	var flagsOnly []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--include-pii" || arg == "-include-pii" {
			flagsOnly = append(flagsOnly, arg)
		} else if arg == "--vault" || arg == "-vault" {
			flagsOnly = append(flagsOnly, arg)
			if i+1 < len(args) {
				flagsOnly = append(flagsOnly, args[i+1])
				i++
			}
		} else if strings.HasPrefix(arg, "--vault=") || strings.HasPrefix(arg, "-vault=") {
			flagsOnly = append(flagsOnly, arg)
		} else if strings.HasPrefix(arg, "-") {
			flagsOnly = append(flagsOnly, arg)
		} else {
			nonFlags = append(nonFlags, arg)
		}
	}

	if err := fs.Parse(flagsOnly); err != nil {
		return "", false, "", err
	}

	if len(nonFlags) == 0 {
		return "", false, "", errors.New("missing target directory argument")
	}
	if len(nonFlags) > 1 {
		return "", false, "", fmt.Errorf("unexpected arguments: %v", nonFlags[1:])
	}

	return nonFlags[0], *includePIIPtr, *vaultPtr, nil
}

// runLoad handles the `resumai load <dir> [--vault <path>]` command.
func runLoad(args []string, stdout, stderr io.Writer, globalVault string) int {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			printLoadUsage(stdout)
			return 0
		}
	}

	dir, vaultPath, err := parseLoadFlags(args, globalVault)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printLoadUsage(stderr)
		return 1
	}

	cleanDir := filepath.Clean(dir)
	info, err := os.Stat(cleanDir)
	if err != nil {
		fmt.Fprintf(stderr, "error reading bundle directory %s: %v\n", cleanDir, err)
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
	if err := v.Load(ctx, cleanDir); err != nil {
		fmt.Fprintf(stderr, "error loading bundle from %s: %v\n", cleanDir, err)
		return 1
	}

	fmt.Fprintf(stdout, "Vault loaded successfully from %s\n", cleanDir)
	return 0
}

func printLoadUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage: resumai load <dir> [flags]

Load a structured YAML directory bundle into the career vault.
Preserves existing PII if pii.yaml is not present in the bundle.
Facts cited by submitted applications are automatically soft-archived if missing from the bundle.

Flags:
  --vault <path>   Path to vault SQLite database (default: ~/.resumai/vault.db)
`)
}

func parseLoadFlags(args []string, defaultVault string) (dir string, vaultPath string, err error) {
	fs := flag.NewFlagSet("load", flag.ContinueOnError)
	vaultPtr := fs.String("vault", defaultVault, "Path to vault SQLite database")

	var nonFlags []string
	var flagsOnly []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--vault" || arg == "-vault" {
			flagsOnly = append(flagsOnly, arg)
			if i+1 < len(args) {
				flagsOnly = append(flagsOnly, args[i+1])
				i++
			}
		} else if strings.HasPrefix(arg, "--vault=") || strings.HasPrefix(arg, "-vault=") {
			flagsOnly = append(flagsOnly, arg)
		} else if strings.HasPrefix(arg, "-") {
			flagsOnly = append(flagsOnly, arg)
		} else {
			nonFlags = append(nonFlags, arg)
		}
	}

	if err := fs.Parse(flagsOnly); err != nil {
		return "", "", err
	}

	if len(nonFlags) == 0 {
		return "", "", errors.New("missing source directory argument")
	}
	if len(nonFlags) > 1 {
		return "", "", fmt.Errorf("unexpected arguments: %v", nonFlags[1:])
	}

	return nonFlags[0], *vaultPtr, nil
}
