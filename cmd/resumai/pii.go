package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/vault"
)

// runPII handles the `resumai pii` subcommand hierarchy:
//
//	resumai pii show [--vault <path>] [--json]
//	resumai pii set [--vault <path>] [--name <string>] [--email <string>] ...
func runPII(args []string, stdout, stderr io.Writer, globalVault string) int {
	if len(args) == 0 {
		printPIIUsage(stderr)
		return 1
	}

	subcmd := args[0]
	subargs := args[1:]

	switch subcmd {
	case "show":
		return runPIIShow(subargs, stdout, stderr, globalVault)
	case "set":
		return runPIISet(subargs, stdout, stderr, globalVault)
	case "help", "--help", "-h":
		printPIIUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown pii command: %q\n\n", subcmd)
		printPIIUsage(stderr)
		return 1
	}
}

func printPIIUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage: resumai pii <command> [flags]

Commands:
  show    Display candidate contact PII (air-gapped)
  set     Update candidate contact PII in local vault

Global / Command Flags:
  --vault <path>    Path to vault SQLite database (default: ~/.resumai/vault.db)
`)
}

func runPIIShow(args []string, stdout, stderr io.Writer, globalVault string) int {
	fs := flag.NewFlagSet("pii show", flag.ContinueOnError)
	fs.SetOutput(stderr)

	vaultPath := fs.String("vault", globalVault, "Path to vault SQLite database")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	targetPath := *vaultPath
	if targetPath == "" {
		var err error
		targetPath, err = vault.DefaultPath()
		if err != nil {
			fmt.Fprintf(stderr, "error resolving default vault path: %v\n", err)
			return 1
		}
	}

	v, err := vault.OpenPath(targetPath)
	if err != nil {
		fmt.Fprintf(stderr, "error opening vault (%s): %v\n", targetPath, err)
		return 1
	}
	defer v.Close()

	ctx := context.Background()
	pii, err := v.GetPII(ctx)
	if errors.Is(err, domain.ErrNotFound) {
		if *jsonOutput {
			fmt.Fprintln(stdout, "{}")
			return 0
		}
		fmt.Fprintf(stdout, "No candidate PII configured in vault (%s).\nUse 'resumai pii set' to configure contact info.\n", targetPath)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "error retrieving PII: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(pii); err != nil {
			fmt.Fprintf(stderr, "error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(stdout, "Candidate Contact PII (Air-Gapped):\n")
	fmt.Fprintf(stdout, "  Full Name:     %s\n", pii.FullName)
	fmt.Fprintf(stdout, "  Email:         %s\n", pii.Email)
	fmt.Fprintf(stdout, "  Phone:         %s\n", pii.Phone)
	fmt.Fprintf(stdout, "  Location:      %s\n", pii.Location)
	fmt.Fprintf(stdout, "  LinkedIn:      %s\n", pii.LinkedInURL)
	fmt.Fprintf(stdout, "  GitHub:        %s\n", pii.GitHubURL)
	fmt.Fprintf(stdout, "  Portfolio:     %s\n", pii.PortfolioURL)
	if !pii.UpdatedAt.IsZero() {
		fmt.Fprintf(stdout, "  Updated At:    %s\n", pii.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	return 0
}

func runPIISet(args []string, stdout, stderr io.Writer, globalVault string) int {
	fs := flag.NewFlagSet("pii set", flag.ContinueOnError)
	fs.SetOutput(stderr)

	vaultPath := fs.String("vault", globalVault, "Path to vault SQLite database")
	name := fs.String("name", "", "Candidate full name")
	fullName := fs.String("full-name", "", "Candidate full name (alias)")
	email := fs.String("email", "", "Candidate email address")
	phone := fs.String("phone", "", "Candidate phone number")
	location := fs.String("location", "", "Candidate location/city")
	linkedin := fs.String("linkedin", "", "LinkedIn profile URL")
	linkedinURL := fs.String("linkedin-url", "", "LinkedIn profile URL (alias)")
	github := fs.String("github", "", "GitHub profile URL")
	githubURL := fs.String("github-url", "", "GitHub profile URL (alias)")
	portfolio := fs.String("portfolio", "", "Portfolio or personal website URL")
	portfolioURL := fs.String("portfolio-url", "", "Portfolio or personal website URL (alias)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Track which fields were explicitly provided
	setFields := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		setFields[f.Name] = true
	})

	// Check if any PII field flag was provided
	piiFlagProvided := false
	for k := range setFields {
		if k != "vault" {
			piiFlagProvided = true
			break
		}
	}

	if !piiFlagProvided {
		fmt.Fprintf(stderr, "error: at least one field flag must be specified (e.g. --name, --email, --phone, --location, --linkedin, --github, --portfolio)\n")
		return 1
	}

	targetPath := *vaultPath
	if targetPath == "" {
		var err error
		targetPath, err = vault.DefaultPath()
		if err != nil {
			fmt.Fprintf(stderr, "error resolving default vault path: %v\n", err)
			return 1
		}
	}

	v, err := vault.OpenPath(targetPath)
	if err != nil {
		fmt.Fprintf(stderr, "error opening vault (%s): %v\n", targetPath, err)
		return 1
	}
	defer v.Close()

	ctx := context.Background()
	pii, err := v.GetPII(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			pii = &domain.PIIHeader{}
		} else {
			fmt.Fprintf(stderr, "error reading existing PII: %v\n", err)
			return 1
		}
	}

	if setFields["name"] {
		pii.FullName = *name
	} else if setFields["full-name"] {
		pii.FullName = *fullName
	}

	if setFields["email"] {
		pii.Email = *email
	}
	if setFields["phone"] {
		pii.Phone = *phone
	}
	if setFields["location"] {
		pii.Location = *location
	}

	if setFields["linkedin"] {
		pii.LinkedInURL = *linkedin
	} else if setFields["linkedin-url"] {
		pii.LinkedInURL = *linkedinURL
	}

	if setFields["github"] {
		pii.GitHubURL = *github
	} else if setFields["github-url"] {
		pii.GitHubURL = *githubURL
	}

	if setFields["portfolio"] {
		pii.PortfolioURL = *portfolio
	} else if setFields["portfolio-url"] {
		pii.PortfolioURL = *portfolioURL
	}

	if err := v.SetPII(ctx, pii); err != nil {
		fmt.Fprintf(stderr, "error saving PII to vault: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Candidate PII updated successfully in %s:\n", targetPath)
	fmt.Fprintf(stdout, "  Full Name:     %s\n", pii.FullName)
	fmt.Fprintf(stdout, "  Email:         %s\n", pii.Email)
	fmt.Fprintf(stdout, "  Phone:         %s\n", pii.Phone)
	fmt.Fprintf(stdout, "  Location:      %s\n", pii.Location)
	fmt.Fprintf(stdout, "  LinkedIn:      %s\n", pii.LinkedInURL)
	fmt.Fprintf(stdout, "  GitHub:        %s\n", pii.GitHubURL)
	fmt.Fprintf(stdout, "  Portfolio:     %s\n", pii.PortfolioURL)
	return 0
}
