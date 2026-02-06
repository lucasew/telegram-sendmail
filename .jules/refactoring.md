# Refactoring Journal

## Migration from Python to Go for telegram-sendmail

### Context
Migrated the `telegram-sendmail` service from a single-file Python script to a structured Go application. The goal was to maintain existing behavior (systemd socket activation, file-based queuing) while leveraging Go's performance and binary distribution.

### Technical Decisions
1.  **Systemd Activation**: Used `github.com/coreos/go-systemd/v22/activation` to inherit file descriptors. This is crucial for "drop-in" replacement in the NixOS module.
2.  **Idle Exit Strategy**: Implemented a "wake-on-lan" style behavior where the service exits with status 0 when the queue is empty and no connections are active. This required changing the systemd `Restart` policy from `always` (which causes busy loops on success) to `on-failure`.
3.  **Dependency Management**: Vendored dependencies (`go mod vendor`) and used `vendorHash = null` in `pkgs.buildGoModule` to ensure reproducible builds within NixOS without requiring network access during build or explicit hash management for every update.
4.  **Configuration**: Used `spf13/viper` to seamlessly bind environment variables (legacy `MAIL_TELEGRAM_TOKEN`) to internal configuration flags.

### Learnings
-   **Systemd Restart Policies**: When implementing services that exit on idle (success), `Restart=always` is dangerous. `Restart=on-failure` or `Restart=no` is required.
-   **NixOS Go Modules**: Using `src = ./.` with a committed `vendor` directory allows `vendorHash = null`, simplifying the developer experience for local modules.
-   **Error Handling**: Go's explicit error handling forced a more robust implementation of the Telegram API fallback logic (text -> document on 400 error).

### References
-   Systemd Socket Activation: http://0pointer.de/blog/projects/socket-activation.html
-   Go Systemd Activation: https://github.com/coreos/go-systemd

## Extraction of Telegram Client

### Context
Extracted the Telegram API interaction logic from `cmd/telegram-sendmail/serve.go` to a new package `internal/telegram`.

### Technical Decisions
1.  **Single Responsibility Principle**: The `serve.go` file was mixing server loop, queue management, and external API calls. Extracting the client improves cohesion.
2.  **Encapsulation**: The fallback logic (Text -> Document) is now encapsulated within the `Client.Send` method, hiding complexity from the main service loop.
3.  **Testability**: The new package is independently testable. Added unit tests using `httptest` to verify the fallback logic without real network calls.

### Learnings
-   **Mocking with httptest**: Effective for testing HTTP clients without external dependencies.
-   **Configurable BaseURL**: Adding `APIBaseURL` to the `Client` struct (instead of a hardcoded constant) was necessary to point the client to the mock server during tests.

## Centralized Error Handling

### Context
Retroactively applied strict error handling rules to the codebase. The project required "no silent failures" and a "centralized error reporting" mechanism.

### Technical Decisions
1.  **Centralized Handler**: Created `internal/utils/error.go` with `ReportError(err error, msg string, args ...any)`. This wraps `slog.Error` but provides a single point of interception for future error reporting backends (e.g., Sentry).
2.  **Strict Error Checking**: Audited the codebase for ignored errors (e.g., `_ = ...` or empty catch blocks). Replaced them with explicit checks and calls to `ReportError`.
3.  **Refactoring**: Updated `cmd/telegram-sendmail` (main application logic) and `internal/telegram` (library) to adhere to these rules.
    -   In `serve.go`, errors from `conn.Write` and `os.Remove` are now reported.
    -   In `client.go`, errors from `io.ReadAll` and multipart writing are returned to the caller.

### Learnings
-   **Ignored Errors in Go**: Functions like `conn.Write` and `os.Remove` are frequently ignored in example code but can hide important issues like disk corruption or network instability.
-   **Centralization**: Having a `ReportError` function makes it easy to enforce consistent logging structure (e.g., ensuring the `error` key is always present).
