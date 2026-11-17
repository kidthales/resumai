package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/render"
)

// PIIProvider retrieves the candidate's air-gapped contact PII locally for render-time injection.
// It is consumed strictly inside LocalMaterializer and never exposed on vault.Vault or MCP tools.
type PIIProvider interface {
	GetPII(ctx context.Context) (*domain.PIIHeader, error)
}

// SubmittedPacketVerifier re-verifies that a StateSubmitted application's
// locked packet directory and sealed bundle hash remain intact on disk.
type SubmittedPacketVerifier interface {
	VerifySubmittedPacket(ctx context.Context, app *domain.JobApplication) error
}

type applicationContextProvider interface {
	GetApplicationContext(ctx context.Context, id string) (*domain.JobApplication, error)
}

type applicationListProvider interface {
	ListApplications(ctx context.Context) ([]*domain.JobApplication, error)
}

// PDFRenderFunc renders a ResumeDocument to PDF bytes and DOM layout telemetry.
type PDFRenderFunc func(ctx context.Context, doc *render.ResumeDocument, pii *domain.PIIHeader, theme *render.ThemeManifest, targetPages int) ([]byte, *render.LayoutTelemetry, error)

// MaterializerOption configures optional dependencies on LocalMaterializer.
type MaterializerOption func(*LocalMaterializer)

// WithBaseOutputDir overrides the default ~/.resumai/applications base directory.
func WithBaseOutputDir(dir string) MaterializerOption {
	return func(m *LocalMaterializer) {
		m.baseOutputDir = strings.TrimSpace(dir)
	}
}

// WithFactResolver configures live CareerVault fact verification prior to rendering.
func WithFactResolver(r render.FactResolver) MaterializerOption {
	return func(m *LocalMaterializer) {
		m.factResolver = r
	}
}

// WithPDFRenderer overrides headless Chromium PDF rendering (primarily for unit tests).
func WithPDFRenderer(fn PDFRenderFunc) MaterializerOption {
	return func(m *LocalMaterializer) {
		m.pdfRenderer = fn
	}
}

// WithNowFunc overrides the wall clock used for timestamps and fallback packet folder dates.
func WithNowFunc(fn func() time.Time) MaterializerOption {
	return func(m *LocalMaterializer) {
		if fn != nil {
			m.now = fn
		}
	}
}

// LocalMaterializer implements PacketMaterializer by converting validated ApplicationDrafts
// into ResumeDocument ASTs, injecting local PII strictly into isolated clones, writing
// resume.{html,docx,pdf} + audit.md, and locking submitted packets with SHA-256 + interview_brief.md.
type LocalMaterializer struct {
	piiProvider   PIIProvider
	factResolver  render.FactResolver
	baseOutputDir string
	pdfRenderer   PDFRenderFunc
	now           func() time.Time
}

var (
	_ PacketMaterializer      = (*LocalMaterializer)(nil)
	_ SubmittedPacketVerifier = (*LocalMaterializer)(nil)
)

