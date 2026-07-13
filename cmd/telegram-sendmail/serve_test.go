package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lucasew/telegram-sendmail/internal/telegram"
	"github.com/spf13/viper"
)

func TestSetListenerDeadlineUnix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	defer l.Close()

	if err := setListenerDeadline(l, time.Now().Add(50*time.Millisecond)); err != nil {
		t.Fatalf("setListenerDeadline: %v", err)
	}

	// Accept should time out rather than block forever.
	_, err = l.Accept()
	if err == nil {
		t.Fatal("expected Accept timeout error, got nil")
	}
	var opErr *net.OpError
	if !errors.As(err, &opErr) || !opErr.Timeout() {
		t.Fatalf("expected timeout OpError, got %T %v", err, err)
	}
}

func TestParseMailMessage(t *testing.T) {
	const defaultSubject = "Message"

	tests := []struct {
		name            string
		data            string
		wantSubject     string
		wantBody        string
	}{
		{
			name:        "normal Subject",
			data:        "Subject: Hello World\n\nbody text",
			wantSubject: "Hello World",
			wantBody:    "body text",
		},
		{
			name:        "lowercase subject",
			data:        "subject: lower case\n\nbody text",
			wantSubject: "lower case",
			wantBody:    "body text",
		},
		{
			name:        "missing subject",
			data:        "From: sender@example.com\n\nbody only",
			wantSubject: defaultSubject,
			wantBody:    "body only",
		},
		{
			name:        "body after blank line",
			data:        "Subject: multi\n\nline one\nline two\n",
			wantSubject: "multi",
			wantBody:    "line one\nline two\n",
		},
		{
			name:        "malformed falls back to full payload",
			data:        "not a header line\njust plain text",
			wantSubject: defaultSubject,
			wantBody:    "not a header line\njust plain text",
		},
		{
			name:        "SUBJECT uppercase",
			data:        "SUBJECT: shouted\n\npayload",
			wantSubject: "shouted",
			wantBody:    "payload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSubject, gotBody := parseMailMessage([]byte(tt.data), defaultSubject)
			if gotSubject != tt.wantSubject {
				t.Errorf("subject: got %q, want %q", gotSubject, tt.wantSubject)
			}
			if gotBody != tt.wantBody {
				t.Errorf("body: got %q, want %q", gotBody, tt.wantBody)
			}
		})
	}
}

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
			_, _ = w.Write([]byte(`{"ok":false,"error_code":500,"description":"boom"}`))
		case 2:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
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
