---
name: career-archaeology
description: >-
  Loop 0 Socratic biographer workflow for onboarding, auditing, and curating
  non-PII career history in the ResumAI CareerVault. Activate when the user
  wants to populate or audit organizations, roles, atomic career facts,
  education, persona archetypes, or job-search logistics, or when resolving
  [METRIC: ...] placeholders.
---

# Career Archaeology (Loop 0)

Curate atomic, ground-truth career facts into the local SQLite `CareerVault` (`~/.resumai/vault.db`) through structured Socratic interviewing while keeping personal contact PII strictly air-gapped.

## Core Invariants

1. **PII Air-Gap**: NEVER ask the user to paste private contact PII (email, phone, street address, or personal URLs) into chat or MCP calls. Direct the user to configure contact info locally via the CLI:
   ```bash
   resumai pii set --name "..." --email "..." --phone "..." --location "..." [--linkedin "..."] [--github "..."] [--portfolio "..."]
   ```
   Verify locally with `resumai pii show` (or check `pii_configured` via `resumai_vault_status`).
2. **Zero Fabrication**: Never invent metrics, dates, titles, employers, or skills. If an accomplishment lacks a confirmed number, save it as `metric_pending` with a `[METRIC: ...]` token and ask the user to supply the real figure.
3. **Verified-Gate Enforcement**: `resumai_save_fact` rejects `epistemic_state: "verified"` whenever `headline` or `details` contains `[METRIC:`. Resolve all metric placeholders before marking a fact `verified`.
4. **Submission Provenance**: Never delete facts referenced by submitted applications; retire obsolete facts via `resumai_archive_fact`.

## Workflow

### 1. Audit Current Vault State
Inspect non-PII vault completeness before asking questions:
- `resumai_vault_status` — check counts (`organizations_count`, `roles_count`, `facts_by_state`, `education_count`, `archetypes_count`, `logistics_configured`, `pii_configured`).
- `resumai_get_logistics` — inspect job-search constraints.
- `resumai_list_organizations`, `resumai_list_roles`, `resumai_list_facts`, `resumai_list_education`, `resumai_list_archetypes` — review existing records.

If `pii_configured` is `false`, remind the user to run `resumai pii set` in their local terminal before final rendering.

### 2. Capture Non-PII Logistics
If `logistics_configured` is `false` or stale, ask the user and persist via `resumai_save_logistics`:
- `target_title`, `work_authorization`, `remote_preference`, `willing_to_relocate` (`bool`), `target_compensation`, `notice_period_days` (`int`).

### 3. Curate Organizations & Roles
1. Save each employer/institution via `resumai_save_organization` (`id`, `name`, `location`, `website`).
2. Save each position under its `org_id` via `resumai_save_role` (`id`, `org_id`, `title`, `category` such as `work`, `extra`, or `volunteer`, `location`, `duration`, `tech_stack`).

### 4. Extract Atomic Career Facts (Google X-Y-Z)
Interview the user role-by-role to break broad responsibilities into atomic facts shaped as **Accomplished [X], as measured by [Y], by doing [Z]**. Persist each via `resumai_save_fact` (`id`, `role_id`, `headline`, `details`, `epistemic_state`, `tech_tags`):
- `raw_note`: Rough bullet or unverified memory captured during discovery.
- `metric_pending`: Accomplishment needs quantification (`[METRIC: e.g. % latency reduction]`).
- `verified`: Confirmed by the user with concrete, accurate figures and zero `[METRIC:` tokens.
- `archived`: Retired via `resumai_archive_fact` (`fact_id`).

### 5. Curate Education & Persona Archetypes
1. Save credentials via `resumai_save_education` (`id`, `institution`, `degree`, `field_of_study`, `location`, `duration`, `honors`).
2. Define target personas via `resumai_save_archetype` (`id`, `name`, `profile_overview`, `core_competencies`, `strategic_impact`, `target_roles`).

## Verification
Re-run `resumai_vault_status` and `resumai_list_facts` (`epistemic_state: "metric_pending"` / `"raw_note"`) to confirm remaining gaps and report what is ready for triage and tailoring.
