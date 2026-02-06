# AGENTS.md

This file contains the conventions, rules, and guidelines for working on this repository. It is the single source of truth for all agents.

## Project Overview
- **Name:** Telegram Sendmail
- **Language:** Go (v1.23.4+)
- **Purpose:** A utility that acts as a sendmail replacement to forward emails to Telegram.
- **Architecture:** Systemd socket activation, file-based queuing, HTTP client for Telegram API.

## Workflow & Tooling
- **Mise:** Use `mise` for all task executions (`mise run install`, `mise run build`, `mise run test`, `mise run ci`).
- **Dependencies:** Do not downgrade dependencies.
- **Pre-commit:** Always run `mise run ci` before submitting.
- **CI:** GitHub Actions uses `jdx/mise-action`.

## Coding Conventions
- **Naming:** Rename cryptic names to be self-documenting.
- **Structure:** Group by domain and responsibility (colocation). Directories should have a single clear purpose.
- **Complexity:** Reduce cognitive load. Replace magic numbers with named constants.
- **Error Handling:** Never ignore errors (use `_ = ...` only if strictly necessary and justified). No empty catch/error blocks.
- **HTTP:** Always use `http.Client` with an explicit timeout.
- **Concurrency:** Ensure systemd services exit cleanly (status 0) when idle. Use `Restart=on-failure`.

## Refactoring Guidelines
- **Justification:**
    - **Extraction (Method/Class):** Cite academic sources (Fowler, Martin, GoF).
    - **Positioning:** Provide clear rationale for layout changes.
- **Rule of Three:** Do not abstract until duplication occurs at least three times.
- **Atomic Changes:** Keep PRs small and focused.
- **PR Titles:**
    - Refactoring: `🛠️ Refactor: [Description]`
    - Bug Fix: `Solves #issue_number` in body.
    - Arrumador: `🛷 Arrumador: [Description]`

## Release Process
- **Version:** Managed in `version.txt` (semantic version, no 'v' prefix).
- **Tagging:** `make_release` script handles version increment and tagging.

## Ignoring Patterns
- Check `.jules/CONSISTENTLY_IGNORED.md` (if exists) before planning.

## Memory & Learning
- Maintain learning journals in `.jules/` (e.g., `refactoring.md`).
