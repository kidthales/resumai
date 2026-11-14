package vault

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/kidthales/resumai/internal/domain"
	"gopkg.in/yaml.v3"
)

// LegacyImportOptions configures the behavior of legacy v0.1.0 Markdown ingestion.
type LegacyImportOptions struct {
	DefaultEpistemicState domain.EpistemicState
	SkipPII               bool
}

// LegacyImportReport summarizes entities ingested from legacy files.
type LegacyImportReport struct {
	PIIImported       bool `json:"pii_imported"`
	LogisticsImported bool `json:"logistics_imported"`
	Organizations     int  `json:"organizations"`
	Roles             int  `json:"roles"`
	Facts             int  `json:"facts"`
	Education         int  `json:"education"`
	Archetypes        int  `json:"archetypes"`
}

// ParsedLegacyBundle holds all parsed domain entities prior to transactional database ingestion.
type ParsedLegacyBundle struct {
	PII           *domain.PIIHeader
	Logistics     *domain.CandidateLogistics
	Organizations []*domain.Organization
	Roles         []*domain.Role
	Facts         []*domain.CareerFact
	Education     []*domain.Education
	Archetypes    []*domain.Archetype
}

// ImportLegacy ingests v0.1.0 legacy Markdown files (personal_info.md, education_history.md,
// work_history.md, extras.md, and archetypes/*.md) into the SQLite CareerVault within a single transaction.
func (v *CareerVault) ImportLegacy(ctx context.Context, dir string, opts ...LegacyImportOptions) (*LegacyImportReport, error) {
	var opt LegacyImportOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.DefaultEpistemicState == "" {
		opt.DefaultEpistemicState = domain.EpistemicVerified
	}
	if err := opt.DefaultEpistemicState.Validate(); err != nil {
		return nil, fmt.Errorf("invalid default epistemic state: %w", err)
	}

	bundle, err := ParseLegacyDirectory(dir, opt)
	if err != nil {
		return nil, err
	}

	report := &LegacyImportReport{
		Organizations: len(bundle.Organizations),
		Roles:         len(bundle.Roles),
		Facts:         len(bundle.Facts),
		Education:     len(bundle.Education),
		Archetypes:    len(bundle.Archetypes),
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	err = v.WithTx(ctx, func(tx *sql.Tx) error {
		// 1. PII Header
		if bundle.PII != nil && !opt.SkipPII {
			piiStmt, err := tx.PrepareContext(ctx, `
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
				updated_at = excluded.updated_at;`)
			if err != nil {
				return fmt.Errorf("prepare pii stmt: %w", err)
			}
			defer piiStmt.Close()

			if _, err := piiStmt.ExecContext(ctx,
				bundle.PII.FullName,
				bundle.PII.Email,
				bundle.PII.Phone,
				bundle.PII.Location,
				bundle.PII.LinkedInURL,
				bundle.PII.GitHubURL,
				bundle.PII.PortfolioURL,
				nowStr,
			); err != nil {
				return fmt.Errorf("save pii: %w", err)
			}
			report.PIIImported = true
		}

		// 2. Candidate Logistics
		if bundle.Logistics != nil {
			logisticsStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO candidate_logistics (id, target_title, work_authorization, remote_preference,
			                                willing_to_relocate, target_compensation, notice_period_days, updated_at)
			VALUES (1, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				target_title = excluded.target_title,
				work_authorization = excluded.work_authorization,
				remote_preference = excluded.remote_preference,
				willing_to_relocate = excluded.willing_to_relocate,
				target_compensation = excluded.target_compensation,
				notice_period_days = excluded.notice_period_days,
				updated_at = excluded.updated_at;`)
			if err != nil {
				return fmt.Errorf("prepare logistics stmt: %w", err)
			}
			defer logisticsStmt.Close()

			willingRelocateInt := 0
			if bundle.Logistics.WillingToRelocate {
				willingRelocateInt = 1
			}

			if _, err := logisticsStmt.ExecContext(ctx,
				bundle.Logistics.TargetTitle,
				bundle.Logistics.WorkAuthorization,
				bundle.Logistics.RemotePreference,
				willingRelocateInt,
				bundle.Logistics.TargetCompensation,
				bundle.Logistics.NoticePeriodDays,
				nowStr,
			); err != nil {
				return fmt.Errorf("save logistics: %w", err)
			}
			report.LogisticsImported = true
		}

		// 3. Organizations
		orgStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO organizations (id, name, location, website, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			location = CASE WHEN excluded.location != '' THEN excluded.location ELSE organizations.location END,
			website = CASE WHEN excluded.website != '' THEN excluded.website ELSE organizations.website END,
			updated_at = excluded.updated_at;`)
		if err != nil {
			return fmt.Errorf("prepare org stmt: %w", err)
		}
		defer orgStmt.Close()

		for _, org := range bundle.Organizations {
			if _, err := orgStmt.ExecContext(ctx, org.ID, org.Name, org.Location, org.Website, nowStr, nowStr); err != nil {
				return fmt.Errorf("save organization %s: %w", org.ID, err)
			}
		}

		// 4. Roles
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

		for _, r := range bundle.Roles {
			cat := r.Category
			if cat == "" {
				cat = "work"
			}
			techJSON, err := marshalStringSlice(r.TechStack)
			if err != nil {
				return fmt.Errorf("marshal role %s tech stack: %w", r.ID, err)
			}
			if _, err := roleStmt.ExecContext(ctx, r.ID, r.OrgID, r.Title, cat, r.Location, r.Duration, techJSON, nowStr, nowStr); err != nil {
				return fmt.Errorf("save role %s: %w", r.ID, err)
			}
		}

		// 5. Career Facts
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

		for _, f := range bundle.Facts {
			verifiedInt := 0
			if f.Verified {
				verifiedInt = 1
			}
			techJSON, err := marshalStringSlice(f.TechTags)
			if err != nil {
				return fmt.Errorf("marshal fact %s tech tags: %w", f.ID, err)
			}
			if _, err := factStmt.ExecContext(ctx,
				f.ID,
				f.RoleID,
				f.Headline,
				f.Details,
				string(f.EpistemicState),
				techJSON,
				verifiedInt,
				nowStr,
				nowStr,
			); err != nil {
				return fmt.Errorf("save career fact %s: %w", f.ID, err)
			}
		}

		// 6. Education
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
			return fmt.Errorf("prepare education stmt: %w", err)
		}
		defer eduStmt.Close()

		for _, edu := range bundle.Education {
			if _, err := eduStmt.ExecContext(ctx,
				edu.ID,
				edu.Institution,
				edu.Degree,
				edu.FieldOfStudy,
				edu.Location,
				edu.Duration,
				edu.Honors,
				nowStr,
				nowStr,
			); err != nil {
				return fmt.Errorf("save education %s: %w", edu.ID, err)
			}
		}

		// 7. Archetypes
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
			return fmt.Errorf("prepare archetype stmt: %w", err)
		}
		defer archStmt.Close()

		for _, a := range bundle.Archetypes {
			compJSON, err := marshalStringSlice(a.CoreCompetencies)
			if err != nil {
				return fmt.Errorf("marshal archetype %s core competencies: %w", a.ID, err)
			}
			stratJSON, err := marshalStringSlice(a.StrategicImpact)
			if err != nil {
				return fmt.Errorf("marshal archetype %s strategic impact: %w", a.ID, err)
			}
			rolesJSON, err := marshalStringSlice(a.TargetRoles)
			if err != nil {
				return fmt.Errorf("marshal archetype %s target roles: %w", a.ID, err)
			}
			if _, err := archStmt.ExecContext(ctx,
				a.ID,
				a.Name,
				a.ProfileOverview,
				compJSON,
				stratJSON,
				rolesJSON,
				nowStr,
				nowStr,
			); err != nil {
				return fmt.Errorf("save archetype %s: %w", a.ID, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("import legacy transaction: %w", err)
	}

	return report, nil
}

// ParseLegacyDirectory scans a directory and parses any present legacy Markdown files into domain entities.
func ParseLegacyDirectory(dir string, opts LegacyImportOptions) (*ParsedLegacyBundle, error) {
	cleanDir := filepath.Clean(dir)
	info, err := os.Stat(cleanDir)
	if err != nil {
		return nil, fmt.Errorf("read legacy directory %s: %w", cleanDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %s is not a directory", cleanDir)
	}

	bundle := &ParsedLegacyBundle{}
	filesFound := 0

	// 1. Personal Info
	piiFile := findLegacyFile(cleanDir, "personal_info.md")
	if piiFile != "" {
		filesFound++
		pii, logistics, err := parsePersonalInfoFile(piiFile)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", piiFile, err)
		}
		bundle.PII = pii
		bundle.Logistics = logistics
	}

	// 2. Education History
	eduFile := findLegacyFile(cleanDir, "education_history.md")
	if eduFile != "" {
		filesFound++
		edus, err := parseEducationHistoryFile(eduFile)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", eduFile, err)
		}
		bundle.Education = edus
	}

	// Map to de-duplicate organizations across work_history.md and extras.md
	orgMap := make(map[string]*domain.Organization)

	// 3. Work History
	workFile := findLegacyFile(cleanDir, "work_history.md")
	if workFile != "" {
		filesFound++
		orgs, roles, facts, err := parseOrgRolesAndFactsFile(workFile, "work", opts.DefaultEpistemicState, orgMap)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", workFile, err)
		}
		bundle.Organizations = append(bundle.Organizations, orgs...)
		bundle.Roles = append(bundle.Roles, roles...)
		bundle.Facts = append(bundle.Facts, facts...)
	}

	// 4. Extras History
	extrasFile := findLegacyFile(cleanDir, "extras.md")
	if extrasFile != "" {
		filesFound++
		orgs, roles, facts, err := parseOrgRolesAndFactsFile(extrasFile, "extra", opts.DefaultEpistemicState, orgMap)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", extrasFile, err)
		}
		bundle.Organizations = append(bundle.Organizations, orgs...)
		bundle.Roles = append(bundle.Roles, roles...)
		bundle.Facts = append(bundle.Facts, facts...)
	}

	// 5. Archetypes
	archFiles := findLegacyArchetypeFiles(cleanDir)
	if len(archFiles) > 0 {
		filesFound += len(archFiles)
		for _, af := range archFiles {
			arch, err := parseArchetypeFile(af)
			if err != nil {
				return nil, fmt.Errorf("parse archetype %s: %w", af, err)
			}
			bundle.Archetypes = append(bundle.Archetypes, arch)
		}
	}

	if filesFound == 0 {
		return nil, fmt.Errorf("no legacy files found in %s (expected personal_info.md, education_history.md, work_history.md, extras.md, or archetypes/*.md)", cleanDir)
	}

	return bundle, nil
}

// findLegacyFile searches for a file in dir, dir/pii, dir/content/pii, and checks for .sample.md fallbacks.
func findLegacyFile(dir, filename string) string {
	candidates := []string{
		filepath.Join(dir, filename),
		filepath.Join(dir, "pii", filename),
		filepath.Join(dir, "content", "pii", filename),
	}

	sampleFilename := strings.TrimSuffix(filename, ".md") + ".sample.md"
	sampleCandidates := []string{
		filepath.Join(dir, sampleFilename),
		filepath.Join(dir, "pii", sampleFilename),
		filepath.Join(dir, "content", "pii", sampleFilename),
	}

	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	for _, c := range sampleCandidates {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

// findLegacyArchetypeFiles finds all archetype .md files in dir/archetypes, dir/content/archetypes, or dir itself.
func findLegacyArchetypeFiles(dir string) []string {
	dirs := []string{
		filepath.Join(dir, "archetypes"),
		filepath.Join(dir, "content", "archetypes"),
	}

	var results []string
	seenBases := make(map[string]bool)

	for _, d := range dirs {
		if dirExists(d) {
			entries, err := os.ReadDir(d)
			if err != nil {
				continue
			}
			// Prefer non-sample .md files if both exist
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".sample.md") {
					baseKey := strings.TrimSuffix(name, ".md")
					seenBases[baseKey] = true
					results = append(results, filepath.Join(d, name))
				}
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.HasSuffix(name, ".sample.md") {
					baseKey := strings.TrimSuffix(name, ".sample.md")
					if !seenBases[baseKey] {
						seenBases[baseKey] = true
						results = append(results, filepath.Join(d, name))
					}
				}
			}
		}
	}

	// If no archetypes directory was found, check if dir itself contains archetype files
	if len(results) == 0 {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.HasSuffix(name, ".md") && !isStandardLegacyFileName(name) {
					baseKey := strings.TrimSuffix(strings.TrimSuffix(name, ".sample.md"), ".md")
					if !seenBases[baseKey] {
						seenBases[baseKey] = true
						results = append(results, filepath.Join(dir, name))
					}
				}
			}
		}
	}

	return results
}

