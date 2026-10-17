---
description: General engineering, testing, and documentation standards for ResumAI (Tauri v2 + Rust + Vanilla TypeScript)
globs: ["**/*"]
---

# Engineering Standards

1. **Test-First & Verified Execution**:
   - Include Rust unit/integration tests (`cargo test --manifest-path src-tauri/Cargo.toml`) for all non-trivial domain logic (vault boundary checks, PII prompt interpolation, JSON output sanitization, DOCX/HTML export).
   - Verify TypeScript compilation and Vite bundling via `npm run build` before proposing or committing changes.
2. **Clean Modularity & Minimalist Dependencies**:
   - **Frontend (`src/`)**: Pure Vanilla TypeScript + Vite + CSS. Do not introduce heavy UI frameworks (React, Vue, Angular) or CSS framework bloat. Keep runtime npm dependencies limited to essentials (`@tauri-apps/api`, `marked`).
   - **Backend (`src-tauri/`)**: Keep Rust modules focused (`vault`, `llm`, `pipeline`, `export`), strictly typed, and free of compiler warnings.
3. **Strict Local-First PII Privacy**:
   - Personally Identifiable Information (`content/pii/*.md`) and generated applications (`var/applications/*`) are **sensitive**, **git-ignored**, and must remain 100% on the local machine, transmitted only during user-initiated LLM inference calls (Ollama or user-configured cloud API).
4. **Zero Warnings**:
   - Ensure both `cargo check` / `cargo test` and `npm run build` complete with zero errors or warnings.
