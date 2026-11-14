package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Standard domain error sentinels for JobApplication FSM and invariants.
var (
	ErrInvalidTriageCitation       = errors.New("invalid triage citation: external claim missing citation or quote not in job description")
	ErrMissingRationale            = errors.New("missing rejection rationale")
	ErrUngroundedBullet            = errors.New("ungrounded bullet: cited fact does not exist in career facts vault")
	ErrIncompleteAudit             = errors.New("incomplete audit: missing fact-checker or alignment-checker report")
	ErrUnverifiedFactOrPlaceholder = errors.New("unverified fact or placeholder present in draft")
	ErrPageBudgetOverflow          = errors.New("rendered document exceeds max page budget")
	ErrAlreadySubmitted            = errors.New("application already submitted and locked immutable")
	ErrInvalidStateTransition      = errors.New("invalid application state transition")
	ErrMissingPIIHeader            = errors.New("missing required candidate pii_header")
	ErrMissingExportArtifact       = errors.New("missing required rendered artifact path")
	ErrMissingSubmissionArtifact   = errors.New("missing required submission bundle hash or interview brief")
)

// ApplicationState represents the lifecycle state of a job application.
type ApplicationState string

const (
	StateTriaged             ApplicationState = "Triaged"
	StateDeclinedByCandidate ApplicationState = "DeclinedByCandidate"
	StateDrafted             ApplicationState = "Drafted"
	StateAudited             ApplicationState = "Audited"
	StateReadyToRender       ApplicationState = "ReadyToRender"
	StateExported            ApplicationState = "Exported"
	StateSubmitted           ApplicationState = "Submitted"
)

// Validate checks whether the ApplicationState is a recognized lifecycle state.
func (s ApplicationState) Validate() error {
	switch s {
	case StateTriaged, StateDeclinedByCandidate, StateDrafted, StateAudited,
		StateReadyToRender, StateExported, StateSubmitted:
		return nil
	default:
		return fmt.Errorf("%w: unrecognized state %q", ErrInvalidStateTransition, s)
	}
}

// IsTerminal reports whether the application is in a terminal immutable state.
func (s ApplicationState) IsTerminal() bool {
	return s == StateSubmitted || s == StateDeclinedByCandidate
}

// ContainsMetricPlaceholder checks whether a string contains any unresolved [METRIC: token.
func ContainsMetricPlaceholder(text string) bool {
	return strings.Contains(strings.ToUpper(text), "[METRIC:")
}

// BulletNode represents a single bullet point in a draft resume.
type BulletNode struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	FactIDs  []string `json:"fact_ids"`
	Verified bool     `json:"verified"`
}

// CareerFact represents an atomic ground-truth career accomplishment.
type CareerFact struct {
	ID             string         `json:"id"`
	RoleID         string         `json:"role_id"`
	Headline       string         `json:"headline"`
	Details        string         `json:"details"`
	EpistemicState EpistemicState `json:"epistemic_state"`
	TechTags       []string       `json:"tech_tags"`
	Verified       bool           `json:"verified"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// JobApplication represents the domain entity tracking an application lifecycle.
type JobApplication struct {
	ID                 string              `json:"id"`
	RoleTitle          string              `json:"role_title"`
	Company            string              `json:"company"`
	RawJD              string              `json:"raw_jd"`
	State              ApplicationState    `json:"state"`
	RejectionRationale string              `json:"rejection_rationale,omitempty"`
	ArchetypeID        string              `json:"archetype_id,omitempty"`
	Dossier            *OpportunityDossier `json:"dossier,omitempty"`
	Draft              *ApplicationDraft   `json:"draft,omitempty"`
	Audits             *AuditBundle        `json:"audits,omitempty"`
	Export             *ExportRecord       `json:"export,omitempty"`
	BundleSHA256       string              `json:"bundle_sha256,omitempty"`
	InterviewBriefPath string              `json:"interview_brief_path,omitempty"`
	SubmittedAt        *time.Time          `json:"submitted_at,omitempty"`
	CreatedAt          time.Time           `json:"created_at"`
	UpdatedAt          time.Time           `json:"updated_at"`
}
