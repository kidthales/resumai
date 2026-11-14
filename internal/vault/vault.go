package vault

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	_ "modernc.org/sqlite"
)

// Vault defines the air-gapped contract for persistent career facts, vault curation,
// and job applications. Contact PII methods (GetPII, SetPII) are strictly excluded.
type Vault interface {
	domain.DraftResolver
	domain.FactLookup

	GetCareerFact(id string) (*domain.CareerFact, error)
	GetApplication(id string) (*domain.JobApplication, error)

	HasPII(ctx context.Context) (bool, error)

	GetCandidateLogistics(ctx context.Context) (*domain.CandidateLogistics, error)
	SetCandidateLogistics(ctx context.Context, cl *domain.CandidateLogistics) error

	SaveOrganization(ctx context.Context, org *domain.Organization) error
	GetOrganization(ctx context.Context, id string) (*domain.Organization, error)
	ListOrganizations(ctx context.Context) ([]*domain.Organization, error)

	SaveRole(ctx context.Context, role *domain.Role) error
	GetRole(ctx context.Context, id string) (*domain.Role, error)
	ListRoles(ctx context.Context) ([]*domain.Role, error)
	ListRolesByOrg(ctx context.Context, orgID string) ([]*domain.Role, error)

	SaveCareerFact(ctx context.Context, fact *domain.CareerFact) error
	GetCareerFactContext(ctx context.Context, id string) (*domain.CareerFact, error)
	ListCareerFacts(ctx context.Context) ([]*domain.CareerFact, error)
	ListCareerFactsByRole(ctx context.Context, roleID string) ([]*domain.CareerFact, error)
	ArchiveCareerFact(ctx context.Context, id string) error

	SaveEducation(ctx context.Context, edu *domain.Education) error
	GetEducation(ctx context.Context, id string) (*domain.Education, error)
	ListEducation(ctx context.Context) ([]*domain.Education, error)

	SaveArchetype(ctx context.Context, arch *domain.Archetype) error
	GetArchetype(ctx context.Context, id string) (*domain.Archetype, error)
	ListArchetypes(ctx context.Context) ([]*domain.Archetype, error)

	SaveApplication(ctx context.Context, app *domain.JobApplication) error
	GetApplicationContext(ctx context.Context, id string) (*domain.JobApplication, error)
	ListApplications(ctx context.Context) ([]*domain.JobApplication, error)

	RecordApplicationCitations(ctx context.Context, appID string, factIDs []string) error
	GetCitationsForApp(ctx context.Context, appID string) ([]string, error)
}

// CareerVault provides embedded SQLite persistence for career data and job applications.
type CareerVault struct {
	db *sql.DB
}

// Ensure CareerVault implements Vault interface.
var _ Vault = (*CareerVault)(nil)

// Open creates a new CareerVault connection from a DSN.
func Open(dsn string) (*CareerVault, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enforce single connection for embedded SQLite to prevent table lock contention
	// and ensure per-connection pragmas apply to all operations.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Pragmas for performance and correctness
	pragmas := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("exec %q: %w", p, err)
		}
	}

	v := &CareerVault{db: db}
	return v, nil
}

// DefaultPath returns the default path to the vault database file (~/.resumai/vault.db),
// or the value of RESUMAI_VAULT_PATH if set.
func DefaultPath() (string, error) {
	if p := os.Getenv("RESUMAI_VAULT_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".resumai", "vault.db"), nil
}

// OpenPath opens or creates a CareerVault database file at the specified filesystem path,
// ensuring parent directories exist and running all pending migrations.
func OpenPath(path string) (*CareerVault, error) {
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create vault dir: %w", err)
		}
	}

	v, err := Open(path)
	if err != nil {
		return nil, err
	}

	if err := v.Migrate(context.Background()); err != nil {
		_ = v.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return v, nil
}

// Close closes the underlying SQLite database connection.
func (v *CareerVault) Close() error {
	if v.db == nil {
		return nil
	}
	return v.db.Close()
}

// DB returns the underlying sql.DB instance.
func (v *CareerVault) DB() *sql.DB {
	return v.db
}

// Ping verifies the database connection is alive.
func (v *CareerVault) Ping(ctx context.Context) error {
	return v.db.PingContext(ctx)
}

// WithTx executes the provided function within an isolated transaction.
func (v *CareerVault) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// --- PII Header (Air-Gapped) ---

