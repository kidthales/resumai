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
	if count != 1 {
		t.Errorf("expected 1 migration recorded, got %d", count)
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