// NewLocalMaterializer constructs a LocalMaterializer backed by the provided local PIIProvider.
// If piiProvider also implements render.FactResolver (e.g. *vault.CareerVault), it is wired automatically.
func NewLocalMaterializer(piiProvider PIIProvider, opts ...MaterializerOption) *LocalMaterializer {
	m := &LocalMaterializer{
		piiProvider: piiProvider,
		now:         func() time.Time { return time.Now().UTC() },
	}
	if fr, ok := piiProvider.(render.FactResolver); ok {
		m.factResolver = fr
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// DefaultApplicationsDir resolves the default base directory for materialized application packets
// (~/.resumai/applications), or RESUMAI_APPLICATIONS_DIR if set.
func DefaultApplicationsDir() (string, error) {
	if env := strings.TrimSpace(os.Getenv("RESUMAI_APPLICATIONS_DIR")); env != "" {
		return filepath.Clean(env), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home dir: %w", err)
	}
	return filepath.Join(home, ".resumai", "applications"), nil
}

// DraftToResumeDocument converts an air-gapped JobApplication and its ApplicationDraft
// into a render.ResumeDocument AST with zero PII populated in Header.
func DraftToResumeDocument(app *domain.JobApplication) (*render.ResumeDocument, error) {
	if app == nil {
		return nil, fmt.Errorf("%w: nil JobApplication", domain.ErrInvalidStateTransition)
	}
	if app.Draft == nil {
		return nil, fmt.Errorf("%w: application %q has no draft", domain.ErrInvalidStateTransition, app.ID)
	}

	draft := app.Draft
	pageBudget := draft.PageBudget
	if pageBudget <= 0 {
		pageBudget = 1
	}
	archetypeID := strings.TrimSpace(draft.ArchetypeID)
	if archetypeID == "" {
		archetypeID = strings.TrimSpace(app.ArchetypeID)
	}

	doc := render.NewResumeDocument()
	doc.Metadata = render.DocumentMetadata{
		DocumentID:    app.ID,
		ApplicationID: app.ID,
		ArchetypeID:   archetypeID,
		TargetRole:    app.RoleTitle,
		TargetCompany: app.Company,
		PageBudget:    pageBudget,
		CreatedAt:     app.CreatedAt,
		UpdatedAt:     app.UpdatedAt,
	}
	doc.Header = render.Header{
		Headline: draft.Headline,
	}

	if strings.TrimSpace(draft.Summary) != "" {
		doc.Summary = &render.SummarySection{
			Text: draft.Summary,
			Runs: render.ParseInlineRuns(draft.Summary),
		}
	}

	doc.Experience = make([]render.ExperienceItem, 0, len(draft.Experience))
	for _, exp := range draft.Experience {
		duration := exp.Duration
		if strings.TrimSpace(duration) == "" {
			duration = formatDateRange(exp.StartDate, exp.EndDate)
		}
		highlights := make([]render.Bullet, 0, len(exp.Highlights))
		for _, b := range exp.Highlights {
			highlights = append(highlights, render.NewBulletFromNode(b))
		}
		doc.Experience = append(doc.Experience, render.ExperienceItem{
			ID:           exp.ID,
			RoleTitle:    exp.RoleTitle,
			Organization: exp.Organization,
			Location:     exp.Location,
			StartDate:    exp.StartDate,
			EndDate:      exp.EndDate,
			Duration:     duration,
			Highlights:   highlights,
		})
	}

	doc.Education = make([]render.EducationItem, 0, len(draft.Education))
	for _, edu := range draft.Education {
		duration := edu.Duration
		if strings.TrimSpace(duration) == "" {
			duration = formatDateRange(edu.StartDate, edu.EndDate)
		}
		highlights := make([]render.Bullet, 0, len(edu.Highlights))
		for _, b := range edu.Highlights {
			highlights = append(highlights, render.NewBulletFromNode(b))
		}
		doc.Education = append(doc.Education, render.EducationItem{
			ID:           edu.ID,
			Institution:  edu.Institution,
			Degree:       edu.Degree,
			FieldOfStudy: edu.FieldOfStudy,
			Location:     edu.Location,
			StartDate:    edu.StartDate,
			EndDate:      edu.EndDate,
			Duration:     duration,
			Honors:       edu.Honors,
			Highlights:   highlights,
		})
	}

	doc.Skills = make([]render.SkillCategory, 0, len(draft.Skills))
	for _, sc := range draft.Skills {
		skillsCopy := make([]string, len(sc.Skills))
		copy(skillsCopy, sc.Skills)
		doc.Skills = append(doc.Skills, render.SkillCategory{
			Name:   sc.Name,
			Skills: skillsCopy,
		})
	}

	doc.Projects = make([]render.ProjectItem, 0, len(draft.Projects))
	for _, proj := range draft.Projects {
		techCopy := make([]string, len(proj.Technologies))
		copy(techCopy, proj.Technologies)
		highlights := make([]render.Bullet, 0, len(proj.Highlights))
		for _, b := range proj.Highlights {
			highlights = append(highlights, render.NewBulletFromNode(b))
		}
		doc.Projects = append(doc.Projects, render.ProjectItem{
			Name:         proj.Name,
			Role:         proj.Role,
			URL:          proj.URL,
			Duration:     proj.Duration,
			Technologies: techCopy,
			Highlights:   highlights,
		})
	}

	if len(draft.Certifications) > 0 {
		doc.Certifications = make([]string, len(draft.Certifications))
		copy(doc.Certifications, draft.Certifications)
	}

	if len(draft.CustomSections) > 0 {
		doc.CustomSections = make([]render.CustomSection, 0, len(draft.CustomSections))
		for _, sec := range draft.CustomSections {
			highlights := make([]render.Bullet, 0, len(sec.Highlights))
			for _, b := range sec.Highlights {
				highlights = append(highlights, render.NewBulletFromNode(b))
			}
			doc.CustomSections = append(doc.CustomSections, render.CustomSection{
				Title:      sec.Title,
				Highlights: highlights,
			})
		}
	}

	return doc, nil
}

// RenderAndMarkExported converts the application draft to a ResumeDocument, injects local PII
// strictly into isolated render clones, enforces page_count <= max_pages, writes
// resume.{html,docx,pdf} + audit.md under ~/.resumai/applications/YYYY-MM-DD-<org>-<role>-<hash>/,
// and transitions ReadyToRender -> Exported.
func (m *LocalMaterializer) RenderAndMarkExported(ctx context.Context, req RenderBundleRequest) (*RenderBundleResult, error) {
	app := req.Application
	if app == nil {
		return nil, fmt.Errorf("%w: nil JobApplication", domain.ErrInvalidStateTransition)
	}
	if app.State == domain.StateSubmitted {
		return nil, m.VerifySubmittedPacket(ctx, app)
	}
	if app.State != domain.StateReadyToRender && app.State != domain.StateExported {
		return nil, fmt.Errorf("%w: cannot render bundle from state %q", domain.ErrInvalidStateTransition, app.State)
	}
	if m.piiProvider == nil {
		return nil, fmt.Errorf("%w: local PII provider is not configured", domain.ErrMissingPIIHeader)
	}
	if lp, ok := m.piiProvider.(applicationContextProvider); ok && strings.TrimSpace(app.ID) != "" {
		if persisted, perr := lp.GetApplicationContext(ctx, app.ID); perr == nil && persisted != nil && persisted.State == domain.StateSubmitted {
			return nil, m.VerifySubmittedPacket(ctx, persisted)
		}
	}

	pii, err := m.piiProvider.GetPII(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: configure contact info via 'resumai pii set' before rendering", domain.ErrMissingPIIHeader)
		}
		return nil, scrubPIIError(fmt.Errorf("load local PII header: %w", err), nil)
	}
	if pii == nil || strings.TrimSpace(pii.FullName) == "" {
		return nil, scrubPIIError(fmt.Errorf("%w: full_name must be populated via 'resumai pii set' before rendering", domain.ErrMissingPIIHeader), pii)
	}
	if m.factResolver == nil {
		return nil, scrubPIIError(fmt.Errorf("%w: live fact resolver is required for render-time verification", domain.ErrUnverifiedFactOrPlaceholder), pii)
	}
	if app.Draft != nil {
		for _, factID := range app.Draft.CitedFactIDs() {
			fact, ferr := m.factResolver.GetCareerFact(factID)
			if ferr != nil || fact == nil {
				return nil, scrubPIIError(fmt.Errorf("%w: cited career fact %q could not be resolved from vault: %v", domain.ErrUngroundedBullet, factID, ferr), pii)
			}
			if !fact.Verified || fact.EpistemicState != domain.EpistemicVerified {
				return nil, scrubPIIError(fmt.Errorf("%w: cited career fact %q is not verified (verified=%t, epistemic_state=%q)", domain.ErrUnverifiedFactOrPlaceholder, factID, fact.Verified, fact.EpistemicState), pii)
			}
		}
	}

	doc, err := DraftToResumeDocument(app)
	if err != nil {
		return nil, scrubPIIError(err, pii)
	}

	// Validate on an isolated PII-injected clone before invoking renderers.
	validationClone, err := doc.Clone()
	if err != nil {
		return nil, scrubPIIError(fmt.Errorf("clone resume document for validation: %w", err), pii)
	}
	validationClone.InjectPII(pii)
	if err := render.ValidateReadyToRender(validationClone, m.factResolver); err != nil {
		return nil, scrubPIIError(err, pii)
	}

	packetDir, err := m.resolvePacketDir(app, req.OutputDir)
	if err != nil {
		return nil, scrubPIIError(err, pii)
	}
	if err := m.ensurePacketDirAvailable(ctx, app, packetDir); err != nil {
		return nil, scrubPIIError(err, pii)
	}

	themeName := strings.TrimSpace(req.Theme)
	if themeName == "" {
		themeName = "default"
	}
	theme, err := render.LoadEmbeddedTheme(themeName)
	if err != nil {
		return nil, scrubPIIError(err, pii)
	}

	htmlRenderer, err := render.NewHTMLRenderer(theme)
	if err != nil {
		return nil, scrubPIIError(err, pii)
	}
	docxWriter, err := render.NewDocxWriter(theme)
	if err != nil {
		return nil, scrubPIIError(err, pii)
	}

	htmlBytes, err := htmlRenderer.Render(doc, pii)
	if err != nil {
		return nil, scrubPIIError(fmt.Errorf("render HTML: %w", err), pii)
	}
	docxBytes, err := docxWriter.Render(doc, pii)
	if err != nil {
		return nil, scrubPIIError(fmt.Errorf("render DOCX: %w", err), pii)
	}

	maxPages := doc.Metadata.PageBudget
	if maxPages <= 0 {
		maxPages = 1
	}

	pdfBytes, telemetry, err := m.renderPDF(ctx, doc, pii, theme, htmlRenderer, maxPages)
	if err != nil {
		return nil, scrubPIIError(fmt.Errorf("render PDF: %w", err), pii)
	}
	if telemetry == nil || telemetry.PageCount <= 0 {
		return nil, scrubPIIError(fmt.Errorf("%w: missing valid PDF layout telemetry", domain.ErrPageBudgetOverflow), pii)
	}
	if telemetry.PageCount > maxPages {
		return nil, scrubPIIError(
			fmt.Errorf("%w: page_count=%d exceeds max_pages=%d (overflow_px=%.1f)",
				domain.ErrPageBudgetOverflow, telemetry.PageCount, maxPages, telemetry.OverflowPX),
			pii,
		)
	}

	if err := os.MkdirAll(packetDir, 0700); err != nil {
		return nil, scrubPIIError(fmt.Errorf("create packet directory: %w", err), pii)
	}

	now := m.now().UTC()
	htmlPath := filepath.Join(packetDir, "resume.html")
	docxPath := filepath.Join(packetDir, "resume.docx")
	pdfPath := filepath.Join(packetDir, "resume.pdf")
	auditPath := filepath.Join(packetDir, "audit.md")

	if err := m.writeFileEnsuringWritable(ctx, app, htmlPath, htmlBytes, 0600); err != nil {
		return nil, scrubPIIError(fmt.Errorf("write resume.html: %w", err), pii)
	}
	if err := m.writeFileEnsuringWritable(ctx, app, docxPath, docxBytes, 0600); err != nil {
		return nil, scrubPIIError(fmt.Errorf("write resume.docx: %w", err), pii)
	}
	if err := m.writeFileEnsuringWritable(ctx, app, pdfPath, pdfBytes, 0600); err != nil {
		return nil, scrubPIIError(fmt.Errorf("write resume.pdf: %w", err), pii)
	}

	auditMD := buildAuditMarkdown(app, theme.Name, telemetry, maxPages, now)
	if err := m.writeFileEnsuringWritable(ctx, app, auditPath, []byte(auditMD), 0600); err != nil {
		return nil, scrubPIIError(fmt.Errorf("write audit.md: %w", err), pii)
	}
	if app.State == domain.StateExported && app.Export != nil && filepath.Clean(app.Export.PacketDir) == filepath.Clean(packetDir) {
		briefPath := filepath.Join(packetDir, "interview_brief.md")
		if info, serr := os.Lstat(briefPath); serr == nil {
			if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return nil, scrubPIIError(fmt.Errorf("refusing symlink or irregular path %s", filepath.Base(briefPath)), pii)
			}
			if !info.IsDir() {
				if cerr := m.ensureWritableForApp(ctx, app, briefPath, 0600); cerr != nil {
					return nil, scrubPIIError(fmt.Errorf("prepare %s: %w", filepath.Base(briefPath), cerr), pii)
				}
				if rerr := os.Remove(briefPath); rerr != nil {
					return nil, scrubPIIError(fmt.Errorf("remove stale %s: %w", filepath.Base(briefPath), rerr), pii)
				}
			}
		}
	}

	exportRec := domain.ExportRecord{
		PacketDir:  packetDir,
		HTMLPath:   htmlPath,
		DocxPath:   docxPath,
		PDFPath:    pdfPath,
		PageCount:  telemetry.PageCount,
		MaxPages:   maxPages,
		ExportedAt: now,
	}
	if err := app.MarkExported(pii, exportRec); err != nil {
		return nil, scrubPIIError(err, pii)
	}

	return &RenderBundleResult{
		Export:    *app.Export,
		Telemetry: telemetry,
	}, nil
}

