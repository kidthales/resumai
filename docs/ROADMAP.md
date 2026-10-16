# ResumAI v2 Product & Engineering Roadmap

This document tracks the phased evolution of **ResumAI v2** from a personal power-user Career & Consulting Studio to a polished, cross-platform desktop application accessible to non-technical users.

---

## Phase 1: Power-User Career & Consulting Studio (Implemented in `main-v2`)

Designed for rapid daily iteration across senior engineering job searches and technical/studio consulting proposals:

- [x] **Local-First Career Vault (`content/` & `assets/`)**:
  - Full compatibility with `content/pii/*.md`, `content/archetypes/*.md` (including `technical_consultant.sample.md`), `content/prompts/*.md`, and `assets/styles/resumes/*.css`.
  - Safe runtime PII interpolation (immune to `%` character escaping bugs).
  - Sample-to-custom fallback (`*.md` overrides `*.sample.md` automatically).
- [x] **Multi-Provider Streaming Agent Engine (`src-tauri/src/llm.rs`)**:
  - Native streaming support for **Ollama** (`localhost:11434`, `qwen2.5:14b-instruct-q5_K_M`) and **Google Gemini** (`gemini-3.5-flash` / `gemini-3.5-flash-lite`) with real-time `.thonk` reasoning and Markdown delta events.
  - Automatic exponential backoff retry on transient `429`/`503` errors.
  - Autonomous 2-phase tool execution (`list_archetypes` / `read_archetype`) for Stage 1 Archetype Selection.
- [x] **Parallel Verification & Full Pipeline Orchestration (`src-tauri/src/pipeline.rs`)**:
  - Run all stages with 1 click (**Full Pipeline**) or step-by-step (**1. Archetype** $\rightarrow$ **2. Draft** $\rightarrow$ **3. Parallel Checks** $\rightarrow$ **4. Edit** $\rightarrow$ **5. Cover Letter / Consulting Pitch**).
  - Concurrent execution of **Fact-Checker** and **Job-Alignment Checker** via `tokio::join!`.
  - Interactive `[METRIC: ...]` placeholder detector with 1-click write-back to `content/pii/work_history.md`.
- [x] **Live Paginated Preview & Zero-Container Export (`src-tauri/src/export.rs`)**:
  - Real-time `<iframe srcdoc>` preview with live **Page-Budget Meter** (warns on page overflow) and live CSS typography controls (font size, line height, page margins).
  - Native **Print-to-PDF**, standalone **HTML**, and pure-Rust **DOCX** export into organized `var/applications/<slug>/` workspace folders.

---

## Phase 2: Document Layout Polish & Page-Budget Optimization

- [x] **Cover Letter & Consulting Proposal Generator**:
  - Synthesizes a grounded cover letter or fractional/consulting capability pitch tailored to the JD/RFP and selected archetype.
- [x] **Enhanced Pure-Rust OpenXML DOCX Layout Fidelity ([#21](https://github.com/kidthales/resumai/issues/21))**:
  - Rich WordprocessingML generation: Markdown table parsing (`<w:tbl>`), borderless 2-column header layout grids, left-bordered callout boxes (`>`), and preserved inline formatting (`<w:b>`, `<w:i>`, `<w:rFonts>`).
- [x] **Automated Page-Fit Prompt Feedback Loop ([#23](https://github.com/kidthales/resumai/issues/23))**:
  - Live feedback loop linking the Letter 1056px Page-Budget meter with the Editor Agent to automatically condense or expand bullet phrasing for exact 1.00 or 2.00 page budgets.

---

## Phase 3: Consulting Intelligence & Application CRM (Deferred from Phase 2)

- [ ] **Interview & Client Discovery Gap-Defense Brief ([#22](https://github.com/kidthales/resumai/issues/22))** *(Deferred from Phase 2)*:
  - Dedicated agent that converts the Job-Alignment Checker's *Critical & Secondary Skill Gaps* into a 1-page interview preparation sheet linking missing keywords to adjacent systems in `work_history.md`.
- [ ] **Application & Consulting Lead CRM Board ([#24](https://github.com/kidthales/resumai/issues/24))** *(Deferred from Phase 2)*:
  - Status pipeline view (*Drafted* $\rightarrow$ *Applied / Pitched* $\rightarrow$ *Interviewing* $\rightarrow$ *Offer / Won* $\rightarrow$ *Archived*) with search and match-score analytics across `var/applications/`.

---

## Phase 4: Non-Technical User Onboarding & Distribution

- [ ] **Guided Onboarding Wizard ("Import Existing Resume")**:
  - Drag-and-drop an existing `.pdf` or `.docx` resume (or LinkedIn profile PDF).
  - Onboarding LLM agent automatically extracts and populates `personal_info.md`, `work_history.md`, `education_history.md`, and generates 2–3 tailored candidate archetypes.
- [ ] **Visual Form Editor Mode**:
  - Structured card-based forms for editing Work History roles, dates, technology tags, and XYZ bullets alongside the raw Markdown editor.
- [ ] **OS Keyring & 1-Click Local AI Setup**:
  - Store cloud API keys in Windows Credential Manager / macOS Keychain / Linux Secret Service (`keyring` crate).
  - Built-in Ollama status detector and 1-click model puller with download progress bar.
- [ ] **Signed Cross-Platform Installers & Auto-Updater**:
  - GitHub Actions release pipeline building `.msi`/`.exe` (Windows), `.dmg` (macOS), and `.AppImage` (Linux) bundles.
