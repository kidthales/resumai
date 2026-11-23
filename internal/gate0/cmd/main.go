package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kidthales/resumai/internal/gate0"
)

type jsonResult struct {
	Violations []string `json:"violations"`
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("gate0", flag.ContinueOnError)
	workflowsDir := fs.String("workflows", "", "path to GitHub Actions workflows directory")
	jsonMode := fs.Bool("json", false, "emit JSON result to stdout")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *workflowsDir == "" {
		fmt.Fprintln(os.Stderr, "Gate 0 failed: -workflows path is required.")
		return 1
	}

	violations, err := gate0.VerifyWorkflowsDir(*workflowsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	if *jsonMode {
		if violations == nil {
			violations = []string{}
		}
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(jsonResult{Violations: violations}); err != nil {
			fmt.Fprintf(os.Stderr, "Gate 0 failed to encode output: %v\n", err)
			return 1
		}
		return 0
	}

	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "Gate 0 violation: %s\n", v)
		}
		fmt.Fprintf(os.Stderr, "Gate 0 workflow single-source verification failed (%d violation(s)).\n", len(violations))
		return 1
	}

	fmt.Println("All workflow run steps conform to ci-local.ps1 -CI or exact job-id/step-name allowlist.")
	return 0
}
