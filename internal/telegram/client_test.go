package telegram

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_SendText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/botTOKEN/sendMessage" {
			t.Errorf("Expected path /botTOKEN/sendMessage, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
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
		w.Write([]byte(`{"ok":true}`))
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
	// Test that Send falls back to SendDocument on 400
	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "sendMessage") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"ok":false, "error_code": 400, "description": "Bad Request"}`))
			return
		}
		if strings.Contains(r.URL.Path, "sendDocument") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
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
