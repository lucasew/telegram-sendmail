package utils

import "log/slog"

// ReportError reports an error to the centralized logging system.
// Currently it logs to slog.Error, but it can be extended to support Sentry or other services.
func ReportError(err error, msg string, args ...any) {
	// Ensure the error is included in the args
	args = append(args, "error", err)
	slog.Error(msg, args...)
}
