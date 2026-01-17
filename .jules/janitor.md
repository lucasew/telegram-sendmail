## 2024-07-22 - Replace Magic Numbers with Named Constants
**Issue:** The `service` script contained several hardcoded "magic numbers" related to Telegram API limits.
**Root Cause:** These values were likely introduced directly during initial development for simplicity, without considering future readability or maintainability.
**Solution:** I introduced named constants (`MESSAGE_LENGTH_LIMIT_BEFORE_FILE`, `FILE_SUMMARY_LENGTH`, `MAX_CAPTION_LENGTH`, `TRUNCATED_CAPTION_LENGTH`) to replace the magic numbers.
**Pattern:** When encountering hardcoded, unexplained numbers in the code, replace them with named constants to improve clarity and ease of future updates. This is especially important when the numbers represent external constraints, such as API limits.

## 2026-01-17 - Move Local Imports to Module Level
**Issue:** The `service` script contained imports (`from os import urandom`, `import binascii`) nested inside the `encode_multipart_formdata` function, as well as an unused `Optional` import.
**Root Cause:** Nested imports are often used to avoid circular dependencies or for optional dependencies, but here they were likely just placed near their usage without need, causing potential performance overhead and reducing clarity.
**Solution:** I moved the imports to the top of the file and removed the unused `Optional` import.
**Pattern:** Keep imports at the top of the file unless there is a specific reason (like avoiding circular imports) to place them locally. This improves readability and follows standard Python conventions (PEP 8).
