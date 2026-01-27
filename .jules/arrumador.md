# Arrumador Journal

## [2025-01-20] Tooling Configuration
- Configured `mise` for task management and tool versioning.
- Set up `ruff` for Python linting and formatting.
- Created standard `mise` tasks: `lint`, `fmt`, `test`, `install`, `ci`, `codegen`.
- Established a single GitHub Actions workflow `autorelease.yml` for CI/CD.
- Addressed linting issues in `service` (unused `Optional`).
- Noted that `service` script requires explicit file argument for `ruff` due to missing extension.
