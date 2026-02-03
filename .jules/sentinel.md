# Sentinel Journal

## 2026-02-03 - Fix Missing HTTP Client Timeout

**Vulnerability:** The application uses default HTTP clients without timeouts when communicating with the Telegram API.
**Impact:** A malicious or slow API server could cause the application to hang indefinitely, leading to Denial of Service (DoS) and stopping the mail queue processing.
**Learning:** Default `http.Client` in Go has no timeout. Always define explicit timeouts for external API calls.
**Prevention:** Use `http.Client{Timeout: ...}` or `context.WithTimeout` for all network requests.