func isStandardLegacyFileName(name string) bool {
	base := strings.ToLower(name)
	return strings.HasPrefix(base, "personal_info") ||
		strings.HasPrefix(base, "education_history") ||
		strings.HasPrefix(base, "work_history") ||
		strings.HasPrefix(base, "extras") ||
		strings.HasPrefix(base, "readme")
}

// --- Parsers ---

// parsePersonalInfoFile parses personal_info.md into PIIHeader and CandidateLogistics.
func parsePersonalInfoFile(path string) (*domain.PIIHeader, *domain.CandidateLogistics, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	content := string(data)
	pii := &domain.PIIHeader{}
	logistics := &domain.CandidateLogistics{}
	hasPII := false
	hasLogistics := false

	// 1. Check for YAML frontmatter
	body := content
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		var fm map[string]any
		remaining, err := extractFrontmatter(content, &fm)
		if err == nil {
			body = remaining
			for k, v := range fm {
				valStr := fmt.Sprintf("%v", v)
				if applyPIIOrLogisticsKey(k, valStr, pii, logistics, &hasPII, &hasLogistics) {
					continue
				}
			}
		}
	}

	// 2. Parse Markdown lines
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		clean := stripBulletPrefix(line)
		colonIdx := strings.Index(clean, ":")
		if colonIdx <= 0 {
			continue
		}

		key := cleanMarkdownFormatting(clean[:colonIdx])
		val := cleanMarkdownFormatting(clean[colonIdx+1:])

		applyPIIOrLogisticsKey(key, val, pii, logistics, &hasPII, &hasLogistics)
	}

	var retPII *domain.PIIHeader
	if hasPII {
		retPII = pii
	}
	var retLogistics *domain.CandidateLogistics
	if hasLogistics {
		retLogistics = logistics
	}

	return retPII, retLogistics, nil
}

