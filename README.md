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
    - First-time project starts will take a few minutes to complete while docker images are built, services are started, and ollama models are created.
    - Once started, docker log output will continue in the current shell. When all services are healthy, and all ollama models have been created, the application is ready to accept commands in another shell.
4. Within the `content/pii` directory, create and populate `education_history.md`, `personal_info.md`, and `work_history.md` with your own information; refer to the corresponding sample files for guidance.

## Content Structure

```
resumai/
└── content/
    ├── pii/                             # Personally Identifiable Information
    │   ├── .gitignore                   # Ignores all non-sample files
    │   ├── education_history.sample.md  # Sample education history file
    │   ├── personal_info.sample.md      # Sample personal information file
    │   └── work_history.sample.md       # Sample work history file
    └── prompts/
        └── resume_drafter.md            # Resume drafter agent prompt
```
