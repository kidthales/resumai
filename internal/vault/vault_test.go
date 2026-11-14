package vault_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/vault"
)

func newTestVault(t *testing.T) *vault.CareerVault {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_vault.db")
	v, err := vault.OpenPath(dbPath)
	if err != nil {
		t.Fatalf("failed to open test vault: %v", err)
	}
	t.Cleanup(func() {
		_ = v.Close()
	})
	return v
}

func TestMigrationsIdempotency(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Running migration again must be idempotent and succeed without error
	if err := v.Migrate(ctx); err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}

	var count int
	err := v.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations;").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count schema migrations: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 migrations recorded, got %d", count)
	}
}

func TestPIIHeader(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Initial get must return ErrNotFound
	_, err := v.GetPII(ctx)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unset PII, got %v", err)
	}

	// Set PII
	pii := &domain.PIIHeader{
		FullName:     "Jane Doe",
		Email:        "jane.doe@example.com",
		Phone:        "(555) 555-5555",
		Location:     "Krakoa, Pacific Ocean",
		LinkedInURL:  "https://linkedin.com/in/janedoe",
		GitHubURL:    "https://github.com/janedoe",
		PortfolioURL: "https://janedoe.dev",
	}

	if err := v.SetPII(ctx, pii); err != nil {
		t.Fatalf("failed to set PII: %v", err)
	}

	got, err := v.GetPII(ctx)
	if err != nil {
		t.Fatalf("failed to get PII: %v", err)
	}

	if got.FullName != pii.FullName || got.Email != pii.Email || got.Location != pii.Location {
		t.Errorf("PII mismatch: got %+v, want %+v", got, pii)
	}

	// Update PII
	pii.FullName = "Jane X. Doe"
	if err := v.SetPII(ctx, pii); err != nil {
		t.Fatalf("failed to update PII: %v", err)
	}

	gotUpdated, err := v.GetPII(ctx)
	if err != nil {
		t.Fatalf("failed to get updated PII: %v", err)
	}
	if gotUpdated.FullName != "Jane X. Doe" {
		t.Errorf("expected updated full name, got %s", gotUpdated.FullName)
	}
}

func TestCandidateLogistics(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Initial get must return ErrNotFound
	_, err := v.GetCandidateLogistics(ctx)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unset logistics, got %v", err)
	}

	cl := &domain.CandidateLogistics{
		TargetTitle:        "Principal Infrastructure Architect",
		WorkAuthorization:  "Citizen",
		RemotePreference:   "Remote / Hybrid",
		WillingToRelocate:  true,
		TargetCompensation: "$250k - $300k",
		NoticePeriodDays:   14,
	}

	if err := v.SetCandidateLogistics(ctx, cl); err != nil {
		t.Fatalf("failed to set logistics: %v", err)
	}

	got, err := v.GetCandidateLogistics(ctx)
	if err != nil {
		t.Fatalf("failed to get logistics: %v", err)
	}

	if got.TargetTitle != cl.TargetTitle || !got.WillingToRelocate || got.NoticePeriodDays != 14 {
		t.Errorf("logistics mismatch: got %+v, want %+v", got, cl)
	}
}