// HasPII reports whether a non-empty candidate PII header has been configured locally
// without returning any PII field values across the air-gap boundary.
func (v *CareerVault) HasPII(ctx context.Context) (bool, error) {
	var count int
	err := v.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM pii_header WHERE id = 1 AND TRIM(full_name) != '';`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check pii configured: %w", err)
	}
	return count > 0, nil
}

// GetPII retrieves the air-gapped candidate PII header. Returns ErrNotFound if not set.
func (v *CareerVault) GetPII(ctx context.Context) (*domain.PIIHeader, error) {
	query := `
	SELECT full_name, email, phone, location, linkedin_url, github_url, portfolio_url, updated_at
	FROM pii_header WHERE id = 1;`

	var pii domain.PIIHeader
	var updatedAtStr string
	err := v.db.QueryRowContext(ctx, query).Scan(
		&pii.FullName,
		&pii.Email,
		&pii.Phone,
		&pii.Location,
		&pii.LinkedInURL,
		&pii.GitHubURL,
		&pii.PortfolioURL,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get pii: %w", err)
	}
	pii.UpdatedAt = parseDBTime(updatedAtStr)
	return &pii, nil
}

// SetPII sets or updates the air-gapped candidate PII header.
func (v *CareerVault) SetPII(ctx context.Context, pii *domain.PIIHeader) error {
	now := time.Now().UTC()
	pii.UpdatedAt = now

	query := `
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

	_, err := v.db.ExecContext(ctx, query,
		pii.FullName,
		pii.Email,
		pii.Phone,
		pii.Location,
		pii.LinkedInURL,
		pii.GitHubURL,
		pii.PortfolioURL,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("set pii: %w", err)
	}
	return nil
}

// --- Candidate Logistics ---

// GetCandidateLogistics retrieves candidate employment logistics and preferences.
func (v *CareerVault) GetCandidateLogistics(ctx context.Context) (*domain.CandidateLogistics, error) {
	query := `
	SELECT target_title, work_authorization, remote_preference, willing_to_relocate,
	       target_compensation, notice_period_days, updated_at
	FROM candidate_logistics WHERE id = 1;`

	var cl domain.CandidateLogistics
	var willingRelocateInt int
	var updatedAtStr string
	err := v.db.QueryRowContext(ctx, query).Scan(
		&cl.TargetTitle,
		&cl.WorkAuthorization,
		&cl.RemotePreference,
		&willingRelocateInt,
		&cl.TargetCompensation,
		&cl.NoticePeriodDays,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get candidate logistics: %w", err)
	}
	cl.WillingToRelocate = willingRelocateInt == 1
	cl.UpdatedAt = parseDBTime(updatedAtStr)
	return &cl, nil
}

// SetCandidateLogistics sets or updates candidate logistics.
func (v *CareerVault) SetCandidateLogistics(ctx context.Context, cl *domain.CandidateLogistics) error {
	now := time.Now().UTC()
	cl.UpdatedAt = now
	willingRelocateInt := 0
	if cl.WillingToRelocate {
		willingRelocateInt = 1
	}

	query := `
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
		updated_at = excluded.updated_at;`

	_, err := v.db.ExecContext(ctx, query,
		cl.TargetTitle,
		cl.WorkAuthorization,
		cl.RemotePreference,
		willingRelocateInt,
		cl.TargetCompensation,
		cl.NoticePeriodDays,
		now.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("set candidate logistics: %w", err)
	}
	return nil
}

// --- Organizations ---

// SaveOrganization creates or updates an organization record.
func (v *CareerVault) SaveOrganization(ctx context.Context, org *domain.Organization) error {
	if org.ID == "" {
		return errors.New("organization ID cannot be empty")
	}
	now := time.Now().UTC()
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now
	}
	org.UpdatedAt = now

	query := `
	INSERT INTO organizations (id, name, location, website, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		location = excluded.location,
		website = excluded.website,
		updated_at = excluded.updated_at;`

	_, err := v.db.ExecContext(ctx, query,
		org.ID,
		org.Name,
		org.Location,
		org.Website,
		org.CreatedAt.Format(time.RFC3339),
		org.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save organization: %w", err)
	}
	return nil
}

// GetOrganization retrieves an organization by ID.
func (v *CareerVault) GetOrganization(ctx context.Context, id string) (*domain.Organization, error) {
	query := `SELECT id, name, location, website, created_at, updated_at FROM organizations WHERE id = ?;`
	var org domain.Organization
	var createdAtStr, updatedAtStr string

	err := v.db.QueryRowContext(ctx, query, id).Scan(
		&org.ID,
		&org.Name,
		&org.Location,
		&org.Website,
		&createdAtStr,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}
	org.CreatedAt = parseDBTime(createdAtStr)
	org.UpdatedAt = parseDBTime(updatedAtStr)
	return &org, nil
}

// ListOrganizations lists all organizations ordered by name.
func (v *CareerVault) ListOrganizations(ctx context.Context) ([]*domain.Organization, error) {
	query := `SELECT id, name, location, website, created_at, updated_at FROM organizations ORDER BY name ASC;`
	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	defer rows.Close()

	var list []*domain.Organization
	for rows.Next() {
		var org domain.Organization
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(&org.ID, &org.Name, &org.Location, &org.Website, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan organization: %w", err)
		}
		org.CreatedAt = parseDBTime(createdAtStr)
		org.UpdatedAt = parseDBTime(updatedAtStr)
		list = append(list, &org)
	}
	return list, rows.Err()
}

