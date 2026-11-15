package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kidthales/resumai/internal/domain"
)

func (s *Server) registerVaultTools() {
	s.registerTool(Tool{
		Name:        "resumai_vault_status",
		Description: "Return non-PII summary counts of organizations, roles, career facts by epistemic state, education, archetypes, and job applications.",
		InputSchema: schemaObject(nil),
	}, s.handleVaultStatus)

	s.registerTool(Tool{
		Name:        "resumai_get_logistics",
		Description: "Retrieve non-PII candidate logistics and job search preferences.",
		InputSchema: schemaObject(nil),
	}, s.handleGetLogistics)

	s.registerTool(Tool{
		Name:        "resumai_save_logistics",
		Description: "Create or update non-PII candidate logistics and job search preferences.",
		InputSchema: schemaObject(map[string]any{
			"target_title":        strProp("Target role title"),
			"work_authorization":  strProp("Work authorization status"),
			"remote_preference":   strProp("Remote/hybrid/onsite preference"),
			"willing_to_relocate": boolProp("Whether candidate is willing to relocate"),
			"target_compensation": strProp("Target compensation range"),
			"notice_period_days":  intProp("Notice period in days"),
		}),
	}, s.handleSaveLogistics)

	s.registerTool(Tool{
		Name:        "resumai_list_organizations",
		Description: "List all organizations stored in the CareerVault.",
		InputSchema: schemaObject(nil),
	}, s.handleListOrganizations)

	s.registerTool(Tool{
		Name:        "resumai_save_organization",
		Description: "Create or update an organization record in the CareerVault.",
		InputSchema: schemaObject(map[string]any{
			"id":       strProp("Unique organization identifier (slug)"),
			"name":     strProp("Organization display name"),
			"location": strProp("Organization location"),
			"website":  strProp("Organization website URL"),
		}, "id", "name"),
	}, s.handleSaveOrganization)

	s.registerTool(Tool{
		Name:        "resumai_list_roles",
		Description: "List roles in the CareerVault, optionally filtered by org_id.",
		InputSchema: schemaObject(map[string]any{
			"org_id": strProp("Optional organization ID filter"),
		}),
	}, s.handleListRoles)

	s.registerTool(Tool{
		Name:        "resumai_save_role",
		Description: "Create or update a role under an existing organization in the CareerVault.",
		InputSchema: schemaObject(map[string]any{
			"id":         strProp("Unique role identifier (slug)"),
			"org_id":     strProp("Parent organization ID"),
			"title":      strProp("Role title"),
			"category":   strProp("Role category (e.g. work, extra, volunteer)"),
			"location":   strProp("Role location"),
			"duration":   strProp("Role date range or duration"),
			"tech_stack": strArrayProp("Technologies used in this role"),
		}, "id", "org_id", "title"),
	}, s.handleSaveRole)

	s.registerTool(Tool{
		Name:        "resumai_list_facts",
		Description: "List career facts in the CareerVault, optionally filtered by role_id and/or epistemic_state.",
		InputSchema: schemaObject(map[string]any{
			"role_id":         strProp("Optional role ID filter"),
			"epistemic_state": strProp("Optional epistemic state filter (raw_note, metric_pending, verified, archived)"),
		}),
	}, s.handleListFacts)

	s.registerTool(Tool{
		Name:        "resumai_save_fact",
		Description: "Create or update an atomic career fact under an existing role.",
		InputSchema: schemaObject(map[string]any{
			"id":              strProp("Unique career fact identifier (slug)"),
			"role_id":         strProp("Parent role ID"),
			"headline":        strProp("Short fact headline"),
			"details":         strProp("Full ground-truth accomplishment details"),
			"epistemic_state": strProp("Epistemic state: raw_note, metric_pending, verified, or archived"),
			"tech_tags":       strArrayProp("Relevant technical tags"),
		}, "id", "role_id", "details"),
	}, s.handleSaveFact)

	s.registerTool(Tool{
		Name:        "resumai_archive_fact",
		Description: "Soft-archive a career fact (sets epistemic_state=archived, verified=false) while preserving submitted provenance.",
		InputSchema: schemaObject(map[string]any{
			"fact_id": strProp("Career fact ID to archive"),
		}, "fact_id"),
	}, s.handleArchiveFact)

	s.registerTool(Tool{
		Name:        "resumai_list_education",
		Description: "List all education credentials stored in the CareerVault.",
		InputSchema: schemaObject(nil),
	}, s.handleListEducation)

	s.registerTool(Tool{
		Name:        "resumai_save_education",
		Description: "Create or update an education credential in the CareerVault.",
		InputSchema: schemaObject(map[string]any{
			"id":             strProp("Unique education identifier (slug)"),
			"institution":    strProp("Institution name"),
			"degree":         strProp("Degree name"),
			"field_of_study": strProp("Field of study / major"),
			"location":       strProp("Institution location"),
			"duration":       strProp("Attendance dates or duration"),
			"honors":         strProp("Academic honors or distinctions"),
		}, "id", "institution"),
	}, s.handleSaveEducation)

	s.registerTool(Tool{
		Name:        "resumai_list_archetypes",
		Description: "List all resume persona archetypes stored in the CareerVault.",
		InputSchema: schemaObject(nil),
	}, s.handleListArchetypes)

	s.registerTool(Tool{
		Name:        "resumai_save_archetype",
		Description: "Create or update a resume persona archetype in the CareerVault.",
		InputSchema: schemaObject(map[string]any{
			"id":                strProp("Unique archetype identifier (slug)"),
			"name":              strProp("Archetype display name"),
			"profile_overview":  strProp("Target executive/profile summary framing"),
			"core_competencies": strArrayProp("Core competency themes"),
			"strategic_impact":  strArrayProp("Strategic impact pillars"),
			"target_roles":      strArrayProp("Target role titles"),
		}, "id", "name"),
	}, s.handleSaveArchetype)
}

