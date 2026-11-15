---
name: resume-tailor
description: >-
  Loops 3 & 4 tailored resume drafting, clean-room dual-critic orchestration,
  verified promotion, headless Chrome CDP PDF/HTML/DOCX typesetting, and
  immutable submission locking. Activate when drafting, auditing, editing,
  rendering, or finalizing a resume for a Triaged/Drafted/Audited/Exported
  application.
---

# Resume Tailor & Typesetter Press (Loops 3 & 4)

Guide a `Triaged` `JobApplication` through grounded AST drafting, clean-room critic audits, verified promotion, local PII-injected rendering, and immutable submission.

## Non-Negotiable Invariants

1. **Fact Grounding (`ErrUngroundedBullet`)**: Every bullet in `ApplicationDraft` (`experience`, `education`, `projects`, `custom_sections`) MUST cite at least one existing `fact_ids` entry in `CareerVault`.
2. **Fact-over-Keyword Precedence**: When `resume-alignment-checker` keyword/framing recommendations conflict with `resume-fact-checker` truth, timeline, or metric constraints, **factual accuracy always takes precedence**.
3. **Zero Unresolved Placeholders (`ErrUnverifiedFactOrPlaceholder`)**: `resumai_promote_ready` requires `0` High factual discrepancies, both critic audits `passed: true`, `100%` of cited facts in `epistemic_state: "verified"`, and `0` `[METRIC:` tokens across headline, summary, and bullets.
4. **PII Air-Gap**: Never include contact PII in `ApplicationDraft` or critic prompts. `resumai_render_bundle` injects `pii_header` locally from SQLite.
5. **Immutable Submission (`ErrAlreadySubmitted`)**: `resumai_mark_submitted` locks packet artifacts read-only (`0444`) and transitions `Exported -> Submitted` irreversibly.

## Workflow

### 1. Inspect Application & Vault Ground Truth (Loop 3)
- Call `resumai_get_application` (`application_id`) to load `raw_jd`, `dossier`, and current `state`.
- Call `resumai_list_archetypes`, `resumai_list_roles`, `resumai_list_facts`, and `resumai_list_education`.
- Select the single best `archetype_id` in strict order: (1) IC vs. Management gate, (2) Seniority & Scope alignment, (3) Tech Stack overlap.

### 2. Author Grounded `ApplicationDraft` (`Triaged -> Drafted`)
Construct the `draft` object using exact `ApplicationDraft` JSON keys (do **not** use guessed keys such as `bullets`, `title`, or `company`, or a flat `string[]` for `skills`):
- `archetype_id`: Selected archetype ID.
- `headline`, `summary`: Tailored 2–3 sentence summary grounded in verified career history.
- `experience[]`: `{id?, role_title, organization, location?, start_date?, end_date?, duration?, highlights: [{id, text, fact_ids}]}`
- `education[]`: `{id?, institution, degree, field_of_study?, location?, start_date?, end_date?, duration?, honors?, highlights?: [{id, text, fact_ids}]}`
- `skills[]`: `{name, skills: string[]}`
- `projects[]`: `{name, role?, url?, duration?, technologies?: string[], highlights?: [{id, text, fact_ids}]}`
- `certifications`: `string[]`
- `custom_sections[]`: `{title, highlights: [{id, text, fact_ids}]}`
- `page_budget`: `1` or `2`.
- Frame every `highlights[]` bullet using Google X-Y-Z (**Accomplished [X], as measured by [Y], by doing [Z]**) with `id`, `text`, and non-empty `fact_ids`.

Call `resumai_save_draft` (`application_id`, `draft`). Note: saving a draft from `Drafted`, `Audited`, `ReadyToRender`, or `Exported` resets the state to `Drafted` and clears prior audits/exports.

### 3. Dispatch Clean-Room Critic Audits (`Drafted -> Audited`)
Invoke **both** clean-room subagents independently with `Model: "flash"` (required for `GUARD-12` / `LINT-MODEL-01`), passing **only** `application_id` (and no parent assumptions or inline draft overrides) so each critic fetches the air-gapped ground truth directly via `resumai_get_audit_bundle`:
1. `resume-fact-checker` (`Model: "flash"`) -> produces `FactCheckReport` (`critic_agent: "resume-fact-checker"`, `summary`, `discrepancies`, `passed`).
2. `resume-alignment-checker` (`Model: "flash"`) -> produces `AlignmentCheckReport` (`critic_agent: "resume-alignment-checker"`, `summary`, `matched_signals`, `missing_signals`, `recommendations`, `passed`).

Persist both reports via `resumai_record_audits` (`application_id`, `fact_check`, `alignment_check`).

### 4. Reconcile & Promote (`Audited -> ReadyToRender`)
- If either critic reported `passed: false`, any `severity: "high"` discrepancy exists, or any cited fact is `raw_note` / `metric_pending` / contains `[METRIC:`:
  1. Resolve missing metrics or factual edits with the user and update facts via `resumai_save_fact`.
  2. Apply revisions honoring **Fact-over-Keyword precedence** and save via `resumai_save_draft`.
  3. Re-run both clean-room critics and `resumai_record_audits`.
- Once all discrepancies are resolved and all cited facts are `verified`, call `resumai_promote_ready` (`application_id`).

### 5. Typeset & Verify Page Physics (`ReadyToRender -> Exported`) (Loop 4)
1. Confirm `pii_configured: true` via `resumai_vault_status`. If `false`, instruct the user to run `resumai pii set ...` in their local terminal.
2. Call `resumai_render_bundle` (`application_id`, optional `theme`: `"default"`, `"modern"`, or `"classic"`, optional `output_dir` relative subdirectory inside `~/.resumai/applications` or `RESUMAI_APPLICATIONS_DIR`).
3. Inspect `export` (`packet_dir`, `html_path`, `docx_path`, `pdf_path`, `page_count`, `max_pages`) and `telemetry` (`last_page_fill_ratio`, `overflow_px`, `orphan_bullets`).
4. If `ErrPageBudgetOverflow` occurs or telemetry shows awkward trailing whitespace / orphan bullets, tighten bullets or adjust `page_budget` via `resumai_save_draft`, re-audit, re-promote, and re-render.

### 6. Lock Submitted Packet (`Exported -> Submitted`)
Only after the user confirms they have reviewed the exported artifacts and wants to lock the submission:
- Call `resumai_mark_submitted` (`application_id`, optional `notes` for `interview_brief.md`).
- Report the generated `bundle_sha256`, `interview_brief_path`, and read-only packet directory.