// DeleteOrganization deletes an organization and cascading child roles/facts.
// Guard: If any child facts under this organization are cited in a Submitted application,
// returns domain.ErrCannotDeleteSubmittedFact to prevent cascade deletion of provenance.
func (v *CareerVault) DeleteOrganization(ctx context.Context, id string) error {
	var citedCount int
	checkQuery := `
	SELECT COUNT(1) FROM application_citations ac
	JOIN applications a ON a.id = ac.application_id
	JOIN career_facts f ON f.id = ac.fact_id
	JOIN roles r ON r.id = f.role_id
	WHERE r.org_id = ? AND a.state = 'Submitted';`
	if err := v.db.QueryRowContext(ctx, checkQuery, id).Scan(&citedCount); err != nil {
		return fmt.Errorf("check cited org facts: %w", err)
	}
	if citedCount > 0 {
		return domain.ErrCannotDeleteSubmittedFact
	}

	res, err := v.db.ExecContext(ctx, `DELETE FROM organizations WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Roles ---

// SaveRole creates or updates a role record.
func (v *CareerVault) SaveRole(ctx context.Context, role *domain.Role) error {
	if role.ID == "" {
		return errors.New("role ID cannot be empty")
	}
	if role.OrgID == "" {
		return errors.New("role OrgID cannot be empty")
	}
	now := time.Now().UTC()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = now
	}
	role.UpdatedAt = now

	techJSON, err := marshalStringSlice(role.TechStack)
	if err != nil {
		return fmt.Errorf("marshal tech stack: %w", err)
	}

	query := `
	INSERT INTO roles (id, org_id, title, category, location, duration, tech_stack, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		org_id = excluded.org_id,
		title = excluded.title,
		category = excluded.category,
		location = excluded.location,
		duration = excluded.duration,
		tech_stack = excluded.tech_stack,
		updated_at = excluded.updated_at;`

	_, err = v.db.ExecContext(ctx, query,
		role.ID,
		role.OrgID,
		role.Title,
		role.Category,
		role.Location,
		role.Duration,
		techJSON,
		role.CreatedAt.Format(time.RFC3339),
		role.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save role: %w", err)
	}
	return nil
}

// GetRole retrieves a role by ID.
func (v *CareerVault) GetRole(ctx context.Context, id string) (*domain.Role, error) {
	query := `SELECT id, org_id, title, category, location, duration, tech_stack, created_at, updated_at FROM roles WHERE id = ?;`
	var r domain.Role
	var techJSON string
	var createdAtStr, updatedAtStr string

	err := v.db.QueryRowContext(ctx, query, id).Scan(
		&r.ID,
		&r.OrgID,
		&r.Title,
		&r.Category,
		&r.Location,
		&r.Duration,
		&techJSON,
		&createdAtStr,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get role: %w", err)
	}
	_ = json.Unmarshal([]byte(techJSON), &r.TechStack)
	r.CreatedAt = parseDBTime(createdAtStr)
	r.UpdatedAt = parseDBTime(updatedAtStr)
	return &r, nil
}

// ListRoles lists all roles across organizations.
func (v *CareerVault) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	query := `SELECT id, org_id, title, category, location, duration, tech_stack, created_at, updated_at FROM roles ORDER BY created_at DESC;`
	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()

	var list []*domain.Role
	for rows.Next() {
		var r domain.Role
		var techJSON, createdAtStr, updatedAtStr string
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Title, &r.Category, &r.Location, &r.Duration, &techJSON, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		_ = json.Unmarshal([]byte(techJSON), &r.TechStack)
		r.CreatedAt = parseDBTime(createdAtStr)
		r.UpdatedAt = parseDBTime(updatedAtStr)
		list = append(list, &r)
	}
	return list, rows.Err()
}

// ListRolesByOrg lists all roles for a given organization ID.
func (v *CareerVault) ListRolesByOrg(ctx context.Context, orgID string) ([]*domain.Role, error) {
	query := `SELECT id, org_id, title, category, location, duration, tech_stack, created_at, updated_at FROM roles WHERE org_id = ? ORDER BY created_at DESC;`
	rows, err := v.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("list roles by org: %w", err)
	}
	defer rows.Close()

	var list []*domain.Role
	for rows.Next() {
		var r domain.Role
		var techJSON, createdAtStr, updatedAtStr string
		if err := rows.Scan(&r.ID, &r.OrgID, &r.Title, &r.Category, &r.Location, &r.Duration, &techJSON, &createdAtStr, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("scan role: %w", err)
		}
		_ = json.Unmarshal([]byte(techJSON), &r.TechStack)
		r.CreatedAt = parseDBTime(createdAtStr)
		r.UpdatedAt = parseDBTime(updatedAtStr)
		list = append(list, &r)
	}
	return list, rows.Err()
}

// DeleteRole deletes a role and cascading facts.
// Guard: If any child facts under this role are cited in a Submitted application,
// returns domain.ErrCannotDeleteSubmittedFact to prevent cascade deletion of provenance.
func (v *CareerVault) DeleteRole(ctx context.Context, id string) error {
	var citedCount int
	checkQuery := `
	SELECT COUNT(1) FROM application_citations ac
	JOIN applications a ON a.id = ac.application_id
	JOIN career_facts f ON f.id = ac.fact_id
	WHERE f.role_id = ? AND a.state = 'Submitted';`
	if err := v.db.QueryRowContext(ctx, checkQuery, id).Scan(&citedCount); err != nil {
		return fmt.Errorf("check cited role facts: %w", err)
	}
	if citedCount > 0 {
		return domain.ErrCannotDeleteSubmittedFact
	}

	res, err := v.db.ExecContext(ctx, `DELETE FROM roles WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Career Facts ---

// SaveCareerFact creates or updates a career fact. Validates epistemic state and syncs verified boolean.
func (v *CareerVault) SaveCareerFact(ctx context.Context, fact *domain.CareerFact) error {
	if fact.ID == "" {
		return errors.New("career fact ID cannot be empty")
	}
	if fact.RoleID == "" {
		return errors.New("career fact RoleID cannot be empty")
	}
	if fact.EpistemicState == "" {
		if fact.Verified {
			fact.EpistemicState = domain.EpistemicVerified
		} else {
			fact.EpistemicState = domain.EpistemicRawNote
		}
	}
	if err := fact.EpistemicState.Validate(); err != nil {
		return err
	}

	// Epistemic state is the primary source of truth for verification
	fact.Verified = (fact.EpistemicState == domain.EpistemicVerified)

	now := time.Now().UTC()
	if fact.CreatedAt.IsZero() {
		fact.CreatedAt = now
	}
	fact.UpdatedAt = now

	techJSON, err := marshalStringSlice(fact.TechTags)
	if err != nil {
		return fmt.Errorf("marshal tech tags: %w", err)
	}

	verifiedInt := 0
	if fact.Verified {
		verifiedInt = 1
	}

	query := `
	INSERT INTO career_facts (id, role_id, headline, details, epistemic_state, tech_tags, verified, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		role_id = excluded.role_id,
		headline = excluded.headline,
		details = excluded.details,
		epistemic_state = excluded.epistemic_state,
		tech_tags = excluded.tech_tags,
		verified = excluded.verified,
		updated_at = excluded.updated_at;`

	_, err = v.db.ExecContext(ctx, query,
		fact.ID,
		fact.RoleID,
		fact.Headline,
		fact.Details,
		string(fact.EpistemicState),
		techJSON,
		verifiedInt,
		fact.CreatedAt.Format(time.RFC3339),
		fact.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save career fact: %w", err)
	}
	return nil
}

// GetCareerFact retrieves a career fact by ID (satisfies Vault interface).
func (v *CareerVault) GetCareerFact(id string) (*domain.CareerFact, error) {
	return v.GetCareerFactContext(context.Background(), id)
}

// GetCareerFactByID resolves a career fact by ID (satisfies domain.DraftResolver and domain.FactLookup).
func (v *CareerVault) GetCareerFactByID(id string) (*domain.CareerFact, error) {
	return v.GetCareerFactContext(context.Background(), id)
}

// GetCareerFactContext retrieves a career fact by ID with context.
func (v *CareerVault) GetCareerFactContext(ctx context.Context, id string) (*domain.CareerFact, error) {
	query := `
	SELECT id, role_id, headline, details, epistemic_state, tech_tags, verified, created_at, updated_at
	FROM career_facts WHERE id = ?;`

	var f domain.CareerFact
	var techJSON, epistemicStr, createdAtStr, updatedAtStr string
	var verifiedInt int

	err := v.db.QueryRowContext(ctx, query, id).Scan(
		&f.ID,
		&f.RoleID,
		&f.Headline,
		&f.Details,
		&epistemicStr,
		&techJSON,
		&verifiedInt,
		&createdAtStr,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get career fact: %w", err)
	}

	f.EpistemicState = domain.EpistemicState(epistemicStr)
	f.Verified = verifiedInt == 1
	_ = json.Unmarshal([]byte(techJSON), &f.TechTags)
	f.CreatedAt = parseDBTime(createdAtStr)
	f.UpdatedAt = parseDBTime(updatedAtStr)
	return &f, nil
}

// ListCareerFacts lists all career facts.
func (v *CareerVault) ListCareerFacts(ctx context.Context) ([]*domain.CareerFact, error) {
	query := `
	SELECT id, role_id, headline, details, epistemic_state, tech_tags, verified, created_at, updated_at
	FROM career_facts ORDER BY created_at DESC;`
	return v.queryFacts(ctx, query)
}

// ListCareerFactsByRole lists all career facts for a specific role.
func (v *CareerVault) ListCareerFactsByRole(ctx context.Context, roleID string) ([]*domain.CareerFact, error) {
	query := `
	SELECT id, role_id, headline, details, epistemic_state, tech_tags, verified, created_at, updated_at
	FROM career_facts WHERE role_id = ? ORDER BY created_at DESC;`
	return v.queryFacts(ctx, query, roleID)
}

func (v *CareerVault) queryFacts(ctx context.Context, query string, args ...any) ([]*domain.CareerFact, error) {
	rows, err := v.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query facts: %w", err)
	}
	defer rows.Close()

	var list []*domain.CareerFact
	for rows.Next() {
		var f domain.CareerFact
		var techJSON, epistemicStr, createdAtStr, updatedAtStr string
		var verifiedInt int

		if err := rows.Scan(
			&f.ID,
			&f.RoleID,
			&f.Headline,
			&f.Details,
			&epistemicStr,
			&techJSON,
			&verifiedInt,
			&createdAtStr,
			&updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("scan fact: %w", err)
		}

		f.EpistemicState = domain.EpistemicState(epistemicStr)
		f.Verified = verifiedInt == 1
		_ = json.Unmarshal([]byte(techJSON), &f.TechTags)
		f.CreatedAt = parseDBTime(createdAtStr)
		f.UpdatedAt = parseDBTime(updatedAtStr)
		list = append(list, &f)
	}
	return list, rows.Err()
}

// DeleteCareerFact deletes a career fact, guarding against deleting facts cited in Submitted applications.
// If the fact is cited in a Submitted application, returns domain.ErrCannotDeleteSubmittedFact.
func (v *CareerVault) DeleteCareerFact(ctx context.Context, id string) error {
	cited, err := v.IsFactCitedInSubmittedApp(ctx, id)
	if err != nil {
		return fmt.Errorf("check cited fact: %w", err)
	}
	if cited {
		return domain.ErrCannotDeleteSubmittedFact
	}

	res, err := v.db.ExecContext(ctx, `DELETE FROM career_facts WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete career fact: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ArchiveCareerFact transitions a fact's epistemic state to archived.
func (v *CareerVault) ArchiveCareerFact(ctx context.Context, id string) error {
	query := `
	UPDATE career_facts
	SET epistemic_state = ?, verified = 0, updated_at = ?
	WHERE id = ?;`

	res, err := v.db.ExecContext(ctx, query, string(domain.EpistemicArchived), time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("archive career fact: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Education ---

// SaveEducation creates or updates an education credential.
func (v *CareerVault) SaveEducation(ctx context.Context, edu *domain.Education) error {
	if edu.ID == "" {
		return errors.New("education ID cannot be empty")
	}
	now := time.Now().UTC()
	if edu.CreatedAt.IsZero() {
		edu.CreatedAt = now
	}
	edu.UpdatedAt = now

	query := `
	INSERT INTO education (id, institution, degree, field_of_study, location, duration, honors, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		institution = excluded.institution,
		degree = excluded.degree,
		field_of_study = excluded.field_of_study,
		location = excluded.location,
		duration = excluded.duration,
		honors = excluded.honors,
		updated_at = excluded.updated_at;`

	_, err := v.db.ExecContext(ctx, query,
		edu.ID,
		edu.Institution,
		edu.Degree,
		edu.FieldOfStudy,
		edu.Location,
		edu.Duration,
		edu.Honors,
		edu.CreatedAt.Format(time.RFC3339),
		edu.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save education: %w", err)
	}
	return nil
}

// GetEducation retrieves an education credential by ID.
func (v *CareerVault) GetEducation(ctx context.Context, id string) (*domain.Education, error) {
	query := `SELECT id, institution, degree, field_of_study, location, duration, honors, created_at, updated_at FROM education WHERE id = ?;`
	var edu domain.Education
	var createdAtStr, updatedAtStr string

	err := v.db.QueryRowContext(ctx, query, id).Scan(
		&edu.ID,
		&edu.Institution,
		&edu.Degree,
		&edu.FieldOfStudy,
		&edu.Location,
		&edu.Duration,
		&edu.Honors,
		&createdAtStr,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get education: %w", err)
	}
	edu.CreatedAt = parseDBTime(createdAtStr)
	edu.UpdatedAt = parseDBTime(updatedAtStr)
	return &edu, nil
}

// ListEducation lists all education credentials.
func (v *CareerVault) ListEducation(ctx context.Context) ([]*domain.Education, error) {
	query := `SELECT id, institution, degree, field_of_study, location, duration, honors, created_at, updated_at FROM education ORDER BY created_at DESC;`
	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list education: %w", err)
	}
	defer rows.Close()

	var list []*domain.Education
	for rows.Next() {
		var edu domain.Education
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(
			&edu.ID,
			&edu.Institution,
			&edu.Degree,
			&edu.FieldOfStudy,
			&edu.Location,
			&edu.Duration,
			&edu.Honors,
			&createdAtStr,
			&updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("scan education: %w", err)
		}
		edu.CreatedAt = parseDBTime(createdAtStr)
		edu.UpdatedAt = parseDBTime(updatedAtStr)
		list = append(list, &edu)
	}
	return list, rows.Err()
}

// DeleteEducation deletes an education credential by ID.
func (v *CareerVault) DeleteEducation(ctx context.Context, id string) error {
	res, err := v.db.ExecContext(ctx, `DELETE FROM education WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete education: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Archetypes ---

// SaveArchetype creates or updates a resume archetype.
func (v *CareerVault) SaveArchetype(ctx context.Context, arch *domain.Archetype) error {
	if arch.ID == "" {
		return errors.New("archetype ID cannot be empty")
	}
	now := time.Now().UTC()
	if arch.CreatedAt.IsZero() {
		arch.CreatedAt = now
	}
	arch.UpdatedAt = now

	compJSON, err := marshalStringSlice(arch.CoreCompetencies)
	if err != nil {
		return fmt.Errorf("marshal core competencies: %w", err)
	}
	stratJSON, err := marshalStringSlice(arch.StrategicImpact)
	if err != nil {
		return fmt.Errorf("marshal strategic impact: %w", err)
	}
	rolesJSON, err := marshalStringSlice(arch.TargetRoles)
	if err != nil {
		return fmt.Errorf("marshal target roles: %w", err)
	}

	query := `
	INSERT INTO archetypes (id, name, profile_overview, core_competencies, strategic_impact, target_roles, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		profile_overview = excluded.profile_overview,
		core_competencies = excluded.core_competencies,
		strategic_impact = excluded.strategic_impact,
		target_roles = excluded.target_roles,
		updated_at = excluded.updated_at;`

	_, err = v.db.ExecContext(ctx, query,
		arch.ID,
		arch.Name,
		arch.ProfileOverview,
		compJSON,
		stratJSON,
		rolesJSON,
		arch.CreatedAt.Format(time.RFC3339),
		arch.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save archetype: %w", err)
	}
	return nil
}

// GetArchetypeByID resolves an archetype by ID (satisfies domain.DraftResolver).
func (v *CareerVault) GetArchetypeByID(id string) (*domain.Archetype, error) {
	return v.GetArchetype(context.Background(), id)
}

// GetArchetype retrieves an archetype by ID.
func (v *CareerVault) GetArchetype(ctx context.Context, id string) (*domain.Archetype, error) {
	query := `
	SELECT id, name, profile_overview, core_competencies, strategic_impact, target_roles, created_at, updated_at
	FROM archetypes WHERE id = ?;`

	var a domain.Archetype
	var compJSON, stratJSON, rolesJSON, createdAtStr, updatedAtStr string

	err := v.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID,
		&a.Name,
		&a.ProfileOverview,
		&compJSON,
		&stratJSON,
		&rolesJSON,
		&createdAtStr,
		&updatedAtStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get archetype: %w", err)
	}

	_ = json.Unmarshal([]byte(compJSON), &a.CoreCompetencies)
	_ = json.Unmarshal([]byte(stratJSON), &a.StrategicImpact)
	_ = json.Unmarshal([]byte(rolesJSON), &a.TargetRoles)
	a.CreatedAt = parseDBTime(createdAtStr)
	a.UpdatedAt = parseDBTime(updatedAtStr)
	return &a, nil
}

// ListArchetypes lists all archetypes.
func (v *CareerVault) ListArchetypes(ctx context.Context) ([]*domain.Archetype, error) {
	query := `
	SELECT id, name, profile_overview, core_competencies, strategic_impact, target_roles, created_at, updated_at
	FROM archetypes ORDER BY name ASC;`

	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list archetypes: %w", err)
	}
	defer rows.Close()

	var list []*domain.Archetype
	for rows.Next() {
		var a domain.Archetype
		var compJSON, stratJSON, rolesJSON, createdAtStr, updatedAtStr string
		if err := rows.Scan(
			&a.ID,
			&a.Name,
			&a.ProfileOverview,
			&compJSON,
			&stratJSON,
			&rolesJSON,
			&createdAtStr,
			&updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("scan archetype: %w", err)
		}
		_ = json.Unmarshal([]byte(compJSON), &a.CoreCompetencies)
		_ = json.Unmarshal([]byte(stratJSON), &a.StrategicImpact)
		_ = json.Unmarshal([]byte(rolesJSON), &a.TargetRoles)
		a.CreatedAt = parseDBTime(createdAtStr)
		a.UpdatedAt = parseDBTime(updatedAtStr)
		list = append(list, &a)
	}
	return list, rows.Err()
}

// DeleteArchetype deletes an archetype by ID.
func (v *CareerVault) DeleteArchetype(ctx context.Context, id string) error {
	res, err := v.db.ExecContext(ctx, `DELETE FROM archetypes WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete archetype: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Applications ---

// SaveApplication creates or updates a JobApplication.
// Returns domain.ErrAlreadySubmitted if the existing persisted row is already Submitted.
func (v *CareerVault) SaveApplication(ctx context.Context, app *domain.JobApplication) error {
	if app.ID == "" {
		return errors.New("application ID cannot be empty")
	}

	var existingState string
	err := v.db.QueryRowContext(ctx, `SELECT state FROM applications WHERE id = ?;`, app.ID).Scan(&existingState)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("check existing application state: %w", err)
	}
	if err == nil && domain.ApplicationState(existingState) == domain.StateSubmitted {
		return domain.ErrAlreadySubmitted
	}

	now := time.Now().UTC()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	app.UpdatedAt = now

	var archetypeID sql.NullString
	if app.ArchetypeID != "" {
		archetypeID = sql.NullString{String: app.ArchetypeID, Valid: true}
	}

	dossierJSON, err := marshalOptionalJSON(app.Dossier)
	if err != nil {
		return fmt.Errorf("marshal dossier: %w", err)
	}
	draftJSON, err := marshalOptionalJSON(app.Draft)
	if err != nil {
		return fmt.Errorf("marshal draft: %w", err)
	}
	auditsJSON, err := marshalOptionalJSON(app.Audits)
	if err != nil {
		return fmt.Errorf("marshal audits: %w", err)
	}
	exportJSON, err := marshalOptionalJSON(app.Export)
	if err != nil {
		return fmt.Errorf("marshal export: %w", err)
	}

	var submittedAt sql.NullString
	if app.SubmittedAt != nil && !app.SubmittedAt.IsZero() {
		submittedAt = sql.NullString{String: app.SubmittedAt.UTC().Format(time.RFC3339), Valid: true}
	}

	query := `
	INSERT INTO applications (
		id, role_title, company, raw_jd, state, rejection_rationale, archetype_id,
		dossier_json, draft_json, audits_json, export_json, bundle_sha256, interview_brief_path, submitted_at,
		created_at, updated_at
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		role_title = excluded.role_title,
		company = excluded.company,
		raw_jd = excluded.raw_jd,
		state = excluded.state,
		rejection_rationale = excluded.rejection_rationale,
		archetype_id = excluded.archetype_id,
		dossier_json = excluded.dossier_json,
		draft_json = excluded.draft_json,
		audits_json = excluded.audits_json,
		export_json = excluded.export_json,
		bundle_sha256 = excluded.bundle_sha256,
		interview_brief_path = excluded.interview_brief_path,
		submitted_at = excluded.submitted_at,
		updated_at = excluded.updated_at;`

	_, err = v.db.ExecContext(ctx, query,
		app.ID,
		app.RoleTitle,
		app.Company,
		app.RawJD,
		string(app.State),
		app.RejectionRationale,
		archetypeID,
		dossierJSON,
		draftJSON,
		auditsJSON,
		exportJSON,
		app.BundleSHA256,
		app.InterviewBriefPath,
		submittedAt,
		app.CreatedAt.Format(time.RFC3339),
		app.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save application: %w", err)
	}
	return nil
}

// GetApplication retrieves an application by ID (satisfies Vault interface).
func (v *CareerVault) GetApplication(id string) (*domain.JobApplication, error) {
	return v.GetApplicationContext(context.Background(), id)
}

const selectApplicationColumns = `
	id, role_title, company, raw_jd, state, rejection_rationale, archetype_id,
	dossier_json, draft_json, audits_json, export_json, bundle_sha256, interview_brief_path, submitted_at,
	created_at, updated_at`

type applicationScanner interface {
	Scan(dest ...any) error
}

func scanApplication(s applicationScanner) (*domain.JobApplication, error) {
	var app domain.JobApplication
	var stateStr, createdAtStr, updatedAtStr string
	var archetypeID, submittedAtStr sql.NullString
	var dossierJSON, draftJSON, auditsJSON, exportJSON string

	if err := s.Scan(
		&app.ID,
		&app.RoleTitle,
		&app.Company,
		&app.RawJD,
		&stateStr,
		&app.RejectionRationale,
		&archetypeID,
		&dossierJSON,
		&draftJSON,
		&auditsJSON,
		&exportJSON,
		&app.BundleSHA256,
		&app.InterviewBriefPath,
		&submittedAtStr,
		&createdAtStr,
		&updatedAtStr,
	); err != nil {
		return nil, err
	}

	app.State = domain.ApplicationState(stateStr)
	if archetypeID.Valid {
		app.ArchetypeID = archetypeID.String
	}
	if err := unmarshalOptionalJSON(dossierJSON, &app.Dossier); err != nil {
		return nil, fmt.Errorf("unmarshal dossier: %w", err)
	}
	if err := unmarshalOptionalJSON(draftJSON, &app.Draft); err != nil {
		return nil, fmt.Errorf("unmarshal draft: %w", err)
	}
	if err := unmarshalOptionalJSON(auditsJSON, &app.Audits); err != nil {
		return nil, fmt.Errorf("unmarshal audits: %w", err)
	}
	if err := unmarshalOptionalJSON(exportJSON, &app.Export); err != nil {
		return nil, fmt.Errorf("unmarshal export: %w", err)
	}
	if submittedAtStr.Valid && submittedAtStr.String != "" {
		t := parseDBTime(submittedAtStr.String)
		if !t.IsZero() {
			app.SubmittedAt = &t
		}
	}
	app.CreatedAt = parseDBTime(createdAtStr)
	app.UpdatedAt = parseDBTime(updatedAtStr)
	return &app, nil
}

// GetApplicationContext retrieves an application by ID with context.
func (v *CareerVault) GetApplicationContext(ctx context.Context, id string) (*domain.JobApplication, error) {
	query := `SELECT` + selectApplicationColumns + ` FROM applications WHERE id = ?;`
	app, err := scanApplication(v.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}
	return app, nil
}

// ListApplications lists all job applications.
func (v *CareerVault) ListApplications(ctx context.Context) ([]*domain.JobApplication, error) {
	query := `SELECT` + selectApplicationColumns + ` FROM applications ORDER BY created_at DESC;`
	rows, err := v.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()

	var list []*domain.JobApplication
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}
		list = append(list, app)
	}
	return list, rows.Err()
}

func marshalOptionalJSON[T any](ptr *T) (string, error) {
	if ptr == nil {
		return "", nil
	}
	b, err := json.Marshal(ptr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalOptionalJSON[T any](raw string, dest **T) error {
	if raw == "" {
		return nil
	}
	var val T
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return err
	}
	*dest = &val
	return nil
}

// DeleteApplication deletes an application by ID.
func (v *CareerVault) DeleteApplication(ctx context.Context, id string) error {
	res, err := v.db.ExecContext(ctx, `DELETE FROM applications WHERE id = ?;`, id)
	if err != nil {
		return fmt.Errorf("delete application: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- Citations & Soft-Archive Provenance Guards ---

// RecordApplicationCitations associates career facts as citations for a job application.
func (v *CareerVault) RecordApplicationCitations(ctx context.Context, appID string, factIDs []string) error {
	return v.WithTx(ctx, func(tx *sql.Tx) error {
		// Clear existing citations for this application
		if _, err := tx.ExecContext(ctx, `DELETE FROM application_citations WHERE application_id = ?;`, appID); err != nil {
			return fmt.Errorf("delete old citations: %w", err)
		}

		stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO application_citations (application_id, fact_id) VALUES (?, ?);`)
		if err != nil {
			return fmt.Errorf("prepare insert citation: %w", err)
		}
		defer stmt.Close()

		for _, fid := range factIDs {
			if _, err := stmt.ExecContext(ctx, appID, fid); err != nil {
				return fmt.Errorf("insert citation (%s, %s): %w", appID, fid, err)
			}
		}
		return nil
	})
}

// GetCitationsForApp returns all fact IDs cited by an application.
func (v *CareerVault) GetCitationsForApp(ctx context.Context, appID string) ([]string, error) {
	rows, err := v.db.QueryContext(ctx, `SELECT fact_id FROM application_citations WHERE application_id = ? ORDER BY fact_id ASC;`, appID)
	if err != nil {
		return nil, fmt.Errorf("get citations: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan citation id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IsFactCitedInSubmittedApp checks if a fact is cited by any application currently in Submitted state.
func (v *CareerVault) IsFactCitedInSubmittedApp(ctx context.Context, factID string) (bool, error) {
	query := `
	SELECT COUNT(1)
	FROM application_citations ac
	JOIN applications a ON ac.application_id = a.id
	WHERE ac.fact_id = ? AND a.state = ?;`

	var count int
	err := v.db.QueryRowContext(ctx, query, factID, string(domain.StateSubmitted)).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check cited fact in submitted app: %w", err)
	}
	return count > 0, nil
}

// parseDBTime parses ISO8601 or SQLite datetime strings into time.Time.
func parseDBTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// marshalStringSlice serializes a string slice to a JSON array string for SQLite TEXT storage.
// Empty or nil slices are normalized to "[]".
func marshalStringSlice(s []string) (string, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
