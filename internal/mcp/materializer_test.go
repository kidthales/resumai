package mcp_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/mcp"
	"github.com/kidthales/resumai/internal/render"
)

func TestDraftToResumeDocument_NoPII(t *testing.T) {
	created := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	app := &domain.JobApplication{
		ID:          "app-ast-1",
		RoleTitle:   "Staff Systems Engineer",
		Company:     "Acme Corp",
		ArchetypeID: "arch-sys",
		CreatedAt:   created,
		UpdatedAt:   created,
		Draft: &domain.ApplicationDraft{
			ArchetypeID: "arch-sys",
			Headline:    "Staff Distributed Systems Engineer",
			Summary:     "Designs **pure-Go** kernels.",
			Experience: []domain.DraftExperience{
				{
					ID:           "exp-1",
					RoleTitle:    "Senior Engineer",
					Organization: "Prior Co",
					Location:     "Remote",
					StartDate:    "2022",
					EndDate:      "2026",
					Highlights: []domain.BulletNode{
						{ID: "b-1", Text: "Reduced tail latency by **42%**.", FactIDs: []string{"fact-1"}, Verified: true},
					},
				},
			},
			Education: []domain.DraftEducation{
				{
					ID:          "edu-1",
					Institution: "State Tech",
					Degree:      "B.S.",
					StartDate:   "2014",
					EndDate:     "2018",
				},
			},
			Skills: []domain.DraftSkillCategory{
				{Name: "Languages", Skills: []string{"Go", "SQL"}},
			},
			Projects: []domain.DraftProject{
				{
					Name:         "resumai",
					URL:          "https://github.com/kidthales/resumai",
					Technologies: []string{"Go", "SQLite"},
					Highlights: []domain.BulletNode{
						{ID: "b-2", Text: "Built pure-Go OpenXML exporter.", FactIDs: []string{"fact-1"}, Verified: true},
					},
				},
			},
			Certifications: []string{"CKA"},
			CustomSections: []domain.DraftCustomSection{
				{
					Title: "Publications",
					Highlights: []domain.BulletNode{
						{ID: "b-3", Text: "Published storage engine paper.", FactIDs: []string{"fact-1"}, Verified: true},
					},
				},
			},
			PageBudget: 2,
		},
	}

	doc, err := mcp.DraftToResumeDocument(app)
	if err != nil {
		t.Fatalf("DraftToResumeDocument failed: %v", err)
	}

	if doc.Header.FullName != "" || doc.Header.Email != "" || doc.Header.Phone != "" ||
		doc.Header.Location != "" || doc.Header.LinkedInURL != "" || doc.Header.GitHubURL != "" || doc.Header.PortfolioURL != "" {
		t.Fatalf("expected zero PII in converted ResumeDocument header, got %+v", doc.Header)
	}
	if doc.Metadata.PageBudget != 2 || doc.Metadata.TargetCompany != "Acme Corp" || doc.Metadata.TargetRole != "Staff Systems Engineer" {
		t.Fatalf("unexpected metadata: %+v", doc.Metadata)
	}
	if len(doc.Experience) != 1 || doc.Experience[0].Duration != "2022 - 2026" {
		t.Fatalf("expected formatted fallback duration, got %+v", doc.Experience)
	}
	if len(doc.Education) != 1 || doc.Education[0].Duration != "2014 - 2018" {
		t.Fatalf("expected education duration fallback, got %+v", doc.Education)
	}
	if doc.Summary == nil || len(doc.Summary.Runs) == 0 {
		t.Fatalf("expected parsed inline runs on summary, got %+v", doc.Summary)
	}
	if len(doc.AllBullets()) != 3 {
		t.Fatalf("expected 3 bullets across all sections, got %d", len(doc.AllBullets()))
	}
}