func applyPIIOrLogisticsKey(key, val string, pii *domain.PIIHeader, cl *domain.CandidateLogistics, hasPII, hasLogistics *bool) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	v := strings.TrimSpace(val)
	if v == "" {
		return false
	}

	switch k {
	case "name", "full name", "full_name", "candidate name":
		pii.FullName = v
		*hasPII = true
		return true
	case "email", "email address", "e-mail", "email_address":
		pii.Email = v
		*hasPII = true
		return true
	case "phone", "phone number", "telephone", "mobile", "phone_number":
		pii.Phone = v
		*hasPII = true
		return true
	case "location", "address", "city", "residence":
		pii.Location = v
		*hasPII = true
		return true
	case "linkedin", "linkedin url", "linkedin_url", "linkedin profile":
		pii.LinkedInURL = v
		*hasPII = true
		return true
	case "github", "github url", "github_url", "github profile":
		pii.GitHubURL = v
		*hasPII = true
		return true
	case "portfolio", "portfolio url", "portfolio_url", "website", "personal website", "site":
		pii.PortfolioURL = v
		*hasPII = true
		return true

	// Logistics
	case "target title", "target_title", "desired title", "preferred title":
		cl.TargetTitle = v
		*hasLogistics = true
		return true
	case "work authorization", "work_authorization", "visa", "citizenship":
		cl.WorkAuthorization = v
		*hasLogistics = true
		return true
	case "remote preference", "remote_preference", "remote":
		cl.RemotePreference = v
		*hasLogistics = true
		return true
	case "willing to relocate", "willing_to_relocate", "relocation":
		cl.WillingToRelocate = parseBool(v)
		*hasLogistics = true
		return true
	case "target compensation", "target_compensation", "compensation", "salary", "desired salary":
		cl.TargetCompensation = v
		*hasLogistics = true
		return true
	case "notice period", "notice period days", "notice_period_days", "notice period (days)", "notice":
		cl.NoticePeriodDays = parseDays(v)
		*hasLogistics = true
		return true
	}

	return false
}

