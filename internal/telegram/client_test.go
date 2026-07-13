package telegram

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