func TestLocalMaterializer_EndToEndPacketAndLock(t *testing.T) {
	baseDir := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				_ = os.Chmod(path, 0644)
			}
			return nil
		})
	})

	sentinelPII := &domain.PIIHeader{
		FullName:     "SENTINEL_NAME_7A9C",
		Email:        "sentinel-7a9c@private.invalid",
		Phone:        "+1-555-019-7799",
		Location:     "Sentinel Fortress, XX",
		LinkedInURL:  "https://linkedin.com/in/sentinel-7a9c",
		GitHubURL:    "https://github.com/sentinel-7a9c",
		PortfolioURL: "https://sentinel-7a9c.invalid",
	}

	fixedNow := time.Date(2026, 11, 13, 18, 30, 0, 0, time.UTC)
	pdfCalledWithPII := false
	fakePDF := func(_ context.Context, doc *render.ResumeDocument, pii *domain.PIIHeader, _ *render.ThemeManifest, targetPages int) ([]byte, *render.LayoutTelemetry, error) {
		if doc.Header.FullName != "" || doc.Header.Email != "" {
			t.Errorf("source ResumeDocument passed to PDF renderer must not contain PII, got %+v", doc.Header)
		}
		if pii != nil && pii.FullName == sentinelPII.FullName {
			pdfCalledWithPII = true
		}
		return []byte("%PDF-1.4\n%fake-pdf-payload\n"), &render.LayoutTelemetry{
			PageCount:         1,
			LastPageFillRatio: 0.82,
			OverflowPX:        0,
		}, nil
	}

	v, _ := newTestVaultAndServer(t)
	if err := v.SetPII(context.Background(), sentinelPII); err != nil {
		t.Fatalf("SetPII: %v", err)
	}

	mat := mcp.NewLocalMaterializer(
		v,
		mcp.WithBaseOutputDir(baseDir),
		mcp.WithPDFRenderer(fakePDF),
		mcp.WithNowFunc(func() time.Time { return fixedNow }),
	)
	srv := mcp.NewServer(v, mcp.WithPacketMaterializer(mat))

	// Seed vault entities
	callToolJSON(t, srv, "resumai_save_organization", map[string]any{"id": "org-prior", "name": "Prior Co"})
	callToolJSON(t, srv, "resumai_save_role", map[string]any{"id": "role-prior", "org_id": "org-prior", "title": "Senior Go Engineer"})
	callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id":              "fact-wal",
		"role_id":         "role-prior",
		"headline":        "Cut P99 latency",
		"details":         "Cut P99 latency by 42% using SQLite WAL batching.",
		"epistemic_state": "verified",
	})
	callToolJSON(t, srv, "resumai_save_archetype", map[string]any{"id": "arch-go", "name": "Go Systems"})

	// Progress application through ReadyToRender
	res, _ := callToolJSON(t, srv, "resumai_save_triage", map[string]any{
		"application_id": "app-acme",
		"dossier": map[string]any{
			"role_title":  "Staff Go Engineer",
			"company":     "Acme Corp",
			"raw_jd":      "Looking for a Staff Go Engineer experienced in SQLite WAL tuning.",
			"fit_summary": "Strong Go and SQLite kernel alignment.",
			"jd_claims": []map[string]any{
				{"summary": "SQLite WAL", "verbatim_jd_quote": "SQLite WAL tuning."},
			},
			"external_claims": []map[string]any{
				{"claim": "On-call rotation cadence", "unknown_ask_recruiter": true},
			},
		},
	})
	if res.IsError {
		t.Fatalf("save_triage failed: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_save_draft", map[string]any{
		"application_id": "app-acme",
		"draft": map[string]any{
			"archetype_id": "arch-go",
			"headline":     "Staff Go Systems Engineer",
			"summary":      "Builds pure-Go storage kernels.",
			"experience": []map[string]any{
				{
					"role_title":   "Senior Go Engineer",
					"organization": "Prior Co",
					"duration":     "2022 - 2026",
					"highlights": []map[string]any{
						{"id": "b-1", "text": "Cut P99 latency by 42% using SQLite WAL batching.", "fact_ids": []string{"fact-wal"}},
					},
				},
			},
			"page_budget": 1,
		},
	})
	if res.IsError {
		t.Fatalf("save_draft failed: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_record_audits", map[string]any{
		"application_id": "app-acme",
		"fact_check": map[string]any{
			"critic_agent": domain.CriticFactChecker,
			"summary":      "All claims verified.",
			"passed":       true,
		},
		"alignment_check": map[string]any{
			"critic_agent":    domain.CriticAlignmentChecker,
			"summary":         "Strong fit.",
			"matched_signals": []string{"Go", "SQLite"},
			"missing_signals": []string{"Kubernetes"},
			"recommendations": []string{"Prepare SQLite WAL deep-dive example."},
			"passed":          true,
		},
	})
	if res.IsError {
		t.Fatalf("record_audits failed: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_promote_ready", map[string]any{"application_id": "app-acme"})
	if res.IsError {
		t.Fatalf("promote_ready failed: %s", res.Content[0].Text)
	}

	// Render bundle
	res, renderOut := callToolJSON(t, srv, "resumai_render_bundle", map[string]any{
		"application_id": "app-acme",
		"theme":          "modern",
	})
	if res.IsError {
		t.Fatalf("render_bundle failed: %s", res.Content[0].Text)
	}
	if !pdfCalledWithPII {
		t.Fatalf("expected PDF renderer to receive local PIIHeader")
	}

	// Verify zero PII in MCP tool response
	assertNoSentinelPII(t, res.Content[0].Text, sentinelPII)

	persistedBeforeSubmit, err := v.GetApplicationContext(context.Background(), "app-acme")
	if err != nil {
		t.Fatalf("GetApplicationContext: %v", err)
	}
	exportMap := renderOut["export"].(map[string]any)
	packetDir := exportMap["packet_dir"].(string)
	expectedSuffix := filepath.Join(baseDir, persistedBeforeSubmit.CreatedAt.UTC().Format("2006-01-02")+"-acme-corp-staff-go-engineer")
	if packetDir != expectedSuffix {
		t.Errorf("expected packet_dir %q, got %q", expectedSuffix, packetDir)
	}

	// Verify HTML contains PII
	htmlBytes, err := os.ReadFile(filepath.Join(packetDir, "resume.html"))
	if err != nil {
		t.Fatalf("read resume.html: %v", err)
	}
	if !strings.Contains(string(htmlBytes), sentinelPII.FullName) || !strings.Contains(string(htmlBytes), sentinelPII.Email) {
		t.Errorf("expected resume.html to include injected PII")
	}

	// Verify DOCX contains PII in word/document.xml
	docxBytes, err := os.ReadFile(filepath.Join(packetDir, "resume.docx"))
	if err != nil {
		t.Fatalf("read resume.docx: %v", err)
	}
	docXML := readZipEntry(t, docxBytes, "word/document.xml")
	if !strings.Contains(docXML, sentinelPII.FullName) || !strings.Contains(docXML, sentinelPII.Email) {
		t.Errorf("expected resume.docx word/document.xml to include injected PII")
	}

	// Verify PDF exists
	pdfBytes, err := os.ReadFile(filepath.Join(packetDir, "resume.pdf"))
	if err != nil || !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("expected valid resume.pdf, err=%v", err)
	}

	// Verify audit.md exists and has zero PII
	auditBytes, err := os.ReadFile(filepath.Join(packetDir, "audit.md"))
	if err != nil {
		t.Fatalf("read audit.md: %v", err)
	}
	if !strings.Contains(string(auditBytes), "fact-wal") || !strings.Contains(string(auditBytes), "All claims verified.") {
		t.Errorf("unexpected audit.md contents: %s", string(auditBytes))
	}
	assertNoSentinelPII(t, string(auditBytes), sentinelPII)

	// Re-rendering while Exported should succeed
	res, _ = callToolJSON(t, srv, "resumai_render_bundle", map[string]any{"application_id": "app-acme"})
	if res.IsError {
		t.Fatalf("re-render from Exported failed: %s", res.Content[0].Text)
	}

	// Finalize submission
	res, submitOut := callToolJSON(t, srv, "resumai_mark_submitted", map[string]any{
		"application_id": "app-acme",
		"notes":          "Submitted via greenhouse portal.",
	})
	if res.IsError {
		t.Fatalf("mark_submitted failed: %s", res.Content[0].Text)
	}
	assertNoSentinelPII(t, res.Content[0].Text, sentinelPII)

	appOut := submitOut["application"].(map[string]any)
	sha := appOut["bundle_sha256"].(string)
	if len(sha) != 64 {
		t.Fatalf("expected 64-char SHA-256 hash, got %q", sha)
	}

	briefPath := filepath.Join(packetDir, "interview_brief.md")
	briefBytes, err := os.ReadFile(briefPath)
	if err != nil {
		t.Fatalf("read interview_brief.md: %v", err)
	}
	briefText := string(briefBytes)
	if !strings.Contains(briefText, sha) ||
		!strings.Contains(briefText, "On-call rotation cadence") ||
		!strings.Contains(briefText, domain.UnknownAskRecruiterToken) ||
		!strings.Contains(briefText, "Submitted via greenhouse portal.") {
		t.Errorf("interview_brief.md missing expected fields:\n%s", briefText)
	}
	assertNoSentinelPII(t, briefText, sentinelPII)

	// Verify all packet files are locked read-only (no write bits set)
	for _, fname := range []string{"resume.html", "resume.docx", "resume.pdf", "audit.md", "interview_brief.md"} {
		info, err := os.Stat(filepath.Join(packetDir, fname))
		if err != nil {
			t.Fatalf("stat %s: %v", fname, err)
		}
		if info.Mode().Perm()&0222 != 0 {
			t.Errorf("expected %s to be locked read-only, got mode %v", fname, info.Mode().Perm())
		}
	}

	// Verify subsequent render_bundle fails with ErrAlreadySubmitted
	res, _ = callToolJSON(t, srv, "resumai_render_bundle", map[string]any{"application_id": "app-acme"})
	if !res.IsError || !strings.Contains(res.Content[0].Text, "already submitted") {
		t.Fatalf("expected ErrAlreadySubmitted after submission, got isError=%v text=%s", res.IsError, res.Content[0].Text)
	}
}

func TestLocalMaterializer_MissingPIIAndPageBudgetOverflow(t *testing.T) {
	baseDir := t.TempDir()
	v, _ := newTestVaultAndServer(t)

	// Seed ready application without setting PII
	ctx := context.Background()
	_ = v.SaveOrganization(ctx, &domain.Organization{ID: "org-1", Name: "Prior Co"})
	_ = v.SaveRole(ctx, &domain.Role{ID: "role-1", OrgID: "org-1", Title: "Engineer"})
	_ = v.SaveCareerFact(ctx, &domain.CareerFact{
		ID:             "fact-1",
		RoleID:         "role-1",
		Headline:       "Built API",
		Details:        "Built API serving 10k RPS.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	})
	_ = v.SaveArchetype(ctx, &domain.Archetype{ID: "arch-1", Name: "Systems"})

	app, err := domain.NewTriagedApplication("app-overflow", domain.OpportunityDossier{
		RoleTitle: "Staff Go",
		Company:   "Acme",
		RawJD:     "Build Go APIs.",
	})
	if err != nil {
		t.Fatalf("NewTriagedApplication: %v", err)
	}
	if err := app.SaveDraft(domain.ApplicationDraft{
		ArchetypeID: "arch-1",
		Experience: []domain.DraftExperience{
			{
				RoleTitle:    "Engineer",
				Organization: "Prior Co",
				Highlights:   []domain.BulletNode{{ID: "b-1", Text: "Built API serving 10k RPS.", FactIDs: []string{"fact-1"}}},
			},
		},
		PageBudget: 1,
	}, v); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	if err := app.RecordAudits(domain.AuditBundle{
		FactCheck:      &domain.FactCheckReport{Summary: "ok", Passed: true},
		AlignmentCheck: &domain.AlignmentCheckReport{Summary: "ok", Passed: true},
	}); err != nil {
		t.Fatalf("RecordAudits: %v", err)
	}
	if err := app.PromoteReady(v); err != nil {
		t.Fatalf("PromoteReady: %v", err)
	}

	overflowPDF := func(_ context.Context, _ *render.ResumeDocument, _ *domain.PIIHeader, _ *render.ThemeManifest, _ int) ([]byte, *render.LayoutTelemetry, error) {
		return []byte("%PDF-1.4\n"), &render.LayoutTelemetry{
			PageCount:  2,
			OverflowPX: 120.5,
		}, nil
	}

	mat := mcp.NewLocalMaterializer(v, mcp.WithBaseOutputDir(baseDir), mcp.WithPDFRenderer(overflowPDF))

	// 1. Missing PII must fail with ErrMissingPIIHeader
	if _, err := mat.RenderAndMarkExported(ctx, mcp.RenderBundleRequest{Application: app}); !errors.Is(err, domain.ErrMissingPIIHeader) {
		t.Fatalf("expected ErrMissingPIIHeader, got %v", err)
	}

	// 2. Populate PII, trigger page budget overflow -> ErrPageBudgetOverflow
	pii := &domain.PIIHeader{FullName: "Jane Sentinel", Email: "jane@sentinel.invalid"}
	if err := v.SetPII(ctx, pii); err != nil {
		t.Fatalf("SetPII: %v", err)
	}
	if _, err := mat.RenderAndMarkExported(ctx, mcp.RenderBundleRequest{Application: app}); !errors.Is(err, domain.ErrPageBudgetOverflow) {
		t.Fatalf("expected ErrPageBudgetOverflow, got %v", err)
	}
	if app.State != domain.StateReadyToRender {
		t.Fatalf("expected state to remain ReadyToRender after overflow, got %q", app.State)
	}

	// 3. Even if PDF renderer returns an error embedding PII, LocalMaterializer must scrub it.
	leakingPDF := func(_ context.Context, _ *render.ResumeDocument, p *domain.PIIHeader, _ *render.ThemeManifest, _ int) ([]byte, *render.LayoutTelemetry, error) {
		return nil, nil, errors.New("chromium failed for " + p.FullName + " (" + p.Email + ")")
	}
	leakingMat := mcp.NewLocalMaterializer(v, mcp.WithBaseOutputDir(baseDir), mcp.WithPDFRenderer(leakingPDF))
	_, err = leakingMat.RenderAndMarkExported(ctx, mcp.RenderBundleRequest{Application: app})
	if err == nil {
		t.Fatalf("expected error from leaking PDF renderer")
	}
	assertNoSentinelPII(t, err.Error(), pii)
}

func TestLocalMaterializer_DirectoryJailAndPartialLockRecovery(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				_ = os.Chmod(path, 0644)
			}
			return nil
		})
	})

	v, _ := newTestVaultAndServer(t)
	ctx := context.Background()
	pii := &domain.PIIHeader{FullName: "Jane Jailed", Email: "jane@jailed.invalid"}
	if err := v.SetPII(ctx, pii); err != nil {
		t.Fatalf("SetPII: %v", err)
	}
	_ = v.SaveOrganization(ctx, &domain.Organization{ID: "org-1", Name: "Prior Co"})
	_ = v.SaveRole(ctx, &domain.Role{ID: "role-1", OrgID: "org-1", Title: "Engineer"})
	_ = v.SaveCareerFact(ctx, &domain.CareerFact{
		ID:             "fact-1",
		RoleID:         "role-1",
		Headline:       "Built API",
		Details:        "Built API serving 10k RPS.",
		EpistemicState: domain.EpistemicVerified,
		Verified:       true,
	})
	_ = v.SaveArchetype(ctx, &domain.Archetype{ID: "arch-1", Name: "Systems"})

	newReadyApp := func(id string) *domain.JobApplication {
		app, err := domain.NewTriagedApplication(id, domain.OpportunityDossier{
			RoleTitle: "Staff Go",
			Company:   "Acme",
			RawJD:     "Build Go APIs.",
		})
		if err != nil {
			t.Fatalf("NewTriagedApplication: %v", err)
		}
		if err := app.SaveDraft(domain.ApplicationDraft{
			ArchetypeID: "arch-1",
			Experience: []domain.DraftExperience{
				{
					RoleTitle:    "Engineer",
					Organization: "Prior Co",
					Highlights:   []domain.BulletNode{{ID: "b-1", Text: "Built API serving 10k RPS.", FactIDs: []string{"fact-1"}}},
				},
			},
			PageBudget: 1,
		}, v); err != nil {
			t.Fatalf("SaveDraft: %v", err)
		}
		if err := app.RecordAudits(domain.AuditBundle{
			FactCheck:      &domain.FactCheckReport{Summary: "ok", Passed: true},
			AlignmentCheck: &domain.AlignmentCheckReport{Summary: "ok", Passed: true},
		}); err != nil {
			t.Fatalf("RecordAudits: %v", err)
		}
		if err := app.PromoteReady(v); err != nil {
			t.Fatalf("PromoteReady: %v", err)
		}
		return app
	}

	okPDF := func(_ context.Context, _ *render.ResumeDocument, _ *domain.PIIHeader, _ *render.ThemeManifest, _ int) ([]byte, *render.LayoutTelemetry, error) {
		return []byte("%PDF-1.4\n"), &render.LayoutTelemetry{PageCount: 1, LastPageFillRatio: 0.8}, nil
	}
	mat := mcp.NewLocalMaterializer(v, mcp.WithBaseOutputDir(baseDir), mcp.WithPDFRenderer(okPDF))

	// 1. Escaping output_dir (relative traversal & absolute path outside baseDir) must be rejected.
	for _, badOut := range []string{"../../../arbitrary/dir", outsideDir} {
		app := newReadyApp("app-escape")
		if _, err := mat.RenderAndMarkExported(ctx, mcp.RenderBundleRequest{
			Application: app,
			OutputDir:   badOut,
		}); err == nil {
			t.Fatalf("expected RenderAndMarkExported to reject unjailed output_dir %q", badOut)
		}
	}

	// 2. Benign relative subdirectory inside baseDir succeeds and remains jailed.
	app := newReadyApp("app-jailed")
	res, err := mat.RenderAndMarkExported(ctx, mcp.RenderBundleRequest{
		Application: app,
		OutputDir:   "batch-2026",
	})
	if err != nil {
		t.Fatalf("expected jailed relative subdir to succeed, got: %v", err)
	}
	rel, rerr := filepath.Rel(baseDir, res.Export.PacketDir)
	if rerr != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("packetDir %q escaped baseDir %q", res.Export.PacketDir, baseDir)
	}

	// 3. FinalizeSubmission must reject ExportRecord paths escaping packetDir or baseDir.
	tampered := *app
	tamperedExport := *app.Export
	tamperedExport.HTMLPath = filepath.Join(outsideDir, "evil.html")
	tampered.Export = &tamperedExport
	if _, err := mat.FinalizeSubmission(ctx, mcp.SubmissionRequest{Application: &tampered}); err == nil {
		t.Fatalf("expected FinalizeSubmission to reject HTMLPath escaping packetDir")
	}

	tamperedDir := *app
	tamperedDirExport := *app.Export
	tamperedDirExport.PacketDir = outsideDir
	tamperedDir.Export = &tamperedDirExport
	if _, err := mat.FinalizeSubmission(ctx, mcp.SubmissionRequest{Application: &tamperedDir}); err == nil {
		t.Fatalf("expected FinalizeSubmission to reject PacketDir escaping baseDir")
	}

	// 4. Simulate partial 0444 read-only lock left on disk while app remains StateExported
	// (e.g., mid-flight FinalizeSubmission or SaveApplication failure). Both FinalizeSubmission
	// retry and RenderAndMarkExported re-render must recover without permission-denied wedge.
	staleBrief := filepath.Join(app.Export.PacketDir, "interview_brief.md")
	if err := os.WriteFile(staleBrief, []byte("stale brief"), 0444); err != nil {
		t.Fatalf("write stale brief: %v", err)
	}
	if err := os.Chmod(app.Export.HTMLPath, 0444); err != nil {
		t.Fatalf("chmod html 0444: %v", err)
	}

	// Retry FinalizeSubmission while HTMLPath and interview_brief.md are already 0444
	if _, err := mat.FinalizeSubmission(ctx, mcp.SubmissionRequest{
		Application: app,
		Notes:       "retry after failure",
	}); err != nil {
		t.Fatalf("FinalizeSubmission retry on 0444 artifacts failed: %v", err)
	}

	// Re-render while app is still StateExported and all 5 artifacts (including interview_brief.md) are 0444
	if _, err := mat.RenderAndMarkExported(ctx, mcp.RenderBundleRequest{Application: app}); err != nil {
		t.Fatalf("RenderAndMarkExported re-render on 0444 artifacts failed: %v", err)
	}
	if _, statErr := os.Stat(staleBrief); !os.IsNotExist(statErr) {
		t.Fatalf("expected stale unsubmitted interview_brief.md to be cleaned on re-render, stat err=%v", statErr)
	}
}

func assertNoSentinelPII(t *testing.T, payload string, pii *domain.PIIHeader) {
	t.Helper()
	fields := []string{
		pii.FullName,
		pii.Email,
		pii.Phone,
		pii.Location,
		pii.LinkedInURL,
		pii.GitHubURL,
		pii.PortfolioURL,
	}
	for _, f := range fields {
		if f != "" && strings.Contains(payload, f) {
			t.Fatalf("air-gap violation: payload contains sentinel PII %q", f)
		}
	}
}

func readZipEntry(t *testing.T, zipData []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("open docx zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return string(b)
		}
	}
	t.Fatalf("zip entry %q not found", name)
	return ""
}