// parseEducationHistoryFile parses education_history.md into a slice of Education entities.
func parseEducationHistoryFile(path string) ([]*domain.Education, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var edus []*domain.Education
	var curEdu *domain.Education
	var curTitle string

	flush := func() {
		if curEdu != nil {
			if curEdu.Institution == "" && curTitle != "" {
				curEdu.Institution = curTitle
			}
			if curEdu.Degree == "" && curTitle != "" && curEdu.Institution != curTitle {
				curEdu.Degree = curTitle
			}
			if curEdu.ID == "" {
				curEdu.ID = slugify("edu-" + curEdu.Institution + "-" + curEdu.Degree)
			}
			edus = append(edus, curEdu)
			curEdu = nil
			curTitle = ""
		}
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		indent := countLeadingSpaces(rawLine)
		isHeader := strings.HasPrefix(trimmed, "#")
		isBullet := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")

		// Top-level item starts at indent == 0 or via markdown header
		if (indent == 0 && isBullet) || isHeader {
			flush()
			curEdu = &domain.Education{}
			titleLine := cleanMarkdownFormatting(stripBulletPrefix(stripHeaderPrefix(trimmed)))
			curTitle = titleLine

			// Check if title has "Degree, Field of Study" or "Degree in Field of Study"
			if strings.Contains(titleLine, ",") {
				parts := strings.SplitN(titleLine, ",", 2)
				curEdu.Degree = strings.TrimSpace(parts[0])
				curEdu.FieldOfStudy = strings.TrimSpace(parts[1])
			} else if strings.Contains(strings.ToLower(titleLine), " in ") {
				idx := strings.Index(strings.ToLower(titleLine), " in ")
				curEdu.Degree = strings.TrimSpace(titleLine[:idx])
				curEdu.FieldOfStudy = strings.TrimSpace(titleLine[idx+4:])
			}
			continue
		}

		if curEdu == nil {
			continue
		}

		// Sub-bullet attribute
		clean := cleanMarkdownFormatting(stripBulletPrefix(stripHeaderPrefix(trimmed)))
		colonIdx := strings.Index(clean, ":")
		if colonIdx <= 0 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(clean[:colonIdx]))
		val := strings.TrimSpace(clean[colonIdx+1:])
		if val == "" {
			continue
		}

		switch key {
		case "institution", "school", "university", "college":
			curEdu.Institution = val
		case "degree", "diploma", "credential":
			curEdu.Degree = val
		case "field of study", "field_of_study", "major", "subject":
			curEdu.FieldOfStudy = val
		case "location", "city":
			curEdu.Location = val
		case "duration", "dates", "years", "period":
			curEdu.Duration = val
		case "honors", "awards", "gpa", "distinction":
			curEdu.Honors = val
		}
	}

	flush()

	// Ensure unique IDs
	idCounts := make(map[string]int)
	for _, edu := range edus {
		idCounts[edu.ID]++
		if idCounts[edu.ID] > 1 {
			edu.ID = fmt.Sprintf("%s-%d", edu.ID, idCounts[edu.ID])
		}
	}

	return edus, nil
}

