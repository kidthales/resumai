package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Critic agent names required for audit completion.
const (
	CriticFactChecker      = "resume-fact-checker"
	CriticAlignmentChecker = "resume-alignment-checker"
)

// DraftResolver resolves persistent archetypes and career facts without coupling
// the domain package to storage adapters.
type DraftResolver interface {
	GetArchetypeByID(id string) (*Archetype, error)
	GetCareerFactByID(id string) (*CareerFact, error)
}

// FactLookup resolves career facts by ID for pre-render verification.
type FactLookup interface {
	GetCareerFactByID(id string) (*CareerFact, error)
}

// DraftExperience represents a role section inside an application draft.
type DraftExperience struct {
	ID           string       `json:"id,omitempty"`
	RoleTitle    string       `json:"role_title"`
	Organization string       `json:"organization"`
	Location     string       `json:"location,omitempty"`
	StartDate    string       `json:"start_date,omitempty"`
	EndDate      string       `json:"end_date,omitempty"`
	Duration     string       `json:"duration,omitempty"`
	Highlights   []BulletNode `json:"highlights"`
}

// DraftEducation represents an academic credential section inside an application draft.
type DraftEducation struct {
	ID           string       `json:"id,omitempty"`
	Institution  string       `json:"institution"`
	Degree       string       `json:"degree"`
	FieldOfStudy string       `json:"field_of_study,omitempty"`
	Location     string       `json:"location,omitempty"`
	StartDate    string       `json:"start_date,omitempty"`
	EndDate      string       `json:"end_date,omitempty"`
	Duration     string       `json:"duration,omitempty"`
	Honors       string       `json:"honors,omitempty"`
	Highlights   []BulletNode `json:"highlights,omitempty"`
}

// DraftSkillCategory groups technical skills in a draft.
type DraftSkillCategory struct {
	Name   string   `json:"name"`
	Skills []string `json:"skills"`
}

// DraftProject represents a project section inside an application draft.
type DraftProject struct {
	Name         string       `json:"name"`
	Role         string       `json:"role,omitempty"`
	URL          string       `json:"url,omitempty"`
	Duration     string       `json:"duration,omitempty"`
	Technologies []string     `json:"technologies,omitempty"`
	Highlights   []BulletNode `json:"highlights,omitempty"`
}

// DraftCustomSection represents an extensible custom section in a draft.
type DraftCustomSection struct {
	Title      string       `json:"title"`
	Highlights []BulletNode `json:"highlights"`
}

// ApplicationDraft represents the tailored resume AST authored prior to rendering.
type ApplicationDraft struct {
	ArchetypeID    string               `json:"archetype_id"`
	Headline       string               `json:"headline,omitempty"`
	Summary        string               `json:"summary,omitempty"`
	Experience     []DraftExperience    `json:"experience,omitempty"`
	Education      []DraftEducation     `json:"education,omitempty"`
	Skills         []DraftSkillCategory `json:"skills,omitempty"`
	Projects       []DraftProject       `json:"projects,omitempty"`
	Certifications []string             `json:"certifications,omitempty"`
	CustomSections []DraftCustomSection `json:"custom_sections,omitempty"`
	PageBudget     int                  `json:"page_budget,omitempty"`
}

