# Refactoring Log

## 2026-01-30: Decoupling and Complexity Reduction in `service`

### Summary
Refactored the monolithic `service` script to improve testability and maintainability.

### Changes
1.  **Architecture**: Wrapped global execution logic in a `main()` function and guarded it with `if __name__ == "__main__":`. This allows the module to be imported for testing without side effects.
2.  **Dependency Injection**: Updated `handle_send`, `send_one_from_queue`, and `send_payload` to accept `args` as an argument, removing reliance on global state.
3.  **Complexity Reduction**: Extracted `parse_email_data`, `send_text_message`, and `send_file_message` from `send_payload`. This adheres to the Single Responsibility Principle (SRP) and reduces the cyclomatic complexity of `send_payload`.
4.  **Testing**: Added regression tests in `tests/test_service.py` covering message parsing and sending logic (mocking `urlopen`).

### Justification
-   **SRP**: Separating parsing and network I/O makes the code easier to understand and test in isolation.
-   **Testability**: Importing the module without running the server loop is crucial for unit testing.
-   **Maintainability**: Named functions for specific tasks (send text vs file) improve readability.
