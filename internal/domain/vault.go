package domain

import (
	"errors"
	"fmt"
	"time"
)

// EpistemicState represents the verification and lifecycle state of a career fact.
type EpistemicState string

const (
	EpistemicRawNote       EpistemicState = "raw_note"
	EpistemicMetricPending EpistemicState = "metric_pending"
	EpistemicVerified      EpistemicState = "verified"
	EpistemicArchived      EpistemicState = "archived"
)

// Validate checks if the epistemic state is one of the recognized states.
func (s EpistemicState) Validate() error {
	switch s {
	case EpistemicRawNote, EpistemicMetricPending, EpistemicVerified, EpistemicArchived:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidEpistemicState, s)
	}
}

// Vault-specific domain errors.
var (
	ErrNotFound                  = errors.New("entity not found")
	ErrCannotDeleteSubmittedFact = errors.New("cannot delete career fact cited by a submitted application")
	ErrInvalidEpistemicState     = errors.New("invalid epistemic state")
)

// PIIHeader stores the candidate's air-gapped personal contact information.
// Contact PII is strictly isolated from LLM prompts and stdio MCP server responses.
type PIIHeader struct {
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	Location     string    `json:"location"`
	LinkedInURL  string    `json:"linkedin_url"`
	GitHubURL    string    `json:"github_url"`
	PortfolioURL string    `json:"portfolio_url"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CandidateLogistics stores non-PII employment logistics and preferences.
type CandidateLogistics struct {
	TargetTitle         string    `json:"target_title"`
	WorkAuthorization   string    `json:"work_authorization"`
	RemotePreference    string    `json:"remote_preference"`
	WillingToRelocate   bool      `json:"willing_to_relocate"`
	TargetCompensation  string    `json:"target_compensation"`
	NoticePeriodDays    int       `json:"notice_period_days"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Organization represents an employer, institution, or client company.
type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Location  string    `json:"location"`
	Website   string    `json:"website"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Role represents a position or role held within an organization.
type Role struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"` // "work", "extra", "volunteer", etc.
	Location  string    `json:"location"`
	Duration  string    `json:"duration"`
	TechStack []string  `json:"tech_stack"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Education represents an academic credential or diploma.
type Education struct {
	ID           string    `json:"id"`
	Institution  string    `json:"institution"`
	Degree       string    `json:"degree"`
	FieldOfStudy string    `json:"field_of_study"`
	Location     string    `json:"location"`
	Duration     string    `json:"duration"`
	Honors       string    `json:"honors"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Archetype represents a tailored resume persona / profile template.
type Archetype struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	ProfileOverview  string    `json:"profile_overview"`
	CoreCompetencies []string  `json:"core_competencies"`
	StrategicImpact  []string  `json:"strategic_impact"`
	TargetRoles      []string  `json:"target_roles"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
