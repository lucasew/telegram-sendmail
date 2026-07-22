package telegram

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		maxBytes int
		want     string
	}{
		{name: "short unchanged", in: "hello", maxBytes: 10, want: "hello"},
		{name: "exact length", in: "hello", maxBytes: 5, want: "hello"},
		{name: "ascii cut", in: "hello world", maxBytes: 5, want: "hello"},
		// "é" is 2 bytes in UTF-8; cutting at 3 would land mid-rune without care.
		{name: "multi-byte boundary", in: "aébc", maxBytes: 3, want: "aé"},
		{name: "zero max", in: "abc", maxBytes: 0, want: ""},
		{name: "negative max", in: "abc", maxBytes: -1, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateUTF8(tt.in, tt.maxBytes)
			if got != tt.want {
				t.Fatalf("truncateUTF8(%q, %d)=%q want %q", tt.in, tt.maxBytes, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("result is not valid UTF-8: %q", got)
			}
		})
	}
}

func TestClient_SendText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("Expected path /botTOKEN/sendMessage, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected method %s, got %s", http.MethodPost, r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("chat_id") != "123" {
			t.Errorf("Expected chat_id 123, got %s", r.FormValue("chat_id"))
		}
		if r.FormValue("text") != "Hello World" {
			t.Errorf("Expected text Hello World, got %s", r.FormValue("text"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	client := NewClient("TOKEN", ts.Client())
	client.APIBaseURL = ts.URL + "/bot%s"

	err := client.SendText("123", "Hello World")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestClient_Send_Fallback(t *testing.T) {
	// Test that Send falls back to SendDocument on Bad Request
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "sendMessage") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false, "error_code": 400, "description": "Bad Request"}`))
			return
		}
		if strings.Contains(r.URL.Path, "sendDocument") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		t.Errorf("Unexpected path: %s", r.URL.Path)
	}))
	defer ts.Close()

	client := NewClient("TOKEN", ts.Client())
	client.APIBaseURL = ts.URL + "/bot%s"

	err := client.Send("123", "Subject", "Body", "Host")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("Expected 2 calls (text then doc), got %d", calls)
	}
}

func TestClient_Send_EscapesHostnameAndSubject(t *testing.T) {
	// Hostname and subject are embedded in HTML parse_mode markup; both must
	// be escaped so HOSTNAME env / unusual subjects cannot break the payload.
	var gotText string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		gotText = r.FormValue("text")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer ts.Close()

	client := NewClient("TOKEN", ts.Client())
	client.APIBaseURL = ts.URL + "/bot%s"

	err := client.Send("123", "a <b> subj", "body", `host&"x`)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(gotText, "host&amp;&#34;x") {
		t.Fatalf("hostname not escaped in text: %q", gotText)
	}
	if !strings.Contains(gotText, "a &lt;b&gt; subj") {
		t.Fatalf("subject not escaped in text: %q", gotText)
	}
	if strings.Contains(gotText, `host&"x`) || strings.Contains(gotText, "a <b> subj") {
		t.Fatalf("raw special chars leaked into HTML payload: %q", gotText)
	}
}

func TestCheckResponseErrorTruncatesBody(t *testing.T) {
	// Body larger than maxErrorBodyBytes must be truncated, not fully retained.
	huge := strings.Repeat("x", maxErrorBodyBytes+128)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(huge))
	}))
	defer ts.Close()

	client := NewClient("TOKEN", ts.Client())
	client.APIBaseURL = ts.URL + "/bot%s"

	err := client.SendText("123", "hi")
	if err == nil {
		t.Fatal("expected error from 500 response")
	}
	var tErr *Error
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *Error, got %T %v", err, err)
	}
	if len(tErr.Message) > maxErrorBodyBytes+len("...(truncated)") {
		t.Fatalf("error message too large: %d", len(tErr.Message))
	}
	if !strings.HasSuffix(tErr.Message, "...(truncated)") {
		t.Fatalf("expected truncation suffix, got len=%d suffix=%q", len(tErr.Message), tErr.Message[len(tErr.Message)-20:])
	}
}

func TestNewClientNilUsesTimeout(t *testing.T) {
	c := NewClient("TOKEN", nil)
	if c.httpClient == nil {
		t.Fatal("expected non-nil default http client")
	}
	if c.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("Timeout=%v want %v", c.httpClient.Timeout, defaultHTTPTimeout)
	}
}

func TestNewClientKeepsProvidedClient(t *testing.T) {
	custom := &http.Client{Timeout: 5 * time.Second}
	c := NewClient("TOKEN", custom)
	if c.httpClient != custom {
		t.Fatal("expected provided client to be retained")
	}
}
