package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"gopkg.in/yaml.v3"
)

// DumpOptions configures vault export behavior.
type DumpOptions struct {
	IncludePII bool
}

// BundlePII represents candidate contact PII in pii.yaml.
type BundlePII struct {
	FullName     string `yaml:"full_name"`
	Email        string `yaml:"email"`
	Phone        string `yaml:"phone"`
	Location     string `yaml:"location"`
	LinkedInURL  string `yaml:"linkedin_url"`
	GitHubURL    string `yaml:"github_url"`
	PortfolioURL string `yaml:"portfolio_url"`
}

// BundleLogistics represents candidate logistics in logistics.yaml.
type BundleLogistics struct {
	TargetTitle        string `yaml:"target_title"`
	WorkAuthorization  string `yaml:"work_authorization"`
	RemotePreference   string `yaml:"remote_preference"`
	WillingToRelocate  bool   `yaml:"willing_to_relocate"`
	TargetCompensation string `yaml:"target_compensation"`
	NoticePeriodDays   int    `yaml:"notice_period_days"`
}

// BundleFact represents a single career accomplishment atom under a role.
type BundleFact struct {
	ID             string   `yaml:"id"`
	Headline       string   `yaml:"headline"`
	Details        string   `yaml:"details"`
	EpistemicState string   `yaml:"epistemic_state"`
	TechTags       []string `yaml:"tech_tags,omitempty"`
	Verified       bool     `yaml:"verified"`
}

// BundleRole represents a position held within an organization.
type BundleRole struct {
	ID        string       `yaml:"id"`
	Title     string       `yaml:"title"`
	Category  string       `yaml:"category"`
	Location  string       `yaml:"location"`
	Duration  string       `yaml:"duration"`
	TechStack []string     `yaml:"tech_stack,omitempty"`
	Facts     []BundleFact `yaml:"facts,omitempty"`
}

// BundleOrganization represents an organization and its nested roles and facts.
type BundleOrganization struct {
	ID       string       `yaml:"id"`
	Name     string       `yaml:"name"`
	Location string       `yaml:"location"`
	Website  string       `yaml:"website"`
	Roles    []BundleRole `yaml:"roles,omitempty"`
}

// BundleEducation represents an academic credential in education/<id>.yaml.
type BundleEducation struct {
	ID           string `yaml:"id"`
	Institution  string `yaml:"institution"`
	Degree       string `yaml:"degree"`
	FieldOfStudy string `yaml:"field_of_study"`
	Location     string `yaml:"location"`
	Duration     string `yaml:"duration"`
	Honors       string `yaml:"honors"`
}

// BundleArchetype represents a resume persona in archetypes/<id>.yaml.
type BundleArchetype struct {
	ID               string   `yaml:"id"`
	Name             string   `yaml:"name"`
	ProfileOverview  string   `yaml:"profile_overview"`
	CoreCompetencies []string `yaml:"core_competencies,omitempty"`
	StrategicImpact  []string `yaml:"strategic_impact,omitempty"`
	TargetRoles      []string `yaml:"target_roles,omitempty"`
}

// BundleApplication represents a job application in applications/<id>.yaml.
type BundleApplication struct {
	ID                 string   `yaml:"id"`
	RoleTitle          string   `yaml:"role_title"`
	Company            string   `yaml:"company"`
	RawJD              string   `yaml:"raw_jd"`
	State              string   `yaml:"state"`
	RejectionRationale string   `yaml:"rejection_rationale,omitempty"`
	ArchetypeID        string   `yaml:"archetype_id,omitempty"`
	Citations          []string `yaml:"citations,omitempty"`
}

