package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/kidthales/resumai/internal/domain"
)

func (s *Server) registerApplicationTools() {
	s.registerTool(Tool{
		Name:        "resumai_list_applications",
		Description: "List job applications stored in the CareerVault, optionally filtered by FSM state.",
		InputSchema: schemaObject(map[string]any{
			"state": strProp("Optional FSM state filter (Triaged, DeclinedByCandidate, Drafted, Audited, ReadyToRender, Exported, Submitted)"),
		}),
	}, s.handleListApplications)

	s.registerTool(Tool{
		Name:        "resumai_get_application",
		Description: "Retrieve full non-PII JobApplication state, dossier, draft, audits, and export metadata by ID.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
		}, "application_id"),
	}, s.handleGetApplication)

	s.registerTool(Tool{
		Name:        "resumai_save_triage",
		Description: "Create or update a Triaged JobApplication from an OpportunityDossier, enforcing verbatim JD quote substrings and external claim URL/unknown_ask_recruiter citations.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("Optional application ID (auto-generated slug if omitted)"),
			"dossier": map[string]any{
				"type":        "object",
				"description": "Structured OpportunityDossier with raw_jd, jd_claims, and external_claims",
			},
			"role_title":      strProp("Target role title (used if dossier.role_title is omitted)"),
			"company":         strProp("Target company name (used if dossier.company is omitted)"),
			"raw_jd":          strProp("Full raw job description text (used if dossier.raw_jd is omitted)"),
			"fit_summary":     strProp("Candidate fit summary (used if dossier.fit_summary is omitted)"),
			"jd_claims":       map[string]any{"type": "array", "description": "Extracted JD claims with verbatim_jd_quote"},
			"external_claims": map[string]any{"type": "array", "description": "External claims with source_url or unknown_ask_recruiter"},
		}),
	}, s.handleSaveTriage)

	s.registerTool(Tool{
		Name:        "resumai_pass_role",
		Description: "Transition a Triaged application to DeclinedByCandidate with a required rejection rationale.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
			"rationale":      strProp("Non-empty candidate decline rationale"),
		}, "application_id", "rationale"),
	}, s.handlePassRole)

	s.registerTool(Tool{
		Name:        "resumai_save_draft",
		Description: "Save a tailored ApplicationDraft on a Triaged/Drafted/Audited/ReadyToRender/Exported application, verifying archetype_id and grounding every bullet fact_id in CareerVault.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
			"draft": map[string]any{
				"type":        "object",
				"description": "ApplicationDraft AST with archetype_id, sections, and fact-cited bullets",
			},
		}, "application_id", "draft"),
	}, s.handleSaveDraft)

	s.registerTool(Tool{
		Name:        "resumai_get_audit_bundle",
		Description: "Return the clean-room, strictly air-gapped audit bundle (raw JD, dossier, draft, cited vault career facts, roles, orgs, education, and logistics) for critic verification.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
		}, "application_id"),
	}, s.handleGetAuditBundle)

	s.registerTool(Tool{
		Name:        "resumai_record_audits",
		Description: "Attach both resume-fact-checker and resume-alignment-checker reports to transition Drafted -> Audited.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
			"fact_check": map[string]any{
				"type":        "object",
				"description": "FactCheckReport from resume-fact-checker",
			},
			"alignment_check": map[string]any{
				"type":        "object",
				"description": "AlignmentCheckReport from resume-alignment-checker",
			},
		}, "application_id", "fact_check", "alignment_check"),
	}, s.handleRecordAudits)

	s.registerTool(Tool{
		Name:        "resumai_promote_ready",
		Description: "Promote an Audited application to ReadyToRender after verifying 0 High factual discrepancies, 100% verified cited facts, and 0 [METRIC: placeholders.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
		}, "application_id"),
	}, s.handlePromoteReady)

	s.registerTool(Tool{
		Name:        "resumai_render_bundle",
		Description: "Materialize local PII-injected resume.{html,docx,pdf} and audit.md packet and transition ReadyToRender -> Exported if page_count <= max_pages.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
			"theme":          strProp("Optional theme name (default, modern, classic)"),
			"output_dir":     strProp("Optional subdirectory or jailed base output directory inside the configured applications root (defaults to ~/.resumai/applications)"),
		}, "application_id"),
	}, s.handleRenderBundle)

	s.registerTool(Tool{
		Name:        "resumai_mark_submitted",
		Description: "Compute SHA-256 bundle hash, materialize interview_brief.md, lock packet artifacts immutable, and transition Exported -> Submitted.",
		InputSchema: schemaObject(map[string]any{
			"application_id": strProp("JobApplication ID"),
			"notes":          strProp("Optional candidate interview preparation notes"),
		}, "application_id"),
	}, s.handleMarkSubmitted)
}

