## 2024-07-22 - Replace Magic Numbers with Named Constants
**Issue:** The `service` script contained several hardcoded "magic numbers" related to Telegram API limits.
**Root Cause:** These values were likely introduced directly during initial development for simplicity, without considering future readability or maintainability.
**Solution:** I introduced named constants (`MESSAGE_LENGTH_LIMIT_BEFORE_FILE`, `FILE_SUMMARY_LENGTH`, `MAX_CAPTION_LENGTH`, `TRUNCATED_CAPTION_LENGTH`) to replace the magic numbers.
**Pattern:** When encountering hardcoded, unexplained numbers in the code, replace them with named constants to improve clarity and ease of future updates. This is especially important when the numbers represent external constraints, such as API limits.

## 2024-10-24 - Fix Header Parsing Regex
**Issue:** The header parsing regex was too restrictive, failing to match headers with hyphens (e.g., `Content-Type`) and incorrectly truncating values containing `$`. This caused headers like `Content-Type` to leak into the message body and potential data loss in subjects.
**Root Cause:** The regex `([a-zA-Z]*):([^$]*)` incorrectly assumed header keys only contained letters and values did not contain `$`.
**Solution:** Updated regex to `([^:]+):(.*)` to correctly match keys (stopping at first colon) and capture the full value. Also updated `match.groups` usage to the standard `match.group(n)`.
**Pattern:** When using regex for parsing, ensure character classes cover all valid inputs (like hyphens in headers) and avoid unintended exclusions (like `$` in `[^$]*`). Validate regex against edge cases and use standard group access methods.
