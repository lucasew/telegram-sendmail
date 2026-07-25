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
	// Duration must reflect attempts*interval (2ms), not a hard-coded "seconds" unit.
	if !strings.Contains(err.Error(), "2ms") {
		t.Fatalf("expected waited duration in error, got: %v", err)
	}
}

func TestRunSendmail_copiesStdinAndRequiresOK(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	gotCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		// Mirror serve.handleConnection: read until EOF, then ack.
		b, err := io.ReadAll(conn)
		if err != nil {
			errCh <- err
			return
		}
		gotCh <- string(b)
		if _, err := conn.Write([]byte("OK")); err != nil {
			errCh <- err
			return
		}
	}()

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
	case err := <-errCh:
		t.Fatalf("server: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server read")
	}
}

func TestRunSendmail_serverRejection(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.ReadAll(conn)
		_, _ = conn.Write([]byte("Error: payload too big"))
	}()

	sendmailSocketPath = sock
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		_, _ = io.WriteString(w, "Subject: x\n\nbody")
		_ = w.Close()
	}()

	err = runSendmail(nil, nil)
	if err == nil {
		t.Fatal("expected error when server rejects message")
	}
	if !strings.Contains(err.Error(), "payload too big") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSendmail_emptyResponse(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.ReadAll(conn)
		// Close without writing an ack (simulates server crash after read).
	}()

	sendmailSocketPath = sock
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	go func() {
		_, _ = io.WriteString(w, "hi")
		_ = w.Close()
	}()

	err = runSendmail(nil, nil)
	if err == nil {
		t.Fatal("expected error on empty server response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("unexpected error: %v", err)
	}
}
