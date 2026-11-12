package vault_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/vault"
)

func TestBundleRoundtripAndAirGap(t *testing.T) {
	v1 := newTestVault(t)
	ctx := context.Background()

	// 1. Seed complete vault
	pii := &domain.PIIHeader{
		FullName:     "Logan Howlett",
		Email:        "wolverine@krakoa.mesh",
		Phone:        "+1-555-WEAPON-X",
		Location:     "Krakoa / Alberta",
		LinkedInURL:  "https://linkedin.com/in/logan",
		GitHubURL:    "https://github.com/wolverine",
		PortfolioURL: "https://logan.dev",
	}
	if err := v1.SetPII(ctx, pii); err != nil {
		t.Fatalf("set pii: %v", err)
	}

	logistics := &domain.CandidateLogistics{
		TargetTitle:        "Staff Infrastructure Engineer",
		WorkAuthorization:  "Citizen",
		RemotePreference:   "Remote",
		WillingToRelocate:  true,
		TargetCompensation: "$250k+",
		NoticePeriodDays:   14,
	}
	if err := v1.SetCandidateLogistics(ctx, logistics); err != nil {
		t.Fatalf("set logistics: %v", err)
	}

	org := &domain.Organization{
		ID:       "org-krakoan-bio",
		Name:     "Krakoan Habitat & Gateway Systems",
		Location: "Krakoa",
		Website:  "https://krakoa.mesh",
	}
	if err := v1.SaveOrganization(ctx, org); err != nil {
		t.Fatalf("save org: %v", err)
	}

	role := &domain.Role{
		ID:        "role-principal-architect",
		OrgID:     org.ID,
		Title:     "Principal Architect – Bio-Organic Infrastructures",
		Category:  "work",
		Location:  "Krakoa (On-site)",
		Duration:  "Oct 2019 to Sep 2026 (7 years)",
		TechStack: []string{"Krakoan Gateways", "Bio-Organic Telemetry", "Rust", "Linux"},
	}
	if err := v1.SaveRole(ctx, role); err != nil {
		t.Fatalf("save role: %v", err)
	}

	fact1 := &domain.CareerFact{
		ID:             "fact-krakoa-mesh",
		RoleID:         role.ID,
		Headline:       "Psionic Interface Telemetry",
		Details:        "Design fault-tolerant psionic interface protocols enabling seamless transit across global mutant habitats.",
		EpistemicState: domain.EpistemicVerified,
		TechTags:       []string{"Telepathic Mesh", "Rust"},
		Verified:       true,
	}
	if err := v1.SaveCareerFact(ctx, fact1); err != nil {
		t.Fatalf("save fact1: %v", err)
	}

	fact2 := &domain.CareerFact{
		ID:             "fact-gateways",
		RoleID:         role.ID,
		Headline:       "Gateway Network",
		Details:        "Architected self-healing bio-organic gateways.",
		EpistemicState: domain.EpistemicRawNote,
		TechTags:       []string{"Biology", "Networking"},
		Verified:       false,
	}
	if err := v1.SaveCareerFact(ctx, fact2); err != nil {
		t.Fatalf("save fact2: %v", err)
	}

	edu := &domain.Education{
		ID:           "edu-mit-bsc",
		Institution:  "Massachusetts Institute of Technology",
		Degree:       "Bachelor of Science (B.Sc.)",
		FieldOfStudy: "Mutant Studies",
		Location:     "Cambridge, Massachusetts, USA",
		Duration:     "1984 to 1988 (4 years)",
		Honors:       "Magna Cum Laude",
	}
	if err := v1.SaveEducation(ctx, edu); err != nil {
		t.Fatalf("save edu: %v", err)
	}

	arch := &domain.Archetype{
		ID:               "archetype-devops-sre",
		Name:             "DevOps & Site Reliability Engineer",
		ProfileOverview:  "An infrastructure and reliability specialist...",
		CoreCompetencies: []string{"Cloud", "IaC", "CI/CD"},
		StrategicImpact:  []string{"Automates multi-region deployment pipelines"},
		TargetRoles:      []string{"Site Reliability Engineer", "Platform Engineer"},
	}
	if err := v1.SaveArchetype(ctx, arch); err != nil {
		t.Fatalf("save arch: %v", err)
	}

	app := &domain.JobApplication{
		ID:        "app-stark-01",
		RoleTitle: "Staff Systems Engineer",
		Company:   "Stark Enterprises",
		RawJD:     "Seeking staff engineer...",
		State:     domain.StateSubmitted,
	}
	if err := v1.SaveApplication(ctx, app); err != nil {
		t.Fatalf("save app: %v", err)
	}
	if err := v1.RecordApplicationCitations(ctx, app.ID, []string{fact1.ID}); err != nil {
		t.Fatalf("record citations: %v", err)
	}

	// 2. Test Dump WITHOUT PII (air-gap default)
	bundleNoPII := filepath.Join(t.TempDir(), "bundle-nopii")
	if err := v1.Dump(ctx, bundleNoPII, vault.DumpOptions{IncludePII: false}); err != nil {
		t.Fatalf("dump without pii failed: %v", err)
	}

	// Invariant: pii.yaml MUST NOT exist when IncludePII is false
	if _, err := os.Stat(filepath.Join(bundleNoPII, "pii.yaml")); !os.IsNotExist(err) {
		t.Errorf("air-gap violation: pii.yaml exists in dump without pii")
	}

	// Logistics must exist
	if _, err := os.Stat(filepath.Join(bundleNoPII, "logistics.yaml")); err != nil {
		t.Errorf("logistics.yaml should exist: %v", err)
	}

	// 3. Test Load into fresh vault from bundle without PII
	v2 := newTestVault(t)
	if err := v2.Load(ctx, bundleNoPII); err != nil {
		t.Fatalf("load bundleNoPII into v2 failed: %v", err)
	}

	// Verify v2 has no PII
	_, err := v2.GetPII(ctx)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for PII in v2, got: %v", err)
	}

	// Verify v2 has organizations, roles, facts
	gotOrg, err := v2.GetOrganization(ctx, org.ID)
	if err != nil || gotOrg.Name != org.Name {
		t.Errorf("org mismatch in v2: %v", err)
	}
	gotFact1, err := v2.GetCareerFact(fact1.ID)
	if err != nil || gotFact1.EpistemicState != domain.EpistemicVerified {
		t.Errorf("fact1 mismatch in v2: %v", err)
	}
	gotFact2, err := v2.GetCareerFact(fact2.ID)
	if err != nil || gotFact2.EpistemicState != domain.EpistemicRawNote {
		t.Errorf("fact2 mismatch in v2: %v", err)
	}

	// Verify applications and citations loaded
	gotApp, err := v2.GetApplication(app.ID)
	if err != nil || gotApp.State != domain.StateSubmitted {
		t.Errorf("app mismatch in v2: %v", err)
	}
	citations, err := v2.GetCitationsForApp(ctx, app.ID)
	if err != nil || len(citations) != 1 || citations[0] != fact1.ID {
		t.Errorf("citations mismatch in v2: got %v", citations)
	}

	// 4. Test Dump WITH PII
	bundleWithPII := filepath.Join(t.TempDir(), "bundle-pii")
	if err := v1.Dump(ctx, bundleWithPII, vault.DumpOptions{IncludePII: true}); err != nil {
		t.Fatalf("dump with pii failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundleWithPII, "pii.yaml")); err != nil {
		t.Errorf("pii.yaml should exist in dump with pii: %v", err)
	}

	// 5. Load bundle with PII into v2
	if err := v2.Load(ctx, bundleWithPII); err != nil {
		t.Fatalf("load bundleWithPII into v2 failed: %v", err)
	}
	gotPII, err := v2.GetPII(ctx)
	if err != nil || gotPII.FullName != pii.FullName || gotPII.Email != pii.Email {
		t.Errorf("pii mismatch after loading bundleWithPII: %v, got %+v", err, gotPII)
	}

	// 6. Test that loading bundleNoPII into a vault with existing PII does NOT erase existing PII
	if err := v2.Load(ctx, bundleNoPII); err != nil {
		t.Fatalf("reloading bundleNoPII into v2 failed: %v", err)
	}
	gotPreservedPII, err := v2.GetPII(ctx)
	if err != nil || gotPreservedPII.FullName != pii.FullName {
		t.Errorf("loading bundle without pii.yaml should preserve existing vault PII, got: %v", err)
	}
}

