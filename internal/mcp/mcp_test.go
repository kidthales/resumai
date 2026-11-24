package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/mcp"
	"github.com/kidthales/resumai/internal/render"
	"github.com/kidthales/resumai/internal/vault"
)

type stubMaterializer struct {
	pii       *domain.PIIHeader
	pageCount int
	maxPages  int
	errRender error
	errSubmit error
}

func (m *stubMaterializer) RenderAndMarkExported(_ context.Context, req mcp.RenderBundleRequest) (*mcp.RenderBundleResult, error) {
	if m.errRender != nil {
		return nil, m.errRender
	}
	pages := m.pageCount
	if pages == 0 {
		pages = 1
	}
	maxP := m.maxPages
	if maxP == 0 {
		maxP = 1
	}
	rec := domain.ExportRecord{
		PacketDir: "/tmp/applications/2026-11-13-acme-staff-go",
		HTMLPath:  "/tmp/applications/2026-11-13-acme-staff-go/resume.html",
		DocxPath:  "/tmp/applications/2026-11-13-acme-staff-go/resume.docx",
		PDFPath:   "/tmp/applications/2026-11-13-acme-staff-go/resume.pdf",
		PageCount: pages,
		MaxPages:  maxP,
	}
	if err := req.Application.MarkExported(m.pii, rec); err != nil {
		return nil, err
	}
	return &mcp.RenderBundleResult{
		Export: rec,
		Telemetry: &render.LayoutTelemetry{
			PageCount:         pages,
			LastPageFillRatio: 0.85,
		},
	}, nil
}

func (m *stubMaterializer) FinalizeSubmission(_ context.Context, req mcp.SubmissionRequest) (*mcp.SubmissionResult, error) {
	if m.errSubmit != nil {
		return nil, m.errSubmit
	}
	return &mcp.SubmissionResult{
		BundleSHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		InterviewBriefPath: filepath.Join(req.Application.Export.PacketDir, "interview_brief.md"),
	}, nil
}

func newTestVaultAndServer(t *testing.T, opts ...mcp.ServerOption) (*vault.CareerVault, *mcp.Server) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "mcp_test.db")
	v, err := vault.OpenPath(dbPath)
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	t.Cleanup(func() {
		_ = v.Close()
	})
	return v, mcp.NewServer(v, opts...)
}

func callToolJSON(t *testing.T, srv *mcp.Server, name string, args any) (*mcp.CallToolResult, map[string]any) {
	t.Helper()
	var rawArgs json.RawMessage
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			t.Fatalf("marshal args for %s: %v", name, err)
		}
		rawArgs = b
	}
	res, err := srv.CallTool(context.Background(), name, rawArgs)
	if err != nil {
		t.Fatalf("CallTool(%s) unexpected protocol error: %v", name, err)
	}
	var decoded map[string]any
	if !res.IsError && len(res.Content) > 0 {
		if err := json.Unmarshal([]byte(res.Content[0].Text), &decoded); err != nil {
			t.Fatalf("unmarshal result JSON for %s: %v; raw=%s", name, err, res.Content[0].Text)
		}
	}
	return res, decoded
}

func TestServeStdioJSONRPC(t *testing.T) {
	_, srv := newTestVaultAndServer(t)

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"resumai_vault_status","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"resumai_nonexistent"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"unknown/method"}`,
		`{bad-json`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("ServeStdio failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	// 7 requests with IDs (notification produces 0 lines)
	if len(lines) != 7 {
		t.Fatalf("expected 7 JSON-RPC responses (notification omitted), got %d:\n%s", len(lines), out.String())
	}

	var r1 mcp.Response
	if err := json.Unmarshal([]byte(lines[0]), &r1); err != nil {
		t.Fatalf("unmarshal initialize response: %v", err)
	}
	if r1.Error != nil {
		t.Fatalf("initialize error: %+v", r1.Error)
	}

	var r3 struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &r3); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	expectedTools := []string{
		"resumai_vault_status",
		"resumai_save_organization",
		"resumai_save_role",
		"resumai_save_fact",
		"resumai_save_triage",
		"resumai_pass_role",
		"resumai_save_draft",
		"resumai_get_audit_bundle",
		"resumai_record_audits",
		"resumai_promote_ready",
		"resumai_render_bundle",
		"resumai_mark_submitted",
	}
	toolSet := make(map[string]bool)
	for _, tool := range r3.Result.Tools {
		toolSet[tool.Name] = true
		if tool.InputSchema == nil {
			t.Errorf("tool %s missing InputSchema", tool.Name)
		}
	}
	for _, reqName := range expectedTools {
		if !toolSet[reqName] {
			t.Errorf("expected tool %q to be registered in tools/list", reqName)
		}
	}

	var r5 mcp.Response
	if err := json.Unmarshal([]byte(lines[4]), &r5); err != nil {
		t.Fatalf("unmarshal unknown tool response: %v", err)
	}
	if r5.Error == nil || r5.Error.Code != -32602 {
		t.Errorf("expected -32602 for unknown tool, got %+v", r5.Error)
	}

	var r6 mcp.Response
	if err := json.Unmarshal([]byte(lines[5]), &r6); err != nil {
		t.Fatalf("unmarshal unknown method response: %v", err)
	}
	if r6.Error == nil || r6.Error.Code != -32601 {
		t.Errorf("expected -32601 for unknown method, got %+v", r6.Error)
	}

	var r7 mcp.Response
	if err := json.Unmarshal([]byte(lines[6]), &r7); err != nil {
		t.Fatalf("unmarshal parse error response: %v", err)
	}
	if r7.Error == nil || r7.Error.Code != -32700 {
		t.Errorf("expected -32700 for parse error, got %+v", r7.Error)
	}
}

