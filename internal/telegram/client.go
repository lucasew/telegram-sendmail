package telegram

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultAPIBaseURL  = "https://api.telegram.org/bot%s"
	messageLengthLimit = 950
	maxCaptionLength   = 1024
	fileSummaryLength  = 512
	// maxErrorBodyBytes caps Telegram error response bodies kept in *Error.
	maxErrorBodyBytes = 4 << 10 // 4 KiB
	// defaultHTTPTimeout is used when NewClient is given a nil *http.Client.
	defaultHTTPTimeout = 30 * time.Second
)

// Error represents an error returned by the Telegram API.
type Error struct {
	StatusCode int
	Message    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Message)
}

// Client is a Telegram Bot API client.
type Client struct {
	token      string
	httpClient *http.Client
	APIBaseURL string
}

// NewClient creates a new Telegram client.
// If httpClient is nil, a client with defaultHTTPTimeout is used so callers
// cannot accidentally issue unbounded Telegram requests (AGENTS.md HTTP rule).
func NewClient(token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{
		token:      token,
		httpClient: httpClient,
		APIBaseURL: defaultAPIBaseURL,
	}
}

func checkResponseError(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	// Bound error body size so a hostile/huge response cannot bloat memory
	// or log lines; one extra byte detects truncation.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes+1))
	if err != nil {
		return &Error{StatusCode: resp.StatusCode, Message: fmt.Sprintf("failed to read body: %v", err)}
	}
	msg := string(body)
	if len(body) > maxErrorBodyBytes {
		msg = string(body[:maxErrorBodyBytes]) + "...(truncated)"
	}
	return &Error{StatusCode: resp.StatusCode, Message: msg}
}

// Send sends a message to the specified chat.
// It tries to send as a text message first.
// If the message is too long or the API returns Bad Request (likely due to formatting),
// it falls back to sending it as a document.
func (c *Client) Send(chatID, subject, body, hostname string) error {
	heading := fmt.Sprintf("<b>#%s</b>: %s", html.EscapeString(hostname), html.EscapeString(subject))

	if len(body) <= messageLengthLimit {
		finalMsg := fmt.Sprintf("%s\n<pre>\n%s\n</pre>", heading, html.EscapeString(body))
		err := c.SendText(chatID, finalMsg)
		if err == nil {
			return nil
		}

		// On Bad Request, fall through to send as document
		var tErr *Error
		if errors.As(err, &tErr) && tErr.StatusCode == http.StatusBadRequest {
			slog.Warn("Failed to send as text (bad request), retrying as document", "error", err)
		} else {
			return err
		}
	}

	return c.SendDocument(chatID, heading, body)
}

func (c *Client) doRequest(req *http.Request) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return checkResponseError(resp)
}

// SendText sends a text message to the specified chat.
func (c *Client) SendText(chatID, text string) error {
	apiURL := fmt.Sprintf(c.APIBaseURL+"/sendMessage", c.token)
	vals := url.Values{}
	vals.Set("chat_id", chatID)
	vals.Set("parse_mode", "HTML")
	vals.Set("disable_web_page_preview", "1")
	vals.Set("text", text)

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(vals.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.doRequest(req)
}

// SendDocument sends a document message to the specified chat.
func (c *Client) SendDocument(chatID, heading, content string) error {
	apiURL := fmt.Sprintf(c.APIBaseURL+"/sendDocument", c.token)

	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	// Add fields
	if err := writer.WriteField("chat_id", chatID); err != nil {
		return err
	}
	if err := writer.WriteField("parse_mode", "HTML"); err != nil {
		return err
	}

	// Caption (byte limits are UTF-8-safe so multi-byte runes are not split)
	summary := truncateUTF8(content, fileSummaryLength)
	caption := fmt.Sprintf(
		"%s\n<code>%s\n\n⚠️ WARNING: Message too big to be sent as a message. The content is in the file.</code>",
		heading,
		html.EscapeString(summary),
	)
	if len(caption) > maxCaptionLength {
		caption = truncateUTF8(caption, maxCaptionLength-3) + "..."
	}
	if err := writer.WriteField("caption", caption); err != nil {
		return err
	}

	// File
	part, err := writer.CreateFormFile("document", "data.txt")
	if err != nil {
		return err
	}
	if _, err := part.Write([]byte(content)); err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bodyBuf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return c.doRequest(req)
}

// truncateUTF8 returns s shortened to at most maxBytes without splitting a
// multi-byte UTF-8 rune. maxBytes < 0 is treated as 0.
func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	// Walk back from the cut so we do not end mid-rune.
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
}
