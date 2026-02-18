# Consistently Ignored Changes

This file lists patterns of changes that have been consistently rejected by human reviewers. All agents MUST consult this file before proposing a new change. If a planned change matches any pattern described below, it MUST be abandoned.

---

## IGNORE: Legacy Python Script

**- Pattern:** Modifying or refactoring the `service` file (Python script), including security fixes, tooling configuration (e.g., `ruff`, `mypy`), or architectural changes (e.g., extracting classes).
**- Justification:** The project has been rewritten in Go. The Python script is deprecated and should not be maintained or resurrected.
**- Files Affected:** `service`, `*.py`, `mise.toml`

## IGNORE: Silent Error Suppression

**- Pattern:** Assigning function return values (especially errors) to `_` or `_, _` to satisfy linters (e.g., `_ = func()`, `_, _ = func()`).
**- Justification:** Errors must never be silently ignored. They must be handled explicitly or reported via `utils.ReportError`. Silencing them hides potential failures.
**- Files Affected:** `*.go`

## IGNORE: Restricting Socket Permissions

**- Pattern:** Changing UNIX socket permissions to be more restrictive (e.g., `0o770` or `0o600`).
**- Justification:** The application acts as a system-wide mailer and requires broad permissions (`0o777`) so that any process on the system can connect to the socket to send mail.
**- Files Affected:** `nixos-module.nix`, `*.go`
