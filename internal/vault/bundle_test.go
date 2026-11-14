package vault_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	const bundleDumpLoadSLA = 750 * time.Millisecond

	// Benchmark Dump (< 750ms target under -race + Windows NTFS overhead)
	dumpStart := time.Now()
	if err := v.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: true}); err != nil {
		t.Fatalf("perf dump failed: %v", err)
	}
	dumpDuration := time.Since(dumpStart)
	t.Logf("Cold Dump Duration: %v", dumpDuration)
	if dumpDuration > bundleDumpLoadSLA {
		t.Errorf("dump exceeded %v SLA: took %v", bundleDumpLoadSLA, dumpDuration)
	}

	// Benchmark Load (< 750ms target under -race + Windows NTFS overhead)
	vTarget := newTestVault(t)
	loadStart := time.Now()
	if err := vTarget.Load(ctx, bundleDir); err != nil {
		t.Fatalf("perf load failed: %v", err)
	}
	loadDuration := time.Since(loadStart)
	t.Logf("Cold Load Duration: %v", loadDuration)
	if loadDuration > bundleDumpLoadSLA {
		t.Errorf("load exceeded %v SLA: took %v", bundleDumpLoadSLA, loadDuration)
	}
}

func TestDumpCleansStaleEntityYAMLFiles(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Seed 2 of each entity
	orgA := &domain.Organization{ID: "org-alpha", Name: "Alpha Corp"}
	orgB := &domain.Organization{ID: "org-beta", Name: "Beta Corp"}
	_ = v.SaveOrganization(ctx, orgA)
	_ = v.SaveOrganization(ctx, orgB)

	edu1 := &domain.Education{ID: "edu-1", Institution: "University 1"}
	edu2 := &domain.Education{ID: "edu-2", Institution: "University 2"}
	_ = v.SaveEducation(ctx, edu1)
	_ = v.SaveEducation(ctx, edu2)

	arch1 := &domain.Archetype{ID: "arch-1", Name: "Archetype 1"}
	arch2 := &domain.Archetype{ID: "arch-2", Name: "Archetype 2"}
	_ = v.SaveArchetype(ctx, arch1)
	_ = v.SaveArchetype(ctx, arch2)

	app1 := &domain.JobApplication{ID: "app-1", RoleTitle: "Role 1", Company: "Company 1"}
	app2 := &domain.JobApplication{ID: "app-2", RoleTitle: "Role 2", Company: "Company 2"}
	_ = v.SaveApplication(ctx, app1)
	_ = v.SaveApplication(ctx, app2)

	bundleDir := filepath.Join(t.TempDir(), "dump-clean-test")

	// First Dump: all entities written
	if err := v.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: false}); err != nil {
		t.Fatalf("first dump failed: %v", err)
	}

	assertFileExists := func(path string, shouldExist bool) {
		t.Helper()
		_, err := os.Stat(path)
		exists := err == nil
		if exists != shouldExist {
			t.Errorf("file %s expected exist=%v, got=%v", filepath.Base(path), shouldExist, exists)
		}
	}

	assertFileExists(filepath.Join(bundleDir, "organizations", "org-alpha.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "organizations", "org-beta.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "education", "edu-1.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "education", "edu-2.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "archetypes", "arch-1.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "archetypes", "arch-2.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "applications", "app-1.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "applications", "app-2.yaml"), true)

	// Now delete entity 2 from each category
	if err := v.DeleteOrganization(ctx, "org-beta"); err != nil {
		t.Fatalf("delete org-beta: %v", err)
	}
	if err := v.DeleteEducation(ctx, "edu-2"); err != nil {
		t.Fatalf("delete edu-2: %v", err)
	}
	if err := v.DeleteArchetype(ctx, "arch-2"); err != nil {
		t.Fatalf("delete arch-2: %v", err)
	}
	if err := v.DeleteApplication(ctx, "app-2"); err != nil {
		t.Fatalf("delete app-2: %v", err)
	}

	// Re-dump into the exact same bundle directory
	if err := v.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: false}); err != nil {
		t.Fatalf("second dump failed: %v", err)
	}

	// Invariant: Deleted entities MUST have their stale YAML files pruned from disk!
	assertFileExists(filepath.Join(bundleDir, "organizations", "org-alpha.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "organizations", "org-beta.yaml"), false)
	assertFileExists(filepath.Join(bundleDir, "education", "edu-1.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "education", "edu-2.yaml"), false)
	assertFileExists(filepath.Join(bundleDir, "archetypes", "arch-1.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "archetypes", "arch-2.yaml"), false)
	assertFileExists(filepath.Join(bundleDir, "applications", "app-1.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "applications", "app-2.yaml"), false)
}

func TestSoftArchiveProvenanceGuardOnSimultaneousSubmissionAndFactRemoval(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// 1. Setup org, role, fact
	org := &domain.Organization{ID: "org-stark", Name: "Stark Industries"}
	_ = v.SaveOrganization(ctx, org)

	role := &domain.Role{ID: "role-chief-eng", OrgID: org.ID, Title: "Chief Engineer"}
	_ = v.SaveRole(ctx, role)

	fact := &domain.CareerFact{
		ID:             "fact-arc-reactor",
		RoleID:         role.ID,
		Headline:       "Miniaturized Arc Reactor",
		Details:        "Built miniaturized clean energy reactor.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	}
	_ = v.SaveCareerFact(ctx, fact)

	// 2. Setup application in Triaged/Draft state citing the fact
	app := &domain.JobApplication{
		ID:        "app-avengers",
		RoleTitle: "Lead Technology Officer",
		Company:   "Avengers Initiative",
		State:     domain.StateTriaged,
	}
	_ = v.SaveApplication(ctx, app)
	_ = v.RecordApplicationCitations(ctx, app.ID, []string{fact.ID})

	// 3. Dump bundle
	bundleDir := filepath.Join(t.TempDir(), "simultaneous-sub-bundle")
	if err := v.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: false}); err != nil {
		t.Fatalf("dump failed: %v", err)
	}

	// 4. Modify the bundle on disk:
	// - Remove fact-arc-reactor from org-stark.yaml
	// - Set state to 'Submitted' in app-avengers.yaml while keeping citation of fact-arc-reactor
	orgPath := filepath.Join(bundleDir, "organizations", "org-stark.yaml")
	newOrgYAML := `id: org-stark
name: Stark Industries
roles:
  - id: role-chief-eng
    title: Chief Engineer
    category: work
    facts:
      - id: fact-nanotech-suit
        headline: Nanotech Armor
        details: Deployed bleeding-edge nanotech armor.
        epistemic_state: verified
        verified: true
`
	if err := os.WriteFile(orgPath, []byte(newOrgYAML), 0644); err != nil {
		t.Fatalf("write new org yaml: %v", err)
	}

	appPath := filepath.Join(bundleDir, "applications", "app-avengers.yaml")
	newAppYAML := `id: app-avengers
role_title: Lead Technology Officer
company: Avengers Initiative
state: Submitted
citations:
  - fact-arc-reactor
`
	if err := os.WriteFile(appPath, []byte(newAppYAML), 0644); err != nil {
		t.Fatalf("write new app yaml: %v", err)
	}

	// 5. Load bundle into vault
	// This exercises:
	// - Application updated to Submitted in same transaction
	// - Fact missing from organizations/
	// - Soft-archive reconciliation must recognize incoming application's Submitted state
	// - fact-arc-reactor must NOT be deleted; it must be soft-archived to preserve citation provenance!
	if err := v.Load(ctx, bundleDir); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// 6. Assertions
	// Fact was soft-archived
	archivedFact, err := v.GetCareerFact(fact.ID)
	if err != nil {
		t.Fatalf("fact must not be deleted: %v", err)
	}
	if archivedFact.EpistemicState != domain.EpistemicArchived || archivedFact.Verified {
		t.Errorf("expected fact to be archived, got state=%s, verified=%v", archivedFact.EpistemicState, archivedFact.Verified)
	}

	// Application is Submitted
	gotApp, err := v.GetApplication(app.ID)
	if err != nil {
		t.Fatalf("get app: %v", err)
	}
	if gotApp.State != domain.StateSubmitted {
		t.Errorf("expected app state Submitted, got %s", gotApp.State)
	}

	// Citation link is intact
	cits, err := v.GetCitationsForApp(ctx, app.ID)
	if err != nil {
		t.Fatalf("get citations: %v", err)
	}
	if len(cits) != 1 || cits[0] != fact.ID {
		t.Errorf("expected citation to fact-arc-reactor preserved, got: %v", cits)
	}
}

func TestDumpSlugCollisionDisambiguation(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// 1. Setup two organizations whose IDs normalize to the exact same slug: "org-acme"
	org1 := &domain.Organization{ID: "org:acme", Name: "Acme Colon Division"}
	org2 := &domain.Organization{ID: "org/acme", Name: "Acme Slash Division"}
	if err := v.SaveOrganization(ctx, org1); err != nil {
		t.Fatalf("save org1: %v", err)
	}
	if err := v.SaveOrganization(ctx, org2); err != nil {
		t.Fatalf("save org2: %v", err)
	}

	// 2. Setup education entities with colliding slugs
	edu1 := &domain.Education{ID: "edu:mit", Institution: "MIT Colon"}
	edu2 := &domain.Education{ID: "edu/mit", Institution: "MIT Slash"}
	if err := v.SaveEducation(ctx, edu1); err != nil {
		t.Fatalf("save edu1: %v", err)
	}
	if err := v.SaveEducation(ctx, edu2); err != nil {
		t.Fatalf("save edu2: %v", err)
	}

	// 3. Setup archetypes with colliding slugs
	arch1 := &domain.Archetype{ID: "arch:infra", Name: "Infra Colon"}
	arch2 := &domain.Archetype{ID: "arch/infra", Name: "Infra Slash"}
	if err := v.SaveArchetype(ctx, arch1); err != nil {
		t.Fatalf("save arch1: %v", err)
	}
	if err := v.SaveArchetype(ctx, arch2); err != nil {
		t.Fatalf("save arch2: %v", err)
	}

	// 4. Setup applications with colliding slugs
	app1 := &domain.JobApplication{ID: "app:sre", RoleTitle: "SRE Colon", Company: "Acme"}
	app2 := &domain.JobApplication{ID: "app/sre", RoleTitle: "SRE Slash", Company: "Acme"}
	if err := v.SaveApplication(ctx, app1); err != nil {
		t.Fatalf("save app1: %v", err)
	}
	if err := v.SaveApplication(ctx, app2); err != nil {
		t.Fatalf("save app2: %v", err)
	}

	// 5. Dump bundle
	bundleDir := filepath.Join(t.TempDir(), "slug-collision-bundle")
	if err := v.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: false}); err != nil {
		t.Fatalf("dump failed: %v", err)
	}

	assertFileExists := func(path string, shouldExist bool) {
		t.Helper()
		_, err := os.Stat(path)
		exists := err == nil
		if exists != shouldExist {
			t.Errorf("file %s expected exist=%v, got=%v", filepath.Base(path), shouldExist, exists)
		}
	}

	// Assert disambiguated files exist on disk
	assertFileExists(filepath.Join(bundleDir, "organizations", "org-acme.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "organizations", "org-acme-2.yaml"), true)

	assertFileExists(filepath.Join(bundleDir, "education", "edu-mit.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "education", "edu-mit-2.yaml"), true)

	assertFileExists(filepath.Join(bundleDir, "archetypes", "arch-infra.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "archetypes", "arch-infra-2.yaml"), true)

	assertFileExists(filepath.Join(bundleDir, "applications", "app-sre.yaml"), true)
	assertFileExists(filepath.Join(bundleDir, "applications", "app-sre-2.yaml"), true)

	// 6. Load into fresh vault and ensure both distinct entities roundtrip cleanly
	v2 := newTestVault(t)
	if err := v2.Load(ctx, bundleDir); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	loadedOrgs, err := v2.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("list orgs: %v", err)
	}
	if len(loadedOrgs) != 2 {
		t.Fatalf("expected 2 organizations, got %d", len(loadedOrgs))
	}

	loadedEdus, err := v2.ListEducation(ctx)
	if err != nil {
		t.Fatalf("list edus: %v", err)
	}
	if len(loadedEdus) != 2 {
		t.Fatalf("expected 2 education records, got %d", len(loadedEdus))
	}

	loadedArchs, err := v2.ListArchetypes(ctx)
	if err != nil {
		t.Fatalf("list archs: %v", err)
	}
	if len(loadedArchs) != 2 {
		t.Fatalf("expected 2 archetypes, got %d", len(loadedArchs))
	}

	loadedApps, err := v2.ListApplications(ctx)
	if err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(loadedApps) != 2 {
		t.Fatalf("expected 2 applications, got %d", len(loadedApps))
	}
}

func TestLoadSingleFileFallbackNormalizationAndValidation(t *testing.T) {
	ctx := context.Background()

	// 1. Positive case: single-file fallbacks with missing IDs and default epistemic states
	bundleDir := t.TempDir()

	orgsYAML := `- name: Acme Corp
  roles:
    - title: Staff SRE
      facts:
        - headline: Reduced MTTR
        - id: custom-fact
          headline: Automated failover
          epistemic_state: verified
        - id: verified-legacy-fact
          headline: Omitted epistemic state but verified true
          verified: true
- id: beta-corp
  name: Beta Corp
  roles:
    - id: beta-role-1
      title: Lead Architect
      facts:
        - headline: Scaled traffic
`
	if err := os.WriteFile(filepath.Join(bundleDir, "organizations.yaml"), []byte(orgsYAML), 0644); err != nil {
		t.Fatalf("write organizations.yaml: %v", err)
	}

	edusYAML := `- institution: MIT
  degree: B.S. Computer Science
`
	if err := os.WriteFile(filepath.Join(bundleDir, "education.yaml"), []byte(edusYAML), 0644); err != nil {
		t.Fatalf("write education.yaml: %v", err)
	}

	archsYAML := `- name: Platform Engineer
  profile_overview: Core infrastructure builder
`
	if err := os.WriteFile(filepath.Join(bundleDir, "archetypes.yaml"), []byte(archsYAML), 0644); err != nil {
		t.Fatalf("write archetypes.yaml: %v", err)
	}

	appsYAML := `- company: Stripe
  role_title: Senior SRE
  raw_jd: Infrastructure operations
`
	if err := os.WriteFile(filepath.Join(bundleDir, "applications.yaml"), []byte(appsYAML), 0644); err != nil {
		t.Fatalf("write applications.yaml: %v", err)
	}

	v := newTestVault(t)
	if err := v.Load(ctx, bundleDir); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	orgs, err := v.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("list orgs: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("expected 2 orgs, got %d", len(orgs))
	}

	// Verify org 1 ID defaulting and role/fact normalization
	org1 := orgs[0]
	if org1.ID != "org-1" {
		t.Errorf("expected org1 ID 'org-1', got '%s'", org1.ID)
	}
	roles1, err := v.ListRolesByOrg(ctx, org1.ID)
	if err != nil {
		t.Fatalf("list roles for org1: %v", err)
	}
	if len(roles1) != 1 {
		t.Fatalf("expected 1 role in org1, got %d", len(roles1))
	}
	role1 := roles1[0]
	if role1.ID != "org-1-role-1" {
		t.Errorf("expected role ID 'org-1-role-1', got '%s'", role1.ID)
	}
	facts1, err := v.ListCareerFactsByRole(ctx, role1.ID)
	if err != nil {
		t.Fatalf("list facts for role1: %v", err)
	}
	if len(facts1) != 3 {
		t.Fatalf("expected 3 facts in role1, got %d", len(facts1))
	}
	if facts1[0].ID != "org-1-role-1-fact-1" {
		t.Errorf("expected fact[0] ID 'org-1-role-1-fact-1', got '%s'", facts1[0].ID)
	}
	if facts1[0].EpistemicState != domain.EpistemicRawNote {
		t.Errorf("expected fact[0] epistemic state 'raw_note', got '%s'", facts1[0].EpistemicState)
	}
	if facts1[0].Verified {
		t.Errorf("expected fact[0] verified to be false")
	}
	if facts1[1].ID != "custom-fact" {
		t.Errorf("expected fact[1] ID 'custom-fact', got '%s'", facts1[1].ID)
	}
	if facts1[1].EpistemicState != domain.EpistemicVerified {
		t.Errorf("expected fact[1] epistemic state 'verified', got '%s'", facts1[1].EpistemicState)
	}
	if !facts1[1].Verified {
		t.Errorf("expected fact[1] verified to be true")
	}
	if facts1[2].ID != "verified-legacy-fact" {
		t.Errorf("expected fact[2] ID 'verified-legacy-fact', got '%s'", facts1[2].ID)
	}
	if facts1[2].EpistemicState != domain.EpistemicVerified {
		t.Errorf("expected fact[2] epistemic state 'verified', got '%s'", facts1[2].EpistemicState)
	}
	if !facts1[2].Verified {
		t.Errorf("expected fact[2] verified to be true")
	}

	// Verify org 2 preservation
	org2 := orgs[1]
	if org2.ID != "beta-corp" {
		t.Errorf("expected org2 ID 'beta-corp', got '%s'", org2.ID)
	}
	roles2, err := v.ListRolesByOrg(ctx, org2.ID)
	if err != nil {
		t.Fatalf("list roles for org2: %v", err)
	}
	if len(roles2) != 1 {
		t.Fatalf("expected 1 role in org2, got %d", len(roles2))
	}
	role2 := roles2[0]
	facts2, err := v.ListCareerFactsByRole(ctx, role2.ID)
	if err != nil {
		t.Fatalf("list facts for role2: %v", err)
	}
	if len(facts2) != 1 {
		t.Fatalf("expected 1 fact in role2, got %d", len(facts2))
	}
	if facts2[0].ID != "beta-role-1-fact-1" {
		t.Errorf("expected org2 fact ID 'beta-role-1-fact-1', got '%s'", facts2[0].ID)
	}
	if facts2[0].EpistemicState != domain.EpistemicRawNote {
		t.Errorf("expected org2 fact epistemic state 'raw_note', got '%s'", facts2[0].EpistemicState)
	}

	// Verify education ID defaulting
	edus, err := v.ListEducation(ctx)
	if err != nil {
		t.Fatalf("list edus: %v", err)
	}
	if len(edus) != 1 || edus[0].ID != "edu-1" {
		t.Errorf("expected edu ID 'edu-1', got %+v", edus)
	}

	// Verify archetype ID defaulting
	archs, err := v.ListArchetypes(ctx)
	if err != nil {
		t.Fatalf("list archs: %v", err)
	}
	if len(archs) != 1 || archs[0].ID != "arch-1" {
		t.Errorf("expected arch ID 'arch-1', got %+v", archs)
	}

	// Verify application ID defaulting
	apps, err := v.ListApplications(ctx)
	if err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(apps) != 1 || apps[0].ID != "app-1" {
		t.Errorf("expected app ID 'app-1', got %+v", apps)
	}

	// 2. Negative case: organizations.yaml with invalid epistemic state fails validation
	badBundleDir := t.TempDir()
	badOrgsYAML := `- id: org-bad
  name: Bad Corp
  roles:
    - id: role-bad
      title: Tester
      facts:
        - id: fact-bad
          headline: Questionable fact
          epistemic_state: invalid_state_value
`
	if err := os.WriteFile(filepath.Join(badBundleDir, "organizations.yaml"), []byte(badOrgsYAML), 0644); err != nil {
		t.Fatalf("write bad organizations.yaml: %v", err)
	}

	vBad := newTestVault(t)
	err = vBad.Load(ctx, badBundleDir)
	if err == nil {
		t.Fatalf("expected load to fail due to invalid epistemic state, got nil")
	}
	if !strings.Contains(err.Error(), "invalid epistemic state in fact fact-bad") {
		t.Errorf("expected error message to contain 'invalid epistemic state in fact fact-bad', got: %v", err)
	}
}

func TestLoadDirectoryBundleEpistemicStateInference(t *testing.T) {
	ctx := context.Background()
	bundleDir := t.TempDir()
	orgDir := filepath.Join(bundleDir, "organizations")
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		t.Fatalf("mkdir orgDir: %v", err)
	}

	acmeYAML := `id: org-acme
name: Acme Corp
roles:
  - id: role-eng
    title: Principal Engineer
    facts:
      - id: fact-omitted-verified-true
        headline: High availability cluster
        verified: true
      - id: fact-omitted-verified-false
        headline: Scratch pad note
        verified: false
      - id: fact-explicit-raw-verified-true
        headline: Speculative note
        epistemic_state: raw_note
        verified: true
      - id: fact-explicit-verified-flag-false
        headline: Audit verified fact
        epistemic_state: verified
        verified: false
`
	if err := os.WriteFile(filepath.Join(orgDir, "acme.yaml"), []byte(acmeYAML), 0644); err != nil {
		t.Fatalf("write acme.yaml: %v", err)
	}

	v := newTestVault(t)
	if err := v.Load(ctx, bundleDir); err != nil {
		t.Fatalf("load failed: %v", err)
	}

	facts, err := v.ListCareerFactsByRole(ctx, "role-eng")
	if err != nil {
		t.Fatalf("list facts: %v", err)
	}
	if len(facts) != 4 {
		t.Fatalf("expected 4 facts, got %d", len(facts))
	}

	factMap := make(map[string]*domain.CareerFact)
	for _, f := range facts {
		factMap[f.ID] = f
	}

	// 1. Omitted epistemic_state, verified true -> EpistemicVerified, Verified: true
	f1 := factMap["fact-omitted-verified-true"]
	if f1.EpistemicState != domain.EpistemicVerified {
		t.Errorf("f1 epistemic state: expected %s, got %s", domain.EpistemicVerified, f1.EpistemicState)
	}
	if !f1.Verified {
		t.Errorf("f1 verified: expected true, got false")
	}

	// 2. Omitted epistemic_state, verified false -> EpistemicRawNote, Verified: false
	f2 := factMap["fact-omitted-verified-false"]
	if f2.EpistemicState != domain.EpistemicRawNote {
		t.Errorf("f2 epistemic state: expected %s, got %s", domain.EpistemicRawNote, f2.EpistemicState)
	}
	if f2.Verified {
		t.Errorf("f2 verified: expected false, got true")
	}

	// 3. Explicit raw_note, verified true -> EpistemicState is primary source of truth, Verified coerced to false
	f3 := factMap["fact-explicit-raw-verified-true"]
	if f3.EpistemicState != domain.EpistemicRawNote {
		t.Errorf("f3 epistemic state: expected %s, got %s", domain.EpistemicRawNote, f3.EpistemicState)
	}
	if f3.Verified {
		t.Errorf("f3 verified: expected false (coerced by raw_note), got true")
	}

	// 4. Explicit verified, verified false -> EpistemicState is primary source of truth, Verified coerced to true
	f4 := factMap["fact-explicit-verified-flag-false"]
	if f4.EpistemicState != domain.EpistemicVerified {
		t.Errorf("f4 epistemic state: expected %s, got %s", domain.EpistemicVerified, f4.EpistemicState)
	}
	if !f4.Verified {
		t.Errorf("f4 verified: expected true (coerced by verified state), got false")
	}
}

func TestBundleRoundtripMigration0002AndSubmittedImmutability(t *testing.T) {
	ctx := context.Background()
	v1 := newTestVault(t)

	_ = v1.SaveOrganization(ctx, &domain.Organization{ID: "org-1", Name: "Prior Co"})
	_ = v1.SaveRole(ctx, &domain.Role{ID: "role-1", OrgID: "org-1", Title: "Staff Engineer"})
	_ = v1.SaveCareerFact(ctx, &domain.CareerFact{
		ID:             "fact-1",
		RoleID:         "role-1",
		Headline:       "Cut tail latency",
		Details:        "Cut P99 latency by 42%.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	})
	_ = v1.SaveArchetype(ctx, &domain.Archetype{ID: "arch-1", Name: "Systems"})

	submittedAt := time.Date(2026, 11, 13, 19, 45, 0, 0, time.UTC)
	exportedAt := time.Date(2026, 11, 13, 19, 30, 0, 0, time.UTC)
	bundleSHA := strings.Repeat("c", 64)

	app := &domain.JobApplication{
		ID:          "app-full-fsm",
		RoleTitle:   "Principal Go Engineer",
		Company:     "Acme Corp",
		RawJD:       "Seeking Principal Go Engineer for pure-Go SQLite systems.",
		State:       domain.StateSubmitted,
		ArchetypeID: "arch-1",
		Dossier: &domain.OpportunityDossier{
			RoleTitle:  "Principal Go Engineer",
			Company:    "Acme Corp",
			RawJD:      "Seeking Principal Go Engineer for pure-Go SQLite systems.",
			FitSummary: "Strong fit.",
			JDClaims: []domain.JDClaim{
				{Summary: "Pure-Go SQLite", VerbatimJDQuote: "pure-Go SQLite systems."},
			},
			ExternalClaims: []domain.ExternalClaim{
				{Claim: "On-call rotation", UnknownAskRecruiter: true},
			},
		},
		Draft: &domain.ApplicationDraft{
			ArchetypeID: "arch-1",
			Headline:    "Principal Go Systems Engineer",
			Summary:     "Designs pure-Go kernels.",
			Experience: []domain.DraftExperience{
				{
					RoleTitle:    "Staff Engineer",
					Organization: "Prior Co",
					Highlights: []domain.BulletNode{
						{ID: "b-1", Text: "Cut P99 latency by 42%.", FactIDs: []string{"fact-1"}, Verified: true},
					},
				},
			},
			PageBudget: 1,
		},
		Audits: &domain.AuditBundle{
			FactCheck: &domain.FactCheckReport{
				CriticAgent: domain.CriticFactChecker,
				Summary:     "Verified.",
				Passed:      true,
				RecordedAt:  exportedAt,
			},
			AlignmentCheck: &domain.AlignmentCheckReport{
				CriticAgent:    domain.CriticAlignmentChecker,
				Summary:        "Aligned.",
				MatchedSignals: []string{"Go", "SQLite"},
				Passed:         true,
				RecordedAt:     exportedAt,
			},
		},
		Export: &domain.ExportRecord{
			PacketDir:  "/tmp/apps/2026-11-13-acme-corp-principal-go-engineer",
			HTMLPath:   "/tmp/apps/2026-11-13-acme-corp-principal-go-engineer/resume.html",
			DocxPath:   "/tmp/apps/2026-11-13-acme-corp-principal-go-engineer/resume.docx",
			PDFPath:    "/tmp/apps/2026-11-13-acme-corp-principal-go-engineer/resume.pdf",
			PageCount:  1,
			MaxPages:   1,
			ExportedAt: exportedAt,
		},
		BundleSHA256:       bundleSHA,
		InterviewBriefPath: "/tmp/apps/2026-11-13-acme-corp-principal-go-engineer/interview_brief.md",
		SubmittedAt:        &submittedAt,
	}
	if err := v1.SaveApplication(ctx, app); err != nil {
		t.Fatalf("SaveApplication: %v", err)
	}
	if err := v1.RecordApplicationCitations(ctx, app.ID, []string{"fact-1"}); err != nil {
		t.Fatalf("RecordApplicationCitations: %v", err)
	}

	// 1. Dump and Load into fresh vault: all Migration 0002 fields must round-trip faithfully.
	bundleDir := filepath.Join(t.TempDir(), "full-fsm-bundle")
	if err := v1.Dump(ctx, bundleDir, vault.DumpOptions{IncludePII: false}); err != nil {
		t.Fatalf("Dump: %v", err)
	}

	v2 := newTestVault(t)
	if err := v2.Load(ctx, bundleDir); err != nil {
		t.Fatalf("Load into v2: %v", err)
	}

	got, err := v2.GetApplicationContext(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationContext: %v", err)
	}
	if got.State != domain.StateSubmitted || got.BundleSHA256 != bundleSHA || got.InterviewBriefPath != app.InterviewBriefPath {
		t.Fatalf("submission lock metadata lost on round-trip: %+v", got)
	}
	if got.SubmittedAt == nil || !got.SubmittedAt.UTC().Equal(submittedAt) {
		t.Fatalf("expected SubmittedAt=%v, got %v", submittedAt, got.SubmittedAt)
	}
	if got.Dossier == nil || len(got.Dossier.JDClaims) != 1 || got.Dossier.JDClaims[0].VerbatimJDQuote != "pure-Go SQLite systems." {
		t.Fatalf("dossier lost on round-trip: %+v", got.Dossier)
	}
	if got.Draft == nil || got.Draft.Headline != "Principal Go Systems Engineer" || len(got.Draft.AllBullets()) != 1 {
		t.Fatalf("draft lost on round-trip: %+v", got.Draft)
	}
	if got.Audits == nil || got.Audits.FactCheck == nil || !got.Audits.FactCheck.Passed || got.Audits.AlignmentCheck == nil || !got.Audits.AlignmentCheck.Passed {
		t.Fatalf("audits lost on round-trip: %+v", got.Audits)
	}
	if got.Export == nil || got.Export.PDFPath != app.Export.PDFPath || got.Export.PageCount != 1 {
		t.Fatalf("export record lost on round-trip: %+v", got.Export)
	}

	// 2. Removing the Submitted application YAML & cited fact from disk must NOT delete the Submitted row
	// and must soft-archive (not hard-delete) fact-1.
	appFiles, err := filepath.Glob(filepath.Join(bundleDir, "applications", "*.yaml"))
	if err != nil || len(appFiles) != 1 {
		t.Fatalf("expected 1 application yaml, got %v (err=%v)", appFiles, err)
	}
	if err := os.Remove(appFiles[0]); err != nil {
		t.Fatalf("remove app yaml: %v", err)
	}
	orgFiles, err := filepath.Glob(filepath.Join(bundleDir, "organizations", "*.yaml"))
	if err != nil || len(orgFiles) != 1 {
		t.Fatalf("expected 1 org yaml, got %v (err=%v)", orgFiles, err)
	}
	if err := os.WriteFile(orgFiles[0], []byte("id: org-1\nname: Prior Co\nroles: []\n"), 0644); err != nil {
		t.Fatalf("overwrite org yaml: %v", err)
	}

	if err := v2.Load(ctx, bundleDir); err != nil {
		t.Fatalf("Load after removing Submitted app yaml failed: %v", err)
	}
	preservedApp, err := v2.GetApplicationContext(ctx, app.ID)
	if err != nil || preservedApp.State != domain.StateSubmitted || preservedApp.BundleSHA256 != bundleSHA {
		t.Fatalf("Submitted application must not be deleted on bundle reload: err=%v got=%+v", err, preservedApp)
	}
	archivedFact, err := v2.GetCareerFact("fact-1")
	if err != nil || archivedFact.EpistemicState != domain.EpistemicArchived {
		t.Fatalf("cited fact from preserved Submitted app must be soft-archived, err=%v fact=%+v", err, archivedFact)
	}

	// 3. Attempting to downgrade an existing Submitted application via YAML reload must fail with ErrAlreadySubmitted.
	downgradeYAML := `id: app-full-fsm
role_title: Tampered Title
company: Acme Corp
raw_jd: Tampered
state: Triaged
bundle_sha256: ""
`
	if err := os.WriteFile(filepath.Join(bundleDir, "applications", "app-full-fsm.yaml"), []byte(downgradeYAML), 0644); err != nil {
		t.Fatalf("write downgrade yaml: %v", err)
	}
	if err := v2.Load(ctx, bundleDir); !errors.Is(err, domain.ErrAlreadySubmitted) {
		t.Fatalf("expected ErrAlreadySubmitted when YAML attempts to downgrade Submitted app, got %v", err)
	}

	// 4. Attempting to mutate fields on an existing Submitted app while keeping state=Submitted preserves DB row immutable.
	tamperSameStateYAML := `id: app-full-fsm
role_title: Tampered Title
company: Tampered Co
raw_jd: Tampered JD
state: Submitted
bundle_sha256: ""
citations: []
`
	if err := os.WriteFile(filepath.Join(bundleDir, "applications", "app-full-fsm.yaml"), []byte(tamperSameStateYAML), 0644); err != nil {
		t.Fatalf("write tamperSameStateYAML: %v", err)
	}
	if err := v2.Load(ctx, bundleDir); err != nil {
		t.Fatalf("idempotent/same-state load failed: %v", err)
	}
	stillImmutable, err := v2.GetApplicationContext(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationContext after tamper attempt: %v", err)
	}
	if stillImmutable.RoleTitle != "Principal Go Engineer" || stillImmutable.BundleSHA256 != bundleSHA {
		t.Fatalf("Submitted app fields were mutated on reload: %+v", stillImmutable)
	}
	cits, err := v2.GetCitationsForApp(ctx, app.ID)
	if err != nil || len(cits) != 1 || cits[0] != "fact-1" {
		t.Fatalf("Submitted app citations were mutated on reload: %v (err=%v)", cits, err)
	}
}
