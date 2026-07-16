package main

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWaitForSocket_ready(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	if err := waitForSocket(sock, 3, time.Millisecond); err != nil {
		t.Fatalf("waitForSocket: %v", err)
	}
}

func TestWaitForSocket_timeout(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "missing.sock")
	err := waitForSocket(sock, 2, time.Millisecond)
	if err == nil {
		t.Fatal("expected error for missing socket")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSendmail_copiesStdin(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	gotCh := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			gotCh <- "accept:" + err.Error()
			return
		}
		defer conn.Close()
		b, err := io.ReadAll(conn)
		if err != nil {
			gotCh <- "read:" + err.Error()
			return
		}
		gotCh <- string(b)
	}()

	// Point client at our test socket and feed stdin.
	sendmailSocketPath = sock
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	msg := "Subject: hi\n\nbody\n"
	go func() {
		_, _ = io.WriteString(w, msg)
		_ = w.Close()
	}()

	if err := runSendmail(nil, nil); err != nil {
		t.Fatalf("runSendmail: %v", err)
	}

	select {
	case got := <-gotCh:
		if got != msg {
			t.Fatalf("server got %q want %q", got, msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server read")
	}
}
