package utils

import (
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

// sentryFlushTimeout is how long FlushSentry waits for buffered events to leave
// the process before giving up (callers often exit immediately after).
const sentryFlushTimeout = 2 * time.Second

// InitSentry initializes the Sentry client with the provided DSN.
// Tracing is disabled: this process only reports errors (via ReportError),
// not performance transactions.
func InitSentry(dsn string) error {
	if dsn == "" {
		return nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		TracesSampleRate: 0,
	})
	if err != nil {
		return err
	}
	slog.Info("Sentry initialized")
	return nil
}

// ReportError reports an error to the centralized logging system.
// It logs to slog.Error and captures the exception in Sentry if configured.
// Callers that exit soon after should still invoke FlushSentry (see Execute).
func ReportError(err error, msg string, args ...any) {
	// Clone args before append so we never reuse the caller's slice backing array
	// (append may write into spare capacity and corrupt caller-owned data).
	logArgs := make([]any, 0, len(args)+2)
	logArgs = append(logArgs, args...)
	logArgs = append(logArgs, "error", err)
	slog.Error(msg, logArgs...)

	if err != nil {
		hub := sentry.CurrentHub()
		if hub.Client() != nil {
			hub.WithScope(func(scope *sentry.Scope) {
				scope.SetExtra("message", msg)
				for i := 0; i < len(args); i += 2 {
					if i+1 < len(args) {
						if key, ok := args[i].(string); ok {
							scope.SetExtra(key, args[i+1])
						}
					}
				}
				hub.CaptureException(err)
			})
		}
	}
}

// FlushSentry ensures all buffered events are sent.
// No-op when Sentry was never initialized (hub has no client). When a client
// is present and the flush does not finish within sentryFlushTimeout, logs a
// warning so a silent drop of the last events is visible in journalctl.
func FlushSentry() {
	// sentry.Flush returns false both on timeout and when no client is bound;
	// distinguish so we do not warn on every exit without MAIL_SENTRY_DSN.
	if sentry.CurrentHub().Client() == nil {
		return
	}
	if ok := sentry.Flush(sentryFlushTimeout); !ok {
		slog.Warn("Sentry flush timed out; some events may be lost",
			"timeout", sentryFlushTimeout)
	}
}
