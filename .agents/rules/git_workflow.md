---
description: Git workflow, base branch (`main-v2`), and branch naming conventions for ResumAI (building on pm-engine-kit/rules/AGENTS.md §7)
globs: ["**/*"]
---

# Git Workflow & Base Branch Rules (`resumai`)

Universal commit authorship (`--author="Antigravity <antigravity@gemini.google.com>"`), scratch-file message passing (`-F <path>` / `--body-file <path>`), `Relates to #<id>` default linking, CI verification before merge (`gh pr checks`), and non-default base branch verification (`--base main-v2`) are governed globally by `pm-engine-kit/rules/AGENTS.md` §7.

## 1. Base Branch (`main-v2`) & Feature Branch Naming Standard
- During the **ResumAI v2 (Tauri v2 Desktop)** transition, the active integration base branch is `main-v2` (while `main` preserves the legacy v1 Symfony/Docker baseline until v2 is promoted).
- Branch feature work from `main-v2` using the issue-driven convention: `<type>/resumai-<issue#>-<short-slug>`
- Allowed `<type>` prefixes: `feat`, `fix`, `chore`, `docs`, `refactor`, `style`, `test`
- Always pass `--base main-v2` explicitly when opening PRs (`gh pr create --base main-v2 --body-file <path>`).