func (s *Server) handleListApplications(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		State string `json:"state"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}

	var stateFilter domain.ApplicationState
	if trimmed := strings.TrimSpace(in.State); trimmed != "" {
		stateFilter = domain.ApplicationState(trimmed)
		if err := stateFilter.Validate(); err != nil {
			return nil, err
		}
	}

	apps, err := s.vault.ListApplications(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]*domain.JobApplication, 0, len(apps))
	for _, a := range apps {
		if stateFilter != "" && a.State != stateFilter {
			continue
		}
		filtered = append(filtered, a)
	}
	return map[string]any{"applications": filtered}, nil
}

func (s *Server) handleGetApplication(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID string `json:"application_id"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}
	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"application": app}, nil
}

func (s *Server) handleSaveTriage(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID  string                     `json:"application_id"`
		Dossier        *domain.OpportunityDossier `json:"dossier"`
		RoleTitle      string                     `json:"role_title"`
		Company        string                     `json:"company"`
		RawJD          string                     `json:"raw_jd"`
		FitSummary     string                     `json:"fit_summary"`
		JDClaims       []domain.JDClaim           `json:"jd_claims"`
		ExternalClaims []domain.ExternalClaim     `json:"external_claims"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}

	var dossier domain.OpportunityDossier
	if in.Dossier != nil {
		dossier = *in.Dossier
	}
	if dossier.RoleTitle == "" {
		dossier.RoleTitle = in.RoleTitle
	}
	if dossier.Company == "" {
		dossier.Company = in.Company
	}
	if dossier.RawJD == "" {
		dossier.RawJD = in.RawJD
	}
	if dossier.FitSummary == "" {
		dossier.FitSummary = in.FitSummary
	}
	if len(dossier.JDClaims) == 0 && len(in.JDClaims) > 0 {
		dossier.JDClaims = in.JDClaims
	}
	if len(dossier.ExternalClaims) == 0 && len(in.ExternalClaims) > 0 {
		dossier.ExternalClaims = in.ExternalClaims
	}

	explicitID := strings.TrimSpace(in.ApplicationID)
	if explicitID != "" {
		existing, gerr := s.vault.GetApplicationContext(ctx, explicitID)
		if gerr == nil && existing != nil {
			if err := existing.ApplyTriage(dossier); err != nil {
				return nil, err
			}
			if err := s.vault.SaveApplication(ctx, existing); err != nil {
				return nil, err
			}
			return map[string]any{"application": existing}, nil
		}
		if gerr != nil && !errors.Is(gerr, domain.ErrNotFound) {
			return nil, gerr
		}

		app, err := domain.NewTriagedApplication(explicitID, dossier)
		if err != nil {
			return nil, err
		}
		if err := s.vault.SaveApplication(ctx, app); err != nil {
			return nil, err
		}
		return map[string]any{"application": app}, nil
	}

	// Auto-generate deterministic slug ID and avoid accidental collision with existing IDs.
	baseID := generateApplicationID(dossier.Company, dossier.RoleTitle)
	appID := baseID
	for suffix := 2; ; suffix++ {
		_, gerr := s.vault.GetApplicationContext(ctx, appID)
		if errors.Is(gerr, domain.ErrNotFound) {
			break
		}
		if gerr != nil {
			return nil, gerr
		}
		appID = fmt.Sprintf("%s-%d", baseID, suffix)
	}

	app, err := domain.NewTriagedApplication(appID, dossier)
	if err != nil {
		return nil, err
	}
	if err := s.vault.SaveApplication(ctx, app); err != nil {
		return nil, err
	}
	return map[string]any{"application": app}, nil
}

func (s *Server) handlePassRole(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID string `json:"application_id"`
		Rationale     string `json:"rationale"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}

	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	if err := app.PassRole(in.Rationale); err != nil {
		return nil, err
	}
	if err := s.vault.SaveApplication(ctx, app); err != nil {
		return nil, err
	}
	return map[string]any{"application": app}, nil
}