func (s *Server) handleVaultStatus(ctx context.Context, _ json.RawMessage) (any, error) {
	orgs, err := s.vault.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.vault.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	facts, err := s.vault.ListCareerFacts(ctx)
	if err != nil {
		return nil, err
	}
	edu, err := s.vault.ListEducation(ctx)
	if err != nil {
		return nil, err
	}
	archs, err := s.vault.ListArchetypes(ctx)
	if err != nil {
		return nil, err
	}
	apps, err := s.vault.ListApplications(ctx)
	if err != nil {
		return nil, err
	}
	hasPII, err := s.vault.HasPII(ctx)
	if err != nil {
		return nil, err
	}

	logisticsConfigured := true
	if _, err := s.vault.GetCandidateLogistics(ctx); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			logisticsConfigured = false
		} else {
			return nil, err
		}
	}

	factsByState := map[string]int{
		string(domain.EpistemicRawNote):       0,
		string(domain.EpistemicMetricPending): 0,
		string(domain.EpistemicVerified):      0,
		string(domain.EpistemicArchived):      0,
	}
	for _, f := range facts {
		factsByState[string(f.EpistemicState)]++
	}

	appsByState := map[string]int{
		string(domain.StateTriaged):             0,
		string(domain.StateDeclinedByCandidate): 0,
		string(domain.StateDrafted):             0,
		string(domain.StateAudited):             0,
		string(domain.StateReadyToRender):       0,
		string(domain.StateExported):            0,
		string(domain.StateSubmitted):           0,
	}
	for _, a := range apps {
		appsByState[string(a.State)]++
	}

	return map[string]any{
		"organizations_count":   len(orgs),
		"roles_count":           len(roles),
		"facts_total":           len(facts),
		"facts_by_state":        factsByState,
		"education_count":       len(edu),
		"archetypes_count":      len(archs),
		"applications_total":    len(apps),
		"applications_by_state": appsByState,
		"logistics_configured":  logisticsConfigured,
		"pii_configured":        hasPII,
	}, nil
}

func (s *Server) handleGetLogistics(ctx context.Context, _ json.RawMessage) (any, error) {
	cl, err := s.vault.GetCandidateLogistics(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return map[string]any{"configured": false}, nil
		}
		return nil, err
	}
	return map[string]any{
		"configured": true,
		"logistics":  cl,
	}, nil
}