func TestServeStdioOversizedFrameRejectedAndRecovers(t *testing.T) {
	_, srv := newTestVaultAndServer(t)

	oversized := bytes.Repeat([]byte("a"), (10<<20)+128)
	var in bytes.Buffer
	in.Write(oversized)
	in.WriteByte('\n')
	in.WriteString(`{"jsonrpc":"2.0","id":99,"method":"ping"}` + "\n")

	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), &in, &out); err != nil {
		t.Fatalf("ServeStdio failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses (oversized error + subsequent ping), got %d:\n%s", len(lines), out.String())
	}

	var rErr mcp.Response
	if err := json.Unmarshal([]byte(lines[0]), &rErr); err != nil {
		t.Fatalf("unmarshal oversized response: %v", err)
	}
	if rErr.Error == nil || rErr.Error.Code != -32600 {
		t.Fatalf("expected -32600 for oversized frame, got %+v", rErr.Error)
	}

	var rPing mcp.Response
	if err := json.Unmarshal([]byte(lines[1]), &rPing); err != nil {
		t.Fatalf("unmarshal ping response: %v", err)
	}
	if rPing.Error != nil {
		t.Fatalf("expected ping to succeed after oversized frame, got error %+v", rPing.Error)
	}
}

func TestVaultCurationTools(t *testing.T) {
	v, srv := newTestVaultAndServer(t)

	// Set PII locally to confirm vault status reports pii_configured=true without returning PII values.
	if err := v.SetPII(context.Background(), &domain.PIIHeader{
		FullName: "Secret Candidate",
		Email:    "secret@example.com",
	}); err != nil {
		t.Fatalf("SetPII: %v", err)
	}

	res, _ := callToolJSON(t, srv, "resumai_save_organization", map[string]any{
		"id":       "org-acme",
		"name":     "Acme Systems",
		"location": "Remote",
	})
	if res.IsError {
		t.Fatalf("save_organization error: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_save_role", map[string]any{
		"id":         "role-staff",
		"org_id":     "org-acme",
		"title":      "Staff Go Engineer",
		"duration":   "2022 - Present",
		"tech_stack": []string{"Go", "SQLite"},
	})
	if res.IsError {
		t.Fatalf("save_role error: %s", res.Content[0].Text)
	}

	// Reject verified fact with [METRIC: placeholder
	res, _ = callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id":              "fact-bad",
		"role_id":         "role-staff",
		"headline":        "Latency reduction",
		"details":         "Reduced P99 latency by [METRIC: percentage].",
		"epistemic_state": "verified",
	})
	if !res.IsError {
		t.Fatalf("expected error marking fact with [METRIC: as verified")
	}

	res, _ = callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id":              "fact-good",
		"role_id":         "role-staff",
		"headline":        "Latency reduction",
		"details":         "Reduced P99 latency by 42% via WAL batching.",
		"epistemic_state": "verified",
		"tech_tags":       []string{"Go", "SQLite"},
	})
	if res.IsError {
		t.Fatalf("save_fact error: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_save_archetype", map[string]any{
		"id":               "arch-systems",
		"name":             "Systems Engineer",
		"profile_overview": "Builds reliable Go kernels.",
	})
	if res.IsError {
		t.Fatalf("save_archetype error: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_save_education", map[string]any{
		"id":          "edu-bs",
		"institution": "State University",
		"degree":      "B.S. Computer Science",
	})
	if res.IsError {
		t.Fatalf("save_education error: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_save_logistics", map[string]any{
		"target_title":       "Principal Go Engineer",
		"work_authorization": "Citizen",
		"remote_preference":  "Remote",
	})
	if res.IsError {
		t.Fatalf("save_logistics error: %s", res.Content[0].Text)
	}

	res, status := callToolJSON(t, srv, "resumai_vault_status", map[string]any{})
	if res.IsError {
		t.Fatalf("vault_status error: %s", res.Content[0].Text)
	}
	if status["pii_configured"] != true || status["logistics_configured"] != true {
		t.Errorf("expected pii_configured=true and logistics_configured=true, got %v", status)
	}
	if strings.Contains(res.Content[0].Text, "Secret Candidate") || strings.Contains(res.Content[0].Text, "secret@example.com") {
		t.Fatalf("vault_status leaked PII!")
	}
}

func TestApplicationLifecycleEndToEnd(t *testing.T) {
	mat := &stubMaterializer{
		pii: &domain.PIIHeader{FullName: "Jane Candidate", Email: "jane@example.com"},
	}
	v, srv := newTestVaultAndServer(t, mcp.WithPacketMaterializer(mat))

	// Seed vault entities
	callToolJSON(t, srv, "resumai_save_organization", map[string]any{"id": "org-1", "name": "Prior Co"})
	callToolJSON(t, srv, "resumai_save_role", map[string]any{"id": "role-1", "org_id": "org-1", "title": "Senior Engineer"})
	callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id":              "fact-1",
		"role_id":         "role-1",
		"headline":        "Cut P99",
		"details":         "Cut P99 latency by 42% via WAL batching.",
		"epistemic_state": "verified",
	})
	callToolJSON(t, srv, "resumai_save_archetype", map[string]any{"id": "arch-1", "name": "Systems"})

	// 1. Triage failure: non-verbatim quote
	res, _ := callToolJSON(t, srv, "resumai_save_triage", map[string]any{
		"application_id": "app-bad",
		"dossier": map[string]any{
			"role_title": "Staff Go",
			"company":    "Acme",
			"raw_jd":     "Seeking a Staff Go Engineer for pure-Go SQLite systems.",
			"jd_claims": []map[string]any{
				{"verbatim_jd_quote": "fabricated quote not in JD"},
			},
		},
	})
	if !res.IsError {
		t.Fatalf("expected triage failure on non-verbatim quote")
	}

	// 2. Triage success + pass_role branch
	res, _ = callToolJSON(t, srv, "resumai_save_triage", map[string]any{
		"application_id": "app-pass",
		"dossier": map[string]any{
			"role_title": "Staff Go",
			"company":    "SkipCo",
			"raw_jd":     "On-site 5 days/week.",
			"jd_claims": []map[string]any{
				{"verbatim_jd_quote": "On-site 5 days/week."},
			},
		},
	})
	if res.IsError {
		t.Fatalf("save_triage app-pass failed: %s", res.Content[0].Text)
	}
	res, _ = callToolJSON(t, srv, "resumai_pass_role", map[string]any{
		"application_id": "app-pass",
		"rationale":      "Requires 5-day on-site attendance.",
	})
	if res.IsError {
		t.Fatalf("pass_role failed: %s", res.Content[0].Text)
	}

	// 3. Full happy path: Triage -> Draft -> GetAuditBundle -> RecordAudits -> PromoteReady -> RenderBundle -> MarkSubmitted
	res, _ = callToolJSON(t, srv, "resumai_save_triage", map[string]any{
		"application_id": "app-1",
		"dossier": map[string]any{
			"role_title": "Staff Go Engineer",
			"company":    "Acme",
			"raw_jd":     "Seeking a Staff Go Engineer for pure-Go SQLite systems.",
			"jd_claims": []map[string]any{
				{"summary": "Pure-Go SQLite", "verbatim_jd_quote": "pure-Go SQLite systems."},
			},
			"external_claims": []map[string]any{
				{"claim": "Remote engineering culture", "source_url": "https://acme.example.com/blog"},
			},
		},
	})
	if res.IsError {
		t.Fatalf("save_triage app-1 failed: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_save_draft", map[string]any{
		"application_id": "app-1",
		"draft": map[string]any{
			"archetype_id": "arch-1",
			"headline":     "Staff Go Systems Engineer",
			"summary":      "Builds pure-Go storage engines.",
			"experience": []map[string]any{
				{
					"role_title":   "Senior Engineer",
					"organization": "Prior Co",
					"highlights": []map[string]any{
						{"id": "b-1", "text": "Cut P99 latency by 42% via WAL batching.", "fact_ids": []string{"fact-1"}},
					},
				},
			},
			"page_budget": 1,
		},
	})
	if res.IsError {
		t.Fatalf("save_draft failed: %s", res.Content[0].Text)
	}

	// Audit bundle must return cited_facts and zero PII
	res, bundle := callToolJSON(t, srv, "resumai_get_audit_bundle", map[string]any{
		"application_id": "app-1",
	})
	if res.IsError {
		t.Fatalf("get_audit_bundle failed: %s", res.Content[0].Text)
	}
	if bundle["application_id"] != "app-1" {
		t.Errorf("unexpected application_id in bundle: %v", bundle["application_id"])
	}
	citedFacts, ok := bundle["cited_facts"].([]any)
	if !ok || len(citedFacts) != 1 {
		t.Fatalf("expected 1 cited fact in audit bundle, got %v", bundle["cited_facts"])
	}

	// Record audits
	draftSHA, ok := bundle["draft_sha256"].(string)
	if !ok || len(draftSHA) != 64 {
		t.Fatalf("expected 64-char draft_sha256 in audit bundle, got %v", bundle["draft_sha256"])
	}
	res, _ = callToolJSON(t, srv, "resumai_record_audits", map[string]any{
		"application_id": "app-1",
		"draft_sha256":   draftSHA,
		"fact_check": map[string]any{
			"critic_agent": domain.CriticFactChecker,
			"summary":      "All bullets verified.",
			"passed":       true,
		},
		"alignment_check": map[string]any{
			"critic_agent":    domain.CriticAlignmentChecker,
			"summary":         "Strong alignment.",
			"matched_signals": []string{"Go", "SQLite"},
			"passed":          true,
		},
	})
	if res.IsError {
		t.Fatalf("record_audits failed: %s", res.Content[0].Text)
	}

	// Promote ready
	res, _ = callToolJSON(t, srv, "resumai_promote_ready", map[string]any{
		"application_id": "app-1",
	})
	if res.IsError {
		t.Fatalf("promote_ready failed: %s", res.Content[0].Text)
	}

	// Render bundle
	res, _ = callToolJSON(t, srv, "resumai_render_bundle", map[string]any{
		"application_id": "app-1",
	})
	if res.IsError {
		t.Fatalf("render_bundle failed: %s", res.Content[0].Text)
	}
	if strings.Contains(res.Content[0].Text, "Jane Candidate") || strings.Contains(res.Content[0].Text, "jane@example.com") {
		t.Fatalf("render_bundle response leaked PII!")
	}

	// Mark submitted
	res, _ = callToolJSON(t, srv, "resumai_mark_submitted", map[string]any{
		"application_id": "app-1",
		"notes":          "Submitted via company portal.",
	})
	if res.IsError {
		t.Fatalf("mark_submitted failed: %s", res.Content[0].Text)
	}

	// Verify persisted state in SQLite
	persisted, err := v.GetApplicationContext(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("GetApplicationContext: %v", err)
	}
	if persisted.State != domain.StateSubmitted || persisted.BundleSHA256 == "" || persisted.SubmittedAt == nil {
		t.Fatalf("expected persisted Submitted state with SHA-256 and timestamp, got %+v", persisted)
	}

	// Subsequent mutations must fail with ErrAlreadySubmitted
	res, _ = callToolJSON(t, srv, "resumai_mark_submitted", map[string]any{
		"application_id": "app-1",
	})
	if !res.IsError || !strings.Contains(res.Content[0].Text, "already submitted") {
		t.Fatalf("expected ErrAlreadySubmitted on duplicate submit, got isError=%v text=%s", res.IsError, res.Content[0].Text)
	}
}

func TestGetAuditBundleDeterministicOrder(t *testing.T) {
	_, srv := newTestVaultAndServer(t)

	callToolJSON(t, srv, "resumai_save_organization", map[string]any{"id": "org-z", "name": "Zulu Corp"})
	callToolJSON(t, srv, "resumai_save_organization", map[string]any{"id": "org-a", "name": "Alpha Corp"})
	callToolJSON(t, srv, "resumai_save_organization", map[string]any{"id": "org-m", "name": "Mike Corp"})

	callToolJSON(t, srv, "resumai_save_role", map[string]any{"id": "role-z", "org_id": "org-z", "title": "Engineer Z"})
	callToolJSON(t, srv, "resumai_save_role", map[string]any{"id": "role-a", "org_id": "org-a", "title": "Engineer A"})
	callToolJSON(t, srv, "resumai_save_role", map[string]any{"id": "role-m", "org_id": "org-m", "title": "Engineer M"})

	callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id": "fact-z", "role_id": "role-z", "headline": "Z", "details": "Did Z.", "epistemic_state": "verified",
	})
	callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id": "fact-a", "role_id": "role-a", "headline": "A", "details": "Did A.", "epistemic_state": "verified",
	})
	callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id": "fact-m", "role_id": "role-m", "headline": "M", "details": "Did M.", "epistemic_state": "verified",
	})
	callToolJSON(t, srv, "resumai_save_archetype", map[string]any{"id": "arch-1", "name": "Systems"})

	res, _ := callToolJSON(t, srv, "resumai_save_triage", map[string]any{
		"application_id": "app-sort",
		"dossier": map[string]any{
			"role_title": "Staff Engineer",
			"company":    "Acme",
			"raw_jd":     "Build systems.",
		},
	})
	if res.IsError {
		t.Fatalf("save_triage failed: %s", res.Content[0].Text)
	}

	res, _ = callToolJSON(t, srv, "resumai_save_draft", map[string]any{
		"application_id": "app-sort",
		"draft": map[string]any{
			"archetype_id": "arch-1",
			"experience": []map[string]any{
				{
					"role_title":   "Engineers",
					"organization": "Multi",
					"highlights": []map[string]any{
						{"id": "b1", "text": "Did Z, A, M.", "fact_ids": []string{"fact-z", "fact-a", "fact-m"}},
					},
				},
			},
			"page_budget": 1,
		},
	})
	if res.IsError {
		t.Fatalf("save_draft failed: %s", res.Content[0].Text)
	}

	for iter := 0; iter < 10; iter++ {
		res, bundle := callToolJSON(t, srv, "resumai_get_audit_bundle", map[string]any{
			"application_id": "app-sort",
		})
		if res.IsError {
			t.Fatalf("get_audit_bundle failed: %s", res.Content[0].Text)
		}

		rolesRaw, ok := bundle["roles"].([]any)
		if !ok || len(rolesRaw) != 3 {
			t.Fatalf("expected 3 roles, got %v", bundle["roles"])
		}
		expectedRoles := []string{"role-a", "role-m", "role-z"}
		for i, want := range expectedRoles {
			gotID := rolesRaw[i].(map[string]any)["id"]
			if gotID != want {
				t.Fatalf("iter %d: roles[%d].id = %v, want %s", iter, i, gotID, want)
			}
		}

		orgsRaw, ok := bundle["organizations"].([]any)
		if !ok || len(orgsRaw) != 3 {
			t.Fatalf("expected 3 organizations, got %v", bundle["organizations"])
		}
		expectedOrgs := []string{"org-a", "org-m", "org-z"}
		for i, want := range expectedOrgs {
			gotID := orgsRaw[i].(map[string]any)["id"]
			if gotID != want {
				t.Fatalf("iter %d: organizations[%d].id = %v, want %s", iter, i, gotID, want)
			}
		}
	}
}

