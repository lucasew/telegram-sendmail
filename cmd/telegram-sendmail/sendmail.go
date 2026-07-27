package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	// defaultSendmailSocket is the systemd socket path (packaging + NixOS).
	defaultSendmailSocket = "/run/telegram-sendmail/socket.sock"
	// sendmailWaitAttempts matches the historical Nix nc wrapper (30s).
	sendmailWaitAttempts = 30
	// sendmailWaitInterval is the sleep between socket existence checks.
	sendmailWaitInterval = 1 * time.Second
	// sendmailIOTimeout bounds dialed-connection write+ack so cron callers
	// cannot hang forever if the service stalls after Accept.
	sendmailIOTimeout = 30 * time.Second
)

var sendmailSocketPath string

var sendmailCmd = &cobra.Command{
	Use:   "sendmail",
	Short: "sendmail client: pipe stdin to the local telegram-sendmail socket",
	Long: `Drop-in sendmail client. Classic sendmail flags are accepted and ignored;
the message is read from stdin and written to the Unix socket served by
"telegram-sendmail serve" (systemd socket activation). Exit 0 only after the
daemon acks that the message was queued; Telegram delivery is asynchronous.`,
	// Silence usage on dial/copy errors — cron/mail callers treat this as sendmail.
	SilenceUsage: true,
	RunE:         runSendmail,
}

func init() {
	sendmailCmd.Flags().StringVar(&sendmailSocketPath, "socket", defaultSendmailSocket, "Unix socket path for the telegram-sendmail service")
	// Accept and ignore unknown flags so invocations like `sendmail -t -i` work.
	sendmailCmd.FParseErrWhitelist = cobra.FParseErrWhitelist{UnknownFlags: true}
	rootCmd.AddCommand(sendmailCmd)
}

func runSendmail(cmd *cobra.Command, args []string) error {
	if err := waitForSocket(sendmailSocketPath, sendmailWaitAttempts, sendmailWaitInterval); err != nil {
		return err
	}

	conn, err := net.Dial("unix", sendmailSocketPath)
	if err != nil {
		return fmt.Errorf("dial %s: %w", sendmailSocketPath, err)
	}
	defer conn.Close()

	// Bound the whole exchange (write body + read queue ack).
	if err := conn.SetDeadline(time.Now().Add(sendmailIOTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if _, err := io.Copy(conn, os.Stdin); err != nil {
		return fmt.Errorf("copy stdin to socket: %w", err)
	}

	// serve.handleConnection reads with ReadAll until EOF, then writes "OK"
	// after a successful on-disk queue write (or an error line). Half-close
	// the write side so the server finishes the read without the client
	// dropping the reply via a full Close.
	if err := closeWrite(conn); err != nil {
		return fmt.Errorf("close write half: %w", err)
	}

	resp, err := io.ReadAll(conn)
	if err != nil {
		return fmt.Errorf("read server response: %w", err)
	}
	// "OK" = message reached the daemon and was queued (not Telegram delivery).
	if string(resp) != "OK" {
		msg := strings.TrimSpace(string(resp))
		if msg == "" {
			msg = "empty response"
		}
		return &serverRejectedError{detail: msg}
	}
	return nil
}

// closeWrite shuts down the write half of a duplex connection (Unix/TCP).
func closeWrite(conn net.Conn) error {
	type closeWriter interface {
		CloseWrite() error
	}
	cw, ok := conn.(closeWriter)
	if !ok {
		return ErrCloseWriteUnsupported
	}
	return cw.CloseWrite()
}

// sendmailError is a comparable sentinel so errors.Is works through %w.
type sendmailError string

func (e sendmailError) Error() string { return string(e) }

// Sentinel errors for the sendmail client (errors.Is / %w).
const (
	// ErrNotUnixSocket: path exists but is not a Unix socket (stale regular file).
	ErrNotUnixSocket sendmailError = "path is not a unix socket"
	// ErrCloseWriteUnsupported: conn does not implement CloseWrite.
	ErrCloseWriteUnsupported sendmailError = "connection does not support CloseWrite"
	// ErrInvalidSocketWaitAttempts: waitForSocket called with attempts < 1.
	ErrInvalidSocketWaitAttempts sendmailError = "socket wait attempts must be at least 1"
	// ErrServerRejected: serve replied with a non-OK status line.
	ErrServerRejected sendmailError = "server rejected message"
	// ErrSocketUnavailable: waitForSocket exhausted its attempt budget.
	ErrSocketUnavailable sendmailError = "socket not available"
)

// socketUnavailableError is returned when waitForSocket exhausts its attempts.
// Waited is the total budget (attempts * interval) for callers/tests that need
// the duration without parsing Error().
type socketUnavailableError struct {
	Waited time.Duration
	err    error
}

func (e *socketUnavailableError) Error() string {
	return fmt.Sprintf("%s after %s: %v", ErrSocketUnavailable, e.Waited, e.err)
}

func (e *socketUnavailableError) Unwrap() error { return e.err }

func (e *socketUnavailableError) Is(target error) bool {
	return target == ErrSocketUnavailable
}

// serverRejectedError carries the server's non-OK status line detail while
// remaining matchable via errors.Is(..., ErrServerRejected).
type serverRejectedError struct {
	detail string
}

func (e *serverRejectedError) Error() string {
	return fmt.Sprintf("%s: %s", ErrServerRejected, e.detail)
}

func (e *serverRejectedError) Unwrap() error { return ErrServerRejected }

func waitForSocket(path string, attempts int, interval time.Duration) error {
	if attempts < 1 {
		return ErrInvalidSocketWaitAttempts
	}
	var lastErr error
	for i := 1; i <= attempts; i++ {
		fi, err := os.Stat(path)
		if err == nil {
			if fi.Mode()&os.ModeSocket != 0 {
				return nil
			}
			// Path exists but is not a socket (e.g. leftover regular file).
			lastErr = fmt.Errorf("%s: %w", path, ErrNotUnixSocket)
		} else {
			lastErr = err
		}
		if i == attempts {
			break
		}
		fmt.Fprintf(os.Stderr, "Waiting for the sendmail socket to be available... (attempt %d/%d)\n", i, attempts)
		time.Sleep(interval)
	}
	// lastErr is always set after a completed attempt loop (attempts >= 1).
	return &socketUnavailableError{
		Waited: time.Duration(attempts) * interval,
		err:    lastErr,
	}
}