func TestSoftArchiveProvenanceGuardOnReload(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Setup: Org, Role, Fact 1, Fact 2
	org := &domain.Organization{ID: "org-aim", Name: "Advanced Idea Mechanics"}
	_ = v.SaveOrganization(ctx, org)

	role := &domain.Role{ID: "role-scientist", OrgID: org.ID, Title: "Senior Scientist"}
	_ = v.SaveRole(ctx, role)

	factSubmitted := &domain.CareerFact{
		ID:             "fact-quantum-drive",
		RoleID:         role.ID,
		Headline:       "Quantum Drive",
		Details:        "Built sub-atomic quantum telemetry drive.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	}
	_ = v.SaveCareerFact(ctx, factSubmitted)

	factUnsubmitted := &domain.CareerFact{
		ID:             "fact-coffee-bot",
		RoleID:         role.ID,
		Headline:       "Coffee Bot",
		Details:        "Reprogrammed lab coffee maker.",
		EpistemicState: domain.EpistemicRawNote,
		Verified:       false,
	}
	_ = v.SaveCareerFact(ctx, factUnsubmitted)

	// Application in Submitted state citing factSubmitted
	app := &domain.JobApplication{
		ID:        "app-aim-01",
		RoleTitle: "Lead Quantum Engineer",
		Company:   "Stark Quantum",
		State:     domain.StateSubmitted,
	}
	_ = v.SaveApplication(ctx, app)
	_ = v.RecordApplicationCitations(ctx, app.ID, []string{factSubmitted.ID})

	// Dump bundle
	bundleDir := filepath.Join(t.TempDir(), "aim-bundle")
	if err := v.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: false}); err != nil {
		t.Fatalf("dump failed: %v", err)
	}

	// Now modify the bundle on disk:
	// Remove fact-quantum-drive and fact-coffee-bot from the organization YAML file
	orgYAMLPath := filepath.Join(bundleDir, "organizations", "org-aim.yaml")
	newOrgYAML := `id: org-aim
name: Advanced Idea Mechanics
roles:
  - id: role-scientist
    title: Senior Scientist
    category: work
    facts:
      - id: fact-new-only
        headline: New Breakthrough
        details: A brand new accomplishment.
        epistemic_state: verified
        verified: true
`
	if err := os.WriteFile(orgYAMLPath, []byte(newOrgYAML), 0644); err != nil {
		t.Fatalf("write modified org yaml failed: %v", err)
	}

	// Reload the modified bundle into the vault
	if err := v.Load(ctx, bundleDir); err != nil {
		t.Fatalf("load modified bundle failed: %v", err)
	}

	// Invariant Check 1:
	// factUnsubmitted was NOT cited by a Submitted application -> it SHOULD be deleted from the database
	_, err := v.GetCareerFact(factUnsubmitted.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unsubmitted fact should have been deleted, got: %v", err)
	}

	// Invariant Check 2:
	// factSubmitted WAS cited by a Submitted application!
	// It MUST NOT be deleted! It MUST be transitioned to 'archived' state!
	archivedFact, err := v.GetCareerFact(factSubmitted.ID)
	if err != nil {
		t.Fatalf("submitted fact provenance must NOT be destroyed: %v", err)
	}
	if archivedFact.EpistemicState != domain.EpistemicArchived || archivedFact.Verified {
		t.Errorf("expected missing submitted fact to be archived, got state=%s, verified=%v", archivedFact.EpistemicState, archivedFact.Verified)
	}

	// Citation link must still be preserved
	citations, err := v.GetCitationsForApp(ctx, app.ID)
	if err != nil || len(citations) != 1 || citations[0] != factSubmitted.ID {
		t.Errorf("application citation link must be preserved: got %v", citations)
	}

	// Check new fact was added
	newFact, err := v.GetCareerFact("fact-new-only")
	if err != nil || newFact.Headline != "New Breakthrough" {
		t.Errorf("expected new fact to be loaded: %v", err)
	}
}

