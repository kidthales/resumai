package vault_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/vault"
)

func TestImportLegacyActualSamples(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()

	// Locate content directory in the repo
	contentDir := filepath.Join("..", "..", "content")
	if _, err := os.Stat(contentDir); err != nil {
		t.Fatalf("could not locate content directory: %v", err)
	}

	report, err := v.ImportLegacy(ctx, contentDir, vault.LegacyImportOptions{
		DefaultEpistemicState: domain.EpistemicVerified,
	})
	if err != nil {
		t.Fatalf("ImportLegacy failed: %v", err)
	}

	// 1. Verify PII
	if !report.PIIImported {
		t.Fatalf("expected PII to be imported")
	}
	pii, err := v.GetPII(ctx)
	if err != nil {
		t.Fatalf("get pii: %v", err)
	}
	if pii.FullName != "Jane Doe" {
		t.Errorf("expected FullName 'Jane Doe', got %q", pii.FullName)
	}
	if pii.Email != "jane.doe@example.com" {
		t.Errorf("expected Email 'jane.doe@example.com', got %q", pii.Email)
	}
	if pii.Phone != "(555) 555-5555" {
		t.Errorf("expected Phone '(555) 555-5555', got %q", pii.Phone)
	}
	if pii.Location != "Some Place, Far Away" {
		t.Errorf("expected Location 'Some Place, Far Away', got %q", pii.Location)
	}

	// 2. Verify Education
	if report.Education != 2 {
		t.Errorf("expected 2 education records, got %d", report.Education)
	}
	edus, err := v.ListEducation(ctx)
	if err != nil {
		t.Fatalf("list education: %v", err)
	}
	if len(edus) != 2 {
		t.Fatalf("expected 2 education items in DB, got %d", len(edus))
	}

	mitFound := false
	for _, edu := range edus {
		if strings.Contains(edu.Institution, "Technology") {
			mitFound = true
			if !strings.Contains(edu.Degree, "Bachelor of Science") {
				t.Errorf("expected MIT degree Bachelor of Science, got %q", edu.Degree)
			}
			if edu.FieldOfStudy != "Mutant Studies" {
				t.Errorf("expected MIT field of study 'Mutant Studies', got %q", edu.FieldOfStudy)
			}
			if edu.Location != "Cambridge, Massachusetts, USA" {
				t.Errorf("expected MIT location Cambridge, got %q", edu.Location)
			}
		}
	}
	if !mitFound {
		t.Errorf("expected MIT education record not found")
	}

	// 3. Verify Organizations & Roles
	// 6 from work_history + 4 from extras = 10 organizations
	if report.Organizations != 10 {
		t.Errorf("expected 10 organizations, got %d", report.Organizations)
	}
	if report.Roles != 10 {
		t.Errorf("expected 10 roles, got %d", report.Roles)
	}

	orgs, err := v.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("list orgs: %v", err)
	}
	if len(orgs) != 10 {
		t.Errorf("expected 10 orgs in DB, got %d", len(orgs))
	}

	roles, err := v.ListRoles(ctx)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if len(roles) != 10 {
		t.Errorf("expected 10 roles in DB, got %d", len(roles))
	}

	workRoles := 0
	extraRoles := 0
	for _, r := range roles {
		if r.Category == "work" {
			workRoles++
		} else if r.Category == "extra" {
			extraRoles++
		}
	}
	if workRoles != 6 {
		t.Errorf("expected 6 work roles, got %d", workRoles)
	}
	if extraRoles != 4 {
		t.Errorf("expected 4 extra roles, got %d", extraRoles)
	}

	// 4. Verify Career Facts (20 work + 10 extra = 30)
	if report.Facts != 30 {
		t.Errorf("expected 30 career facts, got %d", report.Facts)
	}
	facts, err := v.ListCareerFacts(ctx)
	if err != nil {
		t.Fatalf("list facts: %v", err)
	}
	if len(facts) != 30 {
		t.Errorf("expected 30 facts in DB, got %d", len(facts))
	}

	for _, f := range facts {
		if f.Headline == "" {
			t.Errorf("fact %s has empty headline", f.ID)
		}
		if f.Details == "" {
			t.Errorf("fact %s has empty details", f.ID)
		}
		if f.EpistemicState != domain.EpistemicVerified {
			t.Errorf("expected fact %s epistemic state 'verified', got %q", f.ID, f.EpistemicState)
		}
		if !f.Verified {
			t.Errorf("expected fact %s verified=true", f.ID)
		}
	}

	// 5. Verify Archetypes (7 sample archetypes)
	if report.Archetypes != 7 {
		t.Errorf("expected 7 archetypes, got %d", report.Archetypes)
	}
	archs, err := v.ListArchetypes(ctx)
	if err != nil {
		t.Fatalf("list archetypes: %v", err)
	}
	if len(archs) != 7 {
		t.Errorf("expected 7 archetypes in DB, got %d", len(archs))
	}

	for _, a := range archs {
		if a.Name == "" {
			t.Errorf("archetype %s has empty name", a.ID)
		}
		if a.ProfileOverview == "" {
			t.Errorf("archetype %s has empty overview", a.ID)
		}
		if len(a.CoreCompetencies) == 0 {
			t.Errorf("archetype %s has empty core competencies", a.ID)
		}
		if len(a.StrategicImpact) == 0 {
			t.Errorf("archetype %s has empty strategic impact", a.ID)
		}
		if len(a.TargetRoles) == 0 {
			t.Errorf("archetype %s has empty target roles", a.ID)
		}
	}
}

