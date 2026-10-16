# ResumAI Studio (v2 — Tauri v2 Desktop)

A **local-first AI resume, job application, and technical consulting proposal engineering studio** powered by **Tauri v2 (Rust)** and **Vanilla TypeScript + Vite**.

> [!IMPORTANT]
> **100% Local Career Vault Privacy**: Your personally identifiable information (`content/pii/*.md`), candidate archetypes (`content/archetypes/*.md`), and generated application workspaces (`var/applications/*`) remain strictly on your local filesystem. Nothing leaves your machine unless you trigger an inference request to a cloud provider (or keep inference 100% offline via local **Ollama**).

---

## Key Capabilities

1. **Multi-Agent Anti-Hallucination Pipeline**:
   - **Stage 1: Archetype Selector (`resume_archetype_selector`)**: Uses autonomous tool calls (`list_archetypes`, `read_archetype`) to match an incoming Job Description or Client RFP against your candidate personas (`content/archetypes/*.md`).
   - **Stage 2: Resume Drafter (`resume_drafter`)**: Synthesizes an ATS-friendly, Google X-Y-Z formatted resume grounded strictly in `content/pii/*.md`, inserting explicit `[METRIC: ...]` placeholders rather than fabricating numbers.
   - **Stage 3a & 3b: Parallel Verification (`tokio::join!`)**: Executes the **Resume Fact-Checker** and **Job-Alignment Checker** concurrently in Rust, cutting verification time in half and computing a live **Overall Match Score (%)**.
   - **Stage 4: Resume Editor (`resume_editor`)**: Synthesizes the final resume under a strict precedence rule: **Fact-Check corrections always override Job-Alignment keyword suggestions**.
   - **Stage 5: Cover Letter & Consulting Proposal Generator (`resume_cover_letter`)**: Generates either an executive cover letter or a technical consulting capability proposal grounded in the verified resume.
2. **Interactive `[METRIC: ...]` Write-Back Loop**:
   - Automatically detects unresolved `[METRIC: ...]` placeholders, lets you fill in the real figure inline, and provides a 1-click **"Save Fact to `work_history.md`"** button to permanently enrich your Career Vault.
3. **Live Paginated Letter Preview & Zero-Container Export**:
   - Real-time `8.5in × 11in` paginated preview with a live **Page-Budget Meter** (`e.g. 1.82 / 2.00 pages`), live typography sliders (Font Size, Line Height, Margins), native **Print-to-PDF**, and pure-Rust **OpenXML `.docx` + standalone `.html` export** (zero Docker, Pandoc, or Puppeteer containers required).

---

## Quick Start

### Prerequisites
- **Rust** (`>= 1.85` / `cargo`)
- **Node.js** (`>= 20` / `npm`)
- *(Optional)* Local **Ollama** (`http://localhost:11434`) or a **Google Gemini API Key**

### Development & Verification

```powershell
# 1. Install frontend dependencies
npm install

# 2. Run Rust backend unit tests
cargo test --manifest-path src-tauri/Cargo.toml

# 3. Typecheck and bundle the Vanilla TypeScript + Vite frontend
npm run build

# 4. Launch the native ResumAI Studio desktop application
npm run tauri dev
```

---

## 📦 Desktop Packaging & Releases

Production desktop installers are automatically built and published via GitHub Actions (`.github/workflows/release.yml`):

### Available Desktop Bundles
- **Windows x64**: Single-click NSIS installer (`.exe`) and portable standalone executable.
- **Linux x64**: Standalone `.AppImage` and `.deb` packages.
- **Checksums**: SHA-256 verification manifest (`SHA256SUMS.txt`) attached to every release.

### Release Mechanics
- **Human Operator Flow (GitHub UI)**: Draft a release on the GitHub Releases page with your ceremonial release notes and click **Publish release**. The pipeline automatically compiles the Windows and Linux bundles, computes SHA-256 checksums, and attaches all assets directly to the published release.
- **Autonomous Flow (Git / CLI)**: Pushing a semantic tag (e.g. `git tag v0.2.0 && git push origin v0.2.0`) or triggering `workflow_dispatch` auto-creates the release with structured ceremonial notes, builds installers, and dispatches release embeds to Discord `#dev-ci-builds`.

---

## Career Vault Structure

```text
resumai/
├── content/
│   ├── archetypes/                          # Candidate persona lenses (git-ignores custom *.md)
│   │   ├── technical_consultant.sample.md   # Principal Consultant / Fractional Architect sample
│   │   ├── staff_backend_engineer.sample.md # Staff Backend Engineer sample
│   │   └── ...
│   ├── pii/                                 # Candidate PII & Career Vault (git-ignores custom *.md)
│   │   ├── personal_info.sample.md
│   │   ├── work_history.sample.md
│   │   ├── education_history.sample.md
│   │   └── extras.sample.md
│   └── prompts/                             # Multi-agent system prompts
├── assets/
│   └── styles/resumes/                      # Resume CSS themes (resume.sample.css & custom *.css)
├── docs/
│   ├── ROADMAP.md                           # Phased product & non-technical user roadmap
│   └── learnings/                           # Architecture Decision Records (ADR-001)
├── src/                                     # Minimalist Vanilla TypeScript + Vite Studio UI
└── src-tauri/                               # Tauri v2 Rust Core (Vault, LLM Stream, Pipeline, Export)
```

---

## License

Application source code and content are licensed under the **GNU Affero General Public License v3.0 or later**. See [LICENSE](./LICENSE) for the full license text.
