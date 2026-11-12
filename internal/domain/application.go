package domain

import (
	"errors"
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

// BulletNode represents a single bullet point in a draft resume.
type BulletNode struct {
	ID       string   `json:"id"`
	Text     string   `json:"text"`
	FactIDs  []string `json:"fact_ids"`
	Verified bool     `json:"verified"`
}

// CareerFact represents an atomic ground-truth career accomplishment.
type CareerFact struct {
	ID        string    `json:"id"`
	Headline  string    `json:"headline"`
	Details   string    `json:"details"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// JobApplication represents the domain entity tracking an application lifecycle.
type JobApplication struct {
	ID                 string           `json:"id"`
	RoleTitle          string           `json:"role_title"`
	Company            string           `json:"company"`
	RawJD              string           `json:"raw_jd"`
	State              ApplicationState `json:"state"`
	RejectionRationale string           `json:"rejection_rationale,omitempty"`
	ArchetypeID        string           `json:"archetype_id,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}
