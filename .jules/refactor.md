## 2024-05-22 - Encapsulate Service Logic
**Issue:** The `service` script was a monolithic procedural script with global variables (`args`, `server`, `logger`), making it hard to test and maintain.
**Root Cause:** Initial implementation prioritized simplicity over structure ("Script Pattern").
**Solution:** Applied the **Extract Class** pattern (Fowler) to create `TelegramMailService`. Encapsulated configuration and behavior. Added `main()` and `if __name__ == "__main__":` entry point.
**Principle:** **Single Responsibility Principle (SRP)** - The script now separates the concern of *running* the service (in `main`) from the *logic* of the service (in `TelegramMailService`). **Encapsulation** - Data and methods are bundled.