func TestSaveDraftStrictDecodingAndSchema(t *testing.T) {
	_, srv := newTestVaultAndServer(t)

	// Verify resumai_save_draft InputSchema documents nested ApplicationDraft properties.
	var saveDraftTool *mcp.Tool
	for _, tool := range srv.ListTools() {
		if tool.Name == "resumai_save_draft" {
			tCopy := tool
			saveDraftTool = &tCopy
			break
		}
	}
	if saveDraftTool == nil {
		t.Fatalf("resumai_save_draft tool not registered")
	}
	props, ok := saveDraftTool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties map in InputSchema")
	}
	draftSchema, ok := props["draft"].(map[string]any)
	if !ok {
		t.Fatalf("expected draft schema map")
	}
	draftProps, ok := draftSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested properties on draft schema")
	}
	for _, key := range []string{"archetype_id", "headline", "summary", "experience", "education", "skills", "projects", "certifications", "custom_sections", "page_budget"} {
		if _, exists := draftProps[key]; !exists {
			t.Errorf("draft InputSchema missing property %q", key)
		}
	}

	// Seed minimal vault + triaged app
	callToolJSON(t, srv, "resumai_save_organization", map[string]any{"id": "org-1", "name": "Acme"})
	callToolJSON(t, srv, "resumai_save_role", map[string]any{"id": "role-1", "org_id": "org-1", "title": "Staff Engineer"})
	callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id": "fact-1", "role_id": "role-1", "headline": "Cut P99", "details": "Cut P99 by 42%.", "epistemic_state": "verified",
	})
	callToolJSON(t, srv, "resumai_save_archetype", map[string]any{"id": "arch-1", "name": "Systems"})
	res, _ := callToolJSON(t, srv, "resumai_save_triage", map[string]any{
		"application_id": "app-strict",
		"role_title":     "Staff Engineer",
		"company":        "Acme",
		"raw_jd":         "Build pure-Go systems.",
	})
	if res.IsError {
		t.Fatalf("save_triage failed: %s", res.Content[0].Text)
	}

	cases := []struct {
		name string
		args map[string]any
	}{
		{
			name: "guessed_experience_keys_title_company_bullets",
			args: map[string]any{
				"application_id": "app-strict",
				"draft": map[string]any{
					"archetype_id": "arch-1",
					"experience": []map[string]any{
						{
							"title":   "Staff Engineer",
							"company": "Acme",
							"bullets": []map[string]any{
								{"id": "b-1", "text": "Cut P99 by 42%.", "fact_ids": []string{"fact-1"}},
							},
						},
					},
				},
			},
		},
		{
			name: "flat_skills_string_array",
			args: map[string]any{
				"application_id": "app-strict",
				"draft": map[string]any{
					"archetype_id": "arch-1",
					"experience": []map[string]any{
						{
							"role_title":   "Staff Engineer",
							"organization": "Acme",
							"highlights": []map[string]any{
								{"id": "b-1", "text": "Cut P99 by 42%.", "fact_ids": []string{"fact-1"}},
							},
						},
					},
					"skills": []string{"Go", "SQLite"},
				},
			},
		},
		{
			name: "unknown_top_level_key",
			args: map[string]any{
				"application_id": "app-strict",
				"extra_field":    true,
				"draft": map[string]any{
					"archetype_id": "arch-1",
					"experience": []map[string]any{
						{
							"role_title":   "Staff Engineer",
							"organization": "Acme",
							"highlights": []map[string]any{
								{"id": "b-1", "text": "Cut P99 by 42%.", "fact_ids": []string{"fact-1"}},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := callToolJSON(t, srv, "resumai_save_draft", tc.args)
			if !res.IsError {
				t.Fatalf("expected error for %s, but succeeded: %s", tc.name, res.Content[0].Text)
			}
		})
	}
}

func TestRecordAuditsStrictDecodingAndSchema(t *testing.T) {
	v, srv := newTestVaultAndServer(t)

	// Verify resumai_record_audits InputSchema exposes nested properties and required fields.
	var recordAuditsTool *mcp.Tool
	for _, tool := range srv.ListTools() {
		if tool.Name == "resumai_record_audits" {
			tCopy := tool
			recordAuditsTool = &tCopy
			break
		}
	}
	if recordAuditsTool == nil {
		t.Fatalf("resumai_record_audits tool not registered")
	}

	assertHasKeys := func(t *testing.T, label string, m map[string]any, keys ...string) {
		t.Helper()
		for _, k := range keys {
			if _, ok := m[k]; !ok {
				t.Errorf("%s missing key %q", label, k)
			}
		}
	}
	assertRequired := func(t *testing.T, label string, schema map[string]any, want ...string) {
		t.Helper()
		reqRaw, ok := schema["required"].([]string)
		if !ok {
			if reqAny, okAny := schema["required"].([]any); okAny {
				reqRaw = make([]string, 0, len(reqAny))
				for _, item := range reqAny {
					if s, okStr := item.(string); okStr {
						reqRaw = append(reqRaw, s)
					}
				}
			} else {
				t.Fatalf("%s missing required slice (%T)", label, schema["required"])
			}
		}
		got := make(map[string]bool, len(reqRaw))
		for _, r := range reqRaw {
			got[r] = true
		}
		for _, w := range want {
			if !got[w] {
				t.Errorf("%s required missing %q (got %v)", label, w, reqRaw)
			}
		}
	}

	assertRequired(t, "resumai_record_audits", recordAuditsTool.InputSchema, "application_id", "draft_sha256", "fact_check", "alignment_check")
	topProps, ok := recordAuditsTool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level properties map in InputSchema")
	}
	assertHasKeys(t, "resumai_record_audits.properties", topProps, "application_id", "draft_sha256", "fact_check", "alignment_check")

	fcSchema, ok := topProps["fact_check"].(map[string]any)
	if !ok {
		t.Fatalf("expected fact_check schema map")
	}
	assertRequired(t, "fact_check", fcSchema, "critic_agent", "summary", "passed")
	fcProps, ok := fcSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested properties on fact_check schema")
	}
	assertHasKeys(t, "fact_check.properties", fcProps, "critic_agent", "summary", "discrepancies", "passed")

	discSchema, ok := fcProps["discrepancies"].(map[string]any)
	if !ok {
		t.Fatalf("expected discrepancies schema map")
	}
	if discSchema["type"] != "array" {
		t.Errorf("expected discrepancies type=array, got %v", discSchema["type"])
	}
	discItems, ok := discSchema["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected discrepancies.items schema map")
	}
	assertRequired(t, "discrepancies.items", discItems, "severity", "finding")
	discProps, ok := discItems["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested properties on discrepancies.items")
	}
	assertHasKeys(t, "discrepancies.items.properties", discProps, "bullet_id", "fact_id", "severity", "finding")
	sevSchema, ok := discProps["severity"].(map[string]any)
	if !ok {
		t.Fatalf("expected severity schema map")
	}
	if enumSlice, okEnum := sevSchema["enum"].([]string); !okEnum || len(enumSlice) != 3 {
		t.Errorf("expected severity enum [low medium high], got %v", sevSchema["enum"])
	}

	acSchema, ok := topProps["alignment_check"].(map[string]any)
	if !ok {
		t.Fatalf("expected alignment_check schema map")
	}
	assertRequired(t, "alignment_check", acSchema, "critic_agent", "summary", "passed")
	acProps, ok := acSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested properties on alignment_check schema")
	}
	assertHasKeys(t, "alignment_check.properties", acProps, "critic_agent", "summary", "matched_signals", "missing_signals", "recommendations", "passed")

	// Seed vault + triaged/drafted application.
	callToolJSON(t, srv, "resumai_save_organization", map[string]any{"id": "org-1", "name": "Acme"})
	callToolJSON(t, srv, "resumai_save_role", map[string]any{"id": "role-1", "org_id": "org-1", "title": "Staff Engineer"})
	callToolJSON(t, srv, "resumai_save_fact", map[string]any{
		"id": "fact-1", "role_id": "role-1", "headline": "Cut P99", "details": "Cut P99 by 42%.", "epistemic_state": "verified",
	})
	callToolJSON(t, srv, "resumai_save_archetype", map[string]any{"id": "arch-1", "name": "Systems"})
	if res, _ := callToolJSON(t, srv, "resumai_save_triage", map[string]any{
		"application_id": "app-strict-audit",
		"role_title":     "Staff Engineer",
		"company":        "Acme",
		"raw_jd":         "Build pure-Go systems.",
	}); res.IsError {
		t.Fatalf("save_triage failed: %s", res.Content[0].Text)
	}
	if res, _ := callToolJSON(t, srv, "resumai_save_draft", map[string]any{
		"application_id": "app-strict-audit",
		"draft": map[string]any{
			"archetype_id": "arch-1",
			"experience": []map[string]any{
				{
					"role_title":   "Staff Engineer",
					"organization": "Acme",
					"highlights": []map[string]any{
						{"id": "b-1", "text": "Cut P99 by 42%.", "fact_ids": []string{"fact-1"}},
					},
				},
			},
			"page_budget": 1,
		},
	}); res.IsError {
		t.Fatalf("save_draft failed: %s", res.Content[0].Text)
	}
	res, bundle := callToolJSON(t, srv, "resumai_get_audit_bundle", map[string]any{"application_id": "app-strict-audit"})
	if res.IsError {
		t.Fatalf("get_audit_bundle failed: %s", res.Content[0].Text)
	}
	draftSHA, ok := bundle["draft_sha256"].(string)
	if !ok || len(draftSHA) != 64 {
		t.Fatalf("expected 64-char draft_sha256, got %v", bundle["draft_sha256"])
	}

	mustJSON := func(m map[string]any) string {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		return string(b)
	}
	validFactCheck := func() map[string]any {
		return map[string]any{
			"critic_agent": domain.CriticFactChecker,
			"summary":      "All claims verified.",
			"passed":       true,
		}
	}
	validAlignmentCheck := func() map[string]any {
		return map[string]any{
			"critic_agent":    domain.CriticAlignmentChecker,
			"summary":         "Strong alignment.",
			"matched_signals": []string{"Go"},
			"missing_signals": []string{},
			"recommendations": []string{"Emphasize SQLite."},
			"passed":          true,
		}
	}

	secretToken := "ghp_" + strings.Repeat("a", 36)
	cases := []struct {
		name          string
		rawArgs       string
		wantError     bool
		wantSubstr    string
		forbidSubstr  string
		wantState     domain.ApplicationState
		wantAuditsSet bool
	}{
		{
			name: "omitted_draft_sha256",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "draft_sha256",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "empty_draft_sha256",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"draft_sha256":    "",
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "draft_sha256",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "whitespace_draft_sha256",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"draft_sha256":    "  \t\n ",
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "draft_sha256",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "mismatched_draft_sha256",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"draft_sha256":    strings.Repeat("0", 64),
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "does not match",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "unknown_top_level_field",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"draft_sha256":    draftSHA,
				"extra_field":     true,
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "unknown field",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "unknown_fact_check_field_issues",
			rawArgs: mustJSON(map[string]any{
				"application_id": "app-strict-audit",
				"draft_sha256":   draftSHA,
				"fact_check": map[string]any{
					"critic_agent": domain.CriticFactChecker,
					"summary":      "Mistyped key.",
					"issues":       []any{},
					"passed":       true,
				},
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "unknown field",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "unknown_discrepancy_item_field",
			rawArgs: mustJSON(map[string]any{
				"application_id": "app-strict-audit",
				"draft_sha256":   draftSHA,
				"fact_check": map[string]any{
					"critic_agent": domain.CriticFactChecker,
					"summary":      "Mistyped discrepancy field.",
					"discrepancies": []map[string]any{
						{"severity": "low", "finding": "Minor", "reason": "typo"},
					},
					"passed": true,
				},
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "unknown field",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "unknown_alignment_check_field_gaps",
			rawArgs: mustJSON(map[string]any{
				"application_id": "app-strict-audit",
				"draft_sha256":   draftSHA,
				"fact_check":     validFactCheck(),
				"alignment_check": map[string]any{
					"critic_agent": domain.CriticAlignmentChecker,
					"summary":      "Mistyped alignment key.",
					"gaps":         []string{"Go"},
					"passed":       true,
				},
			}),
			wantError:     true,
			wantSubstr:    "unknown field",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "mistyped_fact_check_passed_string",
			rawArgs: mustJSON(map[string]any{
				"application_id": "app-strict-audit",
				"draft_sha256":   draftSHA,
				"fact_check": map[string]any{
					"critic_agent": domain.CriticFactChecker,
					"summary":      "Wrong bool type.",
					"passed":       "yes",
				},
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "invalid tool arguments",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "trailing_json_token",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"draft_sha256":    draftSHA,
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}) + `{"trailing":1}`,
			wantError:     true,
			wantSubstr:    "trailing JSON",
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "unknown_field_secret_redacted",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"draft_sha256":    draftSHA,
				secretToken:       true,
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     true,
			wantSubstr:    "[REDACTED]",
			forbidSubstr:  secretToken,
			wantState:     domain.StateDrafted,
			wantAuditsSet: false,
		},
		{
			name: "valid_strict_payload_with_discrepancy",
			rawArgs: mustJSON(map[string]any{
				"application_id": "app-strict-audit",
				"draft_sha256":   draftSHA,
				"fact_check": map[string]any{
					"critic_agent": domain.CriticFactChecker,
					"summary":      "Verified with minor phrasing note.",
					"discrepancies": []map[string]any{
						{"bullet_id": "b-1", "fact_id": "fact-1", "severity": "low", "finding": "Minor phrasing note"},
					},
					"passed": true,
				},
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     false,
			wantState:     domain.StateAudited,
			wantAuditsSet: true,
		},
		{
			name: "valid_uppercase_padded_draft_sha256_reaudit",
			rawArgs: mustJSON(map[string]any{
				"application_id":  "app-strict-audit",
				"draft_sha256":    "  " + strings.ToUpper(draftSHA) + "\n",
				"fact_check":      validFactCheck(),
				"alignment_check": validAlignmentCheck(),
			}),
			wantError:     false,
			wantState:     domain.StateAudited,
			wantAuditsSet: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := srv.CallTool(context.Background(), "resumai_record_audits", json.RawMessage(tc.rawArgs))
			if err != nil {
				t.Fatalf("CallTool unexpected protocol error: %v", err)
			}
			text := ""
			if len(res.Content) > 0 {
				text = res.Content[0].Text
			}
			if tc.wantError {
				if !res.IsError {
					t.Fatalf("expected error for %s, but succeeded: %s", tc.name, text)
				}
				if tc.wantSubstr != "" && !strings.Contains(text, tc.wantSubstr) {
					t.Errorf("expected error text containing %q, got %q", tc.wantSubstr, text)
				}
				if tc.forbidSubstr != "" && strings.Contains(text, tc.forbidSubstr) {
					t.Errorf("error text leaked forbidden secret %q: %q", tc.forbidSubstr, text)
				}
			} else if res.IsError {
				t.Fatalf("unexpected error for %s: %s", tc.name, text)
			}

			persisted, err := v.GetApplicationContext(context.Background(), "app-strict-audit")
			if err != nil {
				t.Fatalf("GetApplicationContext: %v", err)
			}
			if persisted.State != tc.wantState {
				t.Errorf("persisted state = %q, want %q", persisted.State, tc.wantState)
			}
			if tc.wantAuditsSet {
				if persisted.Audits == nil || persisted.Audits.DraftSHA256 != draftSHA {
					t.Errorf("expected persisted Audits.DraftSHA256=%q, got %+v", draftSHA, persisted.Audits)
				}
			} else if persisted.Audits != nil {
				t.Errorf("expected persisted Audits to remain nil on rejection, got %+v", persisted.Audits)
			}
		})
	}
}
