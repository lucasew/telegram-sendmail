## 2024-07-22 - Replace Magic Numbers with Named Constants
**Issue:** The `service` script contained several hardcoded "magic numbers" related to Telegram API limits.
**Root Cause:** These values were likely introduced directly during initial development for simplicity, without considering future readability or maintainability.
**Solution:** I introduced named constants (`MESSAGE_LENGTH_LIMIT_BEFORE_FILE`, `FILE_SUMMARY_LENGTH`, `MAX_CAPTION_LENGTH`, `TRUNCATED_CAPTION_LENGTH`) to replace the magic numbers.
**Pattern:** When encountering hardcoded, unexplained numbers in the code, replace them with named constants to improve clarity and ease of future updates. This is especially important when the numbers represent external constraints, such as API limits.

## 2026-01-14 - Extract Queue Processing Logic and Reduce Duplication
**Issue:** The `service` script duplicated the queue processing logic in two places (`try` block and `except socket.timeout` block), including a loop with a sleep call and a magic number for retries.
**Root Cause:** The logic for processing the queue when idle or after a successful send was likely copy-pasted.
**Solution:** I extracted the queue processing logic into a `process_queue(max_retries: int)` function and replaced the magic numbers `4096` and `0.5` with `SOCKET_BUFFER_SIZE` and `QUEUE_PROCESS_DELAY`.
**Pattern:** Extract repeated logic blocks, especially loops with side effects (like `sleep`), into named functions. This ensures consistency (e.g., if we change the delay, we change it everywhere) and improves readability.
