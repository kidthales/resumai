---
description: PowerShell scripting standards, strict-mode robustness, and path resolution guardrails
globs: ["scripts/**/*.ps1", "**/*.ps1"]
---

# PowerShell Scripting Standards & Guardrails

When authoring or executing PowerShell commands or scripts (`.ps1`) on Windows, agents MUST adhere to these syntax and strict-mode requirements:

## 1. Here-String Header Invariant
- A PowerShell here-string header (`@"`) MUST be followed immediately by a newline.
- **NEVER** place text, markdown headers, or YAML fences on the same line as the opening `@"`.
- Use **single-quoted here-strings (`@' ... '@`)** for markdown content in CLI commands to guarantee that backticks (`` ` ``) and dollar signs (`$`) remain 100% literal and unescaped.
- **NEVER** use backslashes (`\``) to escape backticks in PowerShell.

## 2. Array Subexpression Wrapping for Collections
- When `Set-StrictMode -Version Latest` is enabled, accessing `.Count` on a single object returned by `Get-ChildItem` throws a terminating property error.
- **ALWAYS** wrap collection queries in the array subexpression `@(...)`:
  ```powershell
  $files = @(Get-ChildItem -Path $dir -Filter "*.md" -File)
  ```

## 3. GitHub CLI (`gh.exe`) Path Resolution
- When invoking `gh` in Windows scripts, do not assume it is in the active session `$env:PATH`.
- Always implement fallback resolution:
  ```powershell
  $ghPath = "gh"
  if (-not (Get-Command "gh" -ErrorAction SilentlyContinue)) {
      $standardPath = "C:\Program Files\GitHub CLI\gh.exe"
      if (Test-Path $standardPath) {
          $ghPath = $standardPath
      } else {
          throw "GitHub CLI (gh.exe) not found."
      }
  }
  ```
