---
description: Tauri v2 Rust backend, Svelte 5 (Runes), and @kidthales/picotol frontend architecture invariants for ResumAI
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

## 4. Pure-Rust OpenXML DOCX Export Fidelity (Zero External CLI Dependencies)
- **Zero Pandoc / LibreOffice**: The export engine (`src-tauri/src/export.rs`) must remain pure Rust using `zip` and manual WordprocessingML generation to ensure zero desktop environment dependencies.
- **Table Layout Differentiation**:
  - **2-Column Header Grids**: Format role/company/date/location headers with borderless `<w:tblBorders>`, a 65%/35% column width ratio (`7020` / `3780` dxa of 10,800 dxa body width), and right-aligned text in Column 2 (`<w:jc w:val="right"/>`).
  - **3+ Column Matrices**: Format skills and data matrices with subtle horizontal gridlines (`#D1D5DB`/`#E5E7EB`), omitted outer vertical borders, and evenly distributed column widths.
- **Callouts & Monospace Runs**:
  - Blockquotes (`> text`) must emit `<w:pBdr><w:left w:val="single" w:sz="24" w:space="12" w:color="004D40"/>` accent borders with 360 dxa indentation.
  - Code spans (``` `code` ```) must emit Consolas monospace runs (`<w:rFonts w:ascii="Consolas"/><w:sz w:val="19"/>`).
- **Preview Parity**: Any changes to WordprocessingML styling must have corresponding classes in `resume.sample.css` (`.table-borderless`, `.table-header-grid`, `.table-matrix`, `blockquote`) to maintain 1:1 preview-to-export parity.

## 5. Deterministic Page-Budget & Surgical PageFitOptimizer Invariants
- **Line-Delta Budget Formula**: Compute budget telemetry deterministically using standard line height (18px) against 1-page (1,056px) or 2-page (2,112px) envelopes:
  $$\Delta \text{lines} = \text{round}\left(\frac{\text{scrollHeightPx} - \text{targetHeightPx}}{18}\right)$$
- **Refinement Trigger**: Trigger UI visual alerts (`needsRefinement = true`) when $|\Delta \text{lines}| \ge 1$.
- **PageFitOptimizer Invariant**: When invoking `AgentStage::PageFitOptimizer` (`temperature = 0.15`, `top_p = 0.85`, Gemini Flash):
  - Strictly enforce preservation of all facts, metrics, percentages, dates, employer names, and bullet items.
  - Only compress or expand phrasing, conjunctions, and sentence structure to achieve the requested line delta.

## 6. Svelte 5 (Runes) & `@kidthales/picotol` Frontend Invariants
- **Zero-Runtime Reactive State (`src/lib/state.svelte.ts`)**: All shared studio state lives in the `ResumaiState` class using Svelte 5 `$state` and `$derived` runes.
- **Avoid `store_rune_conflict` in `.svelte` Components**: Never import the shared state singleton under the identifier `state` inside a `.svelte` file that also uses the `$state(...)` rune (Svelte 5 treats `$state` as an ambiguous store auto-subscription on `state`). Always alias the import:
  ```ts
  import { state as studio } from "../lib/state.svelte";
  ```
- **Synchronous DOM Flushing (`flushSync`)**: Wrap synchronous UI state transitions (`setTab`, `setTheme`, `toggleConsole`, etc.) and command lifecycle updates in `flushSync()` from `'svelte'` so both interactive users and synchronous Vitest DOM assertions observe immediate DOM updates without requiring microtask ticks.
- **Design Tokens & Accessibility (`@kidthales/picotol`)**: Import `@kidthales/picotol/tokens` and `@kidthales/picotol/palettes` in `src/styles/main.css` from the vendored tarball (`file:./vendor/kidthales-picotol-0.1.0.tgz`). Map semantic state colors to Paul Tol CVD-safe tokens (`--tol-vibrant-blue`, `--tol-vibrant-teal`, `--tol-vibrant-orange`, `--tol-vibrant-red`, `--tol-vibrant-cyan`).
- **Vite 6 & Vitest Client Resolution**: Keep `@sveltejs/vite-plugin-svelte` on `^5.0.0` while using `vite@^6.1.0`, and include `svelteTesting()` from `@testing-library/svelte/vite` in `vite.config.ts` so Vitest mounts Svelte 5 components using the browser bundle in `happy-dom`.
