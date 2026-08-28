## 2026-08-28

### Fixed
- `duplo-jit aws --admin`/`--duplo-ops` now honor `--tenant`: the returned `Region` and console URL open in the selected tenant's region instead of the master account's default region (DUPLO-43460).

### Changed
- `duplo-jit aws --admin`/`--duplo-ops` with `--tenant` resolves the tenant before reading the credential cache (the tenant name is part of the cache key), so a cache hit can still prompt for Duplo login when the cached Duplo token has expired, as the `--tenant`-only path already does. The tenant's region comes from `v3/features/tenant/{id}`; if that lookup fails, or the tenant has no region configured, or the console URL's region cannot be rewritten, the master default region is kept and a warning is written to stderr. Credentials produced after a failed lookup are not cached, so the next invocation retries it.
- `make test` now runs `go test ./...`.

## 2026-02-24

### Added
- Opt-in auth cooldown to prevent browser tab spam when multiple processes request interactive credentials simultaneously. Set `DUPLO_JIT_AUTH_COOLDOWN=true` (or a duration like `30m`) to enable. Thanks to @scholzie for the original contribution in #52.

### Changed
- Upgraded all direct and indirect Go module dependencies.

## 2024-02-14

### Added
- Introduced a new section in the README for Homebrew installation, enhancing the accessibility of the tool for macOS users.

## 2024-01-24

### Fixed
- Improved error message format when a tenant is missing or not allowed.
- Prevented appending a nil error object to fatal error messages.