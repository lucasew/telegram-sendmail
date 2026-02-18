package main

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/lucasew/telegram-sendmail/internal/telegram"
	"github.com/lucasew/telegram-sendmail/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

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
		// Check for incoming connections with a short timeout
		// This allows us to check the queue and exit if idle
		if tcpL, ok := l.(*net.TCPListener); ok {
			tcpL.SetDeadline(time.Now().Add(1 * time.Second))
		} else if unixL, ok := l.(*net.UnixListener); ok {
			unixL.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := l.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				// Timeout, just proceed to queue processing
			} else {
				utils.ReportError(err, "Accept error")
				// If permanent error, maybe exit? But let's try to continue.
				time.Sleep(1 * time.Second)
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

func handleConnection(conn net.Conn, stateDir string, timeout float64, maxSize int64) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(time.Duration(timeout * float64(time.Second))))

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
			// If we fail to send one, we stop processing the rest to preserve order/retry logic
			// return false (not empty), sentCount, errCount
			return false, sentCount, errCount
		}

		// Success
		slog.Info("Message sent", "file", fpath)
		if err := os.Remove(fpath); err != nil {
			utils.ReportError(err, "Failed to remove sent file", "file", fpath)
		}
		sentCount++
	}

	return true, sentCount, errCount
}

func sendTelegram(client *telegram.Client, chat string, data []byte) error {
	// Parse subject
	lines := strings.Split(string(data), "\n")
	subject := viper.GetString("default_subject")
	var bodyLines []string
	isHeader := true

	for _, line := range lines {
		if isHeader {
			if strings.TrimSpace(line) == "" {
				isHeader = false
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				if key == "Subject" {
					subject = val
				}
			} else {
				// Not a header line?
				isHeader = false
				bodyLines = append(bodyLines, line)
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}
	message := strings.Join(bodyLines, "\n")
	hostname := viper.GetString("hostname")

	return client.Send(chat, subject, message, hostname)
}