// Dump exports the CareerVault data into a structured YAML directory bundle.
// Contact PII is air-gapped and strictly omitted unless opts.IncludePII is true.
func (v *CareerVault) Dump(ctx context.Context, dir string, opts DumpOptions) error {
	cleanDir := filepath.Clean(dir)
	if err := os.MkdirAll(cleanDir, 0755); err != nil {
		return fmt.Errorf("create dump dir: %w", err)
	}

	piiPath := filepath.Join(cleanDir, "pii.yaml")
	if opts.IncludePII {
		pii, err := v.GetPII(ctx)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("dump pii: %w", err)
		}
		if pii != nil {
			bPII := BundlePII{
				FullName:     pii.FullName,
				Email:        pii.Email,
				Phone:        pii.Phone,
				Location:     pii.Location,
				LinkedInURL:  pii.LinkedInURL,
				GitHubURL:    pii.GitHubURL,
				PortfolioURL: pii.PortfolioURL,
			}
			if err := writeYAML(piiPath, bPII); err != nil {
				return fmt.Errorf("write pii.yaml: %w", err)
			}
		} else {
			_ = os.Remove(piiPath)
		}
	} else {
		// Strict air-gap: ensure no stale pii.yaml exists in the dump directory
		_ = os.Remove(piiPath)
	}

	// 2. Candidate Logistics
	logistics, err := v.GetCandidateLogistics(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("dump logistics: %w", err)
	}
	if logistics != nil {
		bLog := BundleLogistics{
			TargetTitle:        logistics.TargetTitle,
			WorkAuthorization:  logistics.WorkAuthorization,
			RemotePreference:   logistics.RemotePreference,
			WillingToRelocate:  logistics.WillingToRelocate,
			TargetCompensation: logistics.TargetCompensation,
			NoticePeriodDays:   logistics.NoticePeriodDays,
		}
		if err := writeYAML(filepath.Join(cleanDir, "logistics.yaml"), bLog); err != nil {
			return fmt.Errorf("write logistics.yaml: %w", err)
		}
	}

	// 3. Organizations -> Roles -> Facts
	orgs, err := v.ListOrganizations(ctx)
	if err != nil {
		return fmt.Errorf("dump list organizations: %w", err)
	}
	orgDir := filepath.Join(cleanDir, "organizations")
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		return fmt.Errorf("create organizations dir: %w", err)
	}

	for _, org := range orgs {
		roles, err := v.ListRolesByOrg(ctx, org.ID)
		if err != nil {
			return fmt.Errorf("dump list roles for org %s: %w", org.ID, err)
		}

		bOrg := BundleOrganization{
			ID:       org.ID,
			Name:     org.Name,
			Location: org.Location,
			Website:  org.Website,
		}

		for _, r := range roles {
			facts, err := v.ListCareerFactsByRole(ctx, r.ID)
			if err != nil {
				return fmt.Errorf("dump list facts for role %s: %w", r.ID, err)
			}

			bRole := BundleRole{
				ID:        r.ID,
				Title:     r.Title,
				Category:  r.Category,
				Location:  r.Location,
				Duration:  r.Duration,
				TechStack: r.TechStack,
			}

			for _, f := range facts {
				bRole.Facts = append(bRole.Facts, BundleFact{
					ID:             f.ID,
					Headline:       f.Headline,
					Details:        f.Details,
					EpistemicState: string(f.EpistemicState),
					TechTags:       f.TechTags,
					Verified:       f.Verified,
				})
			}
			bOrg.Roles = append(bOrg.Roles, bRole)
		}

		fileSlug := slugify(org.ID)
		if fileSlug == "" || fileSlug == "item" {
			fileSlug = slugify(org.Name)
		}
		if err := writeYAML(filepath.Join(orgDir, fileSlug+".yaml"), bOrg); err != nil {
			return fmt.Errorf("write org yaml (%s): %w", org.ID, err)
		}
	}

	// 4. Education
	edus, err := v.ListEducation(ctx)
	if err != nil {
		return fmt.Errorf("dump list education: %w", err)
	}
	eduDir := filepath.Join(cleanDir, "education")
	if err := os.MkdirAll(eduDir, 0755); err != nil {
		return fmt.Errorf("create education dir: %w", err)
	}
	for _, edu := range edus {
		bEdu := BundleEducation{
			ID:           edu.ID,
			Institution:  edu.Institution,
			Degree:       edu.Degree,
			FieldOfStudy: edu.FieldOfStudy,
			Location:     edu.Location,
			Duration:     edu.Duration,
			Honors:       edu.Honors,
		}
		fileSlug := slugify(edu.ID)
		if fileSlug == "" || fileSlug == "item" {
			fileSlug = slugify(edu.Institution)
		}
		if err := writeYAML(filepath.Join(eduDir, fileSlug+".yaml"), bEdu); err != nil {
			return fmt.Errorf("write education yaml (%s): %w", edu.ID, err)
		}
	}

	// 5. Archetypes
	archs, err := v.ListArchetypes(ctx)
	if err != nil {
		return fmt.Errorf("dump list archetypes: %w", err)
	}
	archDir := filepath.Join(cleanDir, "archetypes")
	if err := os.MkdirAll(archDir, 0755); err != nil {
		return fmt.Errorf("create archetypes dir: %w", err)
	}
	for _, a := range archs {
		bArch := BundleArchetype{
			ID:               a.ID,
			Name:             a.Name,
			ProfileOverview:  a.ProfileOverview,
			CoreCompetencies: a.CoreCompetencies,
			StrategicImpact:  a.StrategicImpact,
			TargetRoles:      a.TargetRoles,
		}
		fileSlug := slugify(a.ID)
		if fileSlug == "" || fileSlug == "item" {
			fileSlug = slugify(a.Name)
		}
		if err := writeYAML(filepath.Join(archDir, fileSlug+".yaml"), bArch); err != nil {
			return fmt.Errorf("write archetype yaml (%s): %w", a.ID, err)
		}
	}

	// 6. Applications
	apps, err := v.ListApplications(ctx)
	if err != nil {
		return fmt.Errorf("dump list applications: %w", err)
	}
	appDir := filepath.Join(cleanDir, "applications")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("create applications dir: %w", err)
	}
	for _, app := range apps {
		citations, err := v.GetCitationsForApp(ctx, app.ID)
		if err != nil {
			return fmt.Errorf("dump citations for app %s: %w", app.ID, err)
		}
		bApp := BundleApplication{
			ID:                 app.ID,
			RoleTitle:          app.RoleTitle,
			Company:            app.Company,
			RawJD:              app.RawJD,
			State:              string(app.State),
			RejectionRationale: app.RejectionRationale,
			ArchetypeID:        app.ArchetypeID,
			Citations:          citations,
		}
		fileSlug := slugify(app.ID)
		if fileSlug == "" || fileSlug == "item" {
			fileSlug = slugify(app.Company + "-" + app.RoleTitle)
		}
		if err := writeYAML(filepath.Join(appDir, fileSlug+".yaml"), bApp); err != nil {
			return fmt.Errorf("write application yaml (%s): %w", app.ID, err)
		}
	}

	return nil
}

