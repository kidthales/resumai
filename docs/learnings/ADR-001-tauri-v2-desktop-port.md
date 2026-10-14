# ADR-001: Port from Symfony 8.1 + Docker to Tauri v2 + Vanilla TypeScript

- **Date**: 2026-10-13
- **Status**: Accepted (`main-v2`)
- **Context**: `kidthales/resumai`

## 1. Context & Problem Statement

ResumAI v1 was built as a Symfony 8.1 CLI application running across a 4-container Docker Compose environment (`php`/FrankenPHP, `node`/Puppeteer+Chromium, `pandoc`, and `ollama`) mounted over WSL2.

While the **domain architecture**—a 5-stage multi-agent pipeline (`ArchetypeSelector` $\rightarrow$ `ResumeDrafter` $\rightarrow$ `FactChecker` + `JobAlignmentChecker` $\rightarrow$ `ResumeEditor`) plus Candidate Archetypes (`content/archetypes/*.md`) and `.thonk` reasoning capture—proved highly effective, the infrastructure and execution model suffered from several critical issues:

1. **Disproportionate Runtime Weight**: Executing ~900 lines of PHP CLI code and exporting a PDF required spinning up 4 containers totaling >3 GB of container images plus WSL2 filesystem bind-mount overhead.
2. **Compile-Time PII Interpolation Bug (`%env(resolve:file:...)%`)**: In `config/services.yaml`, PII files (`content/pii/*.md`) were injected into system prompts via Symfony's `resolve:` env processor at DI container build time. Any literal `%` sign in a candidate's work history (e.g., `"Reduced p99 latency by 40%"`) caused Symfony to interpret `%...%` as a container parameter reference, crashing container compilation unless escaped as `%%`. Additionally, editing PII files required clearing/rebuilding the container cache.
3. **Sequential Manual Ceremony**: Users had to invoke up to 6 separate `make` commands and manually pass intermediate file paths (`draft.md`, `fc.md`, `jac.md`, `final.md`), even though Fact-Checking and Job-Alignment Checking are completely independent and can run concurrently.
4. **Data Privacy vs. SaaS Trade-off**: Scaling the Symfony app into a multi-tenant cloud SaaS would require storing highly sensitive unredacted career vaults (home addresses, phone numbers, internal metrics under NDA) on a remote server and absorbing heavy cloud LLM + headless Chromium costs.

## 2. Decision

Port ResumAI on branch `main-v2` to a **Local-First Tauri v2 Desktop Application** using:
- **Backend (`src-tauri/`)**: Rust (`tokio`, `reqwest`, `serde`, `zip`) handling all Career Vault file I/O, runtime literal PII prompt interpolation, streaming Ollama & Gemini HTTP inference with exponential backoff, autonomous `list_archetypes` / `read_archetype` tool-calling loops, concurrent `tokio::join!` verification checks, and zero-container HTML + native `.docx` export.
- **Frontend (`src/`)**: **Minimalist Vanilla TypeScript + Vite + CSS** (`@tauri-apps/api` and `marked` as the only runtime dependencies), providing a 3-pane Career Engineering Studio with a live paginated Letter (`8.5in × 11in`) `<iframe srcdoc>` preview, real-time page-budget calculation, `[METRIC]` write-back loop, and native WebView Print-to-PDF.

## 3. Consequences

### Positive
- **Zero Docker / Zero WSL2 Mount Overhead**: Compiles to a single ~12 MB native binary on Windows, macOS, and Linux.
- **100% On-Device Privacy**: All PII (`content/pii/`), archetypes (`content/archetypes/`), and generated job applications (`var/applications/`) stay on the user's local machine in Git-friendly Markdown files.
- **Zero Framework Churn**: By placing stateful orchestration in Rust and keeping the UI in clean modular Vanilla TypeScript, the frontend builds in `<500ms` with no React/Svelte/Vue lifecycle lock-in.
- **2x Faster Verification Stage**: Running Fact-Check and Job-Alignment Check concurrently via `tokio::join!` cuts pipeline verification latency in half.

### Mitigations
- Legacy v1 Symfony/Docker history remains preserved on `origin/main` while v2 matures on `main-v2`.
