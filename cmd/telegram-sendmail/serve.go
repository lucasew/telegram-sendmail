package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/lucasew/telegram-sendmail/internal/telegram"
	"github.com/lucasew/telegram-sendmail/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// acceptPollInterval is how long Accept waits before yielding so the serve
// loop can drain the queue and exit when idle (systemd socket activation).
const acceptPollInterval = 1 * time.Second

var httpClient = &http.Client{Timeout: 30 * time.Second}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the sendmail server (systemd activated)",
	Run:   runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) {
	token := viper.GetString("telegram_token")
	chat := viper.GetString("telegram_chat")
	stateDir := viper.GetString("state_dir")
	socketTimeout := viper.GetFloat64("socket_timeout")
	maxPayloadSize := viper.GetInt64("max_payload_size")

	if token == "" || chat == "" {
		slog.Error("Telegram token or chat ID not set")
		os.Exit(1)
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		utils.ReportError(err, "Failed to create state directory", "dir", stateDir)
		os.Exit(1)
	}

	listeners, err := activation.Listeners()
	if err != nil {
		utils.ReportError(err, "Failed to get systemd listeners")
		os.Exit(1)
	}

	if len(listeners) == 0 {
		utils.ReportError(nil, "No systemd socket listeners found. This service requires systemd socket activation.")
		os.Exit(1)
	}

	l := listeners[0]
	defer l.Close()

	slog.Info("Service started", "state_dir", stateDir)

	client := telegram.NewClient(token, httpClient)

	for {
		// Short Accept deadline so we can drain the queue and exit when idle.
		if err := setListenerDeadline(l, time.Now().Add(acceptPollInterval)); err != nil {
			utils.ReportError(err, "Failed to set accept deadline")
		}

		conn, err := l.Accept()
		if err != nil {
			var opErr *net.OpError
			if errors.As(err, &opErr) && opErr.Timeout() {
				// Timeout, just proceed to queue processing
			} else {
				utils.ReportError(err, "Accept error")
				// Transient accept failures: back off, then keep serving.
				time.Sleep(acceptPollInterval)
			}
		} else {
			// Handle connection
			handleConnection(conn, stateDir, socketTimeout, maxPayloadSize)
		}

		// Process Queue
		empty, sentCount, errCount := processQueue(client, stateDir, chat)

		if empty {
			// Queue is empty. If we didn't just handle a connection (which we might have), we are idle.
			// But wait, if we just handled a connection, we added to the queue, so processQueue should have seen it.
			// So if processQueue says empty, it means we really have nothing to do.
			slog.Info("Queue is empty, exiting for systemd activation")
			os.Exit(0)
		}

		if errCount > 0 && sentCount == 0 {
			// We failed to send anything, probably network issue.
			// Sleep a bit to avoid busy loop
			slog.Warn("Failed to process queue, retrying in 5 seconds")
			time.Sleep(5 * time.Second)
		}
	}
}

// setListenerDeadline sets a deadline on TCP or Unix listeners used for Accept.
// Other listener types are left unchanged (no deadline API).
func setListenerDeadline(l net.Listener, deadline time.Time) error {
	switch ln := l.(type) {
	case *net.TCPListener:
		return ln.SetDeadline(deadline)
	case *net.UnixListener:
		return ln.SetDeadline(deadline)
	default:
		return nil
	}
}

func handleConnection(conn net.Conn, stateDir string, timeout float64, maxSize int64) {
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Duration(timeout * float64(time.Second)))); err != nil {
		utils.ReportError(err, "Failed to set connection deadline")
		return
	}

	// Read all data
	// We use a limited reader to prevent DoS
	data, err := io.ReadAll(io.LimitReader(conn, maxSize+1))
	if err != nil {
		utils.ReportError(err, "Failed to read from connection")
		return
	}

	if int64(len(data)) > maxSize {
		slog.Warn("Payload too big", "size", len(data))
		if _, err := conn.Write([]byte("Error: payload too big")); err != nil {
			utils.ReportError(err, "Failed to write error response (payload too big)")
		}
		return
	}

	if len(data) == 0 {
		return
	}

	// Save to file
	timestamp := time.Now().UnixNano()
	fname := filepath.Join(stateDir, fmt.Sprintf("%d", timestamp))
	if err := os.WriteFile(fname, data, 0600); err != nil {
		utils.ReportError(err, "Failed to write to queue", "file", fname)
		if _, err := conn.Write([]byte("Error: internal error saving message")); err != nil {
			utils.ReportError(err, "Failed to write error response (save failed)")
		}
		return
	}

	if _, err := conn.Write([]byte("OK")); err != nil {
		utils.ReportError(err, "Failed to write OK response")
	}
}

func processQueue(client *telegram.Client, stateDir, chat string) (empty bool, sentCount int, errCount int) {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		utils.ReportError(err, "Failed to read state directory")
		return false, 0, 1
	}

	if len(entries) == 0 {
		return true, 0, 0
	}

	// Sort by name (timestamp)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fpath := filepath.Join(stateDir, entry.Name())
		content, err := os.ReadFile(fpath)
		if err != nil {
			utils.ReportError(err, "Failed to read message file", "file", fpath)
			if err := os.Remove(fpath); err != nil {
				utils.ReportError(err, "Failed to remove corrupted file", "file", fpath)
			}
			continue
		}

		if err := sendTelegram(client, chat, content); err != nil {
			utils.ReportError(err, "Failed to send message", "file", fpath)
			errCount++
			// Keep the failed item in the queue and continue with the next one.
			// This preserves retry behavior while preventing one bad delivery from blocking later items.
			continue
		}

		// Success
		slog.Info("Message sent", "file", fpath)
		if err := os.Remove(fpath); err != nil {
			utils.ReportError(err, "Failed to remove sent file", "file", fpath)
		}
		sentCount++
	}

	return errCount == 0, sentCount, errCount
}

// parseMailMessage extracts Subject and body from an RFC 822 message using
// net/mail (case-insensitive headers). On parse failure the whole payload is
// treated as the body so sendmail callers are not bricked by malformed input.
func parseMailMessage(data []byte, defaultSubject string) (subject, body string) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return defaultSubject, string(data)
	}

	subject = defaultSubject
	if s := msg.Header.Get("Subject"); s != "" {
		subject = s
	}

	b, err := io.ReadAll(msg.Body)
	if err != nil {
		// bytes.Reader should not fail; fall back so delivery still works.
		return defaultSubject, string(data)
	}
	return subject, string(b)
}

func sendTelegram(client *telegram.Client, chat string, data []byte) error {
	subject, message := parseMailMessage(data, viper.GetString("default_subject"))
	hostname := viper.GetString("hostname")
	return client.Send(chat, subject, message, hostname)
}
