package domain

import (
	"fmt"
	"net/url"
	"strings"
)

// UnknownAskRecruiterToken is the canonical sentinel value indicating an external claim
// lacks an external citation URL and must be clarified with a recruiter.
const UnknownAskRecruiterToken = "unknown_ask_recruiter"

// JDClaim captures a requirement or signal extracted from a job description,
// grounded by a verbatim quote substring present in raw_jd.
type JDClaim struct {
	Category        string `json:"category,omitempty" yaml:"category,omitempty"`
	Summary         string `json:"summary,omitempty" yaml:"summary,omitempty"`
	VerbatimJDQuote string `json:"verbatim_jd_quote" yaml:"verbatim_jd_quote"`
}

// ExternalClaim captures an external reconnaissance claim about the role or organization.
// Every external claim must provide a valid HTTP/HTTPS source_url or mark unknown_ask_recruiter.
type ExternalClaim struct {
	Category            string `json:"category,omitempty" yaml:"category,omitempty"`
	Claim               string `json:"claim" yaml:"claim"`
	SourceURL           string `json:"source_url,omitempty" yaml:"source_url,omitempty"`
	UnknownAskRecruiter bool   `json:"unknown_ask_recruiter,omitempty" yaml:"unknown_ask_recruiter,omitempty"`
}

// OpportunityDossier represents the structured triage artifact for a job application.
type OpportunityDossier struct {
	RoleTitle      string          `json:"role_title,omitempty" yaml:"role_title,omitempty"`
	Company        string          `json:"company,omitempty" yaml:"company,omitempty"`
	RawJD          string          `json:"raw_jd" yaml:"raw_jd"`
	FitSummary     string          `json:"fit_summary,omitempty" yaml:"fit_summary,omitempty"`
	JDClaims       []JDClaim       `json:"jd_claims,omitempty" yaml:"jd_claims,omitempty"`
	ExternalClaims []ExternalClaim `json:"external_claims,omitempty" yaml:"external_claims,omitempty"`
}

// Validate enforces deterministic triage citation invariants:
// 1. raw_jd must be non-empty.
// 2. Every JDClaim.VerbatimJDQuote must be non-empty and match a verbatim substring of raw_jd.
// 3. Every ExternalClaim must carry a valid HTTP/HTTPS source_url or set unknown_ask_recruiter.
func (d *OpportunityDossier) Validate() error {
	if d == nil {
		return fmt.Errorf("%w: nil opportunity dossier", ErrInvalidTriageCitation)
	}
	if strings.TrimSpace(d.RawJD) == "" {
		return fmt.Errorf("%w: raw_jd cannot be empty", ErrInvalidTriageCitation)
	}

	normalizedJD := normalizeNewlines(d.RawJD)

	for i, claim := range d.JDClaims {
		quote := normalizeNewlines(claim.VerbatimJDQuote)
		if strings.TrimSpace(quote) == "" {
			return fmt.Errorf("%w: jd_claims[%d] missing verbatim_jd_quote", ErrInvalidTriageCitation, i)
		}
		if !strings.Contains(normalizedJD, quote) {
			return fmt.Errorf("%w: jd_claims[%d] verbatim_jd_quote %q not found in raw_jd", ErrInvalidTriageCitation, i, claim.VerbatimJDQuote)
		}
	}

	for i, ext := range d.ExternalClaims {
		if strings.TrimSpace(ext.Claim) == "" {
			return fmt.Errorf("%w: external_claims[%d] missing claim description", ErrInvalidTriageCitation, i)
		}

		trimmedURL := strings.TrimSpace(ext.SourceURL)
		markedUnknown := ext.UnknownAskRecruiter || strings.EqualFold(trimmedURL, UnknownAskRecruiterToken)

		if markedUnknown {
			// If a non-sentinel URL was also supplied, ensure it is still a valid HTTP/HTTPS URL.
			if trimmedURL != "" && !strings.EqualFold(trimmedURL, UnknownAskRecruiterToken) && !isValidHTTPURL(trimmedURL) {
				return fmt.Errorf("%w: external_claims[%d] has invalid source_url %q", ErrInvalidTriageCitation, i, ext.SourceURL)
			}
			continue
		}

		if !isValidHTTPURL(trimmedURL) {
			return fmt.Errorf("%w: external_claims[%d] requires valid http/https source_url or %s", ErrInvalidTriageCitation, i, UnknownAskRecruiterToken)
		}
	}

	return nil
}

func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func isValidHTTPURL(raw string) bool {
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	return u.Host != ""
}
