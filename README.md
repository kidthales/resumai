# ResumAI (`v0.2.0`)

[![CI](https://github.com/kidthales/resumai/actions/workflows/ci.yaml/badge.svg)](https://github.com/kidthales/resumai/actions/workflows/ci.yaml)

**The Biographer's Vault & Typesetter's Press** — an Antigravity Plugin paired with a single-static pure-Go CLI and stdio MCP kernel (`resumai`) for verifiable, ATS-linearized resume generation.

## Architecture Overview

ResumAI `v0.2.0` replaces the legacy `v0.1.0` Docker/PHP/Symfony/Pandoc stack with two tightly bounded components:

- **Pure-Go CLI & Stdio MCP Kernel (`cmd/resumai`)**:
  - Embedded pure-Go SQLite `CareerVault` (`modernc.org/sqlite`, zero CGO required).
  - Structural **PII Air-Gap**: contact PII (`pii_header`) is stored in SQLite, managed strictly via local CLI commands (`resumai pii`), and injected only during local file rendering (`resumai_render_bundle`). It is structurally unreachable over MCP.
  - Deterministic `JobApplication` finite-state machine (`Triaged` -> `Drafted` -> `Audited` -> `ReadyToRender` -> `Exported` -> `Submitted` / `DeclinedByCandidate`).
  - Embedded HTML and pure-Go OpenXML `.docx` renderers (`default`, `modern`, and `classic` themes) plus headless Chromium CDP `.pdf` printing and DOM layout telemetry (`Range.getClientRects()`).
- **Antigravity Plugin (`plugin.json`, `mcp_config.json`)**:
  - Progressive-disclosure **Skills** (`skills/`) covering Loops 0–4 with **zero** always-on `rules/AGENTS.md` context bloat.
  - Clean-room read-only **Subagents** (`agents/resume-fact-checker.md`, `agents/resume-alignment-checker.md`) enforcing strict **Fact-over-Keyword** precedence via `resumai_get_audit_bundle`.

## Requirements

- **`resumai` binary** on `PATH` (for CLI usage and `mcp_config.json`: `"command": "resumai", "args": ["mcp"]`).
- **Google Chrome or Microsoft Edge** on the host for `.pdf` export and page-budget DOM telemetry (optional override: `RESUMAI_CHROME_BIN` or `CHROME_BIN`).
- **Building from source (optional)**: Go `1.26.7+` (`go.mod`) and PowerShell (`pwsh`) for `./scripts/build.ps1` and `./scripts/ci-local.ps1`.

## Installation

### Option 1: Prebuilt Release Binary (Recommended)

Download the binary for your platform and `SHA256SUMS` from [GitHub Releases](https://github.com/kidthales/resumai/releases):

- Windows (`amd64`): `resumai-windows-amd64.exe`
- Linux (`amd64` / `arm64`): `resumai-linux-amd64`, `resumai-linux-arm64`
- macOS (`amd64` / `arm64`): `resumai-darwin-amd64`, `resumai-darwin-arm64`

Verify the SHA-256 checksum, place the binary on your `PATH` as `resumai` (`resumai.exe` on Windows), and confirm installation:

```shell
# Linux / macOS
sha256sum -c SHA256SUMS --ignore-missing
chmod +x resumai-linux-amd64
mv resumai-linux-amd64 ~/.local/bin/resumai
resumai version
```

```powershell
# Windows (PowerShell)
Get-FileHash .\resumai-windows-amd64.exe -Algorithm SHA256
Move-Item .\resumai-windows-amd64.exe "$env:USERPROFILE\go\bin\resumai.exe"
resumai version
```

### Option 2: Build from Source

```powershell
git clone https://github.com/kidthales/resumai.git
cd resumai
pwsh ./scripts/build.ps1
resumai version
```

`scripts/build.ps1` runs unit tests, builds `bin/resumai(.exe)` with version metadata from `plugin.json`, and installs a copy to `GOPATH/bin` (default `~/go/bin`).

### Option 3: Antigravity Plugin Activation

Install or link this repository as an Antigravity plugin (`plugin.json` is enabled with `"disabled": false`). Antigravity automatically starts the stdio MCP server using `mcp_config.json`:

```json
{
  "mcpServers": {
    "resumai": {
      "command": "resumai",
      "args": ["mcp"]
    }
  }
}
```

## Quickstart: CLI Onboarding

By default, ResumAI stores your SQLite vault at `~/.resumai/vault.db` (override with `--vault <path>` or `RESUMAI_VAULT_PATH`).

### 1. Configure Air-Gapped Contact PII (`resumai pii`)

Contact details never enter LLM prompts or MCP responses. Set them locally in your terminal:

```shell
resumai pii set \
  --name "Jane Doe" \
  --email "jane@example.com" \
  --phone "+1-555-0100" \
  --location "Seattle, WA" \
  --linkedin "https://linkedin.com/in/janedoe" \
  --github "https://github.com/janedoe" \
  --portfolio "https://janedoe.dev"

resumai pii show
```

Supported aliases: `--full-name`, `--linkedin-url`, `--github-url`, `--portfolio-url`, plus `--json` on `resumai pii show`.

### 2. Migrate from `v0.1.0` Markdown (Optional)

Import legacy `personal_info.md`, `education_history.md`, `work_history.md`, `extras.md`, and `archetypes/*.md`:

```shell
resumai import-legacy ./content --default-state raw_note
```

- `--default-state verified|raw_note`: set to `raw_note` (recommended) when imported bullets need metric verification before rendering.
- `--skip-pii`: omit `personal_info.md` import when preserving existing local PII or testing sample files.

### 3. Back Up & Edit Structured YAML (`dump` / `load`)

```shell
# Export non-PII vault records to YAML (omits pii.yaml by default)
resumai dump ./vault-backup

# Include pii.yaml ONLY for private local backups (never commit to public VCS)
resumai dump ./vault-backup --include-pii

# Restore or sync edited YAML back into SQLite
resumai load ./vault-backup
```

`resumai load` preserves existing `pii_header` when `pii.yaml` is absent and automatically soft-archives (`archived`) any fact cited by a `Submitted` application if removed from the YAML bundle.

### 4. Run the Stdio MCP Server Manually (Optional)

```shell
resumai mcp [--vault <path>]
```

## Loops 0–4 Plugin Workflow

Once the plugin is active in Antigravity, the four progressive-disclosure skills and two clean-room critic subagents guide applications end-to-end:

| Loop | Skill / Agents | Purpose & Key Transitions |
| :--- | :--- | :--- |
| **Loop 0** | `skills/career-archaeology` | Socratic interview to curate organizations, roles, Google X-Y-Z atomic facts (`raw_note`, `metric_pending`, `verified`, `archived`), education, job-search logistics, and persona archetypes. |
| **Loop 1** | `skills/vault-portability` | Local CLI management for `resumai pii`, `resumai dump`, `resumai load`, and `resumai import-legacy`. |
| **Loop 2** | `skills/opportunity-triage` | JD reconnaissance (`[*] -> Triaged` via `resumai_save_triage`, enforcing verbatim `raw_jd` quotes and cited URLs / `unknown_ask_recruiter`) and Go/No-Go decision (`Triaged -> DeclinedByCandidate` via `resumai_pass_role`). |
| **Loop 3** | `skills/resume-tailor` + `agents/resume-fact-checker` + `agents/resume-alignment-checker` | Grounded `ApplicationDraft` authoring (`Triaged -> Drafted`), clean-room dual-critic audits via `resumai_get_audit_bundle` (`Drafted -> Audited`) under strict **Fact-over-Keyword** precedence, and verified promotion (`Audited -> ReadyToRender` with `0` High discrepancies, `100%` `verified` facts, and `0` `[METRIC:` tokens). |
| **Loop 4** | `skills/resume-tailor` | Local PII-injected typesetting (`ReadyToRender -> Exported` via `resumai_render_bundle` into `~/.resumai/applications` or `RESUMAI_APPLICATIONS_DIR` with `.html`, `.docx`, `.pdf`, and Chrome CDP telemetry) and immutable packet lock (`Exported -> Submitted` via `resumai_mark_submitted` with SHA-256 hash and `interview_brief.md`). |

### Registered MCP Tools (`resumai mcp`)

- **CareerVault**: `resumai_vault_status`, `resumai_get_logistics`, `resumai_save_logistics`, `resumai_list_organizations`, `resumai_save_organization`, `resumai_list_roles`, `resumai_save_role`, `resumai_list_facts`, `resumai_save_fact`, `resumai_archive_fact`, `resumai_list_education`, `resumai_save_education`, `resumai_list_archetypes`, `resumai_save_archetype`
- **JobApplication FSM & Press**: `resumai_list_applications`, `resumai_get_application`, `resumai_save_triage`, `resumai_pass_role`, `resumai_save_draft`, `resumai_get_audit_bundle`, `resumai_record_audits`, `resumai_promote_ready`, `resumai_render_bundle`, `resumai_mark_submitted`

## Development & Verification

Run the single-source local CI pipeline (workflow Gate 0/1 checks, `gofmt`, `go vet`, unit & hexagonal architecture boundary tests, `govulncheck`, `-race`, PowerShell AST checks, and build verification):

```powershell
pwsh ./scripts/ci-local.ps1
```

For quick iteration during development:

```powershell
go test ./...
```

## License

ResumAI is licensed under the GNU Affero General Public License v3.0 or later (`AGPL-3.0-or-later`). See [LICENSE](./LICENSE) for the full license text.
