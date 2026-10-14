---
description: Git workflow, branch naming conventions, commit attribution, and PR rules for ResumAI
globs: ["**/*"]
---

# Git Workflow & Commit Attribution Rules

When developing features, creating branches, and committing code in the `resumai` workspace:

## 1. Base Branch & Feature Branch Naming Standard
- During the **ResumAI v2 (Tauri v2 Desktop)** transition, the active integration base branch is `main-v2` (while `main` preserves the legacy v1 Symfony/Docker baseline until v2 is promoted).
- Branch feature work from `main-v2` using the issue-driven convention: `<type>/resumai-<issue#>-<short-slug>`
- Allowed `<type>` prefixes: `feat`, `fix`, `chore`, `docs`, `refactor`, `style`, `test`
- Examples: `feat/resumai-1-tauri-v2-core`, `feat/resumai-2-cover-letter-agent`

## 2. Commit Standards & Author Attribution
- **Conventional Commits**: Format commit titles as `<type>(<scope>): <summary>` and link any relevant GitHub issue (`Closes #<id>` or `Fixes #<id>`).
- **Author Attribution**: When commits are generated or pair-programmed by the AI assistant, set author attribution explicitly:
  ```bash
  git commit --author="Antigravity <antigravity@gemini.google.com>" -m "<title>" -m "<body>" -m "<trailers>"
  ```
- **Commit Metadata Trailers**: Include standard trailers in all agent commits:
  ```text
  Generated-by: Antigravity (<Model Name>)
  Agent-Session: <conversation-id>
  Co-authored-by: Antigravity <antigravity@gemini.google.com>
  ```

## 3. Remote Push & PR Lifecycle
- **Remote Push**: Push branches to `origin` tracking the branch name:
  ```bash
  git push -u origin <branch-name>
  ```
- **Pull Requests**: Create Pull Requests via `gh pr create --base main-v2` (or `--base main` once `main-v2` is promoted).
- **Merge Strategy**: All feature PRs use **Squash and Merge** to maintain a clean linear history.