func TestTransactionalRollbackOnInvalidBundle(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	org := &domain.Organization{ID: "org-shield", Name: "S.H.I.E.L.D."}
	_ = v.SaveOrganization(ctx, org)

	bundleDir := filepath.Join(t.TempDir(), "invalid-bundle")
	orgDir := filepath.Join(bundleDir, "organizations")
	_ = os.MkdirAll(orgDir, 0755)

	// Create an org YAML with an invalid epistemic state
	invalidYAML := `id: org-shield
name: S.H.I.E.L.D.
roles:
  - id: role-agent
    title: Field Agent
    facts:
      - id: fact-op-01
        details: Classified Operation
        epistemic_state: completely_bogus_state
`
	_ = os.WriteFile(filepath.Join(orgDir, "org-shield.yaml"), []byte(invalidYAML), 0644)

	err := v.Load(ctx, bundleDir)
	if err == nil {
		t.Fatalf("expected error loading bundle with invalid epistemic state, got nil")
	}

	// Verify original organization is unchanged
	gotOrg, err := v.GetOrganization(ctx, org.ID)
	if err != nil || gotOrg.Name != "S.H.I.E.L.D." {
		t.Errorf("database corrupted after failed load: %v", err)
	}
}

func TestDumpLoadPerformanceBenchmark(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Seed vault with realistic portfolio: 10 orgs, 20 roles, 100 facts, 5 education, 5 archetypes, 10 apps
	for oi := 1; oi <= 10; oi++ {
		orgID := "org-perf-" + string(rune('a'+oi))
		_ = v.SaveOrganization(ctx, &domain.Organization{
			ID:       orgID,
			Name:     "Enterprise Tech Group " + orgID,
			Location: "San Francisco, CA",
			Website:  "https://example.com",
		})

		for ri := 1; ri <= 2; ri++ {
			roleID := orgID + "-role-" + string(rune('0'+ri))
			_ = v.SaveRole(ctx, &domain.Role{
				ID:        roleID,
				OrgID:     orgID,
				Title:     "Lead Architect",
				Category:  "work",
				Duration:  "2020-2024",
				TechStack: []string{"Go", "Kubernetes", "PostgreSQL", "Kafka"},
			})

			for fi := 1; fi <= 5; fi++ {
				factID := roleID + "-fact-" + string(rune('0'+fi))
				_ = v.SaveCareerFact(ctx, &domain.CareerFact{
					ID:             factID,
					RoleID:         roleID,
					Headline:       "Infrastructure Automation",
					Details:        "Reduced cloud infrastructure latency by 45% through custom Go microservices and event-driven architectures.",
					EpistemicState: domain.EpistemicVerified,
					TechTags:       []string{"Go", "Cloud"},
					Verified:       true,
				})
			}
		}
	}

	for ei := 1; ei <= 5; ei++ {
		_ = v.SaveEducation(ctx, &domain.Education{
			ID:          "edu-perf-" + string(rune('0'+ei)),
			Institution: "University of Technology",
			Degree:      "Master of Science",
		})
	}

	for ai := 1; ai <= 5; ai++ {
		_ = v.SaveArchetype(ctx, &domain.Archetype{
			ID:               "arch-perf-" + string(rune('0'+ai)),
			Name:             "Platform Engineer Archetype",
			CoreCompetencies: []string{"Go", "Distributed Systems", "Cloud"},
		})
	}

	bundleDir := filepath.Join(t.TempDir(), "perf-bundle")

	// Benchmark Dump (< 250ms target)
	dumpStart := time.Now()
	if err := v.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: true}); err != nil {
		t.Fatalf("perf dump failed: %v", err)
	}
	dumpDuration := time.Since(dumpStart)
	t.Logf("Cold Dump Duration: %v", dumpDuration)
	if dumpDuration > 250*time.Millisecond {
		t.Errorf("dump exceeded 250ms SLA: took %v", dumpDuration)
	}

	// Benchmark Load (< 250ms target)
	vTarget := newTestVault(t)
	loadStart := time.Now()
	if err := vTarget.Load(ctx, bundleDir); err != nil {
		t.Fatalf("perf load failed: %v", err)
	}
	loadDuration := time.Since(loadStart)
	t.Logf("Cold Load Duration: %v", loadDuration)
	if loadDuration > 250*time.Millisecond {
		t.Errorf("load exceeded 250ms SLA: took %v", loadDuration)
	}
}
