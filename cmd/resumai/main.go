package main

import (
	"fmt"
	"io"
	"os"

	"github.com/kidthales/resumai/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	// Parse global flags if any before subcommand
	var vaultPath string
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--vault" && i+1 < len(args) {
			vaultPath = args[i+1]
			i++
		} else if len(arg) > 8 && arg[:8] == "--vault=" {
			vaultPath = arg[8:]
		} else {
			remaining = append(remaining, arg)
		}
	}

	if len(remaining) == 0 {
		printUsage(stdout)
		return 0
	}

	cmd := remaining[0]
	cmdArgs := remaining[1:]

	switch cmd {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "resumai version %s\n", version.Version)
		return 0
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	case "pii":
		return runPII(cmdArgs, stdout, stderr, vaultPath)
	case "dump":
		return runDump(cmdArgs, stdout, stderr, vaultPath)
	case "load":
		return runLoad(cmdArgs, stdout, stderr, vaultPath)
	case "import-legacy":
		return runImportLegacy(cmdArgs, stdout, stderr, vaultPath)
	default:
		fmt.Fprintf(stderr, "unknown command: %q\n\n", cmd)
		printUsage(stderr)
		return 1
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `resumai v%s - resume vault & rendering engine

Usage:
  resumai [global flags] <command> [subcommand] [flags]

Commands:
  pii       Manage air-gapped candidate contact PII (show, set)
  dump            Dump career vault to structured YAML directory bundle
  load            Load structured YAML directory bundle into vault
  import-legacy   Import v0.1.0 Markdown files into career vault
  version         Print resumai version
  help      Display help information

Global Flags:
  --vault <path>   Path to SQLite career vault (default: ~/.resumai/vault.db)
`, version.Version)
}
