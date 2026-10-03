# ResumAI

[![CI](https://github.com/kidthales/resumai/actions/workflows/ci.yaml/badge.svg)](https://github.com/kidthales/resumai/actions/workflows/ci.yaml)

A résumé and job application engineering system.

## Requirements

- [Docker Compose](https://docs.docker.com/compose/install/) (v2.10+)
- [GNU Make](https://www.gnu.org/software/make/) and [GNU Bash](https://www.gnu.org/software/bash/)
- [Git](https://git-scm.com/install/)

## Getting Started

1. `git clone https://github.com/kidthales/resumai.git`
2. `cd resumai`
3. `make start`
    - First-time project starts will take a few minutes to complete while docker images are built and services are started.
    - Once started, Docker logging will continue in the current shell. When all services are healthy, the application is ready to accept commands in another shell.
4. Within the `content/pii` directory, create and populate `education_history.md`, `extras.md`, `personal_info.md`, and `work_history.md` with your own information; refer to the corresponding sample files for guidance.
5. Within the `content/archetypes` directory, create and populate at least 2–3 of your own git-ignored files with your own archetypes; refer to the corresponding sample files for guidance. Keep these archetypes aligned with the content you've added in `content/pii/work_history.md`.
6. **TODO** Ollama model pull instructions...
7. **TODO** Google AI Studio instructions...
8. To create a general-purpose resume based only on `content/pii/`:
    ```shell
    make resume-draft c='var/resume_draft.md' # Ollama
    make resume-draft c='var/resume_draft.md --platform gemini' # Gemini
    ```

## Usage

**TODO** Usage instructions...

## Content Structure

```
resumai/
└── content/
    ├── archetypes/                          # Candidate archetypes
    │   ├── .gitignore                       # Ignores all non-sample files
    │   └── *.sample.md                      # Sample archetype files
    │
    ├── pii/                                 # Personally Identifiable Information
    │   ├── .gitignore                       # Ignores all non-sample files
    │   ├── education_history.sample.md      # Sample education history file
    │   ├── extras.sample.md                 # Sample extras file
    │   ├── personal_info.sample.md          # Sample personal information file
    │   └── work_history.sample.md           # Sample work history file
    │
    └── prompts/                             # Agent prompts
        ├── resume_archetype_selector.md     # Resume archetype selector agent prompt
        ├── resume_drafter.md                # Resume drafter agent prompt
        ├── resume_editor.md                 # Resume editor agent prompt
        ├── resume_fact_checker.md           # Resume fact checker agent prompt
        └── resume_job_alignment_checker.md  # Resume job alignment checker agent prompt
```
