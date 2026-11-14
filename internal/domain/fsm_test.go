package domain

import (
	"errors"
	"strings"
	"testing"
)

type stubDraftResolver struct {
	archetypes map[string]*Archetype
	facts      map[string]*CareerFact
}

func (s *stubDraftResolver) GetArchetypeByID(id string) (*Archetype, error) {
	a, ok := s.archetypes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}

func (s *stubDraftResolver) GetCareerFactByID(id string) (*CareerFact, error) {
	f, ok := s.facts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return f, nil
}

func newTestResolver() *stubDraftResolver {
	return &stubDraftResolver{
		archetypes: map[string]*Archetype{
			"arch-systems": {ID: "arch-systems", Name: "Systems Engineer"},
		},
		facts: map[string]*CareerFact{
			"fact-verified": {
				ID:             "fact-verified",
				RoleID:         "role-1",
				Headline:       "Reduced P99 latency",
				Details:        "Cut P99 tail latency by 42% via WAL batching.",
				EpistemicState: EpistemicVerified,
				Verified:       true,
			},
			"fact-metric-pending": {
				ID:             "fact-metric-pending",
				RoleID:         "role-1",
				Headline:       "Improved CI throughput",
				Details:        "Parallelized test suite.",
				EpistemicState: EpistemicMetricPending,
				Verified:       false,
			},
			"fact-raw": {
				ID:             "fact-raw",
				RoleID:         "role-1",
				Headline:       "Migrated auth",
				Details:        "Moved session tokens.",
				EpistemicState: EpistemicRawNote,
				Verified:       false,
			},
			"fact-archived": {
				ID:             "fact-archived",
				RoleID:         "role-1",
				Headline:       "Old fact",
				Details:        "Deprecated.",
				EpistemicState: EpistemicArchived,
				Verified:       false,
			},
		},
	}
}

func validTestDossier() OpportunityDossier {
	return OpportunityDossier{
		RoleTitle: "Staff Go Engineer",
		Company:   "Acme Corp",
		RawJD:     "Seeking a Staff Go Engineer to architect pure-Go SQLite kernels.",
		JDClaims: []JDClaim{
			{Summary: "Pure-Go SQLite", VerbatimJDQuote: "architect pure-Go SQLite kernels."},
		},
		ExternalClaims: []ExternalClaim{
			{Claim: "Remote-first engineering org", SourceURL: "https://acme.example.com/careers"},
		},
	}
}

func validTestDraft(factIDs ...string) ApplicationDraft {
	if len(factIDs) == 0 {
		factIDs = []string{"fact-verified"}
	}
	return ApplicationDraft{
		ArchetypeID: "arch-systems",
		Headline:    "Staff Go Systems Engineer",
		Summary:     "Builds deterministic pure-Go storage engines.",
		Experience: []DraftExperience{
			{
				RoleTitle:    "Senior Engineer",
				Organization: "Prior Co",
				Highlights: []BulletNode{
					{
						ID:      "b-1",
						Text:    "Cut P99 tail latency by 42% via WAL batching.",
						FactIDs: factIDs,
					},
				},
			},
		},
		PageBudget: 1,
	}
}

func validTestAudits() AuditBundle {
	return AuditBundle{
		FactCheck: &FactCheckReport{
			CriticAgent: CriticFactChecker,
			Summary:     "All bullets match cited vault facts.",
			Passed:      true,
		},
		AlignmentCheck: &AlignmentCheckReport{
			CriticAgent:    CriticAlignmentChecker,
			Summary:        "Strong alignment with JD SQLite kernel signals.",
			MatchedSignals: []string{"pure-Go SQLite"},
			Passed:         true,
		},
	}
}

