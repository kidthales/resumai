package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kidthales/resumai/internal/domain"
	"github.com/kidthales/resumai/internal/mcp"
	"github.com/kidthales/resumai/internal/render"
	"github.com/kidthales/resumai/internal/vault"
)

// TestVaultInterfaceAirGap verifies that the vault.Vault interface consumed by
// the MCP Server package strictly enforces the PII air-gap. Contact PII methods
// (GetPII, SetPII) and domain.PIIHeader types must never be exposed in vault.Vault.
func TestVaultInterfaceAirGap(t *testing.T) {
	vaultType := reflect.TypeOf((*vault.Vault)(nil)).Elem()
	piiHeaderType := reflect.TypeOf(domain.PIIHeader{})

	for i := 0; i < vaultType.NumMethod(); i++ {
		method := vaultType.Method(i)
		if method.Name == "GetPII" || method.Name == "SetPII" {
			t.Fatalf("air-gap violation: vault.Vault interface exposes PII method %q to MCP", method.Name)
		}
		for j := 0; j < method.Type.NumIn(); j++ {
			if containsType(method.Type.In(j), piiHeaderType) {
				t.Fatalf("air-gap violation: vault.Vault.%s accepts domain.PIIHeader parameter", method.Name)
			}
		}
		for j := 0; j < method.Type.NumOut(); j++ {
			if containsType(method.Type.Out(j), piiHeaderType) {
				t.Fatalf("air-gap violation: vault.Vault.%s returns domain.PIIHeader", method.Name)
			}
		}
	}
}

func containsType(candidate, target reflect.Type) bool {
	if candidate == target {
		return true
	}
	switch candidate.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
		return containsType(candidate.Elem(), target)
	case reflect.Map:
		return containsType(candidate.Key(), target) || containsType(candidate.Elem(), target)
	default:
		return false
	}
}