func TestOrganizationRoleFactLifecycle(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// 1. Organization
	org := &domain.Organization{
		ID:       "org-krakoan-bio",
		Name:     "Krakoan Habitat & Gateway Systems",
		Location: "Krakoa",
		Website:  "https://krakoa.mesh",
	}
	if err := v.SaveOrganization(ctx, org); err != nil {
		t.Fatalf("save organization failed: %v", err)
	}

	gotOrg, err := v.GetOrganization(ctx, org.ID)
	if err != nil {
		t.Fatalf("get organization failed: %v", err)
	}
	if gotOrg.Name != org.Name {
		t.Errorf("org name mismatch: got %s, want %s", gotOrg.Name, org.Name)
	}

	// 2. Role
	role := &domain.Role{
		ID:        "role-principal-architect",
		OrgID:     org.ID,
		Title:     "Principal Architect – Bio-Organic Infrastructures",
		Category:  "work",
		Location:  "Krakoa (On-site)",
		Duration:  "Oct 2019 to Sep 2026 (7 years)",
		TechStack: []string{"Krakoan Gateways", "Bio-Organic Telemetry", "Rust", "Linux"},
	}
	if err := v.SaveRole(ctx, role); err != nil {
		t.Fatalf("save role failed: %v", err)
	}

	gotRole, err := v.GetRole(ctx, role.ID)
	if err != nil {
		t.Fatalf("get role failed: %v", err)
	}
	if len(gotRole.TechStack) != 4 || gotRole.TechStack[2] != "Rust" {
		t.Errorf("role tech stack mismatch: got %v", gotRole.TechStack)
	}

	// 3. Career Fact with Epistemic States
	factRaw := &domain.CareerFact{
		ID:             "fact-krakoa-mesh",
		RoleID:         role.ID,
		Headline:       "Psionic Interface Telemetry",
		Details:        "Design fault-tolerant psionic interface protocols enabling seamless transit across global mutant habitats.",
		EpistemicState: domain.EpistemicRawNote,
		TechTags:       []string{"Telepathic Mesh", "Rust"},
		Verified:       false,
	}
	if err := v.SaveCareerFact(ctx, factRaw); err != nil {
		t.Fatalf("save raw fact failed: %v", err)
	}

	gotFact, err := v.GetCareerFact(factRaw.ID)
	if err != nil {
		t.Fatalf("get career fact failed: %v", err)
	}
	if gotFact.EpistemicState != domain.EpistemicRawNote || gotFact.Verified {
		t.Errorf("expected raw unverified fact, got state=%s, verified=%v", gotFact.EpistemicState, gotFact.Verified)
	}

	// Test Epistemic State Promotion
	factRaw.EpistemicState = domain.EpistemicVerified
	if err := v.SaveCareerFact(ctx, factRaw); err != nil {
		t.Fatalf("promote fact failed: %v", err)
	}

	gotPromoted, err := v.GetCareerFact(factRaw.ID)
	if err != nil {
		t.Fatalf("get promoted fact failed: %v", err)
	}
	if !gotPromoted.Verified || gotPromoted.EpistemicState != domain.EpistemicVerified {
		t.Errorf("expected verified state, got state=%s, verified=%v", gotPromoted.EpistemicState, gotPromoted.Verified)
	}

	// Invalid epistemic state rejected
	factInvalid := &domain.CareerFact{
		ID:             "fact-invalid",
		RoleID:         role.ID,
		Details:        "Invalid fact test",
		EpistemicState: domain.EpistemicState("bogus_state"),
	}
	if err := v.SaveCareerFact(ctx, factInvalid); !errors.Is(err, domain.ErrInvalidEpistemicState) {
		t.Fatalf("expected ErrInvalidEpistemicState, got %v", err)
	}
}