func (s *Server) handleSaveDraft(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID string                   `json:"application_id"`
		Draft         *domain.ApplicationDraft `json:"draft"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}
	if in.Draft == nil {
		return nil, fmt.Errorf("%w: draft is required", domain.ErrUngroundedBullet)
	}

	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	if err := app.SaveDraft(*in.Draft, s.vault); err != nil {
		return nil, err
	}
	if err := s.vault.SaveApplication(ctx, app); err != nil {
		return nil, err
	}

	cited := app.Draft.CitedFactIDs()
	if err := s.vault.RecordApplicationCitations(ctx, app.ID, cited); err != nil {
		return nil, err
	}
	return map[string]any{
		"application":    app,
		"cited_fact_ids": cited,
	}, nil
}

func (s *Server) handleGetAuditBundle(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID string `json:"application_id"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}

	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app.Draft == nil {
		return nil, fmt.Errorf("%w: application %q has no saved draft to audit", domain.ErrInvalidStateTransition, app.ID)
	}

	citedIDs := app.Draft.CitedFactIDs()
	citedFacts := make([]*domain.CareerFact, 0, len(citedIDs))
	missingFactIDs := make([]string, 0)
	roleMap := make(map[string]*domain.Role)
	orgMap := make(map[string]*domain.Organization)

	for _, fid := range citedIDs {
		fact, ferr := s.vault.GetCareerFactContext(ctx, fid)
		if ferr != nil {
			if errors.Is(ferr, domain.ErrNotFound) {
				missingFactIDs = append(missingFactIDs, fid)
				continue
			}
			return nil, ferr
		}
		citedFacts = append(citedFacts, fact)

		if _, seenRole := roleMap[fact.RoleID]; !seenRole {
			if role, rerr := s.vault.GetRole(ctx, fact.RoleID); rerr == nil && role != nil {
				roleMap[role.ID] = role
				if _, seenOrg := orgMap[role.OrgID]; !seenOrg {
					if org, oerr := s.vault.GetOrganization(ctx, role.OrgID); oerr == nil && org != nil {
						orgMap[org.ID] = org
					}
				}
			}
		}
	}

	roles := make([]*domain.Role, 0, len(roleMap))
	for _, r := range roleMap {
		roles = append(roles, r)
	}
	sort.Slice(roles, func(i, j int) bool {
		return roles[i].ID < roles[j].ID
	})
	orgs := make([]*domain.Organization, 0, len(orgMap))
	for _, o := range orgMap {
		orgs = append(orgs, o)
	}
	sort.Slice(orgs, func(i, j int) bool {
		return orgs[i].ID < orgs[j].ID
	})

	var archetype *domain.Archetype
	if app.ArchetypeID != "" {
		if arch, aerr := s.vault.GetArchetype(ctx, app.ArchetypeID); aerr == nil {
			archetype = arch
		}
	}

	education, err := s.vault.ListEducation(ctx)
	if err != nil {
		return nil, err
	}
	if education == nil {
		education = []*domain.Education{}
	}

	var logistics *domain.CandidateLogistics
	if cl, lerr := s.vault.GetCandidateLogistics(ctx); lerr == nil {
		logistics = cl
	}

	return map[string]any{
		"application_id":   app.ID,
		"role_title":       app.RoleTitle,
		"company":          app.Company,
		"state":            app.State,
		"raw_jd":           app.RawJD,
		"dossier":          app.Dossier,
		"archetype":        archetype,
		"draft":            app.Draft,
		"cited_facts":      citedFacts,
		"missing_fact_ids": missingFactIDs,
		"roles":            roles,
		"organizations":    orgs,
		"education":        education,
		"logistics":        logistics,
		"existing_audits":  app.Audits,
	}, nil
}

