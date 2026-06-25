# Consistently Ignored Changes

This file lists patterns of changes that have been consistently rejected by human reviewers. All agents MUST consult this file before proposing a new change. If a planned change matches any pattern described below, it MUST be abandoned.

---

## IGNORE: Legacy Python Script

**- Pattern:** Modifying or refactoring the `service` file (Python script). This includes security fixes (e.g., global connection timeouts for Slowloris DoS protection), bug fixes (e.g., modifying header parsing regex), tooling configuration (e.g., `ruff`, `mypy`), and architectural changes (e.g., object-oriented refactoring, decoupling functions, extracting classes).
**- Justification:** The project has been rewritten in Go. The Python script is strictly deprecated and should not be maintained or resurrected under any circumstances.
**- Files Affected:** `service`, `tests/test_service.py`, `*.py`, `mise.toml`

## IGNORE: Silent Error Suppression

**- Pattern:** Assigning function return values (especially errors) to `_` or `_, _` to satisfy linters (e.g., `_ = func()`, `_, _ = func()`).
**- Justification:** Errors must never be silently ignored. They must be handled explicitly or reported via `utils.ReportError`. Silencing them hides potential failures.
**- Files Affected:** `*.go`

## IGNORE: Restricting Socket Permissions

**- Pattern:** Changing UNIX socket permissions to be more restrictive (e.g., `0o770` or `0o600`).
**- Justification:** The application acts as a system-wide mailer and requires broad permissions (`0o777`) so that any process on the system can connect to the socket to send mail.
**- Files Affected:** `nixos-module.nix`, `*.go`

## IGNORE: Complex Linting Setup

**- Pattern:** Introducing complex linting tools (e.g., `golangci-lint` for Go, `ruff` for Python) or creating complex lint configurations.
**- Justification:** The project prefers minimal tooling (`go vet ./...` as the standard linter). Additional complexity in tooling is not desired.
**- Files Affected:** `mise.toml`, `.golangci.yml`, `service`

## IGNORE: Automated Action Upgrades

**- Pattern:** Upgrading GitHub Actions versions in workflow files (e.g., `actions/checkout@v6` instead of `v4`, or `jdx/mise-action@v3` instead of `v2`) without explicit instructions.
**- Justification:** Automated updates can introduce breaking changes or violate pinned tool requirements. Actions should remain at their currently configured stable versions.
**- Files Affected:** `.github/workflows/autorelease.yml`
