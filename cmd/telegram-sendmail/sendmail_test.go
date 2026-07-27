package main

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeAndClose writes body to w then closes it (always closes, even on write error).
func writeAndClose(w *os.File, body string) error {
	_, werr := io.WriteString(w, body)
	return errors.Join(werr, w.Close())
}

// startStdinWriter feeds body into the write end of a pipe on a background
// goroutine so runSendmail can read stdin concurrently.
func startStdinWriter(w *os.File, body string, errCh chan<- error) {
	go func() {
		if err := writeAndClose(w, body); err != nil {
			errCh <- err
		}
	}()
}

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
	if !errors.Is(err, ErrSocketUnavailable) {
		t.Fatalf("expected ErrSocketUnavailable, got: %v", err)
	}
	// Duration must reflect attempts*interval (2ms), not a hard-coded "seconds" unit.
	var sue *socketUnavailableError
	if !errors.As(err, &sue) {
		t.Fatalf("expected *socketUnavailableError, got: %T %v", err, err)
	}
	if sue.Waited != 2*time.Millisecond {
		t.Fatalf("Waited=%v want 2ms", sue.Waited)
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
	if !errors.Is(err, ErrNotUnixSocket) {
		t.Fatalf("expected wrapped ErrNotUnixSocket, got: %v", err)
	}
	if !errors.Is(err, ErrSocketUnavailable) {
		t.Fatalf("expected ErrSocketUnavailable wrapper, got: %v", err)
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
	startStdinWriter(w, msg, errCh)

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

	startStdinWriter(w, "Subject: x\n\nbody", errCh)

	err = runSendmail(nil, nil)
	if err == nil {
		t.Fatal("expected error when server rejects message")
	}
	if !errors.Is(err, ErrServerRejected) {
		t.Fatalf("expected wrapped ErrServerRejected, got: %v", err)
	}
	var rej *serverRejectedError
	if !errors.As(err, &rej) {
		t.Fatalf("expected *serverRejectedError, got: %T %v", err, err)
	}
	// Mock server writes this exact status line; client stores TrimSpace(resp).
	if rej.detail != "Error: payload too big" {
		t.Fatalf("detail=%q want %q", rej.detail, "Error: payload too big")
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

	startStdinWriter(w, "hi", errCh)

	err = runSendmail(nil, nil)
	if err == nil {
		t.Fatal("expected error on empty server response")
	}
	if !errors.Is(err, ErrServerRejected) {
		t.Fatalf("expected ErrServerRejected, got: %v", err)
	}
	var rej *serverRejectedError
	if !errors.As(err, &rej) {
		t.Fatalf("expected *serverRejectedError, got: %T %v", err, err)
	}
	if rej.detail != "empty response" {
		t.Fatalf("detail=%q want %q", rej.detail, "empty response")
	}
}
