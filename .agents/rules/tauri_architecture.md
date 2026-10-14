---
description: Tauri v2 Rust backend and Vanilla TypeScript frontend architecture invariants for ResumAI
globs: ["src/**/*", "src-tauri/**/*", "content/**/*"]
---

# ResumAI Tauri v2 Architecture Rules

## 1. Runtime PII Prompt Interpolation (Never Compile-Time)
- Unlike v1 Symfony DI (`%env(resolve:file:...)%`), which crashed when a user's `content/pii/*.md` contained literal `%` characters (e.g. `"Improved latency by 45%"`), `src-tauri/src/vault.rs` MUST always interpolate `%pii.personal_info%`, `%pii.education_history%`, `%pii.work_history%`, and `%pii.extras%` via **single-pass literal replacement at runtime**.
- Reading PII and Prompts at execution time ensures changes made in the Vault Editor take effect on the very next agent call without restarting the app or clearing caches.

## 2. Sample vs. Custom Vault File Resolution
- **PII Files (`content/pii/`)**: For each category (`personal_info`, `education_history`, `work_history`, `extras`), resolve `<name>.md` first; if it does not exist or is empty, fall back transparently to `<name>.sample.md`.
- **Archetype Discovery (`content/archetypes/`)**: In `list_archetypes`, if at least one non-sample `*.md` file (not ending in `.sample.md`) exists in `content/archetypes/`, exclude all `*.sample.md` files from the returned list.

## 3. Dual-Checker Precedence & Concurrency
- **Concurrency**: Stage 3a (`resume_fact_checker`) and Stage 3b (`resume_job_alignment_checker`) depend only on the Stage 2 draft and JD. Execute them concurrently via `tokio::join!` in full-pipeline or parallel-check runs.
- **Truthfulness Hierarchy**: In `resume_editor`, instructions from the **Fact-Check Report** strictly override the **Job-Alignment Report**. Never allow alignment suggestions to introduce unverified skills or metrics.