// FinalizeSubmission computes a deterministic SHA-256 hash over the exported packet artifacts,
// materializes interview_brief.md, and locks all packet files read-only.
func (m *LocalMaterializer) FinalizeSubmission(ctx context.Context, req SubmissionRequest) (*SubmissionResult, error) {
	app := req.Application
	if app == nil {
		return nil, fmt.Errorf("%w: nil JobApplication", domain.ErrInvalidStateTransition)
	}
	if app.State == domain.StateSubmitted {
		return nil, m.VerifySubmittedPacket(ctx, app)
	}
	if app.State != domain.StateExported {
		return nil, fmt.Errorf("%w: cannot finalize submission from state %q", domain.ErrInvalidStateTransition, app.State)
	}
	if app.Export == nil || strings.TrimSpace(app.Export.PacketDir) == "" {
		return nil, fmt.Errorf("%w: export record missing packet_dir", domain.ErrMissingExportArtifact)
	}

	var pii *domain.PIIHeader
	if m.piiProvider != nil {
		if lp, ok := m.piiProvider.(applicationContextProvider); ok && strings.TrimSpace(app.ID) != "" {
			if persisted, perr := lp.GetApplicationContext(ctx, app.ID); perr == nil && persisted != nil && persisted.State == domain.StateSubmitted {
				return nil, m.VerifySubmittedPacket(ctx, persisted)
			}
		}
		if loaded, err := m.piiProvider.GetPII(ctx); err == nil {
			pii = loaded
		}
	}

	absBase, err := m.resolvedBaseDir()
	if err != nil {
		return nil, scrubPIIError(err, pii)
	}
	packetDir, err := ensurePathWithinDir(absBase, app.Export.PacketDir, false)
	if err != nil {
		return nil, scrubPIIError(fmt.Errorf("%w: invalid packet_dir: %v", domain.ErrMissingExportArtifact, err), pii)
	}

	rawPaths := map[string]string{
		"resume.html": app.Export.HTMLPath,
		"resume.docx": app.Export.DocxPath,
		"resume.pdf":  app.Export.PDFPath,
	}
	artifactPaths := make(map[string]string, len(rawPaths)+1)
	for name, raw := range rawPaths {
		if strings.TrimSpace(raw) == "" {
			return nil, scrubPIIError(fmt.Errorf("%w: missing path for %s", domain.ErrMissingExportArtifact, name), pii)
		}
		jailedPath, err := ensurePathWithinDir(packetDir, raw, false)
		if err != nil {
			return nil, scrubPIIError(fmt.Errorf("%w: %s escapes packet directory: %v", domain.ErrMissingExportArtifact, name, err), pii)
		}
		artifactPaths[name] = jailedPath
	}

	auditPath := filepath.Join(packetDir, "audit.md")
	if info, err := os.Lstat(auditPath); err == nil {
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return nil, scrubPIIError(fmt.Errorf("%w: audit.md must not be a symlink", domain.ErrMissingExportArtifact), pii)
		}
		if !info.IsDir() {
			artifactPaths["audit.md"] = auditPath
		}
	}

	bundleSHA256, fileDigests, err := computeBundleSHA256(artifactPaths)
	if err != nil {
		return nil, scrubPIIError(err, pii)
	}

	now := m.now().UTC()
	briefPath := filepath.Join(packetDir, "interview_brief.md")
	briefMD := buildInterviewBriefMarkdown(app, bundleSHA256, fileDigests, req.Notes, now)
	if err := m.writeFileEnsuringWritable(ctx, app, briefPath, []byte(briefMD), 0600); err != nil {
		return nil, scrubPIIError(fmt.Errorf("write interview_brief.md: %w", err), pii)
	}

	// Lock all packet artifacts read-only so submitted deliverables remain immutable on disk.
	lockTargets := make([]string, 0, len(artifactPaths)+1)
	for _, p := range artifactPaths {
		lockTargets = append(lockTargets, p)
	}
	lockTargets = append(lockTargets, briefPath)
	sort.Strings(lockTargets)

	type priorPerm struct {
		path string
		mode os.FileMode
	}
	var locked []priorPerm
	restoreLocked := func() {
		for _, prev := range locked {
			if prev.mode&0222 != 0 {
				_ = os.Chmod(prev.path, prev.mode)
			}
		}
	}
	for _, target := range lockTargets {
		info, serr := os.Lstat(target)
		if serr != nil {
			restoreLocked()
			return nil, scrubPIIError(fmt.Errorf("inspect submitted packet artifact %s: %w", filepath.Base(target), serr), pii)
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			restoreLocked()
			return nil, scrubPIIError(fmt.Errorf("refusing to lock symlink packet artifact %s", filepath.Base(target)), pii)
		}
		locked = append(locked, priorPerm{path: target, mode: info.Mode().Perm()})
		if err := os.Chmod(target, 0400); err != nil {
			restoreLocked()
			return nil, scrubPIIError(fmt.Errorf("lock submitted packet artifact %s: %w", filepath.Base(target), err), pii)
		}
	}

	return &SubmissionResult{
		BundleSHA256:       bundleSHA256,
		InterviewBriefPath: briefPath,
	}, nil
}

