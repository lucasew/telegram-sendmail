package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var telegramAPIBase = "https://api.telegram.org/bot%s"

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
		slog.Error("Failed to create state directory", "dir", stateDir, "error", err)
		os.Exit(1)
	}

	listeners, err := activation.Listeners()
	if err != nil {
		slog.Error("Failed to get systemd listeners", "error", err)
		os.Exit(1)
	}

	if len(listeners) == 0 {
		slog.Error("No systemd socket listeners found. This service requires systemd socket activation.")
		os.Exit(1)
	}

	l := listeners[0]
	defer l.Close()

	slog.Info("Service started", "state_dir", stateDir)

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
				slog.Error("Accept error", "error", err)
				// If permanent error, maybe exit? But let's try to continue.
				time.Sleep(1 * time.Second)
			}
		} else {
			// Handle connection
			handleConnection(conn, stateDir, socketTimeout, maxPayloadSize)
		}

		// Process Queue
		empty, sentCount, errCount := processQueue(stateDir, token, chat)

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
		slog.Error("Failed to read from connection", "error", err)
		return
	}

	if int64(len(data)) > maxSize {
		slog.Warn("Payload too big", "size", len(data))
		conn.Write([]byte("Error: payload too big"))
		return
	}

	if len(data) == 0 {
		return
	}

	// Save to file
	timestamp := time.Now().UnixNano()
	fname := filepath.Join(stateDir, fmt.Sprintf("%d", timestamp))
	if err := os.WriteFile(fname, data, 0600); err != nil {
		slog.Error("Failed to write to queue", "file", fname, "error", err)
		conn.Write([]byte("Error: internal error saving message"))
		return
	}

	conn.Write([]byte("OK"))
}

func processQueue(stateDir, token, chat string) (empty bool, sentCount int, errCount int) {
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		slog.Error("Failed to read state directory", "error", err)
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
			slog.Error("Failed to read message file", "file", fpath, "error", err)
			os.Remove(fpath) // Corrupted?
			continue
		}

		if err := sendTelegram(token, chat, content); err != nil {
			slog.Error("Failed to send message", "file", fpath, "error", err)
			errCount++
			// If we fail to send one, we stop processing the rest to preserve order/retry logic
			// return false (not empty), sentCount, errCount
			return false, sentCount, errCount
		}

		// Success
		slog.Info("Message sent", "file", fpath)
		os.Remove(fpath)
		sentCount++
	}

	return true, sentCount, errCount
}

type telegramError struct {
	StatusCode int
	Message    string
}

func (e *telegramError) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Message)
}

func sendTelegram(token, chat string, data []byte) error {
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

	// Prepare content
	// Limits
	const MessageLengthLimit = 950 // Before file
	// const FileSummaryLength = 512
	// const MaxCaptionLength = 1024

	heading := fmt.Sprintf("<b>#%s</b>: %s", hostname, htmlEscape(subject))

	if len(message) <= MessageLengthLimit {
		// Send as text
		finalMsg := fmt.Sprintf("%s\n<pre>\n%s\n</pre>", heading, htmlEscape(message))
		err := sendTextMessage(token, chat, finalMsg)
		if err == nil {
			return nil
		}
		// If 400, fall through to send as document
		if tErr, ok := err.(*telegramError); ok && tErr.StatusCode == 400 {
			slog.Warn("Failed to send as text (400), retrying as document", "error", err)
		} else {
			return err
		}
	}

	// Send as document
	return sendDocumentMessage(token, chat, heading, message)
}

func sendTextMessage(token, chat, text string) error {
	apiURL := fmt.Sprintf(telegramAPIBase+"/sendMessage", token)
	vals := url.Values{}
	vals.Set("chat_id", chat)
	vals.Set("parse_mode", "HTML")
	vals.Set("disable_web_page_preview", "1")
	vals.Set("text", text)

	client := &http.Client{Timeout: httpClientTimeout}
	resp, err := client.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(vals.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &telegramError{StatusCode: resp.StatusCode, Message: string(body)}
	}
	return nil
}

func sendDocumentMessage(token, chat, heading, content string) error {
	apiURL := fmt.Sprintf(telegramAPIBase+"/sendDocument", token)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add fields
	writer.WriteField("chat_id", chat)
	writer.WriteField("parse_mode", "HTML")

	// Caption
	summary := content
	if len(summary) > 512 {
		summary = summary[:512]
	}
	caption := fmt.Sprintf("%s\n<code>%s\n\n⚠️ WARNING: Message too big to be sent as a message. The content is in the file.</code>", heading, htmlEscape(summary))
	if len(caption) > 1024 {
		caption = caption[:1020] + "..."
	}
	writer.WriteField("caption", caption)

	// File
	part, err := writer.CreateFormFile("document", "data.txt")
	if err != nil {
		return err
	}
	part.Write([]byte(content))

	writer.Close()

	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: httpClientTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram api error: %s - %s", resp.Status, string(bodyBytes))
	}
	return nil
}

func htmlEscape(s string) string {
	return html.EscapeString(s)
}