func TestJobApplicationFSM_HappyPathToSubmittedAndImmutability(t *testing.T) {
	resolver := newTestResolver()
	app, err := NewTriagedApplication("app-1", validTestDossier())
	if err != nil {
		t.Fatalf("NewTriagedApplication failed: %v", err)
	}
	if app.State != StateTriaged {
		t.Fatalf("expected StateTriaged, got %q", app.State)
	}

	if err := app.SaveDraft(validTestDraft("fact-verified"), resolver); err != nil {
		t.Fatalf("SaveDraft failed: %v", err)
	}
	if app.State != StateDrafted {
		t.Fatalf("expected StateDrafted, got %q", app.State)
	}

	if err := app.RecordAudits(validTestAudits()); err != nil {
		t.Fatalf("RecordAudits failed: %v", err)
	}
	if app.State != StateAudited {
		t.Fatalf("expected StateAudited, got %q", app.State)
	}

	if err := app.PromoteReady(resolver); err != nil {
		t.Fatalf("PromoteReady failed: %v", err)
	}
	if app.State != StateReadyToRender {
		t.Fatalf("expected StateReadyToRender, got %q", app.State)
	}

	pii := &PIIHeader{FullName: "Jane Candidate", Email: "jane@example.com"}
	export := ExportRecord{
		PacketDir: "/tmp/applications/2026-11-13-acme-staff-go-engineer",
		HTMLPath:  "/tmp/applications/2026-11-13-acme-staff-go-engineer/resume.html",
		DocxPath:  "/tmp/applications/2026-11-13-acme-staff-go-engineer/resume.docx",
		PDFPath:   "/tmp/applications/2026-11-13-acme-staff-go-engineer/resume.pdf",
		PageCount: 1,
		MaxPages:  1,
	}
	if err := app.MarkExported(pii, export); err != nil {
		t.Fatalf("MarkExported failed: %v", err)
	}
	if app.State != StateExported {
		t.Fatalf("expected StateExported, got %q", app.State)
	}

	bundleHash := strings.Repeat("a", 64)
	if err := app.MarkSubmitted(bundleHash, export.PacketDir+"/interview_brief.md"); err != nil {
		t.Fatalf("MarkSubmitted failed: %v", err)
	}
	if app.State != StateSubmitted {
		t.Fatalf("expected StateSubmitted, got %q", app.State)
	}
	if app.SubmittedAt == nil || app.BundleSHA256 != bundleHash {
		t.Fatalf("expected submission metadata populated, got sha=%q submitted_at=%v", app.BundleSHA256, app.SubmittedAt)
	}

	// Verify immutable lock in StateSubmitted across all transition methods.
	immutChecks := []struct {
		name string
		fn   func() error
	}{
		{"ApplyTriage", func() error { return app.ApplyTriage(validTestDossier()) }},
		{"PassRole", func() error { return app.PassRole("too late") }},
		{"SaveDraft", func() error { return app.SaveDraft(validTestDraft(), resolver) }},
		{"RecordAudits", func() error { return app.RecordAudits(validTestAudits()) }},
		{"PromoteReady", func() error { return app.PromoteReady(resolver) }},
		{"MarkExported", func() error { return app.MarkExported(pii, export) }},
		{"MarkSubmitted", func() error { return app.MarkSubmitted(bundleHash, "brief.md") }},
	}
	for _, chk := range immutChecks {
		err := chk.fn()
		if !errors.Is(err, ErrAlreadySubmitted) {
			t.Errorf("%s on Submitted app: expected ErrAlreadySubmitted, got %v", chk.name, err)
		}
	}
}

func TestJobApplicationFSM_DeclinePath(t *testing.T) {
	app, err := NewTriagedApplication("app-pass", validTestDossier())
	if err != nil {
		t.Fatalf("NewTriagedApplication failed: %v", err)
	}

	if err := app.PassRole("   "); !errors.Is(err, ErrMissingRationale) {
		t.Fatalf("expected ErrMissingRationale, got %v", err)
	}
	if app.State != StateTriaged {
		t.Fatalf("expected state to remain Triaged, got %q", app.State)
	}

	if err := app.PassRole("Compensation band below target"); err != nil {
		t.Fatalf("PassRole failed: %v", err)
	}
	if app.State != StateDeclinedByCandidate {
		t.Fatalf("expected StateDeclinedByCandidate, got %q", app.State)
	}
	if !app.State.IsTerminal() {
		t.Fatalf("expected DeclinedByCandidate to be terminal")
	}

	if err := app.SaveDraft(validTestDraft(), newTestResolver()); !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition when drafting declined app, got %v", err)
	}
}

func TestJobApplicationFSM_DraftGuards(t *testing.T) {
	resolver := newTestResolver()

	cases := []struct {
		name     string
		draft    ApplicationDraft
		resolver DraftResolver
	}{
		{
			name:     "nil resolver",
			draft:    validTestDraft(),
			resolver: nil,
		},
		{
			name: "missing archetype_id",
			draft: func() ApplicationDraft {
				d := validTestDraft()
				d.ArchetypeID = ""
				return d
			}(),
			resolver: resolver,
		},
		{
			name: "unknown archetype_id",
			draft: func() ApplicationDraft {
				d := validTestDraft()
				d.ArchetypeID = "arch-does-not-exist"
				return d
			}(),
			resolver: resolver,
		},
		{
			name: "zero bullets",
			draft: ApplicationDraft{
				ArchetypeID: "arch-systems",
			},
			resolver: resolver,
		},
		{
			name: "bullet with zero fact_ids",
			draft: ApplicationDraft{
				ArchetypeID: "arch-systems",
				Experience: []DraftExperience{
					{
						RoleTitle:    "Dev",
						Organization: "Co",
						Highlights:   []BulletNode{{ID: "b1", Text: "Did stuff"}},
					},
				},
			},
			resolver: resolver,
		},
		{
			name:     "bullet citing nonexistent fact",
			draft:    validTestDraft("fact-missing"),
			resolver: resolver,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, _ := NewTriagedApplication("app-draft", validTestDossier())
			err := app.SaveDraft(tc.draft, tc.resolver)
			if !errors.Is(err, ErrUngroundedBullet) {
				t.Fatalf("expected ErrUngroundedBullet, got %v", err)
			}
		})
	}
}