func (m *LocalMaterializer) renderPDF(
	ctx context.Context,
	doc *render.ResumeDocument,
	pii *domain.PIIHeader,
	theme *render.ThemeManifest,
	htmlRenderer *render.HTMLRenderer,
	targetPages int,
) ([]byte, *render.LayoutTelemetry, error) {
	if m.pdfRenderer != nil {
		return m.pdfRenderer(ctx, doc, pii, theme, targetPages)
	}

	printer, err := render.NewPDFPrinter(
		render.WithTheme(theme),
		render.WithHTMLRenderer(htmlRenderer),
		render.WithTargetPages(targetPages),
	)
	if err != nil {
		return nil, nil, err
	}
	return printer.Print(doc, pii)
}

func (m *LocalMaterializer) resolvedBaseDir() (string, error) {
	baseDir := strings.TrimSpace(m.baseOutputDir)
	if baseDir == "" {
		var err error
		baseDir, err = DefaultApplicationsDir()
		if err != nil {
			return "", err
		}
	}
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("resolve base applications dir: %w", err)
	}
	return absBase, nil
}

func (m *LocalMaterializer) resolvePacketDir(app *domain.JobApplication, outputDirOverride string) (string, error) {
	absBase, err := m.resolvedBaseDir()
	if err != nil {
		return "", err
	}

	override := strings.TrimSpace(outputDirOverride)
	if override != "" {
		candidateBase := override
		if !filepath.IsAbs(candidateBase) {
			candidateBase = filepath.Join(absBase, candidateBase)
		}
		jailedBase, err := ensurePathWithinDir(absBase, candidateBase, true)
		if err != nil {
			return "", fmt.Errorf("output_dir escapes configured applications directory: %w", err)
		}
		packetDir := filepath.Join(jailedBase, packetFolderName(app, m.now()))
		return ensurePathWithinDir(absBase, packetDir, false)
	}

	if app.Export != nil && strings.TrimSpace(app.Export.PacketDir) != "" {
		return ensurePathWithinDir(absBase, app.Export.PacketDir, false)
	}

	packetDir := filepath.Join(absBase, packetFolderName(app, m.now()))
	return ensurePathWithinDir(absBase, packetDir, false)
}

