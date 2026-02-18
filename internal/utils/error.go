package utils

import (
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

// InitSentry initializes the Sentry client with the provided DSN.
func InitSentry(dsn string) error {
	if dsn == "" {
		return nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		TracesSampleRate: 1.0,
	})
	if err != nil {
		return err
	}
	slog.Info("Sentry initialized")
	return nil
}

// ReportError reports an error to the centralized logging system.
// It logs to slog.Error and captures the exception in Sentry if configured.
func ReportError(err error, msg string, args ...any) {
	// Ensure the error is included in the args
	logArgs := append(args, "error", err)
	slog.Error(msg, logArgs...)

	if err != nil {
		// Capture in Sentry
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
			// Flush specifically for this event could be slow, but for a sendmail replacement
			// that exits quickly, we might want to consider when to flush.
			// However, since we are a long-running service (in serve mode) or a quick CLI,
			// letting the background transport handle it is usually fine, BUT:
			// If the program crashes or exits immediately after ReportError, we lose the event.
			// For now, we rely on the default transport buffer.
			// Ideally, we should Flush on exit.
		}
	}
}

// FlushSentry ensures all buffered events are sent.
func FlushSentry() {
	sentry.Flush(2 * time.Second)
}