func TestJobApplicationFSM_AuditAndPromoteGuards(t *testing.T) {
	resolver := newTestResolver()

	t.Run("incomplete audits rejected", func(t *testing.T) {
		app, _ := NewTriagedApplication("app-audit", validTestDossier())
		_ = app.SaveDraft(validTestDraft(), resolver)

		badBundles := []AuditBundle{
			{FactCheck: nil, AlignmentCheck: validTestAudits().AlignmentCheck},
			{FactCheck: validTestAudits().FactCheck, AlignmentCheck: nil},
			{
				FactCheck:      &FactCheckReport{CriticAgent: "wrong-agent", Summary: "ok"},
				AlignmentCheck: validTestAudits().AlignmentCheck,
			},
			{
				FactCheck:      &FactCheckReport{CriticAgent: CriticFactChecker, Summary: ""},
				AlignmentCheck: validTestAudits().AlignmentCheck,
			},
			{
				FactCheck: &FactCheckReport{
					CriticAgent: CriticFactChecker,
					Summary:     "has bad severity",
					Discrepancies: []FactualDiscrepancy{
						{Severity: "critical", Finding: "invalid enum"},
					},
				},
				AlignmentCheck: validTestAudits().AlignmentCheck,
			},
		}

		for i, b := range badBundles {
			if err := app.RecordAudits(b); !errors.Is(err, ErrIncompleteAudit) {
				t.Errorf("case %d: expected ErrIncompleteAudit, got %v", i, err)
			}
		}
	})

	t.Run("high discrepancy blocks promotion", func(t *testing.T) {
		app, _ := NewTriagedApplication("app-high", validTestDossier())
		_ = app.SaveDraft(validTestDraft(), resolver)

		audits := validTestAudits()
		audits.FactCheck.Discrepancies = []FactualDiscrepancy{
			{BulletID: "b-1", FactID: "fact-verified", Severity: SeverityHigh, Finding: "Metric inflated from 42% to 90%"},
		}
		if err := app.RecordAudits(audits); err != nil {
			t.Fatalf("RecordAudits failed: %v", err)
		}
		if err := app.PromoteReady(resolver); !errors.Is(err, ErrUnverifiedFactOrPlaceholder) {
			t.Fatalf("expected ErrUnverifiedFactOrPlaceholder on High discrepancy, got %v", err)
		}

		// Redrafting from Audited clears stale audits and returns to Drafted.
		if err := app.SaveDraft(validTestDraft(), resolver); err != nil {
			t.Fatalf("re-SaveDraft from Audited failed: %v", err)
		}
		if app.State != StateDrafted || app.Audits != nil {
			t.Fatalf("expected StateDrafted with cleared audits, got state=%q audits=%v", app.State, app.Audits)
		}
	})

	t.Run("metric placeholder in bullet, summary, or headline blocks promotion", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*ApplicationDraft)
		}{
			{"bullet", func(d *ApplicationDraft) {
				d.Experience[0].Highlights[0].Text = "Cut latency by [METRIC: %] via batching."
			}},
			{"bullet lowercase", func(d *ApplicationDraft) { d.Experience[0].Highlights[0].Text = "Cut latency by [metric: pct]." }},
			{"summary", func(d *ApplicationDraft) { d.Summary = "Over [METRIC: years] experience." }},
			{"headline", func(d *ApplicationDraft) { d.Headline = "[METRIC: title]" }},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				app, _ := NewTriagedApplication("app-metric", validTestDossier())
				draft := validTestDraft("fact-verified")
				tc.mutate(&draft)
				if err := app.SaveDraft(draft, resolver); err != nil {
					t.Fatalf("SaveDraft should allow draft-stage metric placeholder, got %v", err)
				}
				if err := app.RecordAudits(validTestAudits()); err != nil {
					t.Fatalf("RecordAudits failed: %v", err)
				}
				if err := app.PromoteReady(resolver); !errors.Is(err, ErrUnverifiedFactOrPlaceholder) {
					t.Fatalf("expected ErrUnverifiedFactOrPlaceholder, got %v", err)
				}
			})
		}
	})

	t.Run("unverified, raw_note, or archived cited fact blocks promotion", func(t *testing.T) {
		for _, fid := range []string{"fact-metric-pending", "fact-raw", "fact-archived"} {
			app, _ := NewTriagedApplication("app-unverified", validTestDossier())
			if err := app.SaveDraft(validTestDraft(fid), resolver); err != nil {
				t.Fatalf("SaveDraft(%s) should succeed at Drafted stage, got %v", fid, err)
			}
			if err := app.RecordAudits(validTestAudits()); err != nil {
				t.Fatalf("RecordAudits failed: %v", err)
			}
			if err := app.PromoteReady(resolver); !errors.Is(err, ErrUnverifiedFactOrPlaceholder) {
				t.Fatalf("PromoteReady(%s): expected ErrUnverifiedFactOrPlaceholder, got %v", fid, err)
			}
		}
	})

	t.Run("low/medium discrepancy does not block promotion when facts verified", func(t *testing.T) {
		app, _ := NewTriagedApplication("app-low-med", validTestDossier())
		_ = app.SaveDraft(validTestDraft("fact-verified"), resolver)
		audits := validTestAudits()
		audits.FactCheck.Discrepancies = []FactualDiscrepancy{
			{BulletID: "b-1", Severity: SeverityLow, Finding: "Minor phrasing tweak suggested"},
			{BulletID: "b-1", Severity: SeverityMedium, Finding: "Clarify scope"},
		}
		_ = app.RecordAudits(audits)
		if err := app.PromoteReady(resolver); err != nil {
			t.Fatalf("expected promotion to succeed with only low/medium discrepancies, got %v", err)
		}
	})
}