func (s *Server) handleSaveLogistics(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		TargetTitle        string `json:"target_title"`
		WorkAuthorization  string `json:"work_authorization"`
		RemotePreference   string `json:"remote_preference"`
		WillingToRelocate  bool   `json:"willing_to_relocate"`
		TargetCompensation string `json:"target_compensation"`
		NoticePeriodDays   int    `json:"notice_period_days"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}

	cl := &domain.CandidateLogistics{
		TargetTitle:        strings.TrimSpace(in.TargetTitle),
		WorkAuthorization:  strings.TrimSpace(in.WorkAuthorization),
		RemotePreference:   strings.TrimSpace(in.RemotePreference),
		WillingToRelocate:  in.WillingToRelocate,
		TargetCompensation: strings.TrimSpace(in.TargetCompensation),
		NoticePeriodDays:   in.NoticePeriodDays,
	}
	if err := s.vault.SetCandidateLogistics(ctx, cl); err != nil {
		return nil, err
	}
	return cl, nil
}

func (s *Server) handleListOrganizations(ctx context.Context, _ json.RawMessage) (any, error) {
	orgs, err := s.vault.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	if orgs == nil {
		orgs = []*domain.Organization{}
	}
	return map[string]any{"organizations": orgs}, nil
}

func (s *Server) handleSaveOrganization(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Location string `json:"location"`
		Website  string `json:"website"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("id and name are required")
	}

	org := &domain.Organization{
		ID:       strings.TrimSpace(in.ID),
		Name:     strings.TrimSpace(in.Name),
		Location: strings.TrimSpace(in.Location),
		Website:  strings.TrimSpace(in.Website),
	}
	if existing, gerr := s.vault.GetOrganization(ctx, org.ID); gerr == nil && existing != nil {
		org.CreatedAt = existing.CreatedAt
	}
	if err := s.vault.SaveOrganization(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *Server) handleListRoles(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		OrgID string `json:"org_id"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}

	var roles []*domain.Role
	if orgID := strings.TrimSpace(in.OrgID); orgID != "" {
		roles, err = s.vault.ListRolesByOrg(ctx, orgID)
	} else {
		roles, err = s.vault.ListRoles(ctx)
	}
	if err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []*domain.Role{}
	}
	return map[string]any{"roles": roles}, nil
}

func (s *Server) handleSaveRole(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ID        string   `json:"id"`
		OrgID     string   `json:"org_id"`
		Title     string   `json:"title"`
		Category  string   `json:"category"`
		Location  string   `json:"location"`
		Duration  string   `json:"duration"`
		TechStack []string `json:"tech_stack"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.OrgID) == "" || strings.TrimSpace(in.Title) == "" {
		return nil, errors.New("id, org_id, and title are required")
	}
	if _, err := s.vault.GetOrganization(ctx, strings.TrimSpace(in.OrgID)); err != nil {
		return nil, fmt.Errorf("parent organization %q: %w", in.OrgID, err)
	}

	category := strings.TrimSpace(in.Category)
	if category == "" {
		category = "work"
	}
	role := &domain.Role{
		ID:        strings.TrimSpace(in.ID),
		OrgID:     strings.TrimSpace(in.OrgID),
		Title:     strings.TrimSpace(in.Title),
		Category:  category,
		Location:  strings.TrimSpace(in.Location),
		Duration:  strings.TrimSpace(in.Duration),
		TechStack: in.TechStack,
	}
	if existing, gerr := s.vault.GetRole(ctx, role.ID); gerr == nil && existing != nil {
		role.CreatedAt = existing.CreatedAt
	}
	if err := s.vault.SaveRole(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *Server) handleListFacts(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		RoleID         string `json:"role_id"`
		EpistemicState string `json:"epistemic_state"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}

	var stateFilter domain.EpistemicState
	if trimmedState := strings.TrimSpace(in.EpistemicState); trimmedState != "" {
		stateFilter = domain.EpistemicState(trimmedState)
		if err := stateFilter.Validate(); err != nil {
			return nil, err
		}
	}

	var facts []*domain.CareerFact
	if roleID := strings.TrimSpace(in.RoleID); roleID != "" {
		facts, err = s.vault.ListCareerFactsByRole(ctx, roleID)
	} else {
		facts, err = s.vault.ListCareerFacts(ctx)
	}
	if err != nil {
		return nil, err
	}

	filtered := make([]*domain.CareerFact, 0, len(facts))
	for _, f := range facts {
		if stateFilter != "" && f.EpistemicState != stateFilter {
			continue
		}
		filtered = append(filtered, f)
	}
	return map[string]any{"facts": filtered}, nil
}

func (s *Server) handleSaveFact(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ID             string   `json:"id"`
		RoleID         string   `json:"role_id"`
		Headline       string   `json:"headline"`
		Details        string   `json:"details"`
		EpistemicState string   `json:"epistemic_state"`
		TechTags       []string `json:"tech_tags"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.RoleID) == "" || strings.TrimSpace(in.Details) == "" {
		return nil, errors.New("id, role_id, and details are required")
	}
	if _, err := s.vault.GetRole(ctx, strings.TrimSpace(in.RoleID)); err != nil {
		return nil, fmt.Errorf("parent role %q: %w", in.RoleID, err)
	}

	epState := domain.EpistemicState(strings.TrimSpace(in.EpistemicState))
	if epState == "" {
		epState = domain.EpistemicRawNote
	}
	if err := epState.Validate(); err != nil {
		return nil, err
	}
	if epState == domain.EpistemicVerified && (domain.ContainsMetricPlaceholder(in.Headline) || domain.ContainsMetricPlaceholder(in.Details)) {
		return nil, fmt.Errorf("%w: cannot mark career fact verified while [METRIC: placeholder is present", domain.ErrUnverifiedFactOrPlaceholder)
	}

	fact := &domain.CareerFact{
		ID:             strings.TrimSpace(in.ID),
		RoleID:         strings.TrimSpace(in.RoleID),
		Headline:       strings.TrimSpace(in.Headline),
		Details:        strings.TrimSpace(in.Details),
		EpistemicState: epState,
		TechTags:       in.TechTags,
		Verified:       epState == domain.EpistemicVerified,
	}
	if existing, gerr := s.vault.GetCareerFactContext(ctx, fact.ID); gerr == nil && existing != nil {
		fact.CreatedAt = existing.CreatedAt
	}
	if err := s.vault.SaveCareerFact(ctx, fact); err != nil {
		return nil, err
	}
	return fact, nil
}

