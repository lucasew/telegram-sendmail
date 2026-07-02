package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/lucasew/telegram-sendmail/internal/telegram"
	"github.com/spf13/viper"
)

func TestProcessQueueContinuesAfterSendFailure(t *testing.T) {
	tempDir := t.TempDir()
	firstFile := filepath.Join(tempDir, "001")
	secondFile := filepath.Join(tempDir, "002")

	if err := os.WriteFile(firstFile, []byte("Subject: first\n\nbody one"), 0o600); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(secondFile, []byte("Subject: second\n\nbody two"), 0o600); err != nil {
		t.Fatalf("write second file: %v", err)
	}

	viper.Set("default_subject", "Message")
	viper.Set("hostname", "host")

	var calls atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusInternalServerError)
			if _, err := w.Write([]byte(`{"ok":false,"error_code":500,"description":"boom"}`)); err != nil {
				t.Errorf("failed to write mock response: %v", err)
			}
		case 2:
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
				t.Errorf("failed to write mock response: %v", err)
			}
		default:
			t.Fatalf("unexpected extra request %d: %s", calls.Load(), r.URL.Path)
		}
	}))
	defer ts.Close()

	client := telegram.NewClient("TOKEN", ts.Client())
	client.APIBaseURL = ts.URL + "/bot%s"

	empty, sentCount, errCount := processQueue(client, tempDir, "123")
	if empty {
		t.Fatalf("expected queue to remain non-empty because failed item is kept for retry")
	}
	if sentCount != 1 {
		t.Fatalf("expected 1 sent message, got %d", sentCount)
	}
	if errCount != 1 {
		t.Fatalf("expected 1 send error, got %d", errCount)
	}

	if _, err := os.Stat(firstFile); err != nil {
		t.Fatalf("expected failed file to remain for retry: %v", err)
	}
	if _, err := os.Stat(secondFile); !os.IsNotExist(err) {
		t.Fatalf("expected successful file to be removed, got err=%v", err)
	}
}