// ensurePathWithinDir jails target inside baseDir and rejects any existing symlink
// or junction segment along the relative path.
//
// Threat model note: directory-segment traversal is checked via os.Lstat + EvalSymlinks,
// assuming no concurrent local attacker races intermediate directories under the user's
// private applications root; leaf file writes additionally use O_NOFOLLOW on Unix.
func ensurePathWithinDir(baseDir, target string, allowEqual bool) (string, error) {
	absBase, err := filepath.Abs(filepath.Clean(baseDir))
	if err != nil {
		return "", fmt.Errorf("resolve base directory: %w", err)
	}
	absTarget, err := filepath.Abs(filepath.Clean(target))
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return "", fmt.Errorf("path %q outside base directory", target)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes base directory", target)
	}
	if !allowEqual && rel == "." {
		return "", fmt.Errorf("path %q must be a child inside base directory", target)
	}

	// Reject any existing symlink or junction segment at or below absBase.
	if rel != "." {
		cur := absBase
		for _, seg := range strings.Split(rel, string(filepath.Separator)) {
			if seg == "" || seg == "." {
				continue
			}
			cur = filepath.Join(cur, seg)
			info, lerr := os.Lstat(cur)
			if lerr != nil {
				if errors.Is(lerr, os.ErrNotExist) {
					continue
				}
				return "", fmt.Errorf("inspect path %q: %w", cur, lerr)
			}
			if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return "", fmt.Errorf("path %q must not traverse symlink %q", target, cur)
			}
		}
	}

	// Also verify resolved deepest existing ancestors to block escapes via symlinked base/parent directories.
	resolvedBase, err := resolveDeepestSymlinks(absBase)
	if err != nil {
		return "", fmt.Errorf("resolve base symlinks: %w", err)
	}
	resolvedTarget, err := resolveDeepestSymlinks(absTarget)
	if err != nil {
		return "", fmt.Errorf("resolve target symlinks: %w", err)
	}
	resolvedRel, err := filepath.Rel(resolvedBase, resolvedTarget)
	if err != nil {
		return "", fmt.Errorf("resolved path %q outside base directory", target)
	}
	if resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("resolved path %q escapes base directory", target)
	}
	if !allowEqual && resolvedRel == "." {
		return "", fmt.Errorf("resolved path %q must be a child inside base directory", target)
	}

	return absTarget, nil
}