// Load imports a structured YAML directory bundle into the CareerVault.
// Invariant: Reloading or deleting vault YAML bundles never destroys fact_id
// provenance cited by Submitted applications, transitioning missing rows to archived instead.
func (v *CareerVault) Load(ctx context.Context, dir string) error {
	cleanDir := filepath.Clean(dir)
	info, err := os.Stat(cleanDir)
	if err != nil {
		return fmt.Errorf("stat bundle dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("bundle path %q is not a directory", cleanDir)
	}

	// 1. Ingest all YAML files into memory first for validation before starting DB tx
	var piiLoaded *BundlePII
	piiFile := filepath.Join(cleanDir, "pii.yaml")
	if fileExists(piiFile) {
		var pii BundlePII
		if err := readYAML(piiFile, &pii); err != nil {
			return fmt.Errorf("read pii.yaml: %w", err)
		}
		piiLoaded = &pii
	}

	var logisticsLoaded *BundleLogistics
	logisticsFile := filepath.Join(cleanDir, "logistics.yaml")
	if fileExists(logisticsFile) {
		var log BundleLogistics
		if err := readYAML(logisticsFile, &log); err != nil {
			return fmt.Errorf("read logistics.yaml: %w", err)
		}
		logisticsLoaded = &log
	}

	// Organizations
	var orgsLoaded []BundleOrganization
	orgDir := filepath.Join(cleanDir, "organizations")
	if dirExists(orgDir) {
		files, err := listYAMLFiles(orgDir)
		if err != nil {
			return fmt.Errorf("list organizations yaml: %w", err)
		}
		for _, f := range files {
			var org BundleOrganization
			if err := readYAML(f, &org); err != nil {
				return fmt.Errorf("read org yaml (%s): %w", f, err)
			}
			if org.ID == "" {
				org.ID = fileStem(f)
			}
			for ri := range org.Roles {
				if org.Roles[ri].ID == "" {
					org.Roles[ri].ID = fmt.Sprintf("%s-role-%d", org.ID, ri+1)
				}
				for fi := range org.Roles[ri].Facts {
					if org.Roles[ri].Facts[fi].ID == "" {
						org.Roles[ri].Facts[fi].ID = fmt.Sprintf("%s-fact-%d", org.Roles[ri].ID, fi+1)
					}
					// Validate epistemic state
					state := domain.EpistemicState(org.Roles[ri].Facts[fi].EpistemicState)
					if state == "" {
						state = domain.EpistemicRawNote
						org.Roles[ri].Facts[fi].EpistemicState = string(state)
					}
					if err := state.Validate(); err != nil {
						return fmt.Errorf("invalid epistemic state in fact %s: %w", org.Roles[ri].Facts[fi].ID, err)
					}
				}
			}
			orgsLoaded = append(orgsLoaded, org)
		}
	} else if fileExists(filepath.Join(cleanDir, "organizations.yaml")) {
		var orgs []BundleOrganization
		if err := readYAML(filepath.Join(cleanDir, "organizations.yaml"), &orgs); err != nil {
			return fmt.Errorf("read organizations.yaml: %w", err)
		}
		orgsLoaded = orgs
	}

	// Education
	var edusLoaded []BundleEducation
	eduDir := filepath.Join(cleanDir, "education")
	if dirExists(eduDir) {
		files, err := listYAMLFiles(eduDir)
		if err != nil {
			return fmt.Errorf("list education yaml: %w", err)
		}
		for _, f := range files {
			var edu BundleEducation
			if err := readYAML(f, &edu); err != nil {
				return fmt.Errorf("read education yaml (%s): %w", f, err)
			}
			if edu.ID == "" {
				edu.ID = fileStem(f)
			}
			edusLoaded = append(edusLoaded, edu)
		}
	} else if fileExists(filepath.Join(cleanDir, "education.yaml")) {
		var edus []BundleEducation
		if err := readYAML(filepath.Join(cleanDir, "education.yaml"), &edus); err != nil {
			return fmt.Errorf("read education.yaml: %w", err)
		}
		edusLoaded = edus
	}

	// Archetypes
	var archsLoaded []BundleArchetype
	archDir := filepath.Join(cleanDir, "archetypes")
	if dirExists(archDir) {
		files, err := listYAMLFiles(archDir)
		if err != nil {
			return fmt.Errorf("list archetypes yaml: %w", err)
		}
		for _, f := range files {
			var arch BundleArchetype
			if err := readYAML(f, &arch); err != nil {
				return fmt.Errorf("read archetype yaml (%s): %w", f, err)
			}
			if arch.ID == "" {
				arch.ID = fileStem(f)
			}
			archsLoaded = append(archsLoaded, arch)
		}
	} else if fileExists(filepath.Join(cleanDir, "archetypes.yaml")) {
		var archs []BundleArchetype
		if err := readYAML(filepath.Join(cleanDir, "archetypes.yaml"), &archs); err != nil {
			return fmt.Errorf("read archetypes.yaml: %w", err)
		}
		archsLoaded = archs
	}

	// Applications
	var appsLoaded []BundleApplication
	hasApplicationsSection := false
	appDir := filepath.Join(cleanDir, "applications")
	if dirExists(appDir) {
		hasApplicationsSection = true
		files, err := listYAMLFiles(appDir)
		if err != nil {
			return fmt.Errorf("list applications yaml: %w", err)
		}
		for _, f := range files {
			var app BundleApplication
			if err := readYAML(f, &app); err != nil {
				return fmt.Errorf("read application yaml (%s): %w", f, err)
			}
			if app.ID == "" {
				app.ID = fileStem(f)
			}
			appsLoaded = append(appsLoaded, app)
		}
	} else if fileExists(filepath.Join(cleanDir, "applications.yaml")) {
		hasApplicationsSection = true
		var apps []BundleApplication
		if err := readYAML(filepath.Join(cleanDir, "applications.yaml"), &apps); err != nil {
			return fmt.Errorf("read applications.yaml: %w", err)
		}
		appsLoaded = apps
	}

	// 2. Perform transactional updates in a single isolated SQLite transaction
	return v.WithTx(ctx, func(tx *sql.Tx) error {
		nowStr := time.Now().UTC().Format(time.RFC3339)

		// A. PII (Only updated if pii.yaml was present in bundle)
		if piiLoaded != nil {
			queryPII := `
			INSERT INTO pii_header (id, full_name, email, phone, location, linkedin_url, github_url, portfolio_url, updated_at)
			VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				full_name = excluded.full_name,
				email = excluded.email,
				phone = excluded.phone,
				location = excluded.location,
				linkedin_url = excluded.linkedin_url,
				github_url = excluded.github_url,
				portfolio_url = excluded.portfolio_url,
				updated_at = excluded.updated_at;`

			if _, err := tx.ExecContext(ctx, queryPII,
				piiLoaded.FullName,
				piiLoaded.Email,
				piiLoaded.Phone,
				piiLoaded.Location,
				piiLoaded.LinkedInURL,
				piiLoaded.GitHubURL,
				piiLoaded.PortfolioURL,
				nowStr,
			); err != nil {
				return fmt.Errorf("tx save pii: %w", err)
			}
		}

		// B. Logistics (Only updated if logistics.yaml was present)
		if logisticsLoaded != nil {
			willingRelocateInt := 0
			if logisticsLoaded.WillingToRelocate {
				willingRelocateInt = 1
			}
			queryLog := `
			INSERT INTO candidate_logistics (id, target_title, work_authorization, remote_preference, willing_to_relocate, target_compensation, notice_period_days, updated_at)
			VALUES (1, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				target_title = excluded.target_title,
				work_authorization = excluded.work_authorization,
				remote_preference = excluded.remote_preference,
				willing_to_relocate = excluded.willing_to_relocate,
				target_compensation = excluded.target_compensation,
				notice_period_days = excluded.notice_period_days,
				updated_at = excluded.updated_at;`

			if _, err := tx.ExecContext(ctx, queryLog,
				logisticsLoaded.TargetTitle,
				logisticsLoaded.WorkAuthorization,
				logisticsLoaded.RemotePreference,
				willingRelocateInt,
				logisticsLoaded.TargetCompensation,
				logisticsLoaded.NoticePeriodDays,
				nowStr,
			); err != nil {
				return fmt.Errorf("tx save logistics: %w", err)
			}
		}

		// C. Organizations, Roles, Facts Reconciliation with Soft-Archive Guard
		incomingFactIDs := make(map[string]bool)
		incomingRoleIDs := make(map[string]bool)
		incomingOrgIDs := make(map[string]bool)

		for _, org := range orgsLoaded {
			incomingOrgIDs[org.ID] = true
			for _, r := range org.Roles {
				incomingRoleIDs[r.ID] = true
				for _, f := range r.Facts {
					incomingFactIDs[f.ID] = true
				}
			}
		}

		// If organizations/ was present in bundle (or organizations.yaml), reconcile missing rows
		hasOrgSection := dirExists(orgDir) || fileExists(filepath.Join(cleanDir, "organizations.yaml"))
		if hasOrgSection {
			// Find all existing facts
			factRows, err := tx.QueryContext(ctx, `SELECT id FROM career_facts;`)
			if err != nil {
				return fmt.Errorf("tx query facts: %w", err)
			}
			var existingFactIDs []string
			for factRows.Next() {
				var fid string
				if err := factRows.Scan(&fid); err != nil {
					factRows.Close()
					return err
				}
				existingFactIDs = append(existingFactIDs, fid)
			}
			factRows.Close()

			for _, fid := range existingFactIDs {
				if !incomingFactIDs[fid] {
					// Check if cited in a Submitted application
					var citedInSubmitted int
					checkQuery := `
					SELECT COUNT(1)
					FROM application_citations ac
					JOIN applications a ON ac.application_id = a.id
					WHERE ac.fact_id = ? AND a.state = 'Submitted';`
					if err := tx.QueryRowContext(ctx, checkQuery, fid).Scan(&citedInSubmitted); err != nil {
						return fmt.Errorf("tx check submitted fact citation %s: %w", fid, err)
					}

					if citedInSubmitted > 0 {
						// INVARIANT: Do not delete! Soft-archive to preserve provenance!
						archiveQuery := `
						UPDATE career_facts
						SET epistemic_state = ?, verified = 0, updated_at = ?
						WHERE id = ?;`
						if _, err := tx.ExecContext(ctx, archiveQuery, string(domain.EpistemicArchived), nowStr, fid); err != nil {
							return fmt.Errorf("tx soft-archive fact %s: %w", fid, err)
						}
					} else {
						if _, err := tx.ExecContext(ctx, `DELETE FROM career_facts WHERE id = ?;`, fid); err != nil {
							return fmt.Errorf("tx delete missing fact %s: %w", fid, err)
						}
					}
				}
			}

			// Clean up roles that are not in incoming bundle AND have zero facts left
			roleRows, err := tx.QueryContext(ctx, `SELECT id FROM roles;`)
			if err != nil {
				return fmt.Errorf("tx query roles: %w", err)
			}
			var existingRoleIDs []string
			for roleRows.Next() {
				var rid string
				if err := roleRows.Scan(&rid); err != nil {
					roleRows.Close()
					return err
				}
				existingRoleIDs = append(existingRoleIDs, rid)
			}
			roleRows.Close()

			for _, rid := range existingRoleIDs {
				if !incomingRoleIDs[rid] {
					var remainingFacts int
					_ = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM career_facts WHERE role_id = ?;`, rid).Scan(&remainingFacts)
					if remainingFacts == 0 {
						if _, err := tx.ExecContext(ctx, `DELETE FROM roles WHERE id = ?;`, rid); err != nil {
							return fmt.Errorf("tx delete role %s: %w", rid, err)
						}
					}
				}
			}

			// Clean up organizations that are not in incoming bundle AND have zero roles left
			orgRows, err := tx.QueryContext(ctx, `SELECT id FROM organizations;`)
			if err != nil {
				return fmt.Errorf("tx query orgs: %w", err)
			}
			var existingOrgIDs []string
			for orgRows.Next() {
				var oid string
				if err := orgRows.Scan(&oid); err != nil {
					orgRows.Close()
					return err
				}
				existingOrgIDs = append(existingOrgIDs, oid)
			}
			orgRows.Close()

			for _, oid := range existingOrgIDs {
				if !incomingOrgIDs[oid] {
					var remainingRoles int
					_ = tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM roles WHERE org_id = ?;`, oid).Scan(&remainingRoles)
					if remainingRoles == 0 {
						if _, err := tx.ExecContext(ctx, `DELETE FROM organizations WHERE id = ?;`, oid); err != nil {
							return fmt.Errorf("tx delete org %s: %w", oid, err)
						}
					}
				}
			}
		}

		// Upsert Organizations, Roles, Facts
		orgStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO organizations (id, name, location, website, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			location = excluded.location,
			website = excluded.website,
			updated_at = excluded.updated_at;`)
		if err != nil {
			return fmt.Errorf("prepare org stmt: %w", err)
		}
		defer orgStmt.Close()

		roleStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO roles (id, org_id, title, category, location, duration, tech_stack, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			org_id = excluded.org_id,
			title = excluded.title,
			category = excluded.category,
			location = excluded.location,
			duration = excluded.duration,
			tech_stack = excluded.tech_stack,
			updated_at = excluded.updated_at;`)
		if err != nil {
			return fmt.Errorf("prepare role stmt: %w", err)
		}
		defer roleStmt.Close()

		factStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO career_facts (id, role_id, headline, details, epistemic_state, tech_tags, verified, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			role_id = excluded.role_id,
			headline = excluded.headline,
			details = excluded.details,
			epistemic_state = excluded.epistemic_state,
			tech_tags = excluded.tech_tags,
			verified = excluded.verified,
			updated_at = excluded.updated_at;`)
		if err != nil {
			return fmt.Errorf("prepare fact stmt: %w", err)
		}
		defer factStmt.Close()

		for _, org := range orgsLoaded {
			if _, err := orgStmt.ExecContext(ctx, org.ID, org.Name, org.Location, org.Website, nowStr, nowStr); err != nil {
				return fmt.Errorf("exec save org %s: %w", org.ID, err)
			}

			for _, r := range org.Roles {
				cat := r.Category
				if cat == "" {
					cat = "work"
				}
				techJSON, _ := json.Marshal(r.TechStack)
				if _, err := roleStmt.ExecContext(ctx, r.ID, org.ID, r.Title, cat, r.Location, r.Duration, string(techJSON), nowStr, nowStr); err != nil {
					return fmt.Errorf("exec save role %s: %w", r.ID, err)
				}

				for _, f := range r.Facts {
					eState := domain.EpistemicState(f.EpistemicState)
					if eState == "" {
						if f.Verified {
							eState = domain.EpistemicVerified
						} else {
							eState = domain.EpistemicRawNote
						}
					}
					isVerified := (eState == domain.EpistemicVerified)
					verifiedInt := 0
					if isVerified {
						verifiedInt = 1
					}

					tagsJSON, _ := json.Marshal(f.TechTags)
					if _, err := factStmt.ExecContext(ctx, f.ID, r.ID, f.Headline, f.Details, string(eState), string(tagsJSON), verifiedInt, nowStr, nowStr); err != nil {
						return fmt.Errorf("exec save fact %s: %w", f.ID, err)
					}
				}
			}
		}

		// D. Education
		hasEduSection := dirExists(eduDir) || fileExists(filepath.Join(cleanDir, "education.yaml"))
		if hasEduSection {
			incomingEduIDs := make(map[string]bool)
			for _, edu := range edusLoaded {
				incomingEduIDs[edu.ID] = true
			}
			rows, err := tx.QueryContext(ctx, `SELECT id FROM education;`)
			if err != nil {
				return err
			}
			var existingEduIDs []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					existingEduIDs = append(existingEduIDs, id)
				}
			}
			rows.Close()

			for _, id := range existingEduIDs {
				if !incomingEduIDs[id] {
					if _, err := tx.ExecContext(ctx, `DELETE FROM education WHERE id = ?;`, id); err != nil {
						return fmt.Errorf("delete education %s: %w", id, err)
					}
				}
			}

			eduStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO education (id, institution, degree, field_of_study, location, duration, honors, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				institution = excluded.institution,
				degree = excluded.degree,
				field_of_study = excluded.field_of_study,
				location = excluded.location,
				duration = excluded.duration,
				honors = excluded.honors,
				updated_at = excluded.updated_at;`)
			if err != nil {
				return err
			}
			defer eduStmt.Close()

			for _, edu := range edusLoaded {
				if _, err := eduStmt.ExecContext(ctx, edu.ID, edu.Institution, edu.Degree, edu.FieldOfStudy, edu.Location, edu.Duration, edu.Honors, nowStr, nowStr); err != nil {
					return fmt.Errorf("exec save edu %s: %w", edu.ID, err)
				}
			}
		}

		// E. Archetypes
		hasArchSection := dirExists(archDir) || fileExists(filepath.Join(cleanDir, "archetypes.yaml"))
		if hasArchSection {
			incomingArchIDs := make(map[string]bool)
			for _, a := range archsLoaded {
				incomingArchIDs[a.ID] = true
			}
			rows, err := tx.QueryContext(ctx, `SELECT id FROM archetypes;`)
			if err != nil {
				return err
			}
			var existingArchIDs []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					existingArchIDs = append(existingArchIDs, id)
				}
			}
			rows.Close()

			for _, id := range existingArchIDs {
				if !incomingArchIDs[id] {
					if _, err := tx.ExecContext(ctx, `DELETE FROM archetypes WHERE id = ?;`, id); err != nil {
						return fmt.Errorf("delete archetype %s: %w", id, err)
					}
				}
			}

			archStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO archetypes (id, name, profile_overview, core_competencies, strategic_impact, target_roles, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name = excluded.name,
				profile_overview = excluded.profile_overview,
				core_competencies = excluded.core_competencies,
				strategic_impact = excluded.strategic_impact,
				target_roles = excluded.target_roles,
				updated_at = excluded.updated_at;`)
			if err != nil {
				return err
			}
			defer archStmt.Close()

			for _, a := range archsLoaded {
				compJSON, _ := json.Marshal(a.CoreCompetencies)
				stratJSON, _ := json.Marshal(a.StrategicImpact)
				rolesJSON, _ := json.Marshal(a.TargetRoles)
				if _, err := archStmt.ExecContext(ctx, a.ID, a.Name, a.ProfileOverview, string(compJSON), string(stratJSON), string(rolesJSON), nowStr, nowStr); err != nil {
					return fmt.Errorf("exec save archetype %s: %w", a.ID, err)
				}
			}
		}

		// F. Applications & Citations
		if hasApplicationsSection {
			incomingAppIDs := make(map[string]bool)
			for _, app := range appsLoaded {
				incomingAppIDs[app.ID] = true
			}

			rows, err := tx.QueryContext(ctx, `SELECT id FROM applications;`)
			if err != nil {
				return err
			}
			var existingAppIDs []string
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err == nil {
					existingAppIDs = append(existingAppIDs, id)
				}
			}
			rows.Close()

			for _, id := range existingAppIDs {
				if !incomingAppIDs[id] {
					if _, err := tx.ExecContext(ctx, `DELETE FROM applications WHERE id = ?;`, id); err != nil {
						return fmt.Errorf("delete application %s: %w", id, err)
					}
				}
			}

			appStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO applications (id, role_title, company, raw_jd, state, rejection_rationale, archetype_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				role_title = excluded.role_title,
				company = excluded.company,
				raw_jd = excluded.raw_jd,
				state = excluded.state,
				rejection_rationale = excluded.rejection_rationale,
				archetype_id = excluded.archetype_id,
				updated_at = excluded.updated_at;`)
			if err != nil {
				return err
			}
			defer appStmt.Close()

			citStmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO application_citations (application_id, fact_id) VALUES (?, ?);`)
			if err != nil {
				return err
			}
			defer citStmt.Close()

			for _, app := range appsLoaded {
				st := app.State
				if st == "" {
					st = string(domain.StateTriaged)
				}
				var archID sql.NullString
				if app.ArchetypeID != "" {
					archID = sql.NullString{String: app.ArchetypeID, Valid: true}
				}

				if _, err := appStmt.ExecContext(ctx, app.ID, app.RoleTitle, app.Company, app.RawJD, st, app.RejectionRationale, archID, nowStr, nowStr); err != nil {
					return fmt.Errorf("exec save application %s: %w", app.ID, err)
				}

				// Citations
				if _, err := tx.ExecContext(ctx, `DELETE FROM application_citations WHERE application_id = ?;`, app.ID); err != nil {
					return fmt.Errorf("delete old citations for app %s: %w", app.ID, err)
				}
				for _, cid := range app.Citations {
					if _, err := citStmt.ExecContext(ctx, app.ID, cid); err != nil {
						return fmt.Errorf("insert citation (%s, %s): %w", app.ID, cid, err)
					}
				}
			}
		}

		return nil
	})
}

// Helpers for YAML filesystem operations

func writeYAML(path string, val any) error {
	data, err := yaml.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func readYAML(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	return yaml.Unmarshal(data, target)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func listYAMLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files, nil
}

func fileStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' || r == '/' || r == '.' || r == ':' || r == '@' {
			b.WriteRune('-')
		}
	}
	res := b.String()
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	res = strings.Trim(res, "-")
	if res == "" {
		return "item"
	}
	return res
}