// parseOrgRolesAndFactsFile parses work_history.md or extras.md into Organizations, Roles, and CareerFacts.
func parseOrgRolesAndFactsFile(
	path string,
	category string,
	defaultState domain.EpistemicState,
	orgMap map[string]*domain.Organization,
) ([]*domain.Organization, []*domain.Role, []*domain.CareerFact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}

	lines := strings.Split(string(data), "\n")
	var newOrgs []*domain.Organization
	var roles []*domain.Role
	var facts []*domain.CareerFact

	var curOrg *domain.Organization
	var curRole *domain.Role
	roleFactCount := 0

	flushRole := func() {
		if curRole != nil {
			roles = append(roles, curRole)
			curRole = nil
			roleFactCount = 0
		}
	}

	flushOrg := func() {
		flushRole()
		curOrg = nil
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		indent := countLeadingSpaces(rawLine)
		isHeader := strings.HasPrefix(trimmed, "#")
		isBullet := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ")

		// Top-level line: Organization (0 indent bullet, or # / ## header)
		if (indent == 0 && isBullet) || (isHeader && (strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## "))) {
			flushOrg()
			orgName := cleanMarkdownFormatting(stripBulletPrefix(stripHeaderPrefix(trimmed)))
			orgID := slugify("org-" + orgName)

			if existing, found := orgMap[orgID]; found {
				curOrg = existing
			} else {
				curOrg = &domain.Organization{
					ID:   orgID,
					Name: orgName,
				}
				orgMap[orgID] = curOrg
				newOrgs = append(newOrgs, curOrg)
			}
			continue
		}

		if curOrg == nil {
			continue
		}

		cleanText := cleanMarkdownFormatting(stripBulletPrefix(stripHeaderPrefix(trimmed)))
		colonIdx := strings.Index(cleanText, ":")

		// Check for Role start
		if isRoleStart(cleanText, colonIdx, indent) {
			flushRole()
			roleTitle := extractRoleTitle(cleanText, colonIdx)
			roleID := slugify("role-" + strings.TrimPrefix(curOrg.ID, "org-") + "-" + roleTitle)

			curRole = &domain.Role{
				ID:       roleID,
				OrgID:    curOrg.ID,
				Title:    roleTitle,
				Category: category,
			}
			continue
		}

		// Check for Tech Stack
		if isTechStackLine(cleanText, colonIdx) {
			if curRole != nil {
				rawStack := strings.TrimSpace(cleanText[colonIdx+1:])
				curRole.TechStack = splitTechStack(rawStack)
			}
			continue
		}

		// Check for Location or Duration
		if colonIdx > 0 {
			key := strings.ToLower(strings.TrimSpace(cleanText[:colonIdx]))
			val := strings.TrimSpace(cleanText[colonIdx+1:])

			if key == "location" || key == "city" {
				if curRole != nil && curRole.Location == "" {
					curRole.Location = val
				}
				if curOrg.Location == "" {
					curOrg.Location = val
				}
				continue
			}
			if key == "duration" || key == "dates" || key == "period" {
				if curRole != nil {
					curRole.Duration = val
				}
				continue
			}
			if key == "website" || key == "url" {
				curOrg.Website = val
				continue
			}
		}

		// If we are inside a role and this is a deeper indented bullet (or sub-bullet under role),
		// it is a career fact!
		if curRole != nil && (indent >= 4 || isBullet) {
			factText := stripBulletPrefix(trimmed)
			if factText == "" {
				continue
			}

			roleFactCount++
			fact := parseCareerFactLine(factText, curRole, roleFactCount, defaultState)
			facts = append(facts, fact)
		}
	}

	flushOrg()

	return newOrgs, roles, facts, nil
}