func TestJobApplicationFSM_ExportAndTransitionGuards(t *testing.T) {
	resolver := newTestResolver()
	app, _ := NewTriagedApplication("app-export", validTestDossier())

	// Cannot jump ahead in FSM.
	if err := app.RecordAudits(validTestAudits()); !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition for Triaged->Audited, got %v", err)
	}
	if err := app.PromoteReady(resolver); !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition for Triaged->ReadyToRender, got %v", err)
	}

	_ = app.SaveDraft(validTestDraft(), resolver)
	_ = app.RecordAudits(validTestAudits())
	_ = app.PromoteReady(resolver)

	validExport := ExportRecord{
		PacketDir: "/tmp/packet",
		HTMLPath:  "/tmp/packet/resume.html",
		DocxPath:  "/tmp/packet/resume.docx",
		PDFPath:   "/tmp/packet/resume.pdf",
		PageCount: 1,
		MaxPages:  1,
	}
	validPII := &PIIHeader{FullName: "Jane Candidate"}

	// Missing PII header.
	if err := app.MarkExported(nil, validExport); !errors.Is(err, ErrMissingPIIHeader) {
		t.Fatalf("expected ErrMissingPIIHeader, got %v", err)
	}
	if err := app.MarkExported(&PIIHeader{}, validExport); !errors.Is(err, ErrMissingPIIHeader) {
		t.Fatalf("expected ErrMissingPIIHeader for empty FullName, got %v", err)
	}

	// Page budget overflow.
	overBudget := validExport
	overBudget.PageCount = 2
	if err := app.MarkExported(validPII, overBudget); !errors.Is(err, ErrPageBudgetOverflow) {
		t.Fatalf("expected ErrPageBudgetOverflow, got %v", err)
	}

	// Missing artifact paths.
	missingPDF := validExport
	missingPDF.PDFPath = ""
	if err := app.MarkExported(validPII, missingPDF); !errors.Is(err, ErrMissingExportArtifact) {
		t.Fatalf("expected ErrMissingExportArtifact, got %v", err)
	}

	// Cannot submit before export.
	if err := app.MarkSubmitted(strings.Repeat("b", 64), "brief.md"); !errors.Is(err, ErrInvalidStateTransition) {
		t.Fatalf("expected ErrInvalidStateTransition for ReadyToRender->Submitted, got %v", err)
	}

	// Export succeeds.
	if err := app.MarkExported(validPII, validExport); err != nil {
		t.Fatalf("MarkExported failed: %v", err)
	}

	// Invalid hash or missing brief rejected on MarkSubmitted.
	if err := app.MarkSubmitted("not-a-sha256", "brief.md"); !errors.Is(err, ErrMissingSubmissionArtifact) {
		t.Fatalf("expected ErrMissingSubmissionArtifact for bad hash, got %v", err)
	}
	if err := app.MarkSubmitted(strings.Repeat("b", 64), "  "); !errors.Is(err, ErrMissingSubmissionArtifact) {
		t.Fatalf("expected ErrMissingSubmissionArtifact for empty brief path, got %v", err)
	}
}