func (s *Server) handleRecordAudits(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID  string                       `json:"application_id"`
		FactCheck      *domain.FactCheckReport      `json:"fact_check"`
		AlignmentCheck *domain.AlignmentCheckReport `json:"alignment_check"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}

	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	if err := app.RecordAudits(domain.AuditBundle{
		FactCheck:      in.FactCheck,
		AlignmentCheck: in.AlignmentCheck,
	}); err != nil {
		return nil, err
	}
	if err := s.vault.SaveApplication(ctx, app); err != nil {
		return nil, err
	}
	return map[string]any{"application": app}, nil
}

func (s *Server) handlePromoteReady(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID string `json:"application_id"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}

	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	if err := app.PromoteReady(s.vault); err != nil {
		return nil, err
	}
	if err := s.vault.SaveApplication(ctx, app); err != nil {
		return nil, err
	}
	if app.Draft != nil {
		if err := s.vault.RecordApplicationCitations(ctx, app.ID, app.Draft.CitedFactIDs()); err != nil {
			return nil, err
		}
	}
	return map[string]any{"application": app}, nil
}

func (s *Server) handleRenderBundle(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID string `json:"application_id"`
		Theme         string `json:"theme"`
		OutputDir     string `json:"output_dir"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}

	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app.State == domain.StateSubmitted {
		return nil, domain.ErrAlreadySubmitted
	}
	if app.State != domain.StateReadyToRender && app.State != domain.StateExported {
		return nil, fmt.Errorf("%w: cannot render bundle from state %q", domain.ErrInvalidStateTransition, app.State)
	}
	if s.materializer == nil {
		return nil, errors.New("packet materializer is not configured")
	}

	result, err := s.materializer.RenderAndMarkExported(ctx, RenderBundleRequest{
		Application: app,
		Theme:       strings.TrimSpace(in.Theme),
		OutputDir:   strings.TrimSpace(in.OutputDir),
	})
	if err != nil {
		return nil, err
	}
	if err := s.vault.SaveApplication(ctx, app); err != nil {
		return nil, err
	}
	return map[string]any{
		"application": app,
		"export":      result.Export,
		"telemetry":   result.Telemetry,
	}, nil
}

func (s *Server) handleMarkSubmitted(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ApplicationID string `json:"application_id"`
		Notes         string `json:"notes"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	appID := strings.TrimSpace(in.ApplicationID)
	if appID == "" {
		return nil, errors.New("application_id is required")
	}

	app, err := s.vault.GetApplicationContext(ctx, appID)
	if err != nil {
		return nil, err
	}
	if app.State == domain.StateSubmitted {
		return nil, domain.ErrAlreadySubmitted
	}
	if app.State != domain.StateExported {
		return nil, fmt.Errorf("%w: cannot mark submitted from state %q", domain.ErrInvalidStateTransition, app.State)
	}
	if s.materializer == nil {
		return nil, errors.New("packet materializer is not configured")
	}

	subRes, err := s.materializer.FinalizeSubmission(ctx, SubmissionRequest{
		Application: app,
		Notes:       strings.TrimSpace(in.Notes),
	})
	if err != nil {
		return nil, err
	}
	if err := app.MarkSubmitted(subRes.BundleSHA256, subRes.InterviewBriefPath); err != nil {
		return nil, err
	}
	if err := s.vault.SaveApplication(ctx, app); err != nil {
		return nil, err
	}
	return map[string]any{
		"application": app,
	}, nil
}

func generateApplicationID(company, roleTitle string) string {
	datePrefix := time.Now().UTC().Format("2006-01-02")
	orgSlug := slugify(company)
	roleSlug := slugify(roleTitle)
	switch {
	case orgSlug != "" && roleSlug != "":
		return fmt.Sprintf("%s-%s-%s", datePrefix, orgSlug, roleSlug)
	case orgSlug != "":
		return fmt.Sprintf("%s-%s", datePrefix, orgSlug)
	case roleSlug != "":
		return fmt.Sprintf("%s-%s", datePrefix, roleSlug)
	default:
		return fmt.Sprintf("%s-application", datePrefix)
	}
}

func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