func isRoleStart(cleanText string, colonIdx, indent int) bool {
	lower := strings.ToLower(cleanText)
	if strings.HasPrefix(lower, "role:") || strings.HasPrefix(lower, "title:") || strings.HasPrefix(lower, "position:") {
		return true
	}
	if strings.HasPrefix(cleanText, "### Role:") || strings.HasPrefix(cleanText, "### Title:") {
		return true
	}
	// Heading 3 in markdown
	if strings.HasPrefix(cleanText, "### ") {
		return true
	}
	return false
}

func extractRoleTitle(cleanText string, colonIdx int) string {
	if colonIdx > 0 {
		lower := strings.ToLower(cleanText[:colonIdx])
		if strings.Contains(lower, "role") || strings.Contains(lower, "title") || strings.Contains(lower, "position") {
			return strings.TrimSpace(cleanText[colonIdx+1:])
		}
	}
	if strings.HasPrefix(cleanText, "### ") {
		return strings.TrimSpace(strings.TrimPrefix(cleanText, "### "))
	}
	return strings.TrimSpace(cleanText)
}

func isTechStackLine(cleanText string, colonIdx int) bool {
	if colonIdx <= 0 {
		return false
	}
	key := strings.ToLower(strings.TrimSpace(cleanText[:colonIdx]))
	return key == "technology stack" ||
		key == "tech stack" ||
		key == "technologies" ||
		key == "tools" ||
		key == "technology"
}

func splitTechStack(raw string) []string {
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		cleaned := cleanMarkdownFormatting(strings.TrimSpace(p))
		cleaned = strings.Trim(cleaned, ".")
		if cleaned != "" {
			result = append(result, cleaned)
		}
	}
	return result
}

