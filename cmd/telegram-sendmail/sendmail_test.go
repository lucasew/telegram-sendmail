package main

import (
	"errors"
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
	// Underlying Stat failure must be wrapped so ops can see ENOENT vs other causes.
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected wrapped os.ErrNotExist, got: %v", err)
	}
}

func TestWaitForSocket_notASocket(t *testing.T) {
	dir := t.TempDir()
	// Regular file at the socket path (stale leftover).
	path := filepath.Join(dir, "not-a-socket")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := waitForSocket(path, 1, time.Millisecond)
	if err == nil {
		t.Fatal("expected error when path is not a socket")
	}
	if !strings.Contains(err.Error(), "not a unix socket") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(err, ErrNotUnixSocket) {
		t.Fatalf("expected wrapped ErrNotUnixSocket, got: %v", err)
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
		if _, err := io.WriteString(w, msg); err != nil {
			errCh <- err
		}
		if err := w.Close(); err != nil {
			errCh <- err
		}
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

	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		if _, err := io.ReadAll(conn); err != nil {
			errCh <- err
			return
		}
		if _, err := conn.Write([]byte("Error: payload too big")); err != nil {
			errCh <- err
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

	go func() {
		if _, err := io.WriteString(w, "Subject: x\n\nbody"); err != nil {
			errCh <- err
		}
		if err := w.Close(); err != nil {
			errCh <- err
		}
	}()

	err = runSendmail(nil, nil)
	if err == nil {
		t.Fatal("expected error when server rejects message")
	}
	if !strings.Contains(err.Error(), "payload too big") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(err, ErrServerRejected) {
		t.Fatalf("expected wrapped ErrServerRejected, got: %v", err)
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

	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()
		if _, err := io.ReadAll(conn); err != nil {
			errCh <- err
		}
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
		if _, err := io.WriteString(w, "hi"); err != nil {
			errCh <- err
		}
		if err := w.Close(); err != nil {
			errCh <- err
		}
	}()

	err = runSendmail(nil, nil)
	if err == nil {
		t.Fatal("expected error on empty server response")
	}
	if !strings.Contains(err.Error(), "empty response") {
		t.Fatalf("unexpected error: %v", err)
	}
}