func (s *Server) handleArchiveFact(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		FactID string `json:"fact_id"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	factID := strings.TrimSpace(in.FactID)
	if factID == "" {
		return nil, errors.New("fact_id is required")
	}
	if err := s.vault.ArchiveCareerFact(ctx, factID); err != nil {
		return nil, err
	}
	return map[string]any{
		"archived": true,
		"fact_id":  factID,
	}, nil
}

func (s *Server) handleListEducation(ctx context.Context, _ json.RawMessage) (any, error) {
	list, err := s.vault.ListEducation(ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.Education{}
	}
	return map[string]any{"education": list}, nil
}

func (s *Server) handleSaveEducation(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ID           string `json:"id"`
		Institution  string `json:"institution"`
		Degree       string `json:"degree"`
		FieldOfStudy string `json:"field_of_study"`
		Location     string `json:"location"`
		Duration     string `json:"duration"`
		Honors       string `json:"honors"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Institution) == "" {
		return nil, errors.New("id and institution are required")
	}

	edu := &domain.Education{
		ID:           strings.TrimSpace(in.ID),
		Institution:  strings.TrimSpace(in.Institution),
		Degree:       strings.TrimSpace(in.Degree),
		FieldOfStudy: strings.TrimSpace(in.FieldOfStudy),
		Location:     strings.TrimSpace(in.Location),
		Duration:     strings.TrimSpace(in.Duration),
		Honors:       strings.TrimSpace(in.Honors),
	}
	if existing, gerr := s.vault.GetEducation(ctx, edu.ID); gerr == nil && existing != nil {
		edu.CreatedAt = existing.CreatedAt
	}
	if err := s.vault.SaveEducation(ctx, edu); err != nil {
		return nil, err
	}
	return edu, nil
}

func (s *Server) handleListArchetypes(ctx context.Context, _ json.RawMessage) (any, error) {
	list, err := s.vault.ListArchetypes(ctx)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []*domain.Archetype{}
	}
	return map[string]any{"archetypes": list}, nil
}

func (s *Server) handleSaveArchetype(ctx context.Context, raw json.RawMessage) (any, error) {
	type input struct {
		ID               string   `json:"id"`
		Name             string   `json:"name"`
		ProfileOverview  string   `json:"profile_overview"`
		CoreCompetencies []string `json:"core_competencies"`
		StrategicImpact  []string `json:"strategic_impact"`
		TargetRoles      []string `json:"target_roles"`
	}
	in, err := decodeArgs[input](raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("id and name are required")
	}

	arch := &domain.Archetype{
		ID:               strings.TrimSpace(in.ID),
		Name:             strings.TrimSpace(in.Name),
		ProfileOverview:  strings.TrimSpace(in.ProfileOverview),
		CoreCompetencies: in.CoreCompetencies,
		StrategicImpact:  in.StrategicImpact,
		TargetRoles:      in.TargetRoles,
	}
	if existing, gerr := s.vault.GetArchetype(ctx, arch.ID); gerr == nil && existing != nil {
		arch.CreatedAt = existing.CreatedAt
	}
	if err := s.vault.SaveArchetype(ctx, arch); err != nil {
		return nil, err
	}
	return arch, nil
}
