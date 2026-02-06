package telegram

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultAPIBaseURL  = "https://api.telegram.org/bot%s"
	messageLengthLimit = 950
	maxCaptionLength   = 1024
	fileSummaryLength  = 512
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
func NewClient(token string, httpClient *http.Client) *Client {
	return &Client{
		token:      token,
		httpClient: httpClient,
		APIBaseURL: defaultAPIBaseURL,
	}
}

// Send sends a message to the specified chat.
// It tries to send as a text message first.
// If the message is too long or the API returns a 400 Bad Request (likely due to formatting),
// it falls back to sending it as a document.
func (c *Client) Send(chatID, subject, body, hostname string) error {
	heading := fmt.Sprintf("<b>#%s</b>: %s", hostname, html.EscapeString(subject))

	if len(body) <= messageLengthLimit {
		finalMsg := fmt.Sprintf("%s\n<pre>\n%s\n</pre>", heading, html.EscapeString(body))
		err := c.SendText(chatID, finalMsg)
		if err == nil {
			return nil
		}

		// If 400, fall through to send as document
		if tErr, ok := err.(*Error); ok && tErr.StatusCode == 400 {
			slog.Warn("Failed to send as text (400), retrying as document", "error", err)
		} else {
			return err
		}
	}

	return c.SendDocument(chatID, heading, body)
}

// SendText sends a text message to the specified chat.
func (c *Client) SendText(chatID, text string) error {
	apiURL := fmt.Sprintf(c.APIBaseURL+"/sendMessage", c.token)
	vals := url.Values{}
	vals.Set("chat_id", chatID)
	vals.Set("parse_mode", "HTML")
	vals.Set("disable_web_page_preview", "1")
	vals.Set("text", text)

	resp, err := c.httpClient.Post(apiURL, "application/x-www-form-urlencoded", strings.NewReader(vals.Encode()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &Error{StatusCode: resp.StatusCode, Message: string(body)}
	}
	return nil
}

// SendDocument sends a document message to the specified chat.
func (c *Client) SendDocument(chatID, heading, content string) error {
	apiURL := fmt.Sprintf(c.APIBaseURL+"/sendDocument", c.token)

	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	// Add fields
	writer.WriteField("chat_id", chatID)
	writer.WriteField("parse_mode", "HTML")

	// Caption
	summary := content
	if len(summary) > fileSummaryLength {
		summary = summary[:fileSummaryLength]
	}
	caption := fmt.Sprintf("%s\n<code>%s\n\n⚠️ WARNING: Message too big to be sent as a message. The content is in the file.</code>", heading, html.EscapeString(summary))
	if len(caption) > maxCaptionLength {
		caption = caption[:maxCaptionLength-4] + "..."
	}
	writer.WriteField("caption", caption)

	// File
	part, err := writer.CreateFormFile("document", "data.txt")
	if err != nil {
		return err
	}
	part.Write([]byte(content))

	writer.Close()

	req, err := http.NewRequest("POST", apiURL, bodyBuf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
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