func parseCareerFactLine(rawText string, role *domain.Role, index int, defaultState domain.EpistemicState) *domain.CareerFact {
	text := strings.TrimSpace(rawText)

	// Check explicit epistemic state tag: [verified], [raw_note], [metric_pending], [archived]
	eState := defaultState
	if strings.HasPrefix(text, "[verified]") {
		eState = domain.EpistemicVerified
		text = strings.TrimSpace(strings.TrimPrefix(text, "[verified]"))
	} else if strings.HasPrefix(text, "[raw_note]") {
		eState = domain.EpistemicRawNote
		text = strings.TrimSpace(strings.TrimPrefix(text, "[raw_note]"))
	} else if strings.HasPrefix(text, "[metric_pending]") {
		eState = domain.EpistemicMetricPending
		text = strings.TrimSpace(strings.TrimPrefix(text, "[metric_pending]"))
	} else if strings.HasPrefix(text, "[archived]") {
		eState = domain.EpistemicArchived
		text = strings.TrimSpace(strings.TrimPrefix(text, "[archived]"))
	}

	factID := fmt.Sprintf("fact-%s-%d", strings.TrimPrefix(role.ID, "role-"), index)

	var headline, details string

	// Check bold title prefix: **Title:** details or **Title** - details
	boldColonRe := regexp.MustCompile(`^\*\*([^*]+)\*\*:\s*(.*)$`)
	boldDashRe := regexp.MustCompile(`^\*\*([^*]+)\*\*\s*[-–—]\s*(.*)$`)

	if m := boldColonRe.FindStringSubmatch(text); len(m) == 3 {
		headline = strings.TrimSpace(m[1])
		details = strings.TrimSpace(m[2])
	} else if m := boldDashRe.FindStringSubmatch(text); len(m) == 3 {
		headline = strings.TrimSpace(m[1])
		details = strings.TrimSpace(m[2])
	} else {
		// Clean text without markdown bolding for full sentence
		details = cleanMarkdownFormatting(text)
		headline = extractHeadline(details)
	}

	if details == "" {
		details = headline
	}

	// Match tech tags from role's tech stack that appear in the details text
	var matchedTags []string
	lowerDetails := strings.ToLower(details)
	for _, tech := range role.TechStack {
		if strings.Contains(lowerDetails, strings.ToLower(tech)) {
			matchedTags = append(matchedTags, tech)
		}
	}

	return &domain.CareerFact{
		ID:             factID,
		RoleID:         role.ID,
		Headline:       headline,
		Details:        details,
		EpistemicState: eState,
		TechTags:       matchedTags,
		Verified:       eState == domain.EpistemicVerified,
	}
}

func extractHeadline(text string) string {
	// Extract the first sentence or up to 90 characters
	sentenceEnd := strings.Index(text, ". ")
	if sentenceEnd > 0 && sentenceEnd <= 90 {
		return strings.TrimSpace(text[:sentenceEnd])
	}
	if len(text) <= 90 {
		return strings.TrimSuffix(strings.TrimSpace(text), ".")
	}

	// Cut at last space before 90 chars
	cut := text[:90]
	if lastSpace := strings.LastIndex(cut, " "); lastSpace > 40 {
		cut = cut[:lastSpace]
	}
	return strings.TrimSpace(cut) + "..."
}

