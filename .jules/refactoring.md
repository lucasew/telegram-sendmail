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
