package utils

import (
	"errors"
	"testing"
	"time"
)

func TestSentryFlushTimeoutIsPositive(t *testing.T) {
	if sentryFlushTimeout <= 0 {
		t.Fatalf("sentryFlushTimeout must be positive, got %v", sentryFlushTimeout)
	}
	// Keep the bound small so process exit after ReportError stays snappy.
	if sentryFlushTimeout > 10*time.Second {
		t.Fatalf("sentryFlushTimeout too large for exit path: %v", sentryFlushTimeout)
	}
}

// TestFlushSentryWithoutClient ensures FlushSentry is a quiet no-op when
// Sentry was never initialized (must not panic or log a timeout warning).
func TestFlushSentryWithoutClient(t *testing.T) {
	FlushSentry()
}

// TestReportErrorDoesNotAliasCallerSlice verifies that ReportError does not
// append into the caller's slice backing array when spare capacity exists.
func TestReportErrorDoesNotAliasCallerSlice(t *testing.T) {
	// Pre-allocate spare capacity so a buggy append(args, ...) would reuse
	// this array and overwrite trailing slots.
	callerOwned := make([]any, 2, 8)
	callerOwned[0] = "key"
	callerOwned[1] = "value"
	// Poison spare capacity so any in-place append is detectable.
	callerOwned = callerOwned[:8]
	callerOwned[2] = "sentinel-a"
	callerOwned[3] = "sentinel-b"
	callerOwned[4] = "sentinel-c"
	callerOwned[5] = "sentinel-d"
	callerOwned[6] = "sentinel-e"
	callerOwned[7] = "sentinel-f"
	callerOwned = callerOwned[:2]

	beforeCap := cap(callerOwned)
	beforeLen := len(callerOwned)
	spareBefore := append([]any(nil), callerOwned[:cap(callerOwned)][len(callerOwned):]...)

	ReportError(errors.New("boom"), "test message", callerOwned...)

	if len(callerOwned) != beforeLen {
		t.Fatalf("caller slice len changed: got %d want %d", len(callerOwned), beforeLen)
	}
	if cap(callerOwned) != beforeCap {
		t.Fatalf("caller slice cap changed: got %d want %d", cap(callerOwned), beforeCap)
	}
	if callerOwned[0] != "key" || callerOwned[1] != "value" {
		t.Fatalf("caller slice contents corrupted: %#v", callerOwned)
	}

	// Spare capacity beyond len must be unchanged (proves no append aliasing).
	spareAfter := callerOwned[:cap(callerOwned)][len(callerOwned):]
	for i := range spareBefore {
		if spareAfter[i] != spareBefore[i] {
			t.Fatalf("caller spare capacity corrupted at index %d: got %#v want %#v (full spare %#v)",
				i, spareAfter[i], spareBefore[i], spareAfter)
		}
	}
}
