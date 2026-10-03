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
6. Pull the required agent models used by the Ollama platform: `make ollama-pull`.
7. Open a web browser and navigate to https://aistudio.google.com/api-keys and click the _Create API key_ button.
    1. Create a new project or select an existing one.
    2. Copy the API key and save it in the git-ignored `.env.local` file as:
        ```dotenv
        GEMINI_API_KEY=<gemini_api_key>
        ```
8. Generate a general-purpose resume based only on `content/pii/` and output to `var/resume_draft.md`:
    ```shell
    make resume-draft c='var/resume_draft.md' # Ollama
    make resume-draft c='var/resume_draft.md --platform gemini' # Gemini
    ```

## Usage

> [!TIP]
> Use `make help` to reference available make targets and descriptions.

The app is composed of five commands (some of which are optional). They are designed to run in a sequence but are left to be run independently, for two main reasons:

1. Workflow flexibility.
2. Unreliability of free-tier and local model service/output.

### 1. Resume Archetype Selector (Optional)

> [!TIP]
> `make resume-arch c='--help'`

**TODO**

### 2. Resume Drafter

> [!TIP]
> `make resume-draft c='--help'`

**TODO**

### 3. Resume Checkers

These agent commands are designed to provide checks and feedback for a resume. Generated feedback can then be used with [step 4](#4-resume-editor).

> [!NOTE]
> These may also be used as standalone commands to check a hand-crafted resume.

#### 3.1. Resume Fact Checker

> [!TIP]
> `make resume-fc c='--help'`

**TODO**

#### 3.2. Resume Job Alignment Checker (Optional)

> [!TIP]
> `make resume-jac c='--help'`

**TODO**

### 4. Resume Editor

> [!TIP]
> `make resume-edit c='--help'`

**TODO**

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