// parseArchetypeFile parses a single archetype .md file (handling both YAML frontmatter and Markdown sections).
func parseArchetypeFile(path string) (*domain.Archetype, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	arch := &domain.Archetype{}

	// Default ID from filename
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, ".md")
	stem = strings.TrimSuffix(stem, ".sample")
	arch.ID = stem

	body := content

	// 1. Parse YAML Frontmatter if present
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		type archFrontmatter struct {
			ID               string   `yaml:"id"`
			Name             string   `yaml:"name"`
			ProfileOverview  string   `yaml:"profile_overview"`
			CoreCompetencies []string `yaml:"core_competencies"`
			StrategicImpact  []string `yaml:"strategic_impact"`
			TargetRoles      []string `yaml:"target_roles"`
		}
		var fm archFrontmatter
		remaining, err := extractFrontmatter(content, &fm)
		if err == nil {
			body = remaining
			if fm.ID != "" {
				arch.ID = fm.ID
			}
			if fm.Name != "" {
				arch.Name = fm.Name
			}
			if fm.ProfileOverview != "" {
				arch.ProfileOverview = fm.ProfileOverview
			}
			if len(fm.CoreCompetencies) > 0 {
				arch.CoreCompetencies = fm.CoreCompetencies
			}
			if len(fm.StrategicImpact) > 0 {
				arch.StrategicImpact = fm.StrategicImpact
			}
			if len(fm.TargetRoles) > 0 {
				arch.TargetRoles = fm.TargetRoles
			}
		}
	}

	// 2. Parse Markdown Sections
	lines := strings.Split(body, "\n")
	var currentSection string
	var sectionParagraph strings.Builder

	flushSection := func() {
		switch currentSection {
		case "overview":
			if arch.ProfileOverview == "" {
				arch.ProfileOverview = strings.TrimSpace(sectionParagraph.String())
			}
		}
		sectionParagraph.Reset()
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "# ") {
			flushSection()
			if arch.Name == "" {
				title := cleanMarkdownFormatting(strings.TrimPrefix(trimmed, "# "))
				title = strings.TrimSuffix(title, " Sample")
				title = strings.TrimSuffix(title, " Archetype")
				arch.Name = title
			}
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			flushSection()
			header := strings.ToLower(cleanMarkdownFormatting(strings.TrimPrefix(trimmed, "## ")))
			if strings.Contains(header, "overview") || strings.Contains(header, "summary") {
				currentSection = "overview"
			} else if strings.Contains(header, "competencies") || strings.Contains(header, "skills") {
				currentSection = "competencies"
			} else if strings.Contains(header, "impact") {
				currentSection = "impact"
			} else if strings.Contains(header, "roles") || strings.Contains(header, "alignment") {
				currentSection = "roles"
			} else {
				currentSection = "other"
			}
			continue
		}

		isBullet := strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ")
		if isBullet {
			bulletText := cleanMarkdownFormatting(stripBulletPrefix(trimmed))
			if bulletText == "" {
				continue
			}

			switch currentSection {
			case "competencies":
				arch.CoreCompetencies = append(arch.CoreCompetencies, bulletText)
			case "impact":
				arch.StrategicImpact = append(arch.StrategicImpact, bulletText)
			case "roles":
				arch.TargetRoles = append(arch.TargetRoles, bulletText)
			case "overview":
				sectionParagraph.WriteString(bulletText + " ")
			}
		} else {
			if currentSection == "overview" {
				if sectionParagraph.Len() > 0 {
					sectionParagraph.WriteString(" ")
				}
				sectionParagraph.WriteString(trimmed)
			}
		}
	}

	flushSection()

	if arch.Name == "" {
		arch.Name = strings.ReplaceAll(arch.ID, "_", " ")
		arch.Name = titleCase(strings.ReplaceAll(arch.Name, "-", " "))
	}

	return arch, nil
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// --- Text Utility Helpers ---

func extractFrontmatter(content string, target any) (string, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return content, errors.New("no frontmatter")
	}

	rest := strings.TrimPrefix(trimmed, "---")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return content, errors.New("unclosed frontmatter")
	}

	fmBytes := []byte(rest[:idx])
	if err := yaml.Unmarshal(fmBytes, target); err != nil {
		return content, err
	}

	remaining := rest[idx+4:]
	return remaining, nil
}

func countLeadingSpaces(s string) int {
	spaces := 0
	for _, r := range s {
		if r == ' ' {
			spaces++
		} else if r == '\t' {
			spaces += 4
		} else {
			break
		}
	}
	return spaces
}

func stripBulletPrefix(s string) string {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") || strings.HasPrefix(t, "+ ") {
		return strings.TrimSpace(t[2:])
	}
	numRe := regexp.MustCompile(`^\d+\.\s*(.*)$`)
	if m := numRe.FindStringSubmatch(t); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return t
}

func stripHeaderPrefix(s string) string {
	t := strings.TrimSpace(s)
	for strings.HasPrefix(t, "#") {
		t = strings.TrimPrefix(t, "#")
	}
	return strings.TrimSpace(t)
}

func cleanMarkdownFormatting(s string) string {
	s = strings.TrimSpace(s)
	// Strip bold markers: **text** or __text__
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	// Strip inline backticks: `text`
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func parseBool(s string) bool {
	v := strings.ToLower(strings.TrimSpace(s))
	return v == "true" || v == "yes" || v == "1" || v == "y"
}

func parseDays(s string) int {
	s = strings.TrimSpace(s)
	// Extract the first number
	re := regexp.MustCompile(`\d+`)
	m := re.FindString(s)
	if m != "" {
		if n, err := strconv.Atoi(m); err == nil {
			return n
		}
	}
	return 0
}
