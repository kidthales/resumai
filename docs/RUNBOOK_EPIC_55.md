# Epic 55 Runbook: ResumAI v0.2.0 Architecture & Implementation

## 1. Overview & Architectural Parti

ResumAI `v0.2.0` replaces the legacy prototype with a high-performance, single-static-binary pure-Go kernel and an Antigravity Plugin.

### Governing Metaphor: "The Biographer's Vault & Typesetter's Press"
- **The Biographer's Vault**: An interactive Socratic biographer (Antigravity Plugin Skills + Clean-Room Critics) curates atomic, ground-truth career accomplishments into an embedded SQLite database (`vault.db`), keeping contact PII strictly air-gapped on the local filesystem away from LLM prompts.
- **The Typesetter's Press**: A deterministic typesetter (`resumai` Go CLI and stdio MCP server) validates citation provenance, enforces strict page-budget physics via headless Chrome CDP telemetry (`Range.getClientRects()`), and presses pixel-verified `.html`, `.pdf`, and `.docx` packets while preserving linear ATS reading order.

---

## 2. Core Invariants & Governance Rules

1. **Pillar 1: Boundary Purity (Hexagonal Architecture)**
   - `internal/domain` must NEVER import `internal/vault`, `internal/render`, `internal/mcp`, or `cmd/resumai`.
   - Verified continuously via `internal/arch_test.go` using Go AST parsing (`go/parser`).
2. **Pillar 2: Data Architecture & Zero-Panic**
   - Pure-Go SQLite (`modernc.org/sqlite` or pure-Go driver) without CGO dependencies.
   - Strict error propagation (`Result/error`), zero `panic()` in production code paths.
3. **Pillar 3: PII Air-Gap**
   - Personal identifiable information (phone, address, email) is stored exclusively in local vault storage and injected only during final local rendering.
   - PII is structurally unreachable over the MCP server protocol and never included in prompt context.
4. **Pillar 4: Ground-Truth Bullet Provenance**
   - No resume bullet may transition to `ReadyToRender` without citing `verified` `career_facts` rows.
   - `0` `[METRIC:` placeholders permitted upon promotion.
5. **Pillar 5: Zero Context Bloat**
   - Plugin operates with `0`-byte always-on `rules/AGENTS.md` footprint.
   - Capabilities are progressively disclosed via specialized skills and clean-room subagents.

---

## 3. Hexagonal Directory Topology

```
plugins/resumai/
├── cmd/
│   └── resumai/             # CLI & stdio MCP server entrypoint
│       └── main.go
├── internal/
│   ├── domain/              # Core business entities, FSM states, sentinels
│   │   └── application.go
│   ├── vault/               # Pure-Go SQLite CareerVault, YAML dump/load
│   │   └── vault.go
│   ├── render/              # ResumeDocument AST, HTML/OpenXML/PDF rendering
│   │   └── render.go
│   ├── mcp/                 # Stdio MCP server protocol coordination
│   │   └── mcp.go
│   ├── version/             # Semantic version definition
│   │   └── version.go
│   └── arch_test.go         # Hexagonal boundary purity unit tests
├── docs/
│   └── RUNBOOK_EPIC_55.md   # Architectural runbook and sub-issue roadmap
├── scripts/
│   ├── build.ps1            # Lock-rotating build script
│   └── ci-local.ps1         # Local CI runner (vet, test, race, build)
├── plugin.json              # Antigravity plugin manifest (starts disabled)
├── go.mod                   # Go module definition (github.com/kidthales/resumai)
└── LICENSE                  # AGPL-3.0-or-later
```

---

## 4. Entity-State Lifecycle Matrix (`JobApplication`)

| Current State | Action / MCP Tool | Target State | Deterministic Guard / Invariant | Rejection Result |
| :--- | :--- | :--- | :--- | :--- |
| `[*]` | `resumai_save_triage` | `Triaged` | `raw_jd` non-empty; quotes match raw JD verbatim; external claims cite URL or note unknown | `ErrInvalidTriageCitation` |
| `Triaged` | `resumai_pass_role` | `DeclinedByCandidate` | Non-empty rejection rationale recorded | `ErrMissingRationale` |
| `Triaged` | `resumai_save_draft` | `Drafted` | Valid `archetype_id`; all bullet `fact_ids` resolve to real career facts | `ErrUngroundedBullet` |
| `Drafted` | `resumai_record_audits` | `Audited` | Both `resume-fact-checker` and `resume-alignment-checker` reports attached | `ErrIncompleteAudit` |
| `Audited` | `resumai_promote_ready` | `ReadyToRender` | `0` High factual discrepancies; 100% cited facts `verified`; `0` `[METRIC:` placeholders | `ErrUnverifiedFactOrPlaceholder` |
| `ReadyToRender` | `resumai_render_bundle` | `Exported` | Local PII injected; `.html`, `.docx`, `.pdf` generated; `page_count <= max_pages` | `ErrPageBudgetOverflow` |
| `Exported` | `resumai_mark_submitted` | `Submitted` | SHA-256 bundle hash + `interview_brief.md` locked immutable | `ErrAlreadySubmitted` |

---

## 5. Sub-Issue Implementation Roadmap

- **#56 [Active]**: Demolish legacy stack, bootstrap Go module, hexagonal skeleton, boundary tests, runbook, build/CI scripts, and plugin manifest.
- **#57 [Queued]**: Implement pure-Go SQLite CareerVault, CLI PII air-gap, YAML dump/load, and legacy data migration.
- **#58 [Queued]**: Implement ResumeDocument AST validator, embedded HTML and OpenXML `.docx` renderers, and Chrome CDP telemetry.
- **#59 [Queued]**: Implement `resumai` stdio MCP server, triage citation guards, and `JobApplication` FSM transitions.
- **#60 [Queued]**: Author Antigravity plugin skills (`resume-biographer`, `job-triager`, `resume-builder`), clean-room subagents, and release workflow.

---

## 6. Developer Runbook & Verification

### Running Unit & Architecture Tests
```powershell
go -C plugins/resumai test ./...
```

### Local Compilation
```powershell
go -C plugins/resumai build -o bin/resumai.exe ./cmd/resumai
```

### Executing Local CI Suite
```powershell
pwsh ./plugins/resumai/scripts/ci-local.ps1
```
