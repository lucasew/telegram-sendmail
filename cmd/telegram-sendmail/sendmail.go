package main

import (
	"errors"
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
"telegram-sendmail serve" (systemd socket activation).`,
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

	// Bound the whole exchange (write body + read ack).
	if err := conn.SetDeadline(time.Now().Add(sendmailIOTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if _, err := io.Copy(conn, os.Stdin); err != nil {
		return fmt.Errorf("copy stdin to socket: %w", err)
	}

	// serve.handleConnection reads with ReadAll until EOF, then writes "OK"
	// or an error line. Half-close the write side so the server finishes the
	// read without the client dropping the reply via a full Close.
	if err := closeWrite(conn); err != nil {
		return fmt.Errorf("close write half: %w", err)
	}

	resp, err := io.ReadAll(conn)
	if err != nil {
		return fmt.Errorf("read server response: %w", err)
	}
	if string(resp) != "OK" {
		msg := strings.TrimSpace(string(resp))
		if msg == "" {
			msg = "empty response"
		}
		return fmt.Errorf("server rejected message: %s", msg)
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
		return errors.New("connection does not support CloseWrite")
	}
	return cw.CloseWrite()
}

func waitForSocket(path string, attempts int, interval time.Duration) error {
	for i := 1; i <= attempts; i++ {
		fi, err := os.Stat(path)
		if err == nil && fi.Mode()&os.ModeSocket != 0 {
			return nil
		}
		if i == attempts {
			break
		}
		fmt.Fprintf(os.Stderr, "Waiting for the sendmail socket to be available... (attempt %d/%d)\n", i, attempts)
		time.Sleep(interval)
	}
	waited := time.Duration(attempts) * interval
	return fmt.Errorf("socket not available after %s: %s", waited, path)
}