func TestImportLegacyCandidateLogisticsAndPII(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()
	tempDir := t.TempDir()

	personalInfoContent := `---
name: Peter Parker
email: spidey@dailybugle.com
phone: (212) 555-0199
location: Queens, New York
linkedin: https://linkedin.com/in/peterparker
github: https://github.com/peterparker
portfolio: https://spiderman.nyc
target_title: Senior Web Developer
work_authorization: US Citizen
remote_preference: Hybrid
willing_to_relocate: false
target_compensation: $160,000
notice_period_days: 14
---
# Personal Info
`
	if err := os.WriteFile(filepath.Join(tempDir, "personal_info.md"), []byte(personalInfoContent), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	report, err := v.ImportLegacy(ctx, tempDir)
	if err != nil {
		t.Fatalf("import legacy: %v", err)
	}

	if !report.PIIImported {
		t.Errorf("expected PIIImported true")
	}
	if !report.LogisticsImported {
		t.Errorf("expected LogisticsImported true")
	}

	pii, err := v.GetPII(ctx)
	if err != nil {
		t.Fatalf("get pii: %v", err)
	}
	if pii.FullName != "Peter Parker" || pii.LinkedInURL != "https://linkedin.com/in/peterparker" {
		t.Errorf("unexpected PII data: %+v", pii)
	}

	cl, err := v.GetCandidateLogistics(ctx)
	if err != nil {
		t.Fatalf("get logistics: %v", err)
	}
	if cl.TargetTitle != "Senior Web Developer" {
		t.Errorf("expected TargetTitle 'Senior Web Developer', got %q", cl.TargetTitle)
	}
	if cl.WorkAuthorization != "US Citizen" {
		t.Errorf("expected WorkAuthorization 'US Citizen', got %q", cl.WorkAuthorization)
	}
	if cl.RemotePreference != "Hybrid" {
		t.Errorf("expected RemotePreference 'Hybrid', got %q", cl.RemotePreference)
	}
	if cl.WillingToRelocate != false {
		t.Errorf("expected WillingToRelocate false")
	}
	if cl.NoticePeriodDays != 14 {
		t.Errorf("expected NoticePeriodDays 14, got %d", cl.NoticePeriodDays)
	}
}

func TestImportLegacyAirGapSkipPII(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()
	tempDir := t.TempDir()

	personalInfoContent := `- **Name:** Tony Stark
- **Email:** tony@stark.com
`
	if err := os.WriteFile(filepath.Join(tempDir, "personal_info.md"), []byte(personalInfoContent), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	report, err := v.ImportLegacy(ctx, tempDir, vault.LegacyImportOptions{
		SkipPII: true,
	})
	if err != nil {
		t.Fatalf("import legacy: %v", err)
	}

	if report.PIIImported {
		t.Errorf("expected PIIImported to be false when SkipPII=true")
	}

	_, err = v.GetPII(ctx)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for PII, got %v", err)
	}
}

func TestImportLegacyDefaultStateRawNoteAndMarkers(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()
	tempDir := t.TempDir()

	workContent := `- **Stark Industries**
    - **Role:** Lead Automation Engineer
        - [verified] Architected drone telemetry relay.
        - [metric_pending] Reduced factory cycle time by 40%.
        - General bug fixes and maintenance scripts.
    - **Location:** New York, NY
    - **Duration:** 2020 - 2024
`
	if err := os.WriteFile(filepath.Join(tempDir, "work_history.md"), []byte(workContent), 0644); err != nil {
		t.Fatalf("write work_history.md: %v", err)
	}

	report, err := v.ImportLegacy(ctx, tempDir, vault.LegacyImportOptions{
		DefaultEpistemicState: domain.EpistemicRawNote,
	})
	if err != nil {
		t.Fatalf("import legacy: %v", err)
	}

	if report.Facts != 3 {
		t.Fatalf("expected 3 facts, got %d", report.Facts)
	}

	facts, err := v.ListCareerFacts(ctx)
	if err != nil {
		t.Fatalf("list facts: %v", err)
	}

	for _, f := range facts {
		if strings.Contains(f.Details, "Architected drone telemetry") {
			if f.EpistemicState != domain.EpistemicVerified || !f.Verified {
				t.Errorf("expected explicit [verified] fact to be verified, got %s / %t", f.EpistemicState, f.Verified)
			}
		} else if strings.Contains(f.Details, "Reduced factory cycle time") {
			if f.EpistemicState != domain.EpistemicMetricPending || f.Verified {
				t.Errorf("expected explicit [metric_pending] fact, got %s / %t", f.EpistemicState, f.Verified)
			}
		} else if strings.Contains(f.Details, "General bug fixes") {
			if f.EpistemicState != domain.EpistemicRawNote || f.Verified {
				t.Errorf("expected default state raw_note fact, got %s / %t", f.EpistemicState, f.Verified)
			}
		}
	}
}

func TestImportLegacyArchetypeFrontmatter(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()
	tempDir := t.TempDir()
	archDir := filepath.Join(tempDir, "archetypes")
	if err := os.MkdirAll(archDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	archContent := `---
id: staff-infra
name: Staff Infrastructure Architect
profile_overview: Elite systems architect with 15+ years experience.
core_competencies:
  - Distributed Consensus (Raft/Paxos)
  - Zero-Trust Mesh Networking
strategic_impact:
  - Mentored 12 principal engineers
target_roles:
  - Staff Infrastructure Engineer
  - Principal Systems Architect
---
# Ignored Header
## Profile Overview
Ignored body overview
`
	if err := os.WriteFile(filepath.Join(archDir, "staff_infra.md"), []byte(archContent), 0644); err != nil {
		t.Fatalf("write archetype: %v", err)
	}

	report, err := v.ImportLegacy(ctx, tempDir)
	if err != nil {
		t.Fatalf("import legacy: %v", err)
	}

	if report.Archetypes != 1 {
		t.Fatalf("expected 1 archetype, got %d", report.Archetypes)
	}

	arch, err := v.GetArchetype(ctx, "staff-infra")
	if err != nil {
		t.Fatalf("get archetype: %v", err)
	}
	if arch.Name != "Staff Infrastructure Architect" {
		t.Errorf("expected name 'Staff Infrastructure Architect', got %q", arch.Name)
	}
	if arch.ProfileOverview != "Elite systems architect with 15+ years experience." {
		t.Errorf("expected frontmatter overview, got %q", arch.ProfileOverview)
	}
	if len(arch.CoreCompetencies) != 2 || arch.CoreCompetencies[0] != "Distributed Consensus (Raft/Paxos)" {
		t.Errorf("unexpected core competencies: %+v", arch.CoreCompetencies)
	}
}

func TestImportLegacyNoFilesError(t *testing.T) {
	v := newTestVault(t)
	ctx := context.Background()
	emptyDir := t.TempDir()

	_, err := v.ImportLegacy(ctx, emptyDir)
	if err == nil {
		t.Fatalf("expected error importing empty directory, got nil")
	}
	if !strings.Contains(err.Error(), "no legacy files found") {
		t.Errorf("expected 'no legacy files found' error, got %v", err)
	}
}
