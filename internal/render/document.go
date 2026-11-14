package render

import (
	"encoding/json"
	"time"

	"github.com/kidthales/resumai/internal/domain"
)

// DocumentMetadata stores provenance and target information for the resume.
type DocumentMetadata struct {
	DocumentID    string    `json:"document_id,omitempty" yaml:"document_id,omitempty"`
	ApplicationID string    `json:"application_id,omitempty" yaml:"application_id,omitempty"`
	ArchetypeID   string    `json:"archetype_id,omitempty" yaml:"archetype_id,omitempty"`
	TargetRole    string    `json:"target_role,omitempty" yaml:"target_role,omitempty"`
	TargetCompany string    `json:"target_company,omitempty" yaml:"target_company,omitempty"`
	PageBudget    int       `json:"page_budget,omitempty" yaml:"page_budget,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

// Header contains contact information and target headline.
// Contact PII is air-gapped from LLM prompts and injected strictly at render time.
type Header struct {
	FullName     string `json:"full_name" yaml:"full_name"`
	Headline     string `json:"headline,omitempty" yaml:"headline,omitempty"`
	Email        string `json:"email,omitempty" yaml:"email,omitempty"`
	Phone        string `json:"phone,omitempty" yaml:"phone,omitempty"`
	Location     string `json:"location,omitempty" yaml:"location,omitempty"`
	LinkedInURL  string `json:"linkedin_url,omitempty" yaml:"linkedin_url,omitempty"`
	GitHubURL    string `json:"github_url,omitempty" yaml:"github_url,omitempty"`
	PortfolioURL string `json:"portfolio_url,omitempty" yaml:"portfolio_url,omitempty"`
	WebsiteURL   string `json:"website_url,omitempty" yaml:"website_url,omitempty"`
}

// SummarySection holds a high-level executive or profile summary.
type SummarySection struct {
	Title string      `json:"title,omitempty" yaml:"title,omitempty"`
	Text  string      `json:"text" yaml:"text"`
	Runs  []InlineRun `json:"runs,omitempty" yaml:"runs,omitempty"`
}

// ExperienceItem represents a single role within an organization.
type ExperienceItem struct {
	ID           string   `json:"id,omitempty" yaml:"id,omitempty"`
	RoleTitle    string   `json:"role_title" yaml:"role_title"`
	Organization string   `json:"organization" yaml:"organization"`
	Location     string   `json:"location,omitempty" yaml:"location,omitempty"`
	StartDate    string   `json:"start_date,omitempty" yaml:"start_date,omitempty"`
	EndDate      string   `json:"end_date,omitempty" yaml:"end_date,omitempty"`
	Duration     string   `json:"duration,omitempty" yaml:"duration,omitempty"`
	Highlights   []Bullet `json:"highlights" yaml:"highlights"`
}

// EducationItem represents an academic degree or educational credential.
type EducationItem struct {
	ID           string   `json:"id,omitempty" yaml:"id,omitempty"`
	Institution  string   `json:"institution" yaml:"institution"`
	Degree       string   `json:"degree" yaml:"degree"`
	FieldOfStudy string   `json:"field_of_study,omitempty" yaml:"field_of_study,omitempty"`
	Location     string   `json:"location,omitempty" yaml:"location,omitempty"`
	StartDate    string   `json:"start_date,omitempty" yaml:"start_date,omitempty"`
	EndDate      string   `json:"end_date,omitempty" yaml:"end_date,omitempty"`
	Duration     string   `json:"duration,omitempty" yaml:"duration,omitempty"`
	Honors       string   `json:"honors,omitempty" yaml:"honors,omitempty"`
	Highlights   []Bullet `json:"highlights,omitempty" yaml:"highlights,omitempty"`
}

// SkillCategory groups skills by functional domain.
type SkillCategory struct {
	Name   string   `json:"name" yaml:"name"`
	Skills []string `json:"skills" yaml:"skills"`
}

// ProjectItem represents an open-source, client, or personal project.
type ProjectItem struct {
	Name         string   `json:"name" yaml:"name"`
	Role         string   `json:"role,omitempty" yaml:"role,omitempty"`
	URL          string   `json:"url,omitempty" yaml:"url,omitempty"`
	Duration     string   `json:"duration,omitempty" yaml:"duration,omitempty"`
	Technologies []string `json:"technologies,omitempty" yaml:"technologies,omitempty"`
	Highlights   []Bullet `json:"highlights,omitempty" yaml:"highlights,omitempty"`
}

// CustomSection allows extensible custom sections (e.g. Publications, Volunteer Work).
type CustomSection struct {
	Title      string   `json:"title" yaml:"title"`
	Highlights []Bullet `json:"highlights" yaml:"highlights"`
}

// Bullet is an atomic accomplishment bullet within the ResumeDocument AST.
type Bullet struct {
	ID       string      `json:"id,omitempty" yaml:"id,omitempty"`
	Text     string      `json:"text" yaml:"text"`
	Runs     []InlineRun `json:"runs,omitempty" yaml:"runs,omitempty"`
	FactIDs  []string    `json:"fact_ids,omitempty" yaml:"fact_ids,omitempty"`
	Verified bool        `json:"verified" yaml:"verified"`
}

// NewBullet creates a Bullet from raw text and fact IDs, parsing runs automatically.
func NewBullet(id, text string, factIDs []string, verified bool) Bullet {
	runs := ParseInlineRuns(text)
	return Bullet{
		ID:       id,
		Text:     text,
		Runs:     runs,
		FactIDs:  factIDs,
		Verified: verified,
	}
}

// NewBulletFromNode converts a domain.BulletNode into an AST Bullet with parsed inline runs.
func NewBulletFromNode(node domain.BulletNode) Bullet {
	return NewBullet(node.ID, node.Text, node.FactIDs, node.Verified)
}

// ResumeDocument is the root semantic Abstract Syntax Tree for a resume.
type ResumeDocument struct {
	Metadata       DocumentMetadata `json:"metadata" yaml:"metadata"`
	Header         Header           `json:"header" yaml:"header"`
	Summary        *SummarySection  `json:"summary,omitempty" yaml:"summary,omitempty"`
	Experience     []ExperienceItem `json:"experience,omitempty" yaml:"experience,omitempty"`
	Education      []EducationItem  `json:"education,omitempty" yaml:"education,omitempty"`
	Skills         []SkillCategory  `json:"skills,omitempty" yaml:"skills,omitempty"`
	Projects       []ProjectItem    `json:"projects,omitempty" yaml:"projects,omitempty"`
	Certifications []string         `json:"certifications,omitempty" yaml:"certifications,omitempty"`
	CustomSections []CustomSection  `json:"custom_sections,omitempty" yaml:"custom_sections,omitempty"`
}

// NewResumeDocument initializes an empty ResumeDocument with sensible defaults.
func NewResumeDocument() *ResumeDocument {
	return &ResumeDocument{
		Metadata: DocumentMetadata{
			PageBudget: 1,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		},
		Experience: make([]ExperienceItem, 0),
		Education:  make([]EducationItem, 0),
		Skills:     make([]SkillCategory, 0),
	}
}

// InjectPII merges air-gapped personal contact details into the document header strictly at render time.
func (d *ResumeDocument) InjectPII(pii *domain.PIIHeader) {
	if pii == nil {
		return
	}
	if pii.FullName != "" {
		d.Header.FullName = pii.FullName
	}
	if pii.Email != "" {
		d.Header.Email = pii.Email
	}
	if pii.Phone != "" {
		d.Header.Phone = pii.Phone
	}
	if pii.Location != "" {
		d.Header.Location = pii.Location
	}
	if pii.LinkedInURL != "" {
		d.Header.LinkedInURL = pii.LinkedInURL
	}
	if pii.GitHubURL != "" {
		d.Header.GitHubURL = pii.GitHubURL
	}
	if pii.PortfolioURL != "" {
		d.Header.PortfolioURL = pii.PortfolioURL
	}
}

// AllBullets collects pointers to every Bullet across all sections of the document.
func (d *ResumeDocument) AllBullets() []*Bullet {
	var bullets []*Bullet
	for i := range d.Experience {
		for j := range d.Experience[i].Highlights {
			bullets = append(bullets, &d.Experience[i].Highlights[j])
		}
	}
	for i := range d.Education {
		for j := range d.Education[i].Highlights {
			bullets = append(bullets, &d.Education[i].Highlights[j])
		}
	}
	for i := range d.Projects {
		for j := range d.Projects[i].Highlights {
			bullets = append(bullets, &d.Projects[i].Highlights[j])
		}
	}
	for i := range d.CustomSections {
		for j := range d.CustomSections[i].Highlights {
			bullets = append(bullets, &d.CustomSections[i].Highlights[j])
		}
	}
	return bullets
}

// Clone returns a deep copy of the ResumeDocument.
func (d *ResumeDocument) Clone() (*ResumeDocument, error) {
	data, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var clone ResumeDocument
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}
