## 2024-07-22 - Replace Magic Numbers with Named Constants
**Issue:** The `service` script contained several hardcoded "magic numbers" related to Telegram API limits.
**Root Cause:** These values were likely introduced directly during initial development for simplicity, without considering future readability or maintainability.
**Solution:** I introduced named constants (`MESSAGE_LENGTH_LIMIT_BEFORE_FILE`, `FILE_SUMMARY_LENGTH`, `MAX_CAPTION_LENGTH`, `TRUNCATED_CAPTION_LENGTH`) to replace the magic numbers.
**Pattern:** When encountering hardcoded, unexplained numbers in the code, replace them with named constants to improve clarity and ease of future updates. This is especially important when the numbers represent external constraints, such as API limits.
