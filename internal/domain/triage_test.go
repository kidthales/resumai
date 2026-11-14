package domain

import (
	"errors"
	"testing"
)

func TestOpportunityDossierValidate_Success(t *testing.T) {
	rawJD := "Senior Go Engineer\nWe build distributed storage systems in Go and SQLite.\nRemote friendly across North America."

	dossier := OpportunityDossier{
		RoleTitle: "Senior Go Engineer",
		Company:   "Acme Storage",
		RawJD:     rawJD,
		JDClaims: []JDClaim{
			{
				Category:        "core_stack",
				Summary:         "Requires Go and SQLite",
				VerbatimJDQuote: "distributed storage systems in Go and SQLite.",
			},
			{
				Category:        "location",
				Summary:         "Remote in NA",
				VerbatimJDQuote: "Remote friendly across North America.",
			},
		},
		ExternalClaims: []ExternalClaim{
			{
				Category:  "funding",
				Claim:     "Raised Series B in 2025",
				SourceURL: "https://acme.example.com/blog/series-b",
			},
			{
				Category:            "team_size",
				Claim:               "Platform team size and on-call rotation cadence",
				UnknownAskRecruiter: true,
			},
			{
				Category:  "compensation",
				Claim:     "Equity refresh policy",
				SourceURL: UnknownAskRecruiterToken,
			},
		},
	}

	if err := dossier.Validate(); err != nil {
		t.Fatalf("expected valid dossier, got error: %v", err)
	}
}

func TestOpportunityDossierValidate_NormalizesCRLF(t *testing.T) {
	dossier := OpportunityDossier{
		RawJD: "Line one\r\nLine two\r\nLine three",
		JDClaims: []JDClaim{
			{
				VerbatimJDQuote: "Line one\nLine two",
			},
		},
	}

	if err := dossier.Validate(); err != nil {
		t.Fatalf("expected CRLF-normalized quote to match raw_jd, got: %v", err)
	}
}

func TestOpportunityDossierValidate_Failures(t *testing.T) {
	baseJD := "Looking for a Staff Systems Engineer experienced with Go, SQLite, and Chrome CDP."

	cases := []struct {
		name    string
		dossier *OpportunityDossier
	}{
		{
			name:    "nil dossier",
			dossier: nil,
		},
		{
			name: "empty raw_jd",
			dossier: &OpportunityDossier{
				RawJD: "   \n\t  ",
			},
		},
		{
			name: "empty verbatim_jd_quote",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				JDClaims: []JDClaim{
					{Summary: "Go experience", VerbatimJDQuote: "   "},
				},
			},
		},
		{
			name: "quote not in raw_jd",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				JDClaims: []JDClaim{
					{Summary: "Rust experience", VerbatimJDQuote: "experienced with Rust and Postgres"},
				},
			},
		},
		{
			name: "quote altered case",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				JDClaims: []JDClaim{
					{VerbatimJDQuote: "staff systems engineer"},
				},
			},
		},
		{
			name: "external claim missing claim text",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				ExternalClaims: []ExternalClaim{
					{Claim: "  ", SourceURL: "https://example.com/about"},
				},
			},
		},
		{
			name: "external claim missing url and not unknown",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				ExternalClaims: []ExternalClaim{
					{Claim: "Uses Kubernetes in production"},
				},
			},
		},
		{
			name: "external claim invalid scheme",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				ExternalClaims: []ExternalClaim{
					{Claim: "Internal portal link", SourceURL: "ftp://example.com/info"},
				},
			},
		},
		{
			name: "external claim javascript scheme",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				ExternalClaims: []ExternalClaim{
					{Claim: "Bogus citation", SourceURL: "javascript:alert(1)"},
				},
			},
		},
		{
			name: "external claim relative url without host",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				ExternalClaims: []ExternalClaim{
					{Claim: "About page", SourceURL: "/about"},
				},
			},
		},
		{
			name: "external claim url missing host",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				ExternalClaims: []ExternalClaim{
					{Claim: "Empty host", SourceURL: "https://"},
				},
			},
		},
		{
			name: "external claim unknown flag with bogus source url",
			dossier: &OpportunityDossier{
				RawJD: baseJD,
				ExternalClaims: []ExternalClaim{
					{Claim: "Unknown cadence", SourceURL: "not a url", UnknownAskRecruiter: true},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.dossier.Validate()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !errors.Is(err, ErrInvalidTriageCitation) {
				t.Fatalf("expected ErrInvalidTriageCitation, got %v", err)
			}
		})
	}
}
