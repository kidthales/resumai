# Knowledge Base & Learnings Log

This directory serves as the persistent knowledge repository for Architecture Decision Records (ADRs), post-mortems, and tricky workarounds discovered during ResumAI development.

## How Agents Use This Directory
1. When you solve a difficult issue or establish a new architectural pattern, create a markdown note here (e.g. `docs/learnings/ADR-002-<slug>.md`).
2. Add an index entry below summarizing the insight.
3. If the learning is a concise constraint, also update `.agents/rules/`.

## Index of Learnings
<!-- Format: - [Title](path) - Brief summary (Date) -->
- [ADR-001: Port from Symfony 8.1 + Docker to Tauri v2 + Vanilla TypeScript](./ADR-001-tauri-v2-desktop-port.md) - Rationale, technical audit of v1 bottlenecks (`%env(resolve:...)%` PII bug, 4-container WSL2 footprint), frontend trade-offs, and local-first desktop architecture (2026-10-13)