// AllBullets returns pointers to every BulletNode across all sections in the draft.
func (d *ApplicationDraft) AllBullets() []*BulletNode {
	if d == nil {
		return nil
	}
	var bullets []*BulletNode
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

// CitedFactIDs returns a sorted, deduplicated slice of non-empty career fact IDs cited in the draft.
func (d *ApplicationDraft) CitedFactIDs() []string {
	seen := make(map[string]struct{})
	for _, b := range d.AllBullets() {
		for _, id := range b.FactIDs {
			trimmed := strings.TrimSpace(id)
			if trimmed != "" {
				seen[trimmed] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// AuditDiscrepancySeverity indicates the severity of a factual discrepancy reported by a critic.
type AuditDiscrepancySeverity string

const (
	SeverityLow    AuditDiscrepancySeverity = "low"
	SeverityMedium AuditDiscrepancySeverity = "medium"
	SeverityHigh   AuditDiscrepancySeverity = "high"
)

// Validate checks whether the severity value is recognized.
func (s AuditDiscrepancySeverity) Validate() error {
	switch AuditDiscrepancySeverity(strings.ToLower(string(s))) {
	case SeverityLow, SeverityMedium, SeverityHigh:
		return nil
	default:
		return fmt.Errorf("%w: unrecognized discrepancy severity %q", ErrIncompleteAudit, s)
	}
}

// FactualDiscrepancy records a single finding from the clean-room fact checker.
type FactualDiscrepancy struct {
	BulletID string                   `json:"bullet_id,omitempty"`
	FactID   string                   `json:"fact_id,omitempty"`
	Severity AuditDiscrepancySeverity `json:"severity"`
	Finding  string                   `json:"finding"`
}

// FactCheckReport holds the clean-room resume-fact-checker critic output.
type FactCheckReport struct {
	CriticAgent   string               `json:"critic_agent"`
	Summary       string               `json:"summary"`
	Discrepancies []FactualDiscrepancy `json:"discrepancies,omitempty"`
	Passed        bool                 `json:"passed"`
	RecordedAt    time.Time            `json:"recorded_at"`
}

// AlignmentCheckReport holds the clean-room resume-alignment-checker critic output.
type AlignmentCheckReport struct {
	CriticAgent     string    `json:"critic_agent"`
	Summary         string    `json:"summary"`
	MatchedSignals  []string  `json:"matched_signals,omitempty"`
	MissingSignals  []string  `json:"missing_signals,omitempty"`
	Recommendations []string  `json:"recommendations,omitempty"`
	Passed          bool      `json:"passed"`
	RecordedAt      time.Time `json:"recorded_at"`
}

// AuditBundle aggregates both required clean-room critic reports for an application draft.
type AuditBundle struct {
	FactCheck      *FactCheckReport      `json:"fact_check"`
	AlignmentCheck *AlignmentCheckReport `json:"alignment_check"`
}

// ExportRecord stores rendered packet artifact paths and page-budget telemetry.
type ExportRecord struct {
	PacketDir  string    `json:"packet_dir"`
	HTMLPath   string    `json:"html_path"`
	DocxPath   string    `json:"docx_path"`
	PDFPath    string    `json:"pdf_path"`
	PageCount  int       `json:"page_count"`
	MaxPages   int       `json:"max_pages"`
	ExportedAt time.Time `json:"exported_at"`
}

// NewTriagedApplication creates a new JobApplication in StateTriaged after validating the dossier.
func NewTriagedApplication(id string, dossier OpportunityDossier) (*JobApplication, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: application id cannot be empty", ErrInvalidTriageCitation)
	}
	app := &JobApplication{
		ID:        strings.TrimSpace(id),
		CreatedAt: time.Now().UTC(),
	}
	if err := app.ApplyTriage(dossier); err != nil {
		return nil, err
	}
	return app, nil
}

// ApplyTriage validates an OpportunityDossier and transitions [*] or Triaged -> Triaged.
func (a *JobApplication) ApplyTriage(dossier OpportunityDossier) error {
	if err := a.guardNotSubmitted(); err != nil {
		return err
	}
	if a.State != "" && a.State != StateTriaged {
		return fmt.Errorf("%w: cannot apply triage from state %q", ErrInvalidStateTransition, a.State)
	}
	if err := dossier.Validate(); err != nil {
		return err
	}

	dossierCopy := dossier
	if dossier.RoleTitle != "" {
		a.RoleTitle = strings.TrimSpace(dossier.RoleTitle)
	}
	if dossier.Company != "" {
		a.Company = strings.TrimSpace(dossier.Company)
	}
	a.RawJD = dossier.RawJD
	a.Dossier = &dossierCopy
	a.State = StateTriaged
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// PassRole transitions Triaged -> DeclinedByCandidate with a mandatory rejection rationale.
func (a *JobApplication) PassRole(rationale string) error {
	if err := a.guardNotSubmitted(); err != nil {
		return err
	}
	if a.State != StateTriaged {
		return fmt.Errorf("%w: cannot pass role from state %q", ErrInvalidStateTransition, a.State)
	}
	trimmed := strings.TrimSpace(rationale)
	if trimmed == "" {
		return ErrMissingRationale
	}
	a.RejectionRationale = trimmed
	a.State = StateDeclinedByCandidate
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// SaveDraft transitions Triaged (or re-drafts from Drafted/Audited) -> Drafted,
// verifying a valid archetype_id and resolving every BulletNode.fact_ids to real career facts.
func (a *JobApplication) SaveDraft(draft ApplicationDraft, resolver DraftResolver) error {
	if err := a.guardNotSubmitted(); err != nil {
		return err
	}
	if a.State != StateTriaged && a.State != StateDrafted && a.State != StateAudited {
		return fmt.Errorf("%w: cannot save draft from state %q", ErrInvalidStateTransition, a.State)
	}
	if resolver == nil {
		return fmt.Errorf("%w: draft resolver is required", ErrUngroundedBullet)
	}

	archID := strings.TrimSpace(draft.ArchetypeID)
	if archID == "" {
		return fmt.Errorf("%w: archetype_id cannot be empty", ErrUngroundedBullet)
	}
	arch, err := resolver.GetArchetypeByID(archID)
	if err != nil || arch == nil {
		return fmt.Errorf("%w: archetype %q does not exist", ErrUngroundedBullet, archID)
	}

	draftCopy := draft
	draftCopy.ArchetypeID = archID
	bullets := draftCopy.AllBullets()
	if len(bullets) == 0 {
		return fmt.Errorf("%w: draft must contain at least one bullet", ErrUngroundedBullet)
	}

	for idx, bullet := range bullets {
		factIDs := trimNonEmpty(bullet.FactIDs)
		if len(factIDs) == 0 {
			return fmt.Errorf("%w: bullet[%d] (%q) cites zero career facts", ErrUngroundedBullet, idx, bullet.ID)
		}
		allVerified := true
		for _, fid := range factIDs {
			fact, ferr := resolver.GetCareerFactByID(fid)
			if ferr != nil || fact == nil {
				return fmt.Errorf("%w: bullet[%d] (%q) cites missing fact %q", ErrUngroundedBullet, idx, bullet.ID, fid)
			}
			if !fact.Verified || fact.EpistemicState != EpistemicVerified {
				allVerified = false
			}
		}
		bullet.FactIDs = factIDs
		bullet.Verified = allVerified
	}

	a.ArchetypeID = archID
	a.Draft = &draftCopy
	// Any draft modification invalidates prior audit reports.
	a.Audits = nil
	a.State = StateDrafted
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// RecordAudits transitions Drafted (or re-records from Audited) -> Audited,
// requiring both resume-fact-checker and resume-alignment-checker reports.
func (a *JobApplication) RecordAudits(bundle AuditBundle) error {
	if err := a.guardNotSubmitted(); err != nil {
		return err
	}
	if a.State != StateDrafted && a.State != StateAudited {
		return fmt.Errorf("%w: cannot record audits from state %q", ErrInvalidStateTransition, a.State)
	}
	if err := validateAuditBundle(&bundle); err != nil {
		return err
	}

	now := time.Now().UTC()
	fc := *bundle.FactCheck
	if fc.CriticAgent == "" {
		fc.CriticAgent = CriticFactChecker
	}
	if fc.RecordedAt.IsZero() {
		fc.RecordedAt = now
	}
	ac := *bundle.AlignmentCheck
	if ac.CriticAgent == "" {
		ac.CriticAgent = CriticAlignmentChecker
	}
	if ac.RecordedAt.IsZero() {
		ac.RecordedAt = now
	}

	a.Audits = &AuditBundle{
		FactCheck:      &fc,
		AlignmentCheck: &ac,
	}
	a.State = StateAudited
	a.UpdatedAt = now
	return nil
}

// PromoteReady transitions Audited -> ReadyToRender, enforcing:
// 1. Both audit reports attached, with zero High factual discrepancies.
// 2. Zero unresolved [METRIC: tokens across headline, summary, and bullets.
// 3. 100% of cited career facts resolve and are verified in the live vault.
func (a *JobApplication) PromoteReady(lookup FactLookup) error {
	if err := a.guardNotSubmitted(); err != nil {
		return err
	}
	if a.State != StateAudited {
		return fmt.Errorf("%w: cannot promote to ReadyToRender from state %q", ErrInvalidStateTransition, a.State)
	}
	if err := validateAuditBundle(a.Audits); err != nil {
		return err
	}
	if a.Draft == nil {
		return fmt.Errorf("%w: application has no draft", ErrUnverifiedFactOrPlaceholder)
	}
	if lookup == nil {
		return fmt.Errorf("%w: fact lookup is required", ErrUnverifiedFactOrPlaceholder)
	}

	for _, d := range a.Audits.FactCheck.Discrepancies {
		if strings.EqualFold(string(d.Severity), string(SeverityHigh)) {
			return fmt.Errorf("%w: high-severity factual discrepancy on bullet %q: %s", ErrUnverifiedFactOrPlaceholder, d.BulletID, d.Finding)
		}
	}

	if ContainsMetricPlaceholder(a.Draft.Headline) {
		return fmt.Errorf("%w: headline contains unresolved [METRIC: placeholder", ErrUnverifiedFactOrPlaceholder)
	}
	if ContainsMetricPlaceholder(a.Draft.Summary) {
		return fmt.Errorf("%w: summary contains unresolved [METRIC: placeholder", ErrUnverifiedFactOrPlaceholder)
	}

	bullets := a.Draft.AllBullets()
	if len(bullets) == 0 {
		return fmt.Errorf("%w: draft contains zero bullets", ErrUnverifiedFactOrPlaceholder)
	}

	for idx, bullet := range bullets {
		if ContainsMetricPlaceholder(bullet.Text) {
			return fmt.Errorf("%w: bullet[%d] (%q) contains unresolved [METRIC: placeholder", ErrUnverifiedFactOrPlaceholder, idx, bullet.ID)
		}
		factIDs := trimNonEmpty(bullet.FactIDs)
		if len(factIDs) == 0 {
			return fmt.Errorf("%w: bullet[%d] (%q) cites zero facts", ErrUnverifiedFactOrPlaceholder, idx, bullet.ID)
		}
		for _, fid := range factIDs {
			fact, err := lookup.GetCareerFactByID(fid)
			if err != nil || fact == nil {
				return fmt.Errorf("%w: bullet[%d] (%q) cites missing fact %q", ErrUnverifiedFactOrPlaceholder, idx, bullet.ID, fid)
			}
			if !fact.Verified || fact.EpistemicState != EpistemicVerified {
				return fmt.Errorf("%w: bullet[%d] (%q) cites unverified fact %q (epistemic_state=%q)", ErrUnverifiedFactOrPlaceholder, idx, bullet.ID, fid, fact.EpistemicState)
			}
		}
		bullet.Verified = true
	}

	a.State = StateReadyToRender
	a.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkExported transitions ReadyToRender (or re-exports from Exported) -> Exported,
// verifying populated PII header, generated artifact paths, and page_count <= max_pages.
func (a *JobApplication) MarkExported(pii *PIIHeader, export ExportRecord) error {
	if err := a.guardNotSubmitted(); err != nil {
		return err
	}
	if a.State != StateReadyToRender && a.State != StateExported {
		return fmt.Errorf("%w: cannot mark exported from state %q", ErrInvalidStateTransition, a.State)
	}
	if pii == nil || strings.TrimSpace(pii.FullName) == "" {
		return fmt.Errorf("%w: full_name must be populated before exporting", ErrMissingPIIHeader)
	}
	if export.MaxPages <= 0 || export.PageCount <= 0 || export.PageCount > export.MaxPages {
		return fmt.Errorf("%w: page_count=%d max_pages=%d", ErrPageBudgetOverflow, export.PageCount, export.MaxPages)
	}
	if strings.TrimSpace(export.HTMLPath) == "" || strings.TrimSpace(export.DocxPath) == "" || strings.TrimSpace(export.PDFPath) == "" {
		return fmt.Errorf("%w: html, docx, and pdf paths must all be recorded", ErrMissingExportArtifact)
	}

	now := time.Now().UTC()
	exportCopy := export
	if exportCopy.ExportedAt.IsZero() {
		exportCopy.ExportedAt = now
	}
	a.Export = &exportCopy
	a.State = StateExported
	a.UpdatedAt = now
	return nil
}

// MarkSubmitted transitions Exported -> Submitted and locks the application immutable
// with a SHA-256 bundle hash and materialized interview_brief.md path.
func (a *JobApplication) MarkSubmitted(bundleSHA256, interviewBriefPath string) error {
	if err := a.guardNotSubmitted(); err != nil {
		return err
	}
	if a.State != StateExported {
		return fmt.Errorf("%w: cannot mark submitted from state %q", ErrInvalidStateTransition, a.State)
	}
	trimmedHash := strings.ToLower(strings.TrimSpace(bundleSHA256))
	if !isValidSHA256Hex(trimmedHash) {
		return fmt.Errorf("%w: invalid bundle SHA-256 hash %q", ErrMissingSubmissionArtifact, bundleSHA256)
	}
	trimmedBrief := strings.TrimSpace(interviewBriefPath)
	if trimmedBrief == "" {
		return fmt.Errorf("%w: interview_brief.md path is required", ErrMissingSubmissionArtifact)
	}

	now := time.Now().UTC()
	a.BundleSHA256 = trimmedHash
	a.InterviewBriefPath = trimmedBrief
	a.SubmittedAt = &now
	a.State = StateSubmitted
	a.UpdatedAt = now
	return nil
}

func (a *JobApplication) guardNotSubmitted() error {
	if a == nil {
		return fmt.Errorf("%w: nil JobApplication", ErrInvalidStateTransition)
	}
	if a.State == StateSubmitted {
		return fmt.Errorf("%w: %w", ErrAlreadySubmitted, ErrInvalidStateTransition)
	}
	return nil
}

func validateAuditBundle(bundle *AuditBundle) error {
	if bundle == nil || bundle.FactCheck == nil || bundle.AlignmentCheck == nil {
		return ErrIncompleteAudit
	}
	fcAgent := strings.TrimSpace(bundle.FactCheck.CriticAgent)
	if fcAgent != "" && fcAgent != CriticFactChecker {
		return fmt.Errorf("%w: unexpected fact-check critic %q", ErrIncompleteAudit, fcAgent)
	}
	if strings.TrimSpace(bundle.FactCheck.Summary) == "" {
		return fmt.Errorf("%w: fact-check summary cannot be empty", ErrIncompleteAudit)
	}
	for i, d := range bundle.FactCheck.Discrepancies {
		if err := d.Severity.Validate(); err != nil {
			return fmt.Errorf("%w: discrepancies[%d]: %v", ErrIncompleteAudit, i, err)
		}
	}

	acAgent := strings.TrimSpace(bundle.AlignmentCheck.CriticAgent)
	if acAgent != "" && acAgent != CriticAlignmentChecker {
		return fmt.Errorf("%w: unexpected alignment-check critic %q", ErrIncompleteAudit, acAgent)
	}
	if strings.TrimSpace(bundle.AlignmentCheck.Summary) == "" {
		return fmt.Errorf("%w: alignment-check summary cannot be empty", ErrIncompleteAudit)
	}
	return nil
}

func trimNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func isValidSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isDigit := c >= '0' && c <= '9'
		isHex := c >= 'a' && c <= 'f'
		if !isDigit && !isHex {
			return false
		}
	}
	return true
}