func resolveDeepestSymlinks(absPath string) (string, error) {
	cur := absPath
	var tail []string
	for {
		_, err := os.Lstat(cur)
		if err == nil {
			resolved, rerr := filepath.EvalSymlinks(cur)
			if rerr != nil {
				return "", rerr
			}
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return absPath, nil
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}

// VerifySubmittedPacket re-verifies that a StateSubmitted application's packet directory,
// artifacts, and SHA-256 manifest match app.BundleSHA256. It always returns an error
// wrapping domain.ErrAlreadySubmitted.
func (m *LocalMaterializer) VerifySubmittedPacket(ctx context.Context, app *domain.JobApplication) error {
	if app == nil {
		return fmt.Errorf("%w: nil JobApplication", domain.ErrInvalidStateTransition)
	}
	if app.State != domain.StateSubmitted {
		return nil
	}
	var pii *domain.PIIHeader
	if m.piiProvider != nil {
		if loaded, err := m.piiProvider.GetPII(ctx); err == nil {
			pii = loaded
		}
	}
	if app.Export == nil || strings.TrimSpace(app.Export.PacketDir) == "" || strings.TrimSpace(app.BundleSHA256) == "" {
		return fmt.Errorf("%w: submitted application %q missing sealed export metadata", domain.ErrAlreadySubmitted, app.ID)
	}

	absBase, err := m.resolvedBaseDir()
	if err != nil {
		return scrubPIIError(fmt.Errorf("%w: resolve packet root: %v", domain.ErrAlreadySubmitted, err), pii)
	}
	packetDir, err := ensurePathWithinDir(absBase, app.Export.PacketDir, false)
	if err != nil {
		return scrubPIIError(fmt.Errorf("%w: sealed packet verification failed: %v", domain.ErrAlreadySubmitted, err), pii)
	}

	rawPaths := map[string]string{
		"resume.html": app.Export.HTMLPath,
		"resume.docx": app.Export.DocxPath,
		"resume.pdf":  app.Export.PDFPath,
	}
	artifactPaths := make(map[string]string, len(rawPaths)+1)
	for name, raw := range rawPaths {
		if strings.TrimSpace(raw) == "" {
			return scrubPIIError(fmt.Errorf("%w: sealed packet verification failed: missing path for %s", domain.ErrAlreadySubmitted, name), pii)
		}
		jailedPath, jerr := ensurePathWithinDir(packetDir, raw, false)
		if jerr != nil {
			return scrubPIIError(fmt.Errorf("%w: sealed packet verification failed: %s escapes packet directory: %v", domain.ErrAlreadySubmitted, name, jerr), pii)
		}
		artifactPaths[name] = jailedPath
	}

	auditPath := filepath.Join(packetDir, "audit.md")
	if info, lerr := os.Lstat(auditPath); lerr == nil {
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || info.IsDir() {
			return scrubPIIError(fmt.Errorf("%w: sealed packet verification failed: audit.md must be a regular non-symlink file", domain.ErrAlreadySubmitted), pii)
		}
		artifactPaths["audit.md"] = auditPath
	} else if !errors.Is(lerr, os.ErrNotExist) {
		return scrubPIIError(fmt.Errorf("%w: sealed packet verification failed: inspect audit.md: %v", domain.ErrAlreadySubmitted, lerr), pii)
	}

	if briefRaw := strings.TrimSpace(app.InterviewBriefPath); briefRaw != "" {
		briefPath, jerr := ensurePathWithinDir(packetDir, briefRaw, false)
		if jerr != nil {
			return scrubPIIError(fmt.Errorf("%w: sealed packet verification failed: interview_brief.md escapes packet directory: %v", domain.ErrAlreadySubmitted, jerr), pii)
		}
		info, lerr := os.Lstat(briefPath)
		if lerr != nil || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || info.IsDir() {
			return scrubPIIError(fmt.Errorf("%w: sealed packet verification failed: interview_brief.md invalid: %v", domain.ErrAlreadySubmitted, lerr), pii)
		}
	}

	actualSHA, _, err := computeBundleSHA256(artifactPaths)
	if err != nil {
		return scrubPIIError(fmt.Errorf("%w: sealed packet integrity check failed: %v", domain.ErrAlreadySubmitted, err), pii)
	}
	if !strings.EqualFold(actualSHA, strings.TrimSpace(app.BundleSHA256)) {
		return scrubPIIError(fmt.Errorf("%w: sealed packet integrity mismatch (expected sha256=%s, got %s)", domain.ErrAlreadySubmitted, app.BundleSHA256, actualSHA), pii)
	}
	return domain.ErrAlreadySubmitted
}

func (m *LocalMaterializer) ensurePacketDirAvailable(ctx context.Context, app *domain.JobApplication, packetDir string) error {
	cleanTarget := filepath.Clean(packetDir)

	if lp, ok := m.piiProvider.(applicationListProvider); ok {
		if apps, err := lp.ListApplications(ctx); err == nil {
			for _, other := range apps {
				if other == nil || other.ID == app.ID || other.Export == nil || strings.TrimSpace(other.Export.PacketDir) == "" {
					continue
				}
				otherAbs, aerr := filepath.Abs(filepath.Clean(other.Export.PacketDir))
				if aerr != nil {
					continue
				}
				if filepath.Clean(otherAbs) == cleanTarget {
					return fmt.Errorf("packet directory %q is already owned by application %q (%s)", packetDir, other.ID, other.State)
				}
			}
		}
	}

	isSameExportedPacket := app.State == domain.StateExported &&
		app.Export != nil &&
		strings.TrimSpace(app.Export.PacketDir) != "" &&
		filepath.Clean(app.Export.PacketDir) == cleanTarget

	auditPath := filepath.Join(packetDir, "audit.md")
	if info, err := os.Lstat(auditPath); err == nil {
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || info.IsDir() {
			return fmt.Errorf("refusing symlink or directory at %q", auditPath)
		}
		data, rerr := os.ReadFile(auditPath)
		if rerr != nil {
			return fmt.Errorf("inspect existing audit.md: %w", rerr)
		}
		ownerID := extractApplicationIDHeader(string(data))
		if ownerID != "" && ownerID != app.ID {
			return fmt.Errorf("packet directory %q belongs to application %q", packetDir, ownerID)
		}
		if ownerID == "" && !isSameExportedPacket {
			return fmt.Errorf("refusing to overwrite unmanaged packet directory %q containing audit.md", packetDir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect packet directory audit.md: %w", err)
	}

	briefPath := filepath.Join(packetDir, "interview_brief.md")
	if info, err := os.Lstat(briefPath); err == nil {
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || info.IsDir() {
			return fmt.Errorf("refusing symlink or directory at %q", briefPath)
		}
		data, rerr := os.ReadFile(briefPath)
		if rerr != nil {
			return fmt.Errorf("inspect existing interview_brief.md: %w", rerr)
		}
		ownerID := extractApplicationIDHeader(string(data))
		if ownerID != "" && ownerID != app.ID {
			return fmt.Errorf("packet directory %q contains interview_brief.md for application %q", packetDir, ownerID)
		}
		if !isSameExportedPacket {
			return fmt.Errorf("refusing to overwrite packet directory %q containing existing interview_brief.md", packetDir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect packet directory interview_brief.md: %w", err)
	}

	return nil
}

func extractApplicationIDHeader(markdown string) string {
	const prefix = "- **Application ID**: `"
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		rest := strings.TrimPrefix(line, prefix)
		end := strings.IndexByte(rest, '`')
		if end > 0 {
			return strings.TrimSpace(rest[:end])
		}
	}
	return ""
}

func (m *LocalMaterializer) ensureWritableForApp(ctx context.Context, app *domain.JobApplication, path string, perm os.FileMode) error {
	if app != nil && app.State == domain.StateSubmitted {
		return domain.ErrAlreadySubmitted
	}
	if app != nil && strings.TrimSpace(app.ID) != "" {
		if cp, ok := m.piiProvider.(applicationContextProvider); ok {
			if persisted, err := cp.GetApplicationContext(ctx, app.ID); err == nil && persisted != nil && persisted.State == domain.StateSubmitted {
				return domain.ErrAlreadySubmitted
			}
		}
	}
	return ensureWritable(path, perm)
}

func (m *LocalMaterializer) writeFileEnsuringWritable(ctx context.Context, app *domain.JobApplication, path string, data []byte, perm os.FileMode) error {
	if err := m.ensureWritableForApp(ctx, app, path, perm); err != nil {
		return err
	}
	return writeFileNoFollow(path, data, perm)
}

func ensureWritable(path string, perm os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("refusing symlink or irregular path %q", path)
	}
	if info.IsDir() {
		return nil
	}
	if info.Mode().Perm()&0200 == 0 {
		return os.Chmod(path, perm)
	}
	return nil
}

func packetFolderName(app *domain.JobApplication, fallbackNow time.Time) string {
	ts := fallbackNow.UTC()
	if app != nil && !app.CreatedAt.IsZero() {
		ts = app.CreatedAt.UTC()
	}
	datePrefix := ts.Format("2006-01-02")

	var orgSlug, roleSlug, idSlug, rawID string
	if app != nil {
		orgSlug = slugify(app.Company)
		roleSlug = slugify(app.RoleTitle)
		idSlug = slugify(app.ID)
		rawID = strings.TrimSpace(app.ID)
	}

	var base string
	switch {
	case orgSlug != "" && roleSlug != "":
		base = fmt.Sprintf("%s-%s-%s", datePrefix, orgSlug, roleSlug)
	case orgSlug != "":
		base = fmt.Sprintf("%s-%s", datePrefix, orgSlug)
	case roleSlug != "":
		base = fmt.Sprintf("%s-%s", datePrefix, roleSlug)
	case idSlug != "":
		base = fmt.Sprintf("%s-%s", datePrefix, idSlug)
	default:
		base = fmt.Sprintf("%s-application", datePrefix)
	}

	if rawID == "" {
		return base
	}
	sum := sha256.Sum256([]byte(rawID))
	return fmt.Sprintf("%s-%s", base, hex.EncodeToString(sum[:4]))
}

func formatDateRange(startDate, endDate string) string {
	start := strings.TrimSpace(startDate)
	end := strings.TrimSpace(endDate)
	switch {
	case start != "" && end != "":
		return start + " - " + end
	case start != "":
		return start
	default:
		return end
	}
}

func computeBundleSHA256(artifactPaths map[string]string) (string, map[string]string, error) {
	names := make([]string, 0, len(artifactPaths))
	for name := range artifactPaths {
		names = append(names, name)
	}
	sort.Strings(names)

	bundleHasher := sha256.New()
	fileDigests := make(map[string]string, len(names))

	for _, name := range names {
		targetPath := artifactPaths[name]
		info, lerr := os.Lstat(targetPath)
		if lerr != nil {
			return "", nil, fmt.Errorf("%w: stat %s for bundle hash: %v", domain.ErrMissingExportArtifact, name, lerr)
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || info.IsDir() {
			return "", nil, fmt.Errorf("%w: %s must be a regular non-symlink file", domain.ErrMissingExportArtifact, name)
		}
		data, err := os.ReadFile(targetPath)
		if err != nil {
			return "", nil, fmt.Errorf("%w: read %s for bundle hash: %v", domain.ErrMissingExportArtifact, name, err)
		}
		fileSum := sha256.Sum256(data)
		fileDigests[name] = hex.EncodeToString(fileSum[:])

		fmt.Fprintf(bundleHasher, "%s\x00%d\x00", name, len(data))
		_, _ = bundleHasher.Write(data)
	}

	return hex.EncodeToString(bundleHasher.Sum(nil)), fileDigests, nil
}

func buildAuditMarkdown(app *domain.JobApplication, themeName string, telemetry *render.LayoutTelemetry, maxPages int, exportedAt time.Time) string {
	var sb strings.Builder
	sb.WriteString("# ResumAI Packet Audit Report\n\n")
	fmt.Fprintf(&sb, "- **Application ID**: `%s`\n", app.ID)
	fmt.Fprintf(&sb, "- **Role**: %s\n", fallbackText(app.RoleTitle, "(unspecified)"))
	fmt.Fprintf(&sb, "- **Company**: %s\n", fallbackText(app.Company, "(unspecified)"))
	fmt.Fprintf(&sb, "- **Archetype**: `%s`\n", fallbackText(app.ArchetypeID, "(none)"))
	fmt.Fprintf(&sb, "- **Theme**: `%s`\n", fallbackText(themeName, "default"))
	fmt.Fprintf(&sb, "- **Exported At (UTC)**: %s\n", exportedAt.UTC().Format(time.RFC3339))

	if telemetry != nil {
		fmt.Fprintf(&sb, "- **Layout Telemetry**: `page_count=%d/%d`, `last_page_fill_ratio=%.3f`, `overflow_px=%.1f`, `orphan_bullets=%d`\n",
			telemetry.PageCount, maxPages, telemetry.LastPageFillRatio, telemetry.OverflowPX, len(telemetry.OrphanBullets))
	}
	sb.WriteString("\n")

	sb.WriteString("## 1. Clean-Room Critic Audits\n\n")
	if app.Audits != nil && app.Audits.FactCheck != nil {
		fc := app.Audits.FactCheck
		fmt.Fprintf(&sb, "### Fact Check (`%s`)\n", fallbackText(fc.CriticAgent, domain.CriticFactChecker))
		fmt.Fprintf(&sb, "- **Passed**: `%t`\n", fc.Passed)
		fmt.Fprintf(&sb, "- **Summary**: %s\n", fc.Summary)
		if len(fc.Discrepancies) == 0 {
			sb.WriteString("- **Discrepancies**: None\n")
		} else {
			sb.WriteString("- **Discrepancies**:\n")
			for _, d := range fc.Discrepancies {
				fmt.Fprintf(&sb, "  - [%s] bullet=`%s` fact=`%s`: %s\n", d.Severity, d.BulletID, d.FactID, d.Finding)
			}
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("### Fact Check\n- No fact-check report recorded.\n\n")
	}

	if app.Audits != nil && app.Audits.AlignmentCheck != nil {
		ac := app.Audits.AlignmentCheck
		fmt.Fprintf(&sb, "### Alignment Check (`%s`)\n", fallbackText(ac.CriticAgent, domain.CriticAlignmentChecker))
		fmt.Fprintf(&sb, "- **Passed**: `%t`\n", ac.Passed)
		fmt.Fprintf(&sb, "- **Summary**: %s\n", ac.Summary)
		if len(ac.MatchedSignals) > 0 {
			fmt.Fprintf(&sb, "- **Matched Signals**: %s\n", strings.Join(ac.MatchedSignals, ", "))
		}
		if len(ac.MissingSignals) > 0 {
			fmt.Fprintf(&sb, "- **Missing Signals**: %s\n", strings.Join(ac.MissingSignals, ", "))
		}
		if len(ac.Recommendations) > 0 {
			sb.WriteString("- **Recommendations**:\n")
			for _, rec := range ac.Recommendations {
				fmt.Fprintf(&sb, "  - %s\n", rec)
			}
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("### Alignment Check\n- No alignment-check report recorded.\n\n")
	}

	sb.WriteString("## 2. Cited Career Facts\n\n")
	if app.Draft != nil {
		cited := app.Draft.CitedFactIDs()
		if len(cited) == 0 {
			sb.WriteString("- None\n")
		} else {
			for _, fid := range cited {
				fmt.Fprintf(&sb, "- `%s`\n", fid)
			}
		}
	} else {
		sb.WriteString("- No draft attached.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## 3. Triage Citation Provenance\n\n")
	if app.Dossier != nil {
		if strings.TrimSpace(app.Dossier.FitSummary) != "" {
			fmt.Fprintf(&sb, "- **Fit Summary**: %s\n", app.Dossier.FitSummary)
		}
		if len(app.Dossier.JDClaims) > 0 {
			sb.WriteString("- **JD Claims**:\n")
			for _, c := range app.Dossier.JDClaims {
				label := fallbackText(c.Summary, c.Category)
				if label != "" {
					fmt.Fprintf(&sb, "  - %s — `\"%s\"`\n", label, c.VerbatimJDQuote)
				} else {
					fmt.Fprintf(&sb, "  - `\"%s\"`\n", c.VerbatimJDQuote)
				}
			}
		}
		if len(app.Dossier.ExternalClaims) > 0 {
			sb.WriteString("- **External Claims**:\n")
			for _, ext := range app.Dossier.ExternalClaims {
				src := strings.TrimSpace(ext.SourceURL)
				if ext.UnknownAskRecruiter || strings.EqualFold(src, domain.UnknownAskRecruiterToken) {
					src = domain.UnknownAskRecruiterToken
				}
				fmt.Fprintf(&sb, "  - %s (source: `%s`)\n", ext.Claim, src)
			}
		}
	} else {
		sb.WriteString("- No opportunity dossier attached.\n")
	}

	return sb.String()
}

func buildInterviewBriefMarkdown(
	app *domain.JobApplication,
	bundleSHA256 string,
	fileDigests map[string]string,
	notes string,
	submittedAt time.Time,
) string {
	var sb strings.Builder
	sb.WriteString("# Interview Preparation Brief\n\n")
	fmt.Fprintf(&sb, "- **Application ID**: `%s`\n", app.ID)
	fmt.Fprintf(&sb, "- **Company**: %s\n", fallbackText(app.Company, "(unspecified)"))
	fmt.Fprintf(&sb, "- **Role**: %s\n", fallbackText(app.RoleTitle, "(unspecified)"))
	fmt.Fprintf(&sb, "- **Submitted At (UTC)**: %s\n", submittedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&sb, "- **Bundle SHA-256**: `%s`\n\n", bundleSHA256)

	sb.WriteString("## 1. Locked Deliverable Checksums\n\n")
	names := make([]string, 0, len(fileDigests))
	for name := range fileDigests {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&sb, "- `%s`: `%s`\n", name, fileDigests[name])
	}
	sb.WriteString("\n")

	sb.WriteString("## 2. Opportunity Context & Fit Summary\n\n")
	var recruiterQuestions []domain.ExternalClaim
	if app.Dossier != nil {
		if strings.TrimSpace(app.Dossier.FitSummary) != "" {
			fmt.Fprintf(&sb, "%s\n\n", app.Dossier.FitSummary)
		}
		if len(app.Dossier.JDClaims) > 0 {
			sb.WriteString("### Grounded Job Requirements\n")
			for _, c := range app.Dossier.JDClaims {
				label := fallbackText(c.Summary, c.Category)
				if label != "" {
					fmt.Fprintf(&sb, "- **%s**: \"%s\"\n", label, c.VerbatimJDQuote)
				} else {
					fmt.Fprintf(&sb, "- \"%s\"\n", c.VerbatimJDQuote)
				}
			}
			sb.WriteString("\n")
		}
		if len(app.Dossier.ExternalClaims) > 0 {
			sb.WriteString("### Verified External Reconnaissance\n")
			hasVerifiedExt := false
			for _, ext := range app.Dossier.ExternalClaims {
				if ext.UnknownAskRecruiter || strings.EqualFold(strings.TrimSpace(ext.SourceURL), domain.UnknownAskRecruiterToken) {
					recruiterQuestions = append(recruiterQuestions, ext)
					continue
				}
				hasVerifiedExt = true
				fmt.Fprintf(&sb, "- %s ([source](%s))\n", ext.Claim, ext.SourceURL)
			}
			if !hasVerifiedExt {
				sb.WriteString("- None recorded.\n")
			}
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("- No opportunity dossier attached.\n\n")
	}

	sb.WriteString("## 3. Questions for Recruiter / Hiring Manager\n\n")
	if len(recruiterQuestions) > 0 {
		for _, q := range recruiterQuestions {
			fmt.Fprintf(&sb, "- Clarify (`%s`): %s\n", domain.UnknownAskRecruiterToken, q.Claim)
		}
	} else {
		sb.WriteString("- No open `unknown_ask_recruiter` items recorded.\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## 4. Alignment Signals & Preparation Focus\n\n")
	if app.Audits != nil && app.Audits.AlignmentCheck != nil {
		ac := app.Audits.AlignmentCheck
		fmt.Fprintf(&sb, "- **Alignment Summary**: %s\n", ac.Summary)
		if len(ac.MatchedSignals) > 0 {
			fmt.Fprintf(&sb, "- **Strengths to Emphasize**: %s\n", strings.Join(ac.MatchedSignals, ", "))
		}
		if len(ac.MissingSignals) > 0 {
			fmt.Fprintf(&sb, "- **Gaps / Risk Areas to Address**: %s\n", strings.Join(ac.MissingSignals, ", "))
		}
		if len(ac.Recommendations) > 0 {
			sb.WriteString("- **Critic Recommendations**:\n")
			for _, rec := range ac.Recommendations {
				fmt.Fprintf(&sb, "  - %s\n", rec)
			}
		}
	} else {
		sb.WriteString("- No alignment report recorded.\n")
	}
	sb.WriteString("\n")

	if strings.TrimSpace(notes) != "" {
		sb.WriteString("## 5. Candidate Submission Notes\n\n")
		fmt.Fprintf(&sb, "%s\n", strings.TrimSpace(notes))
	}

	return sb.String()
}

func fallbackText(val, def string) string {
	if strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return def
}

// scrubPIIError ensures no candidate PII field value can ever leak through an error payload.
func scrubPIIError(err error, pii *domain.PIIHeader) error {
	if err == nil {
		return nil
	}
	msg := render.RedactSecrets(err.Error())
	if pii != nil {
		secrets := []string{
			pii.FullName,
			pii.Email,
			pii.Phone,
			pii.Location,
			pii.LinkedInURL,
			pii.GitHubURL,
			pii.PortfolioURL,
		}
		for _, sec := range secrets {
			trimmed := strings.TrimSpace(sec)
			if trimmed != "" {
				msg = strings.ReplaceAll(msg, trimmed, "[REDACTED_PII]")
			}
		}
	}
	if msg == err.Error() {
		return err
	}
	// Preserve sentinel error unwrapping while replacing the exposed string.
	return &scrubbedError{msg: msg, cause: err}
}

type scrubbedError struct {
	msg   string
	cause error
}

func (e *scrubbedError) Error() string {
	return e.msg
}

func (e *scrubbedError) Unwrap() error {
	return e.cause
}
