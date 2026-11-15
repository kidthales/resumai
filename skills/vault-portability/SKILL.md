---
name: vault-portability
description: >-
  Loop 1 CLI runbook for backing up, restoring, and migrating the ResumAI
  CareerVault (`resumai dump`, `resumai load`, `resumai import-legacy`, and
  `resumai pii`). Activate when the user wants YAML vault backups, legacy
  v0.1.0 Markdown migration, or local PII CLI management.
---

# Vault Portability & PII CLI (Loop 1)

Manage portable YAML vault bundles, `v0.1.0` Markdown imports, and air-gapped candidate PII using the local `resumai` CLI.

## Core Invariants

1. **CLI-Only PII Boundary**: Contact PII (`pii_header`) is stored in SQLite and accessed strictly through `resumai pii` and `resumai dump --include-pii` / `resumai load`. Never ask the user to paste `pii.yaml` or raw `resumai pii show` output into chat.
2. **Default Air-Gapped Dumps**: `resumai dump <dir>` omits `pii.yaml` by default so YAML bundles can be inspected or edited safely. Only pass `--include-pii` for private local backups that will never be committed to public VCS.
3. **Submitted Fact Protection**: `resumai load <dir>` preserves existing `pii_header` if `pii.yaml` is absent and automatically soft-archives (`archived`) any fact cited by a `Submitted` application if removed from the YAML bundle.
4. **Global Vault Override**: Any command accepts `--vault <path>` to target a non-default database instead of `~/.resumai/vault.db`.

## Workflows

### 1. Manage Air-Gapped Candidate PII
- Inspect configured contact fields locally:
  ```bash
  resumai pii show [--json] [--vault <path>]
  ```
- Set or update individual contact fields (unspecified fields remain unchanged):
  ```bash
  resumai pii set \
    --name "Jane Doe" \
    --email "jane@example.com" \
    --phone "+1-555-0100" \
    --location "Seattle, WA" \
    --linkedin "https://linkedin.com/in/janedoe" \
    --github "https://github.com/janedoe" \
    --portfolio "https://janedoe.dev"
  ```
  Supported aliases: `--full-name`, `--linkedin-url`, `--github-url`, `--portfolio-url`.

### 2. Export Vault to Structured YAML (`resumai dump`)
- Safe non-PII bundle (omits `pii.yaml`):
  ```bash
  resumai dump ./vault-backup [--vault <path>]
  ```
- Full local backup including `pii.yaml` (keep private / gitignored):
  ```bash
  resumai dump ./vault-backup --include-pii [--vault <path>]
  ```

### 3. Restore or Sync from YAML (`resumai load`)
After editing YAML files (`logistics.yaml`, `organizations/`, `education/`, `archetypes/`, `applications/`) or restoring from backup:
```bash
resumai load ./vault-backup [--vault <path>]
```

### 4. Migrate Legacy v0.1.0 Markdown Vault (`resumai import-legacy`)
Import `personal_info.md`, `education_history.md`, `work_history.md`, `extras.md`, and `archetypes/*.md`:
```bash
resumai import-legacy ./legacy-content [--default-state verified|raw_note] [--skip-pii] [--vault <path>]
```
- Use `--default-state raw_note` when imported bullets still need Socratic metric verification before rendering.
- Use `--skip-pii` when importing shared/sample directories or keeping existing local PII untouched.

## Verification
Call `resumai_vault_status` via MCP after any `load` or `import-legacy` run to confirm non-PII record counts and `pii_configured`.
