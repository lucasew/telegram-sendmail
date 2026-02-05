# Arrumador Journal

## 2025-02-23 - Tooling Configuration

Established standard tooling foundation using `mise`.
- Configured `mise.toml` with `go` and `golangci-lint`.
- Defined standard tasks: `lint`, `fmt`, `test`, `codegen`, `install`, `ci`.
- Configured `lint` to use `golangci-lint` (checking `errcheck`, `unused`, etc.).
- Configured GitHub Actions workflow `autorelease.yml` to follow the standard flow: Install -> Codegen -> PR -> CI -> Release.
- Fixed lint errors in `root.go` and `serve.go` (unchecked errors, unused vars).
- Verified CI locally with `mise run ci`.

**Fix:** Downgraded Go to 1.23.4 and golangci-lint to 1.61.0 to resolve CI build failures likely caused by bleeding-edge version incompatibility or mirror issues.