// TestMCPToolsAirGap_AllToolsSuccessAndErrorPaths exercises every registered MCP tool
// returned by tools/list across both success and error paths over JSON-RPC 2.0 stdio
// while sentinel pii_header secrets are populated in SQLite, asserting that zero sentinel
// PII strings appear in any MCP tool output, JSON-RPC response, or error payload.
func TestMCPToolsAirGap_AllToolsSuccessAndErrorPaths(t *testing.T) {
	sentinelPII := &domain.PIIHeader{
		FullName:     "SENTINEL_PII_FULLNAME_9F4B2A",
		Email:        "sentinel-pii-9f4b2a@private.invalid",
		Phone:        "+1-555-019-9420",
		Location:     "Sentinel Citadel, ZZ 99999",
		LinkedInURL:  "https://linkedin.com/in/sentinel-pii-9f4b2a",
		GitHubURL:    "https://github.com/sentinel-pii-9f4b2a",
		PortfolioURL: "https://sentinel-pii-9f4b2a.invalid",
	}

	baseDir := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				_ = os.Chmod(path, 0644)
			}
			return nil
		})
	})

	ctx := context.Background()
	v, _ := newTestVaultAndServer(t)
	if err := v.SetPII(ctx, sentinelPII); err != nil {
		t.Fatalf("SetPII: %v", err)
	}

	fixedNow := time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC)
	fakePDF := func(_ context.Context, doc *render.ResumeDocument, pii *domain.PIIHeader, _ *render.ThemeManifest, _ int) ([]byte, *render.LayoutTelemetry, error) {
		if doc.Header.FullName != "" || doc.Header.Email != "" || doc.Header.Phone != "" ||
			doc.Header.Location != "" || doc.Header.LinkedInURL != "" || doc.Header.GitHubURL != "" || doc.Header.PortfolioURL != "" {
			return nil, nil, errors.New("air-gap violation: source ResumeDocument passed to PDF renderer contained PII")
		}
		if pii == nil || pii.FullName != sentinelPII.FullName {
			return nil, nil, errors.New("expected local PIIHeader injection in PDF renderer")
		}
		return []byte("%PDF-1.4\n%airgap-ok\n"), &render.LayoutTelemetry{
			PageCount:         1,
			LastPageFillRatio: 0.84,
			OverflowPX:        0,
		}, nil
	}

	mat := mcp.NewLocalMaterializer(
		v,
		mcp.WithBaseOutputDir(baseDir),
		mcp.WithPDFRenderer(fakePDF),
		mcp.WithNowFunc(func() time.Time { return fixedNow }),
	)
	srv := mcp.NewServer(v, mcp.WithPacketMaterializer(mat))

	// Secondary server whose PDF renderer deliberately embeds all 7 sentinel PII strings in its error
	// to verify LocalMaterializer's scrubPIIError barrier scrubs every field before returning over MCP.
	leakingPDF := func(_ context.Context, _ *render.ResumeDocument, pii *domain.PIIHeader, _ *render.ThemeManifest, _ int) ([]byte, *render.LayoutTelemetry, error) {
		return nil, nil, errors.New(strings.Join([]string{
			"renderer exploded for",
			pii.FullName,
			pii.Email,
			pii.Phone,
			pii.Location,
			pii.LinkedInURL,
			pii.GitHubURL,
			pii.PortfolioURL,
		}, " | "))
	}
	leakingMat := mcp.NewLocalMaterializer(v, mcp.WithBaseOutputDir(baseDir), mcp.WithPDFRenderer(leakingPDF))
	srvLeakingPDF := mcp.NewServer(v, mcp.WithPacketMaterializer(leakingMat))

	// Separate vault populated with sentinel PII and then closed to exercise error paths on 0-arg read tools.
	vClosed, srvClosed := newTestVaultAndServer(t)
	if err := vClosed.SetPII(ctx, sentinelPII); err != nil {
		t.Fatalf("SetPII(closed): %v", err)
	}
	if err := vClosed.Close(); err != nil {
		t.Fatalf("Close(vClosed): %v", err)
	}

	// Verify JSON-RPC protocol endpoints & error payloads never contain sentinel PII.
	protocolLines := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"resumai_nonexistent_tool"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{}}`,
		`{"jsonrpc":"2.0","id":6,"method":"unknown/method"}`,
		`{malformed-json`,
	}, "\n") + "\n"

	var protoOut bytes.Buffer
	if err := srv.ServeStdio(ctx, strings.NewReader(protocolLines), &protoOut); err != nil {
		t.Fatalf("ServeStdio(protocol): %v", err)
	}
	assertNoSentinelPII(t, protoOut.String(), sentinelPII)

	respLines := strings.Split(strings.TrimSpace(protoOut.String()), "\n")
	if len(respLines) != 7 {
		t.Fatalf("expected 7 protocol responses, got %d:\n%s", len(respLines), protoOut.String())
	}

	var listResp struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(respLines[2]), &listResp); err != nil {
		t.Fatalf("unmarshal tools/list response: %v", err)
	}
	if len(listResp.Result.Tools) == 0 {
		t.Fatalf("tools/list returned 0 tools")
	}

	// Also verify nil-vault defensive error payload contains zero PII.
	nilSrv := mcp.NewServer(nil)
	nilRes, err := nilSrv.CallTool(ctx, "resumai_vault_status", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("CallTool(nil vault) protocol error: %v", err)
	}
	if !nilRes.IsError {
		t.Fatalf("expected IsError=true for nil vault")
	}
	assertNoSentinelPII(t, nilRes.Content[0].Text, sentinelPII)

	successCovered := make(map[string]bool, len(listResp.Result.Tools))
	errorCovered := make(map[string]bool, len(listResp.Result.Tools))
	nextID := 100

	invoke := func(targetSrv *mcp.Server, toolName string, args any, expectError bool) map[string]any {
		t.Helper()
		nextID++
		res := callToolViaStdio(t, targetSrv, nextID, toolName, args, sentinelPII)
		if expectError {
			if !res.IsError {
				t.Fatalf("%s expected tool error, got success: %+v", toolName, res.Content)
			}
			errorCovered[toolName] = true
			return nil
		}
		if res.IsError {
			t.Fatalf("%s unexpected tool error: %s", toolName, res.Content[0].Text)
		}
		successCovered[toolName] = true
		if len(res.Content) == 0 {
			return nil
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(res.Content[0].Text), &decoded); err != nil {
			t.Fatalf("%s unmarshal tool JSON: %v; raw=%s", toolName, err, res.Content[0].Text)
		}
		return decoded
	}

	// 1. Vault curation tools (success + error)
	// Unconfigured get_logistics check, then error on closed vault
	_ = invoke(srv, "resumai_get_logistics", map[string]any{}, false)
	_ = invoke(srvClosed, "resumai_get_logistics", map[string]any{}, true)

	_ = invoke(srv, "resumai_save_logistics", map[string]any{
		"target_title":        "Staff Go Systems Engineer",
		"work_authorization":  "Authorized",
		"remote_preference":   "Remote",
		"willing_to_relocate": false,
		"target_compensation": "$220k",
		"notice_period_days":  14,
	}, false)
	_ = invoke(srv, "resumai_save_logistics", map[string]any{
		"notice_period_days": "not-an-integer",
	}, true)
	// Configured get_logistics check
	_ = invoke(srv, "resumai_get_logistics", map[string]any{}, false)

	_ = invoke(srv, "resumai_save_organization", map[string]any{
		"id":       "org-airgap",
		"name":     "AirGap Systems",
		"location": "Remote",
		"website":  "https://airgap.example.com",
	}, false)
	_ = invoke(srv, "resumai_save_organization", map[string]any{"id": "", "name": ""}, true)
	_ = invoke(srv, "resumai_list_organizations", map[string]any{}, false)
	_ = invoke(srvClosed, "resumai_list_organizations", map[string]any{}, true)

	_ = invoke(srv, "resumai_save_role", map[string]any{
		"id":         "role-airgap",
		"org_id":     "org-airgap",
		"title":      "Staff Engineer",
		"duration":   "2023 - 2026",
		"tech_stack": []string{"Go", "SQLite"},
	}, false)
	_ = invoke(srv, "resumai_save_role", map[string]any{
		"id":     "role-bad",
		"org_id": "missing-org",
		"title":  "Staff Engineer",
	}, true)
	_ = invoke(srv, "resumai_list_roles", map[string]any{"org_id": "org-airgap"}, false)
	_ = invoke(srv, "resumai_list_roles", map[string]any{"org_id": 12345}, true)

	_ = invoke(srv, "resumai_save_fact", map[string]any{
		"id":              "fact-airgap",
		"role_id":         "role-airgap",
		"headline":        "Cut tail latency",
		"details":         "Cut P99 latency by 42% via WAL batching.",
		"epistemic_state": "verified",
		"tech_tags":       []string{"Go", "SQLite"},
	}, false)
	_ = invoke(srv, "resumai_save_fact", map[string]any{
		"id":              "fact-to-archive",
		"role_id":         "role-airgap",
		"headline":        "Legacy note",
		"details":         "Initial raw note pending archival.",
		"epistemic_state": "raw_note",
	}, false)
	_ = invoke(srv, "resumai_save_fact", map[string]any{
		"id":              "fact-bad-metric",
		"role_id":         "role-airgap",
		"details":         "Reduced latency by [METRIC: pct].",
		"epistemic_state": "verified",
	}, true)
	_ = invoke(srv, "resumai_list_facts", map[string]any{"role_id": "role-airgap", "epistemic_state": "verified"}, false)
	_ = invoke(srv, "resumai_list_facts", map[string]any{"epistemic_state": "invalid_state"}, true)

	_ = invoke(srv, "resumai_archive_fact", map[string]any{"fact_id": "fact-to-archive"}, false)
	_ = invoke(srv, "resumai_archive_fact", map[string]any{"fact_id": "missing-fact"}, true)

	_ = invoke(srv, "resumai_save_education", map[string]any{
		"id":             "edu-airgap",
		"institution":    "State Tech",
		"degree":         "B.S.",
		"field_of_study": "Computer Science",
		"duration":       "2014 - 2018",
	}, false)
	_ = invoke(srv, "resumai_save_education", map[string]any{"id": "", "institution": ""}, true)
	_ = invoke(srv, "resumai_list_education", map[string]any{}, false)
	_ = invoke(srvClosed, "resumai_list_education", map[string]any{}, true)

	_ = invoke(srv, "resumai_save_archetype", map[string]any{
		"id":                "arch-airgap",
		"name":              "Systems Architect",
		"profile_overview":  "Builds deterministic pure-Go engines.",
		"core_competencies": []string{"Go", "SQLite"},
	}, false)
	_ = invoke(srv, "resumai_save_archetype", map[string]any{"id": "", "name": ""}, true)
	_ = invoke(srv, "resumai_list_archetypes", map[string]any{}, false)
	_ = invoke(srvClosed, "resumai_list_archetypes", map[string]any{}, true)

	statusOut := invoke(srv, "resumai_vault_status", map[string]any{}, false)
	if statusOut["pii_configured"] != true {
		t.Fatalf("expected pii_configured=true in vault_status, got %v", statusOut["pii_configured"])
	}
	_ = invoke(srvClosed, "resumai_vault_status", map[string]any{}, true)

	// 2. Application FSM tools (success + error)
	_ = invoke(srv, "resumai_save_triage", map[string]any{
		"application_id": "app-airgap",
		"dossier": map[string]any{
			"role_title":  "Staff Go Engineer",
			"company":     "Acme Systems",
			"raw_jd":      "Seeking a Staff Go Engineer for pure-Go SQLite systems.",
			"fit_summary": "Strong systems alignment.",
			"jd_claims": []map[string]any{
				{"summary": "Pure-Go SQLite", "verbatim_jd_quote": "pure-Go SQLite systems."},
			},
			"external_claims": []map[string]any{
				{"claim": "On-call cadence", "unknown_ask_recruiter": true},
			},
		},
	}, false)
	_ = invoke(srv, "resumai_save_triage", map[string]any{
		"application_id": "app-pass-airgap",
		"dossier": map[string]any{
			"role_title": "On-site Engineer",
			"company":    "SkipCo",
			"raw_jd":     "Requires 5 days on-site.",
			"jd_claims": []map[string]any{
				{"verbatim_jd_quote": "Requires 5 days on-site."},
			},
		},
	}, false)
	_ = invoke(srv, "resumai_save_triage", map[string]any{
		"application_id": "app-bad-triage",
		"dossier": map[string]any{
			"role_title": "Staff Go",
			"company":    "BadCo",
			"raw_jd":     "Looking for Go engineers.",
			"jd_claims": []map[string]any{
				{"verbatim_jd_quote": "quote not present in raw_jd"},
			},
		},
	}, true)

	_ = invoke(srv, "resumai_list_applications", map[string]any{"state": "Triaged"}, false)
	_ = invoke(srv, "resumai_list_applications", map[string]any{"state": "InvalidFSMState"}, true)

	_ = invoke(srv, "resumai_get_application", map[string]any{"application_id": "app-airgap"}, false)
	_ = invoke(srv, "resumai_get_application", map[string]any{"application_id": "missing-app"}, true)

	_ = invoke(srv, "resumai_pass_role", map[string]any{
		"application_id": "app-pass-airgap",
		"rationale":      "Requires 5-day on-site attendance.",
	}, false)
	_ = invoke(srv, "resumai_pass_role", map[string]any{
		"application_id": "app-airgap",
		"rationale":      "   ",
	}, true)

	_ = invoke(srv, "resumai_save_draft", map[string]any{
		"application_id": "app-airgap",
		"draft": map[string]any{
			"archetype_id": "arch-airgap",
			"headline":     "Staff Go Systems Engineer",
			"summary":      "Builds pure-Go storage engines.",
			"experience": []map[string]any{
				{
					"role_title":   "Staff Engineer",
					"organization": "AirGap Systems",
					"duration":     "2023 - 2026",
					"highlights": []map[string]any{
						{"id": "b-1", "text": "Cut P99 latency by 42% via WAL batching.", "fact_ids": []string{"fact-airgap"}},
					},
				},
			},
			"page_budget": 1,
		},
	}, false)
	_ = invoke(srv, "resumai_save_draft", map[string]any{
		"application_id": "app-airgap",
		"draft": map[string]any{
			"archetype_id": "arch-airgap",
			"experience": []map[string]any{
				{
					"role_title":   "Staff Engineer",
					"organization": "AirGap Systems",
					"highlights": []map[string]any{
						{"id": "b-bad", "text": "Ungrounded bullet.", "fact_ids": []string{"missing-fact"}},
					},
				},
			},
		},
	}, true)

	_ = invoke(srv, "resumai_get_audit_bundle", map[string]any{"application_id": "app-airgap"}, false)
	_ = invoke(srv, "resumai_get_audit_bundle", map[string]any{"application_id": "app-pass-airgap"}, true)

	_ = invoke(srv, "resumai_record_audits", map[string]any{
		"application_id": "app-airgap",
		"fact_check": map[string]any{
			"critic_agent": domain.CriticFactChecker,
			"summary":      "All claims verified.",
			"passed":       true,
		},
		"alignment_check": map[string]any{
			"critic_agent":    domain.CriticAlignmentChecker,
			"summary":         "Strong alignment.",
			"matched_signals": []string{"Go", "SQLite"},
			"passed":          true,
		},
	}, false)
	_ = invoke(srv, "resumai_record_audits", map[string]any{
		"application_id": "app-airgap",
		"fact_check": map[string]any{
			"critic_agent": domain.CriticFactChecker,
			"summary":      "Missing alignment check.",
			"passed":       true,
		},
	}, true)

	_ = invoke(srv, "resumai_promote_ready", map[string]any{"application_id": "app-airgap"}, false)
	_ = invoke(srv, "resumai_promote_ready", map[string]any{"application_id": "app-pass-airgap"}, true)

	// Verify leaking renderer error is scrubbed of all 7 sentinel PII fields before leaving MCP.
	_ = invoke(srvLeakingPDF, "resumai_render_bundle", map[string]any{"application_id": "app-airgap"}, true)
	_ = invoke(srv, "resumai_render_bundle", map[string]any{"application_id": "app-pass-airgap"}, true)
	_ = invoke(srv, "resumai_render_bundle", map[string]any{"application_id": "app-airgap"}, false)

	_ = invoke(srv, "resumai_mark_submitted", map[string]any{"application_id": "app-pass-airgap"}, true)
	_ = invoke(srv, "resumai_mark_submitted", map[string]any{
		"application_id": "app-airgap",
		"notes":          "Submitted via portal.",
	}, false)
	// Duplicate submission must error with ErrAlreadySubmitted and zero PII.
	_ = invoke(srv, "resumai_mark_submitted", map[string]any{"application_id": "app-airgap"}, true)

	// Enforce 100% tools/list coverage across both success and error paths.
	for _, tool := range listResp.Result.Tools {
		if !successCovered[tool.Name] {
			t.Errorf("air-gap fitness suite missed success path for registered tool %q", tool.Name)
		}
		if !errorCovered[tool.Name] {
			t.Errorf("air-gap fitness suite missed error path for registered tool %q", tool.Name)
		}
	}
}

func callToolViaStdio(t *testing.T, srv *mcp.Server, id int, name string, args any, sentinelPII *domain.PIIHeader) *mcp.CallToolResult {
	t.Helper()

	rawArgs, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal args for %s: %v", name, err)
	}
	reqBytes, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": json.RawMessage(rawArgs),
		},
	})
	if err != nil {
		t.Fatalf("marshal JSON-RPC request for %s: %v", name, err)
	}

	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), bytes.NewReader(append(reqBytes, '\n')), &out); err != nil {
		t.Fatalf("ServeStdio(%s) failed: %v", name, err)
	}

	rawResp := out.String()
	assertNoSentinelPII(t, rawResp, sentinelPII)

	var rpcResp struct {
		JSONRPC string              `json:"jsonrpc"`
		ID      json.RawMessage     `json:"id"`
		Result  *mcp.CallToolResult `json:"result"`
		Error   *mcp.RPCError       `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawResp)), &rpcResp); err != nil {
		t.Fatalf("unmarshal JSON-RPC response for %s: %v; raw=%s", name, err, rawResp)
	}
	if rpcResp.Error != nil {
		t.Fatalf("unexpected JSON-RPC protocol error for %s: %+v", name, rpcResp.Error)
	}
	if rpcResp.Result == nil {
		t.Fatalf("nil CallToolResult for %s; raw=%s", name, rawResp)
	}
	return rpcResp.Result
}
