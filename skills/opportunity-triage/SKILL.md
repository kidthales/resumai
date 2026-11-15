---
name: opportunity-triage
description: >-
  Loop 2 job description reconnaissance, citation-verified triage, and Go/No-Go
  decision workflow. Activate when the user shares a job description or asks
  to evaluate role fit, research a target company, save a Triaged application,
  or decline a role.
---

# Opportunity Triage (Loop 2)

Evaluate a target Job Description (JD) against the candidate's `CareerVault` and persist a citation-verified `OpportunityDossier` (`[*] -> Triaged`), or record a decline (`Triaged -> DeclinedByCandidate`).

## Deterministic Citation Invariants

`resumai_save_triage` enforces strict Go guards (`ErrInvalidTriageCitation`):
1. **`raw_jd` Required**: Must contain the full non-empty JD text.
2. **Exact Verbatim JD Quotes**: Every entry in `jd_claims` (`{category, summary, verbatim_jd_quote}`) MUST contain a `verbatim_jd_quote` that is an **exact substring** of `raw_jd` (CRLF normalized). Copy substrings verbatim; never paraphrase inside `verbatim_jd_quote`.
3. **External Claim Provenance**: Every entry in `external_claims` (`{category, claim, source_url, unknown_ask_recruiter}`) MUST either provide a valid `http://` or `https://` `source_url` from verified reconnaissance OR set `unknown_ask_recruiter: true` (or `source_url: "unknown_ask_recruiter"`). Never present unverified company claims as fact.

## Workflow

### 1. Gather JD & Vault Context
1. Obtain the complete `raw_jd`, `company`, and `role_title` from the user.
2. Inspect candidate baseline via MCP:
   - `resumai_get_logistics` — compare compensation, remote/hybrid/onsite policy, relocation, work authorization, and notice period.
   - `resumai_vault_status` and `resumai_list_archetypes` — check available personas.
   - `resumai_list_roles` and `resumai_list_facts` — identify matching experience and gaps.

### 2. Extract JD Claims & External Signals
- **JD Claims**: Extract required skills, preferred skills, seniority scope, logistics requirements, and red/green flags into `jd_claims` with exact `verbatim_jd_quote` substrings from `raw_jd`.
- **External Reconnaissance**: If verifying company/team context (e.g., engineering blog, product stage, funding, tech stack), cite each claim's exact `source_url`. For unanswered questions (team size, on-call rotation, comp band, visa sponsorship), record an `external_claims` entry with `unknown_ask_recruiter: true`.

### 3. Persist Triaged Dossier
Call `resumai_save_triage`:
- Pass `application_id` (omit on creation to auto-generate `YYYY-MM-DD-<company>-<role>`), `role_title`, `company`, `raw_jd`, `fit_summary`, `jd_claims`, and `external_claims` (either top-level or nested under `dossier`).
- Updating an existing `Triaged` application re-validates and updates the dossier cleanly.

### 4. Go / No-Go Decision Gate
Present the structured fit assessment:
- Logistics alignment vs. blockers.
- Strongest vault fact matches and critical skill gaps.
- Open `unknown_ask_recruiter` questions.

Ask the candidate whether to proceed or pass:
- **Decline (`Triaged -> DeclinedByCandidate`)**: Call `resumai_pass_role` with `application_id` and a non-empty `rationale` (terminal state).
- **Proceed**: Hand off `application_id` to the `resume-tailor` skill (Loops 3–4).