func TestSubmittedFactSoftArchiveGuard(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Setup Org, Role, Fact
	org := &domain.Organization{ID: "org-xcorp", Name: "X-Corporation"}
	_ = v.SaveOrganization(ctx, org)

	role := &domain.Role{ID: "role-director", OrgID: org.ID, Title: "Director"}
	_ = v.SaveRole(ctx, role)

	fact1 := &domain.CareerFact{
		ID:             "fact-xcorp-1",
		RoleID:         role.ID,
		Details:        "Built automated real-time compliance pipelines.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	}
	_ = v.SaveCareerFact(ctx, fact1)

	fact2 := &domain.CareerFact{
		ID:             "fact-xcorp-2",
		RoleID:         role.ID,
		Details:        "Managed distributed engineering teams.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	}
	_ = v.SaveCareerFact(ctx, fact2)

	// Create application in Drafted state citing fact1
	app := &domain.JobApplication{
		ID:        "app-test-01",
		RoleTitle: "Staff Systems Engineer",
		Company:   "Stark Enterprises",
		RawJD:     "Looking for staff engineer...",
		State:     domain.StateDrafted,
	}
	if err := v.SaveApplication(ctx, app); err != nil {
		t.Fatalf("save application failed: %v", err)
	}
	if err := v.RecordApplicationCitations(ctx, app.ID, []string{fact1.ID}); err != nil {
		t.Fatalf("record citations failed: %v", err)
	}

	// Because app is Drafted (NOT Submitted), fact1 CAN be deleted
	if err := v.DeleteCareerFact(ctx, fact1.ID); err != nil {
		t.Fatalf("deleting fact from Drafted application should succeed, got: %v", err)
	}

	// Now cite fact2 and advance application to Submitted state
	if err := v.RecordApplicationCitations(ctx, app.ID, []string{fact2.ID}); err != nil {
		t.Fatalf("record citations failed: %v", err)
	}
	app.State = domain.StateSubmitted
	if err := v.SaveApplication(ctx, app); err != nil {
		t.Fatalf("advance application to submitted failed: %v", err)
	}

	// Invariant: Deleting fact2, its role, or its org MUST fail because fact2 is cited in a Submitted application!
	if err := v.DeleteCareerFact(ctx, fact2.ID); !errors.Is(err, domain.ErrCannotDeleteSubmittedFact) {
		t.Fatalf("expected ErrCannotDeleteSubmittedFact on DeleteCareerFact, got: %v", err)
	}
	if err := v.DeleteRole(ctx, role.ID); !errors.Is(err, domain.ErrCannotDeleteSubmittedFact) {
		t.Fatalf("expected ErrCannotDeleteSubmittedFact on DeleteRole, got: %v", err)
	}
	if err := v.DeleteOrganization(ctx, org.ID); !errors.Is(err, domain.ErrCannotDeleteSubmittedFact) {
		t.Fatalf("expected ErrCannotDeleteSubmittedFact on DeleteOrganization, got: %v", err)
	}

	// Soft-archive should succeed instead
	if err := v.ArchiveCareerFact(ctx, fact2.ID); err != nil {
		t.Fatalf("soft-archive failed: %v", err)
	}

	archivedFact, err := v.GetCareerFact(fact2.ID)
	if err != nil {
		t.Fatalf("get archived fact failed: %v", err)
	}
	if archivedFact.EpistemicState != domain.EpistemicArchived || archivedFact.Verified {
		t.Errorf("expected archived state, got state=%s, verified=%v", archivedFact.EpistemicState, archivedFact.Verified)
	}
}

func TestEducationAndArchetypes(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Education
	edu := &domain.Education{
		ID:           "edu-mit-bsc",
		Institution:  "Massachusetts Institute of Technology",
		Degree:       "Bachelor of Science (B.Sc.)",
		FieldOfStudy: "Mutant Studies",
		Location:     "Cambridge, Massachusetts, USA",
		Duration:     "1984 to 1988 (4 years)",
		Honors:       "Magna Cum Laude",
	}
	if err := v.SaveEducation(ctx, edu); err != nil {
		t.Fatalf("save education failed: %v", err)
	}

	gotEdu, err := v.GetEducation(ctx, edu.ID)
	if err != nil {
		t.Fatalf("get education failed: %v", err)
	}
	if gotEdu.Institution != edu.Institution || gotEdu.Honors != "Magna Cum Laude" {
		t.Errorf("education mismatch: got %+v", gotEdu)
	}

	// Archetype
	arch := &domain.Archetype{
		ID:               "archetype-devops-sre",
		Name:             "DevOps & Site Reliability Engineer",
		ProfileOverview:  "An infrastructure and reliability specialist...",
		CoreCompetencies: []string{"Cloud & Containerization", "IaC", "CI/CD"},
		StrategicImpact:  []string{"Automates multi-region deployment pipelines"},
		TargetRoles:      []string{"Site Reliability Engineer", "Platform Engineer"},
	}
	if err := v.SaveArchetype(ctx, arch); err != nil {
		t.Fatalf("save archetype failed: %v", err)
	}

	gotArch, err := v.GetArchetype(ctx, arch.ID)
	if err != nil {
		t.Fatalf("get archetype failed: %v", err)
	}
	if gotArch.Name != arch.Name || len(gotArch.CoreCompetencies) != 3 {
		t.Errorf("archetype mismatch: got %+v", gotArch)
	}
}

func TestEpistemicStatePrecedence(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	org := &domain.Organization{ID: "org-prec", Name: "Precedence Corp"}
	_ = v.SaveOrganization(ctx, org)
	role := &domain.Role{ID: "role-prec", OrgID: org.ID, Title: "Engineer", Category: "work"}
	_ = v.SaveRole(ctx, role)

	// 1. Create a fact with Verified = true
	fact := &domain.CareerFact{
		ID:             "fact-prec-1",
		RoleID:         role.ID,
		Details:        "Verified fact details.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	}
	if err := v.SaveCareerFact(ctx, fact); err != nil {
		t.Fatalf("save verified fact failed: %v", err)
	}

	saved, err := v.GetCareerFact(fact.ID)
	if err != nil {
		t.Fatalf("get fact failed: %v", err)
	}
	if saved.EpistemicState != domain.EpistemicVerified || !saved.Verified {
		t.Fatalf("expected verified fact, got state=%s, verified=%v", saved.EpistemicState, saved.Verified)
	}

	// 2. Now update EpistemicState to raw_note without changing Verified (leave Verified true in struct)
	fact.EpistemicState = domain.EpistemicRawNote
	// Note: fact.Verified is still true here in memory!
	if err := v.SaveCareerFact(ctx, fact); err != nil {
		t.Fatalf("save updated fact failed: %v", err)
	}

	updated, err := v.GetCareerFact(fact.ID)
	if err != nil {
		t.Fatalf("get fact after downgrade failed: %v", err)
	}
	if updated.EpistemicState != domain.EpistemicRawNote {
		t.Errorf("expected EpistemicRawNote, got %s", updated.EpistemicState)
	}
	if updated.Verified {
		t.Errorf("expected Verified to be false when EpistemicState is raw_note, got true")
	}
}

func TestStringSliceSerialization(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	org := &domain.Organization{ID: "org-slice", Name: "Slice Corp"}
	if err := v.SaveOrganization(ctx, org); err != nil {
		t.Fatalf("save org failed: %v", err)
	}

	// 1. Role with nil tech stack
	roleNil := &domain.Role{
		ID:        "role-nil-tech",
		OrgID:     org.ID,
		Title:     "Nil Tech Role",
		TechStack: nil,
	}
	if err := v.SaveRole(ctx, roleNil); err != nil {
		t.Fatalf("save role with nil tech stack failed: %v", err)
	}
	gotRoleNil, err := v.GetRole(ctx, roleNil.ID)
	if err != nil {
		t.Fatalf("get role failed: %v", err)
	}
	if len(gotRoleNil.TechStack) != 0 {
		t.Errorf("expected empty/nil tech stack, got %+v", gotRoleNil.TechStack)
	}

	// Verify the database raw column is normalized to "[]"
	var rawTech string
	err = v.DB().QueryRowContext(ctx, "SELECT tech_stack FROM roles WHERE id = ?;", roleNil.ID).Scan(&rawTech)
	if err != nil {
		t.Fatalf("query raw tech stack failed: %v", err)
	}
	if rawTech != "[]" {
		t.Errorf("expected raw tech_stack column to be '[]', got %q", rawTech)
	}

	// 2. Archetype with empty and populated slices
	arch := &domain.Archetype{
		ID:               "arch-slice-test",
		Name:             "Slice Test Persona",
		CoreCompetencies: []string{"Go", "SQLite"},
		StrategicImpact:  []string{},
		TargetRoles:      nil,
	}
	if err := v.SaveArchetype(ctx, arch); err != nil {
		t.Fatalf("save archetype failed: %v", err)
	}

	var rawCore, rawStrat, rawRoles string
	err = v.DB().QueryRowContext(ctx, "SELECT core_competencies, strategic_impact, target_roles FROM archetypes WHERE id = ?;", arch.ID).Scan(&rawCore, &rawStrat, &rawRoles)
	if err != nil {
		t.Fatalf("query raw archetype slices failed: %v", err)
	}
	if rawCore != `["Go","SQLite"]` {
		t.Errorf("expected core_competencies to be '[\"Go\",\"SQLite\"]', got %q", rawCore)
	}
	if rawStrat != "[]" {
		t.Errorf("expected strategic_impact to be '[]', got %q", rawStrat)
	}
	if rawRoles != "[]" {
		t.Errorf("expected target_roles to be '[]', got %q", rawRoles)
	}

	gotArch, err := v.GetArchetype(ctx, arch.ID)
	if err != nil {
		t.Fatalf("get archetype failed: %v", err)
	}
	if len(gotArch.CoreCompetencies) != 2 || gotArch.CoreCompetencies[0] != "Go" {
		t.Errorf("unexpected core competencies: %+v", gotArch.CoreCompetencies)
	}
}

func TestApplicationFSMFieldsPersistence(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	org := &domain.Organization{ID: "org-fsm", Name: "Acme Corp"}
	_ = v.SaveOrganization(ctx, org)
	role := &domain.Role{ID: "role-fsm", OrgID: org.ID, Title: "Senior Engineer"}
	_ = v.SaveRole(ctx, role)
	fact := &domain.CareerFact{
		ID:             "fact-fsm-1",
		RoleID:         role.ID,
		Headline:       "Cut latency",
		Details:        "Cut P99 latency by 40%.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	}
	_ = v.SaveCareerFact(ctx, fact)
	arch := &domain.Archetype{ID: "arch-fsm", Name: "Systems"}
	_ = v.SaveArchetype(ctx, arch)

	dossier := domain.OpportunityDossier{
		RoleTitle:  "Staff Go Engineer",
		Company:    "Acme Corp",
		RawJD:      "Looking for Staff Go Engineer to build SQLite kernels.",
		FitSummary: "Strong fit",
		JDClaims: []domain.JDClaim{
			{Summary: "SQLite kernels", VerbatimJDQuote: "build SQLite kernels."},
		},
		ExternalClaims: []domain.ExternalClaim{
			{Claim: "Series B", UnknownAskRecruiter: true},
		},
	}

	app, err := domain.NewTriagedApplication("app-fsm-01", dossier)
	if err != nil {
		t.Fatalf("NewTriagedApplication: %v", err)
	}
	draft := domain.ApplicationDraft{
		ArchetypeID: arch.ID,
		Headline:    "Staff Go Engineer",
		Summary:     "Systems builder.",
		Experience: []domain.DraftExperience{
			{
				RoleTitle:    "Senior Engineer",
				Organization: "Prior Co",
				Highlights: []domain.BulletNode{
					{ID: "b1", Text: "Cut P99 latency by 40%.", FactIDs: []string{fact.ID}},
				},
			},
		},
		PageBudget: 1,
	}
	if err := app.SaveDraft(draft, v); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if err := app.RecordAudits(domain.AuditBundle{
		FactCheck:      &domain.FactCheckReport{Summary: "All verified", Passed: true},
		AlignmentCheck: &domain.AlignmentCheckReport{Summary: "Aligned", Passed: true},
	}); err != nil {
		t.Fatalf("RecordAudits: %v", err)
	}
	if err := app.PromoteReady(v); err != nil {
		t.Fatalf("PromoteReady: %v", err)
	}
	pii := &domain.PIIHeader{FullName: "Jane Doe"}
	if err := app.MarkExported(pii, domain.ExportRecord{
		PacketDir: "/tmp/packet",
		HTMLPath:  "/tmp/packet/resume.html",
		DocxPath:  "/tmp/packet/resume.docx",
		PDFPath:   "/tmp/packet/resume.pdf",
		PageCount: 1,
		MaxPages:  1,
	}); err != nil {
		t.Fatalf("MarkExported: %v", err)
	}
	if err := v.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication (Exported): %v", err)
	}
	if err := v.RecordApplicationCitations(ctx, app.ID, []string{fact.ID}); err != nil {
		t.Fatalf("RecordApplicationCitations before submit: %v", err)
	}
	sha := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := app.MarkSubmitted(sha, "/tmp/packet/interview_brief.md"); err != nil {
		t.Fatalf("MarkSubmitted: %v", err)
	}

	if err := v.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication (Submitted): %v", err)
	}

	got, err := v.GetApplicationContext(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationContext: %v", err)
	}
	if got.State != domain.StateSubmitted || got.Dossier == nil || got.Draft == nil || got.Audits == nil || got.Export == nil {
		t.Fatalf("expected full FSM artifacts restored, got %+v", got)
	}
	if got.BundleSHA256 != sha || got.InterviewBriefPath != "/tmp/packet/interview_brief.md" || got.SubmittedAt == nil {
		t.Fatalf("expected submission lock fields restored, got sha=%q brief=%q submittedAt=%v", got.BundleSHA256, got.InterviewBriefPath, got.SubmittedAt)
	}
	if got.Dossier.JDClaims[0].VerbatimJDQuote != "build SQLite kernels." {
		t.Errorf("dossier quote mismatch: %+v", got.Dossier)
	}
	if len(got.Draft.Experience) != 1 || got.Draft.Experience[0].Highlights[0].ID != "b1" {
		t.Errorf("draft mismatch: %+v", got.Draft)
	}

	// Submitted applications must be immutable in storage as well (SaveApplication, DeleteApplication, RecordApplicationCitations).
	if err := v.SaveApplication(ctx, got); !errors.Is(err, domain.ErrAlreadySubmitted) {
		t.Fatalf("expected ErrAlreadySubmitted on update to Submitted app, got %v", err)
	}
	if err := v.DeleteApplication(ctx, got.ID); !errors.Is(err, domain.ErrAlreadySubmitted) {
		t.Fatalf("expected ErrAlreadySubmitted on DeleteApplication for Submitted app, got %v", err)
	}
	if err := v.RecordApplicationCitations(ctx, got.ID, nil); !errors.Is(err, domain.ErrAlreadySubmitted) {
		t.Fatalf("expected ErrAlreadySubmitted on RecordApplicationCitations(nil) for Submitted app, got %v", err)
	}
	citations, err := v.GetCitationsForApp(ctx, got.ID)
	if err != nil {
		t.Fatalf("GetCitationsForApp: %v", err)
	}
	if len(citations) != 1 || citations[0] != fact.ID {
		t.Fatalf("expected citations for Submitted app to remain intact, got %v", citations)
	}
	if err := v.DeleteApplication(ctx, "app-nonexistent"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting missing application, got %v", err)
	}
}
